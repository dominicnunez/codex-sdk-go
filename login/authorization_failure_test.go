package login

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/login/auth"
)

func TestLoginProviderAuthorizationFailure(t *testing.T) {
	for _, kind := range []string{"access_denied", "server_error", "wrong-state-then-success", "success"} {
		t.Run(kind, func(t *testing.T) {
			reservation, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			port := reservation.Addr().(*net.TCPAddr).Port
			if err = reservation.Close(); err != nil {
				t.Fatal(err)
			}
			callbackURL := "http://127.0.0.1:" + strconv.Itoa(port) + callbackPath
			callbackTransport := &http.Transport{Proxy: nil}
			defer callbackTransport.CloseIdleConnections()
			httpClient := &http.Client{Transport: callbackTransport, Timeout: time.Second}
			var requests atomic.Int32
			payload, _ := json.Marshal(map[string]any{"access_token": fakeAccessToken(t, "synthetic-account", "plus"), "refresh_token": "synthetic-refresh", "expires_in": 3600})
			received := make(chan struct{})
			manualStopped := make(chan struct{})
			cfg := Config{CallbackHost: "127.0.0.1", CallbackPort: port, RedirectURI: callbackURL, TokenEndpoint: "https://token.invalid", HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(payload))), Header: make(http.Header)}, nil
			})}}
			opts := LoginOptions{Config: cfg, ManualCode: func(ctx context.Context, _ AuthPrompt) (string, error) {
				defer close(manualStopped)
				<-ctx.Done()
				return "", ctx.Err()
			}, OnAuthURL: func(ctx context.Context, raw string) error {
				authorize, e := url.Parse(raw)
				if e != nil {
					return e
				}
				state := authorize.Query().Get("state")
				send := func(q url.Values, want int) error {
					request, e := http.NewRequestWithContext(ctx, "GET", callbackURL+"?"+q.Encode(), nil)
					if e != nil {
						return e
					}
					response, e := httpClient.Do(request)
					if e != nil {
						return e
					}
					defer response.Body.Close()
					_, _ = io.Copy(io.Discard, response.Body)
					if response.StatusCode != want {
						return fmt.Errorf("callback status=%d want=%d", response.StatusCode, want)
					}
					return nil
				}
				switch kind {
				case "access_denied", "server_error":
					if e = send(url.Values{"error": {kind}, "state": {state}}, http.StatusBadRequest); e != nil {
						return e
					}
				case "wrong-state-then-success":
					if e = send(url.Values{"error": {"access_denied"}, "state": {"wrong-state"}}, http.StatusBadRequest); e != nil {
						return e
					}
					fallthrough
				case "success":
					if e = send(url.Values{"code": {"synthetic-code"}, "state": {state}}, http.StatusOK); e != nil {
						return e
					}
				}
				close(received)
				return nil
			}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type outcome struct {
				creds auth.Credentials
				err   error
			}
			done := make(chan outcome, 1)
			go func() { creds, e := Login(ctx, opts); done <- outcome{creds, e} }()
			select {
			case <-received:
			case <-time.After(3 * time.Second):
				t.Fatal("callback never processed")
			}
			denial := kind == "access_denied" || kind == "server_error"
			select {
			case got := <-done:
				if denial {
					if !errors.Is(got.err, ErrAuthorizationFailed) || !reflect.DeepEqual(got.creds, auth.Credentials{}) || requests.Load() != 0 {
						t.Fatalf("denial published credentials or exchanged token: err=%v calls=%d", got.err, requests.Load())
					}
				} else if got.err != nil || got.creds.Validate() != nil || requests.Load() != 1 {
					t.Fatalf("success control failed: err=%v calls=%d", got.err, requests.Load())
				}
				if ctx.Err() != nil {
					t.Fatal("flow outcome canceled caller context")
				}
				select {
				case <-manualStopped:
				case <-time.After(time.Second):
					t.Fatal("callback outcome did not cancel competing manual input")
				}
				assertAuthorizationListenerClosed(t, port)
			case <-time.After(500 * time.Millisecond):
				t.Errorf("login still waits after processed %s callback; token calls=%d", kind, requests.Load())
				cancel()
				select {
				case got := <-done:
					if !reflect.DeepEqual(got.creds, auth.Credentials{}) {
						t.Fatal("cancellation published credentials")
					}
				case <-time.After(3 * time.Second):
					t.Fatal("cancellation failed cleanup")
				}
			}
		})
	}
}
func TestManualProviderAuthorizationFailure(t *testing.T) {
	for _, kind := range []string{"query-denial", "query-server-error", "url-denial", "bare-code"} {
		t.Run(kind, func(t *testing.T) {
			var requests atomic.Int32
			payload, _ := json.Marshal(map[string]any{"access_token": fakeAccessToken(t, "synthetic-account", "plus"), "refresh_token": "synthetic-refresh", "expires_in": 3600})
			cfg := Config{CallbackHost: "127.0.0.1", CallbackPort: -1, HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(payload))), Header: make(http.Header)}, nil
			})}}
			creds, err := Login(context.Background(), LoginOptions{Config: cfg, ManualCode: func(_ context.Context, prompt AuthPrompt) (string, error) {
				authorize, e := url.Parse(prompt.AuthURL)
				if e != nil {
					return "", e
				}
				state := authorize.Query().Get("state")
				if kind == "bare-code" {
					return "synthetic-code", nil
				}
				providerError := "access_denied"
				if kind == "query-server-error" {
					providerError = "server_error"
				}
				query := url.Values{"error": {providerError}, "state": {state}}.Encode()
				if kind == "url-denial" {
					return "http://localhost/auth/callback?" + query, nil
				}
				return query, nil
			}})
			if kind == "bare-code" {
				if err != nil || creds.Validate() != nil || requests.Load() != 1 {
					t.Fatalf("bare control failed err=%v calls=%d", err, requests.Load())
				}
				return
			}
			if !errors.Is(err, ErrAuthorizationFailed) || requests.Load() != 0 || !reflect.DeepEqual(creds, auth.Credentials{}) {
				t.Errorf("provider error treated as code: err=%v calls=%d credentials=%v", err, requests.Load(), !reflect.DeepEqual(creds, auth.Credentials{}))
			}
		})
	}
}
