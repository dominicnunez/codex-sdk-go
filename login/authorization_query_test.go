package login

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dominicnunez/codex-sdk-go/login/auth"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestAuthorizationInputRejectsMalformedStateQuery(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"url-percent", "http://localhost/auth/callback?code=synthetic-code&state=%ZZ"},
		{"url-semicolon", "http://localhost/auth/callback?code=synthetic-code&state=wrong;discarded"},
		{"query-percent", "code=synthetic-code&state=%ZZ"},
		{"query-semicolon", "code=synthetic-code&state=wrong;discarded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseAuthorizationInput(tc.input, "expected-state")
			if err == nil || !reflect.DeepEqual(got, AuthorizationCode{}) {
				t.Errorf("malformed query accepted: err=%v returned-code=%v returned-state=%v", err, got.Code != "", got.State != "")
			}
		})
	}
}
func TestLoginRejectsMalformedRedirectState(t *testing.T) {
	for _, kind := range []string{"url-percent", "url-semicolon", "query-percent", "query-semicolon", "valid-url", "bare-code"} {
		t.Run(kind, func(t *testing.T) {
			requests := 0
			access := fakeAccessToken(t, "synthetic-account", "plus")
			payload, _ := json.Marshal(map[string]any{"access_token": access, "refresh_token": "synthetic-refresh", "expires_in": 3600})
			cfg := Config{CallbackHost: "127.0.0.1", CallbackPort: -1, HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
				requests++
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(payload))), Header: make(http.Header)}, nil
			})}}
			creds, err := Login(context.Background(), LoginOptions{Config: cfg, ManualCode: func(_ context.Context, prompt AuthPrompt) (string, error) {
				switch kind {
				case "url-percent":
					return "http://localhost/auth/callback?code=synthetic-code&state=%ZZ", nil
				case "url-semicolon":
					return "http://localhost/auth/callback?code=synthetic-code&state=wrong;discarded", nil
				case "query-percent":
					return "code=synthetic-code&state=%ZZ", nil
				case "query-semicolon":
					return "code=synthetic-code&state=wrong;discarded", nil
				case "valid-url":
					u, e := url.Parse(prompt.AuthURL)
					if e != nil {
						return "", e
					}
					return "http://localhost/auth/callback?code=synthetic-code&state=" + url.QueryEscape(u.Query().Get("state")), nil
				case "bare-code":
					return "synthetic-code", nil
				default:
					return "", fmt.Errorf("unknown fixture")
				}
			}})
			valid := kind == "valid-url" || kind == "bare-code"
			if valid {
				if err != nil || requests != 1 || creds.Validate() != nil {
					t.Fatalf("valid control failed: err=%v requests=%d", err, requests)
				}
				return
			}
			if err == nil || requests != 0 || !reflect.DeepEqual(creds, auth.Credentials{}) {
				t.Errorf("malformed manual input admitted: err=%v token-requests=%d credentials-published=%v", err, requests, !reflect.DeepEqual(creds, auth.Credentials{}))
			}
		})
	}
}

func TestCallbackRejectsMalformedStateControls(t *testing.T) {
	for _, tc := range []struct {
		name, state string
		valid       bool
	}{{"percent", "%ZZ", false}, {"semicolon", "wrong;discarded", false}, {"matching", "expected-state", true}} {
		t.Run(tc.name, func(t *testing.T) {
			ch := make(chan callbackResult, 1)
			handler := callbackHandler("expected-state", ch)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("GET", "http://localhost/auth/callback?code=synthetic-code&state="+tc.state, nil)
			handler.ServeHTTP(recorder, request)
			if tc.valid {
				if recorder.Code != http.StatusOK {
					t.Fatal("valid browser callback rejected")
				}
				select {
				case <-ch:
				default:
					t.Fatal("valid callback not published")
				}
				return
			}
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("malformed state status=%d", recorder.Code)
			}
			select {
			case <-ch:
				t.Error("malformed browser callback published code")
			default:
			}
		})
	}
}

func TestCallbackRejectsMalformedQueryBeforePublishing(t *testing.T) {
	for _, expected := range []string{"", "expected-state"} {
		for _, query := range []string{
			"code=synthetic-code&state=expected-state&ignored=%ZZ",
			"code=synthetic-code&state=%ZZ&state=expected-state",
			"code=synthetic-code&state=expected-state&ignored=bad;pair",
		} {
			ch := make(chan callbackResult, 1)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("GET", "http://localhost/auth/callback?"+query, nil)
			callbackHandler(expected, ch).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("malformed query accepted: expected-state-present=%v status=%d", expected != "", recorder.Code)
			}
			select {
			case <-ch:
				t.Error("malformed callback published authorization code")
			default:
			}
		}
	}
}

func TestCallbackValidQueryRetainsOptionalStatePolicy(t *testing.T) {
	for _, query := range []string{"code=synthetic-code", "code=synthetic-code&state=any-state"} {
		ch := make(chan callbackResult, 1)
		recorder := httptest.NewRecorder()
		callbackHandler("", ch).ServeHTTP(recorder, httptest.NewRequest("GET", "http://localhost/auth/callback?"+query, nil))
		if recorder.Code != http.StatusOK {
			t.Fatal("valid state-opt-out callback rejected")
		}
		select {
		case result := <-ch:
			if result.err != nil || result.code.Code != "synthetic-code" {
				t.Fatal("valid callback lost authorization code")
			}
		default:
			t.Fatal("valid callback did not publish authorization code")
		}
	}
}

func FuzzAuthorizationQueryForms(f *testing.F) {
	for _, seed := range []string{"plain", "%", "&;#=+", "\n\t", "日本語", strings.Repeat("s", 512)} {
		f.Add(seed, seed)
	}
	f.Fuzz(func(t *testing.T, code, state string) {
		if len(code) > 512 {
			code = code[:512]
		}
		if len(state) > 512 {
			state = state[:512]
		}
		code, state = "code-"+code+"-end", "state-"+state+"-end"
		query := "code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state)
		for _, prefix := range []string{"", "?", "http://localhost/auth/callback?"} {
			got, err := ParseAuthorizationInput(prefix+query, state)
			if err != nil || got != (AuthorizationCode{Code: code, State: state}) {
				t.Fatal("valid encoded query did not preserve authorization values")
			}
			for _, suffix := range []string{"&state=%ZZ", "&ignored=%", "&ignored=bad;pair", "&%GG=value"} {
				got, err := ParseAuthorizationInput(prefix+query+suffix, state)
				if err == nil || got != (AuthorizationCode{}) {
					t.Fatal("malformed query pair accepted or partial authorization values published")
				}
			}
		}
	})
}
