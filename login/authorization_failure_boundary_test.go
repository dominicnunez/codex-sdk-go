package login

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/login/auth"
)

func assertAuthorizationListenerClosed(t *testing.T, port int) {
	t.Helper()
	connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), time.Second)
	if err == nil {
		_ = connection.Close()
		t.Fatal("completed authorization flow retained callback listener")
	}
}

func TestManualAuthorizationFailureClosesCallback(t *testing.T) {
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reservation.Addr().(*net.TCPAddr).Port
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	transport := &manualStateNoRequestTransport{}
	var inputContext context.Context
	credentials, err := Login(context.Background(), LoginOptions{
		Config: Config{CallbackHost: "127.0.0.1", CallbackPort: port, HTTPClient: &http.Client{Transport: transport}},
		ManualCode: func(ctx context.Context, prompt AuthPrompt) (string, error) {
			inputContext = ctx
			authorize, err := url.Parse(prompt.AuthURL)
			if err != nil {
				return "", err
			}
			return url.Values{"error": {"access_denied"}, "state": {authorize.Query().Get("state")}}.Encode(), nil
		},
	})
	if !errors.Is(err, ErrAuthorizationFailed) || credentials != (auth.Credentials{}) || transport.requests.Load() != 0 {
		t.Fatal("manual failure did not reject before token exchange")
	}
	if inputContext == nil || !errors.Is(inputContext.Err(), context.Canceled) {
		t.Fatal("manual outcome did not cancel competing wait context")
	}
	assertAuthorizationListenerClosed(t, port)
}

func TestAuthorizationFailureBeforeCallbackServeClosesListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	callback := &CallbackServer{listen: listener, codeCh: make(chan callbackResult, 1), server: &http.Server{}}
	transport := &manualStateNoRequestTransport{}
	// StartCallbackServer returns before its Serve goroutine must run. Hold
	// precisely that phase while an immediate manual failure wins Login's wait.
	credentials, err := loginWithCallbackServer(context.Background(), LoginOptions{
		Config: Config{HTTPClient: &http.Client{Transport: transport}},
		ManualCode: func(_ context.Context, prompt AuthPrompt) (string, error) {
			authorize, err := url.Parse(prompt.AuthURL)
			if err != nil {
				return "", err
			}
			return url.Values{"error": {"access_denied"}, "state": {authorize.Query().Get("state")}}.Encode(), nil
		},
	}, func(context.Context, Config, string) (*CallbackServer, error) { return callback, nil })
	if !errors.Is(err, ErrAuthorizationFailed) || credentials != (auth.Credentials{}) || transport.requests.Load() != 0 {
		t.Fatal("phase-held manual failure did not reject before exchange")
	}
	connection, dialErr := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if dialErr == nil {
		_ = connection.Close()
		t.Error("Login returned while its pre-Serve callback listener remained open")
	}
	// A delayed Serve must also stop without making the closed flow serviceable.
	if err := callback.server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("delayed Serve result=%v, want server closed", err)
	}
}

type failingCallbackListener struct {
	net.Listener
	failure error
}

func (listener failingCallbackListener) Close() error {
	_ = listener.Listener.Close()
	return listener.failure
}

func TestCallbackCloseReportsOwnedListenerFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	failure := errors.New("fixture listener close failure")
	callback := &CallbackServer{listen: failingCallbackListener{listener, failure}, server: &http.Server{}}
	if err := callback.Close(); !errors.Is(err, failure) {
		t.Fatalf("Close lost owned listener error: %v", err)
	}
	if connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second); err == nil {
		_ = connection.Close()
		t.Fatal("listener was not retired when close reported an error")
	}
}

func TestAuthorizationFailurePreservesManualPairState(t *testing.T) {
	state := "state&error=literal-data"
	code, err := ParseAuthorizationInput("synthetic-code#"+state, state)
	if err != nil || code != (AuthorizationCode{Code: "synthetic-code", State: state}) {
		t.Fatal("manual pair state was reinterpreted as an authorization query")
	}
}

func TestAuthorizationFailureStateAndPublication(t *testing.T) {
	for _, response := range []string{"access_denied", "server_error", "extension_error"} {
		for _, state := range []string{"expected", "wrong", ""} {
			query := url.Values{"error": {response}, "state": {state}, "code": {"must-not-exchange"}}.Encode()
			for _, prefix := range []string{"", "?", "http://localhost/auth/callback?"} {
				code, err := ParseAuthorizationInput(prefix+query, "expected")
				want := ErrAuthorizationFailed
				if state == "wrong" {
					want = ErrStateMismatch
				}
				if !errors.Is(err, want) || code != (AuthorizationCode{}) {
					t.Fatalf("manual failure classification=%v, want %v and zero code", err, want)
				}
			}
			ch := make(chan callbackResult, 1)
			recorder := httptest.NewRecorder()
			callbackHandler("expected", ch).ServeHTTP(recorder, httptest.NewRequest("GET", "http://localhost/auth/callback?"+query, nil))
			if recorder.Code != http.StatusBadRequest {
				t.Fatal("provider failure did not receive failure page")
			}
			select {
			case result := <-ch:
				if state != "expected" || !errors.Is(result.err, ErrAuthorizationFailed) || result.code != (AuthorizationCode{}) {
					t.Fatal("callback published an unauthenticated or successful result")
				}
			default:
				if state == "expected" {
					t.Fatal("matching-state provider failure did not terminate callback wait")
				}
			}
		}
	}
}

func TestAuthorizationFailureDiagnosticsAndHTML(t *testing.T) {
	query := url.Values{"error": {strings.Repeat("x", 4096)}, "error_description": {"<b>reason</b>" + strings.Repeat("z", 4096)}, "state": {"expected"}}.Encode()
	_, err := ParseAuthorizationInput(query, "expected")
	if !errors.Is(err, ErrAuthorizationFailed) || len(err.Error()) > 1000 || !strings.Contains(err.Error(), "bytes omitted") {
		t.Fatal("provider error diagnostic is not bounded")
	}
	ch := make(chan callbackResult, 1)
	recorder := httptest.NewRecorder()
	callbackHandler("expected", ch).ServeHTTP(recorder, httptest.NewRequest("GET", "http://localhost/auth/callback?"+query, nil))
	if strings.Contains(recorder.Body.String(), "<b>reason</b>") || !strings.Contains(recorder.Body.String(), "&lt;b&gt;reason&lt;/b&gt;") || recorder.Body.Len() > 1500 {
		t.Fatal("callback error text is not bounded and HTML-escaped")
	}
	select {
	case result := <-ch:
		if !errors.Is(result.err, ErrAuthorizationFailed) || len(result.err.Error()) > 1000 {
			t.Fatal("callback failure lost bounded error")
		}
	default:
		t.Fatal("callback failure was not published")
	}
}

func FuzzAuthorizationFailureForms(f *testing.F) {
	f.Add("access_denied", "declined", "nonce")
	f.Add("server_error", "<b>text</b>\n", "state&error=literal")
	f.Add("extension_error", strings.Repeat("x", 512), "%+;#")
	f.Fuzz(func(t *testing.T, code, description, state string) {
		if len(code) > 512 {
			code = code[:512]
		}
		if len(description) > 512 {
			description = description[:512]
		}
		if len(state) > 512 {
			state = state[:512]
		}
		code, state = "error-"+code+"-end", "state-"+state+"-end"
		query := url.Values{"error": {code}, "error_description": {description}, "state": {state}}.Encode()
		for _, prefix := range []string{"", "?", "http://localhost/auth/callback?"} {
			got, err := ParseAuthorizationInput(prefix+query, state)
			if !errors.Is(err, ErrAuthorizationFailed) || got != (AuthorizationCode{}) {
				t.Fatal("matching-state failure did not reject authorization")
			}
			got, err = ParseAuthorizationInput(prefix+query, state+"-different")
			if !errors.Is(err, ErrStateMismatch) || got != (AuthorizationCode{}) {
				t.Fatal("provider failure bypassed state comparison")
			}
			got, err = ParseAuthorizationInput(prefix+query+"&ignored=%ZZ", state)
			if err == nil || errors.Is(err, ErrAuthorizationFailed) || got != (AuthorizationCode{}) {
				t.Fatal("malformed query was reclassified as a valid provider response")
			}
		}
	})
}
