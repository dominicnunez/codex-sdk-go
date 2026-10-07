package login

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/login/auth"
)

func TestRefreshRetainsSubmittedTokenWhenResponseDoesNotRotate(t *testing.T) {
	access := fakeAccessToken(t, "synthetic-account", "plus")
	for _, replacement := range []struct {
		name  string
		field string
	}{
		{name: "omitted"},
		{name: "null", field: `,"refresh_token":null`},
	} {
		t.Run(replacement.name, func(t *testing.T) {
			payload := `{"access_token":` + mustJSONString(t, access) + replacement.field + `,"expires_in":3600}`
			var requestToken string
			cfg := Config{HTTPClient: &http.Client{Transport: auditTokenTransport(func(req *http.Request) (*http.Response, error) {
				var request refreshRequest
				if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
					t.Fatalf("decode refresh request: %v", err)
				}
				requestToken = request.RefreshToken
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header)}, nil
			})}}

			creds, err := Refresh(context.Background(), cfg, "  synthetic-original-refresh  ")
			if err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}
			if requestToken != "synthetic-original-refresh" || creds.RefreshToken != requestToken {
				t.Fatalf("refresh token request=%q credentials=%q, want trimmed submitted token", requestToken, creds.RefreshToken)
			}
			if creds.AccessToken != access || creds.AccountID != "synthetic-account" || creds.PlanType == nil || *creds.PlanType != "plus" || creds.ExpiresAt.IsZero() {
				t.Fatal("refresh did not preserve the other validated credential fields")
			}

			stored, err := json.Marshal(creds)
			if err != nil {
				t.Fatalf("marshal credentials: %v", err)
			}
			var restored auth.Credentials
			if err := json.Unmarshal(stored, &restored); err != nil || restored.Validate() != nil || restored.RefreshToken != requestToken {
				t.Fatalf("retained credentials do not survive persistence: err=%v credentials=%+v", err, restored.Redacted())
			}
			if _, err := auth.NewAuthTokensLoginParams(restored); err != nil {
				t.Fatalf("retained credentials cannot form login payload: %v", err)
			}
			if _, err := auth.NewAuthTokensRefreshResponse(restored); err != nil {
				t.Fatalf("retained credentials cannot form refresh payload: %v", err)
			}
		})
	}
}

func TestRefreshRejectsInvalidReplacementAndEmptyFallback(t *testing.T) {
	access := fakeAccessToken(t, "synthetic-account", "plus")
	quotedAccess := mustJSONString(t, access)
	tests := []struct {
		name        string
		refresh     string
		replacement string
	}{
		{name: "blank rotation", refresh: "synthetic-original", replacement: `,"refresh_token":""`},
		{name: "whitespace rotation", refresh: "synthetic-original", replacement: `,"refresh_token":"  \t "`},
		{name: "invalid rotation type", refresh: "synthetic-original", replacement: `,"refresh_token":7`},
		{name: "missing replacement without fallback", replacement: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := `{"access_token":` + quotedAccess + tt.replacement + `,"expires_in":3600}`
			cfg := Config{HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header)}, nil
			})}}
			creds, err := Refresh(context.Background(), cfg, tt.refresh)
			if err == nil || !errors.Is(err, auth.ErrMissingTokenFields) && tt.name != "invalid rotation type" {
				t.Fatalf("Refresh() = (%+v, %v), want rejected response", creds.Redacted(), err)
			}
			if !reflect.DeepEqual(creds, auth.Credentials{}) {
				t.Fatal("invalid refresh response published partial credentials")
			}
		})
	}
}

func TestRefreshUsesRotationEvenWhenSubmittedTokenIsEmpty(t *testing.T) {
	access := fakeAccessToken(t, "synthetic-account", "plus")
	payload := `{"access_token":` + mustJSONString(t, access) + `,"refresh_token":"synthetic-rotated-refresh","expires_in":3600}`
	cfg := Config{HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header)}, nil
	})}}
	creds, err := Refresh(context.Background(), cfg, "  ")
	if err != nil || creds.RefreshToken != "synthetic-rotated-refresh" {
		t.Fatalf("Refresh() with valid rotation = (%+v, %v)", creds.Redacted(), err)
	}
}

func TestExchangeCodeStillRequiresIssuedRefreshToken(t *testing.T) {
	access := fakeAccessToken(t, "synthetic-account", "plus")
	for _, replacement := range []struct {
		name  string
		field string
	}{
		{name: "omitted"},
		{name: "null", field: `,"refresh_token":null`},
		{name: "blank", field: `,"refresh_token":"  "`},
	} {
		t.Run(replacement.name, func(t *testing.T) {
			payload := `{"access_token":` + mustJSONString(t, access) + replacement.field + `,"expires_in":3600}`
			cfg := Config{HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header)}, nil
			})}}
			creds, err := ExchangeCode(context.Background(), cfg, "synthetic-code", "synthetic-verifier")
			if !errors.Is(err, auth.ErrMissingTokenFields) || !reflect.DeepEqual(creds, auth.Credentials{}) {
				t.Fatalf("ExchangeCode() = (%+v, %v), want missing issued refresh token and zero credentials", creds.Redacted(), err)
			}
		})
	}
}

func mustJSONString(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
