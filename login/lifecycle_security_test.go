package login

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/login/auth"
)

type auditTokenTransport func(*http.Request) (*http.Response, error)

func (f auditTokenTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestInvalidTokenResponseDoesNotLeak(t *testing.T) {
	cfg := Config{HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"audit-access-secret","refresh_token":"audit-refresh-secret","expires_in":0}`)), Header: make(http.Header)}, nil
	})}}
	_, err := Refresh(context.Background(), cfg, "old-refresh")
	if !errors.Is(err, auth.ErrMissingTokenFields) {
		t.Fatalf("expected missing token fields, got %v", err)
	}
	if strings.Contains(err.Error(), "audit-access-secret") || strings.Contains(err.Error(), "audit-refresh-secret") {
		t.Error("invalid token response exposes secrets in error")
	}
}

func TestOAuthCallbackCancelsManualInput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	promptStarted := make(chan struct{})
	promptStopped := make(chan struct{})
	server := &CallbackServer{codeCh: make(chan callbackResult, 1)}
	go func() {
		<-promptStarted
		server.codeCh <- callbackResult{code: AuthorizationCode{Code: "valid-code", State: "valid-state"}}
	}()
	_, err := waitForAuthorizationCode(ctx, server, nil, func(ctx context.Context, _ AuthPrompt) (string, error) {
		close(promptStarted)
		<-ctx.Done()
		close(promptStopped)
		return "", ctx.Err()
	}, "auth-url", "valid-state")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-promptStopped:
	case <-time.After(time.Second):
		t.Error("manual input context remains live after callback wins")
	}
	if ctx.Err() != nil {
		t.Fatal("caller context was canceled before token exchange")
	}
	cancel()
	<-promptStopped
}

func TestOAuthManualInputCancelsWaitContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var inputContext context.Context
	server := &CallbackServer{codeCh: make(chan callbackResult, 1)}
	_, err := waitForAuthorizationCode(ctx, server, nil, func(ctx context.Context, _ AuthPrompt) (string, error) {
		inputContext = ctx
		return "valid-code", nil
	}, "auth-url", "valid-state")
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(inputContext.Err(), context.Canceled) {
		t.Fatal("input race context was not canceled")
	}
	if ctx.Err() != nil {
		t.Fatal("caller context was canceled")
	}
}

func TestTokenHTTPErrorDoesNotEchoBody(t *testing.T) {
	cfg := Config{HTTPClient: &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(`{"error":"invalid token: audit-secret"}`)), Header: make(http.Header)}, nil
	})}}
	_, err := Refresh(context.Background(), cfg, "audit-secret")
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("missing HTTP status: %v", err)
	}
	if strings.Contains(err.Error(), "audit-secret") {
		t.Fatal("HTTP error echoes credentials")
	}
}
