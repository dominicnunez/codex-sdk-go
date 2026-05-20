package login

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const testState = "0123456789abcdef0123456789abcdef"
const testOpenAIAuthClaim = "https://api.openai.com/auth"

func TestBuildAuthorizeURL(t *testing.T) {
	authURL, err := BuildAuthorizeURL(Config{Originator: "codex-sdk-go-test"}, PKCE{Challenge: "challenge"}, testState)
	if err != nil {
		t.Fatalf("BuildAuthorizeURL() error = %v", err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	query := parsed.Query()
	want := map[string]string{
		"response_type":              "code",
		"client_id":                  DefaultClientID,
		"redirect_uri":               DefaultRedirectURI,
		"scope":                      DefaultScope,
		"code_challenge":             "challenge",
		"code_challenge_method":      "S256",
		"state":                      testState,
		"id_token_add_organizations": "true",
		"codex_cli_simplified_flow":  "true",
		"originator":                 "codex-sdk-go-test",
	}
	for key, value := range want {
		if got := query.Get(key); got != value {
			t.Fatalf("query %s = %q, want %q", key, got, value)
		}
	}
}

func TestParseAuthorizationInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		code  string
		state string
	}{
		{name: "full URL", input: "http://localhost:1455/auth/callback?code=abc&state=" + testState, code: "abc", state: testState},
		{name: "query", input: "code=abc&state=" + testState, code: "abc", state: testState},
		{name: "fragment", input: "abc#" + testState, code: "abc", state: testState},
		{name: "bare", input: "abc", code: "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAuthorizationInput(tt.input, testState)
			if err != nil {
				t.Fatalf("ParseAuthorizationInput() error = %v", err)
			}
			if got.Code != tt.code || got.State != tt.state {
				t.Fatalf("ParseAuthorizationInput() = %+v, want code=%q state=%q", got, tt.code, tt.state)
			}
		})
	}
}

func TestParseAuthorizationInputRejectsStateMismatch(t *testing.T) {
	_, err := ParseAuthorizationInput("code=abc&state=wrong", testState)
	if !errors.Is(err, ErrStateMismatch) {
		t.Fatalf("ParseAuthorizationInput() error = %v, want state mismatch", err)
	}
}

func TestExchangeCodeAndRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		form := tokenRequestForm(t, r)
		grantType := form.Get("grant_type")
		if form.Get("client_id") != DefaultClientID {
			t.Fatalf("client_id = %q", form.Get("client_id"))
		}
		switch grantType {
		case grantTypeAuthorizationCode:
			if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Fatalf("authorization content type = %q", r.Header.Get("Content-Type"))
			}
			if form.Get("code") != "auth-code" || form.Get("code_verifier") != "verifier" || form.Get("redirect_uri") != DefaultRedirectURI {
				t.Fatalf("unexpected auth-code form: %v", form)
			}
		case grantTypeRefreshToken:
			if r.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("refresh content type = %q", r.Header.Get("Content-Type"))
			}
			if form.Get("refresh_token") != "refresh-1" {
				t.Fatalf("refresh_token = %q", form.Get("refresh_token"))
			}
		default:
			t.Fatalf("grant_type = %q", grantType)
		}
		writeTokenResponse(t, w, fakeAccessToken(t, "acct_123", "plus"), "refresh-2")
	}))
	t.Cleanup(server.Close)

	cfg := Config{TokenEndpoint: server.URL, HTTPClient: server.Client()}
	creds, err := ExchangeCode(context.Background(), cfg, "auth-code", "verifier")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
	if creds.AccountID != "acct_123" || creds.RefreshToken != "refresh-2" || creds.ExpiresAt.IsZero() {
		t.Fatalf("ExchangeCode() = %+v", creds.Redacted())
	}

	refreshed, err := Refresh(context.Background(), cfg, "refresh-1")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.AccountID != "acct_123" || refreshed.RefreshToken != "refresh-2" || refreshed.ExpiresAt.IsZero() {
		t.Fatalf("Refresh() = %+v", refreshed.Redacted())
	}
}

func TestCallbackHandler(t *testing.T) {
	codeCh := make(chan callbackResult, 1)
	handler := callbackHandler(testState, codeCh)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state="+testState, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	select {
	case result := <-codeCh:
		if result.err != nil || result.code.Code != "abc" {
			t.Fatalf("callback result = %+v", result)
		}
	default:
		t.Fatalf("callback result not sent")
	}
}

func TestCallbackHandlerRejectsWrongPath(t *testing.T) {
	codeCh := make(chan callbackResult, 1)
	handler := callbackHandler(testState, codeCh)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/wrong?code=abc&state="+testState, nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func writeTokenResponse(t *testing.T, w http.ResponseWriter, accessToken string, refreshToken string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    3600,
	}); err != nil {
		t.Fatalf("encode token response: %v", err)
	}
}

func tokenRequestForm(t *testing.T, r *http.Request) url.Values {
	t.Helper()
	if r.Header.Get("Content-Type") == "application/json" {
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode JSON token request: %v", err)
		}
		values := url.Values{}
		for key, value := range payload {
			values.Set(key, value)
		}
		return values
	}
	if err := r.ParseForm(); err != nil {
		t.Fatalf("ParseForm: %v", err)
	}
	return r.Form
}

func fakeAccessToken(t *testing.T, accountID string, planType string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := map[string]any{
		testOpenAIAuthClaim: map[string]any{
			"chatgpt_account_id": accountID,
			"chatgpt_plan_type":  planType,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fake JWT payload: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payloadBytes) + ".signature"
}
