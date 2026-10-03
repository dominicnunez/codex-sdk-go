package login

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/login/auth"
)

func TestTokenLifetime(t *testing.T) {
	operations := map[string]func(context.Context, Config) (auth.Credentials, error){
		"exchange": func(ctx context.Context, cfg Config) (auth.Credentials, error) {
			return ExchangeCode(ctx, cfg, "synthetic-code", "synthetic-verifier")
		},
		"refresh": func(ctx context.Context, cfg Config) (auth.Credentials, error) {
			return Refresh(ctx, cfg, "synthetic-old-refresh")
		},
	}
	for operation, call := range operations {
		for _, lifetime := range []int64{1, 3600, 9223372036, 0, -1, 9223372037, 18446744074, 9223372036854775807} {
			t.Run(operation+"/"+strconv.FormatInt(lifetime, 10), func(t *testing.T) {
				access := fakeAccessToken(t, "synthetic-account", "plus")
				payload, err := json.Marshal(map[string]any{"access_token": access, "refresh_token": "synthetic-new-refresh", "expires_in": lifetime})
				if err != nil {
					t.Fatal(err)
				}
				body := &countedTokenBody{reader: strings.NewReader(string(payload))}
				cfg := Config{HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
				})}}
				before := time.Now()
				creds, err := call(context.Background(), cfg)
				after := time.Now()
				if !body.closed {
					t.Error("response body not closed")
				}
				if lifetime <= 0 || lifetime > 9223372036 {
					if err == nil {
						t.Fatal("unsupported lifetime published successful credentials")
					}
					if !reflect.DeepEqual(creds, auth.Credentials{}) {
						t.Fatal("rejected lifetime published partial credentials")
					}
					if strings.Contains(err.Error(), access) || strings.Contains(err.Error(), "synthetic-new-refresh") {
						t.Fatal("lifetime error exposes token fields")
					}
					if lifetime <= 0 && !errors.Is(err, auth.ErrMissingTokenFields) {
						t.Fatalf("nonpositive lifetime changed existing error: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				// Compare seconds in Unix time independently of duration multiplication.
				if got := creds.ExpiresAt.Unix(); got < before.Unix()+lifetime || got > after.Unix()+lifetime || !creds.ExpiresAt.After(after) {
					t.Fatalf("expiration lost intended lifetime %d", lifetime)
				}
				if creds.AccessToken != access || creds.RefreshToken != "synthetic-new-refresh" || creds.AccountID != "synthetic-account" || creds.PlanType == nil || *creds.PlanType != "plus" {
					t.Fatal("successful credentials lost fields")
				}
				encoded, err := json.Marshal(creds)
				if err != nil {
					t.Fatalf("expiration cannot be persisted: %v", err)
				}
				var restored auth.Credentials
				if err := json.Unmarshal(encoded, &restored); err != nil || !restored.ExpiresAt.Equal(creds.ExpiresAt) || restored.Validate() != nil {
					t.Fatal("expiration does not survive credential serialization")
				}
			})
		}
	}
}

func TestTokenLifetimeCancellation(t *testing.T) {
	for _, exchange := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		cfg := Config{HTTPClient: &http.Client{Transport: auditTokenTransport(func(req *http.Request) (*http.Response, error) {
			cancel()
			<-req.Context().Done()
			return nil, req.Context().Err()
		})}}
		var creds auth.Credentials
		var err error
		if exchange {
			creds, err = ExchangeCode(ctx, cfg, "synthetic-code", "verifier")
		} else {
			creds, err = Refresh(ctx, cfg, "synthetic-refresh")
		}
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(creds, auth.Credentials{}) {
			t.Fatal("cancellation published credentials or lost error")
		}
	}
}

func TestTokenLifetimeResponseLimit(t *testing.T) {
	body := &countedTokenBody{reader: strings.NewReader(`{"access_token":"` + strings.Repeat("x", int(maxTokenResponseBytes)) + `","refresh_token":"synthetic-refresh","expires_in":9223372037}`)}
	cfg := Config{HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
	})}}
	creds, err := Refresh(context.Background(), cfg, "synthetic-refresh")
	if err == nil || !reflect.DeepEqual(creds, auth.Credentials{}) || body.read > int(maxTokenResponseBytes) || !body.closed {
		t.Fatal("token response admission exceeded read limit")
	}
}
