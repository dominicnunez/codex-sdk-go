package login

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/login/auth"
)

func TestNumericDiagnosticTokenResponses(t *testing.T) {
	for operation, call := range map[string]func(context.Context, Config) (auth.Credentials, error){
		"exchange": func(ctx context.Context, cfg Config) (auth.Credentials, error) {
			return ExchangeCode(ctx, cfg, "synthetic-code", "synthetic-verifier")
		},
		"refresh": func(ctx context.Context, cfg Config) (auth.Credentials, error) {
			return Refresh(ctx, cfg, "synthetic-refresh")
		},
	} {
		t.Run(operation, func(t *testing.T) {
			access := fakeAccessToken(t, "synthetic-account", "plus")
			quoted, err := json.Marshal(access)
			if err != nil {
				t.Fatal(err)
			}
			payload := `{"access_token":` + string(quoted) + `,"refresh_token":"synthetic-new-refresh","expires_in":1` + strings.Repeat("0", 1<<18) + `}`
			var reference tokenResponse
			var want *json.UnmarshalTypeError
			if !errors.As(json.Unmarshal([]byte(payload), &reference), &want) {
				t.Fatal("reference did not overflow")
			}
			var body *countedTokenBody
			cfg := Config{HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
				body = &countedTokenBody{reader: strings.NewReader(payload)}
				return &http.Response{StatusCode: http.StatusOK, Body: body, Header: make(http.Header)}, nil
			})}}
			credentials, err := call(context.Background(), cfg)
			var typed *json.UnmarshalTypeError
			if !errors.As(err, &typed) || len(typed.Value) > 2048 || len(err.Error()) > 4096 || !strings.Contains(typed.Value, "bytes omitted") {
				t.Fatal("provider overflow lost bounded typed rejection")
			}
			metadata := *want
			metadata.Value = typed.Value
			if !reflect.DeepEqual(*typed, metadata) || !reflect.DeepEqual(credentials, auth.Credentials{}) || body == nil || !body.closed {
				t.Fatal("provider overflow changed context, published credentials or left body open")
			}
			payload = `{"access_token":` + string(quoted) + `,"refresh_token":"synthetic-new-refresh","expires_in":3600}`
			credentials, err = call(context.Background(), cfg)
			if err != nil || credentials.AccountID != "synthetic-account" || body == nil || !body.closed {
				t.Fatal("valid provider response did not recover")
			}
		})
	}
}
