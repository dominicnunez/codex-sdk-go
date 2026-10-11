package login

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/login/auth"
)

func TestParseAuthorizationInputBoundsStateMismatchDiagnostic(t *testing.T) {
	longState := strings.Repeat("a", 255) + "\n\t\\\"" + strings.Repeat("b", 600)
	_, err := ParseAuthorizationInput("code=synthetic&state="+url.QueryEscape(longState), testState)
	if !errors.Is(err, ErrStateMismatch) {
		t.Fatalf("error = %v, want state mismatch", err)
	}
	if got := err.Error(); len(got) > 400 || !strings.Contains(got, `\n`) || !strings.Contains(got, "bytes omitted") {
		t.Fatalf("diagnostic is not safely bounded/escaped: len=%d error=%q", len(got), got)
	}

	_, err = ParseAuthorizationInput("code=synthetic&state="+strings.Repeat("p", 1024), testState)
	if !errors.Is(err, ErrStateMismatch) || len(fmt.Sprint(err)) > 400 {
		t.Fatalf("plain large state diagnostic is not bounded: %v", err)
	}
}

func TestParseAuthorizationInputComparesFullStateAndPreservesShortError(t *testing.T) {
	prefix := strings.Repeat("p", 300)
	gotState := prefix + "different-tail"
	wantState := prefix + "expected-tail"
	_, err := ParseAuthorizationInput("synthetic#"+gotState, wantState)
	if !errors.Is(err, ErrStateMismatch) {
		t.Fatalf("equal-prefix state accepted: %v", err)
	}

	code, err := ParseAuthorizationInput("code=synthetic&state=wrong", testState)
	if !errors.Is(err, ErrStateMismatch) || code != (AuthorizationCode{}) || err.Error() != "state mismatch: got wrong" {
		t.Fatalf("short mismatch = (%+v, %v), want zero code and stable short error", code, err)
	}

	matching := prefix + "complete-matching-tail"
	code, err = ParseAuthorizationInput("code=synthetic&state="+matching, matching)
	if err != nil || code != (AuthorizationCode{Code: "synthetic", State: matching}) {
		t.Fatalf("full matching state = (%+v, %v), want complete state preserved", code, err)
	}
}

func TestParseAuthorizationInputBoundsMismatchAcrossInputForms(t *testing.T) {
	badState := strings.Repeat("s", 1000)
	query := "code=synthetic&state=" + url.QueryEscape(badState)
	forms := map[string]string{
		"URL":      "http://localhost/callback?" + query,
		"query":    query,
		"fragment": "synthetic#" + badState,
	}
	for name, input := range forms {
		t.Run(name, func(t *testing.T) {
			_, err := ParseAuthorizationInput(input, testState)
			if !errors.Is(err, ErrStateMismatch) || len(err.Error()) > 400 {
				t.Fatalf("error = %v (len %d), want bounded state mismatch", err, len(fmt.Sprint(err)))
			}
		})
	}
}

type manualStateNoRequestTransport struct{ requests atomic.Int32 }

func (tr *manualStateNoRequestTransport) RoundTrip(*http.Request) (*http.Response, error) {
	tr.requests.Add(1)
	return nil, fmt.Errorf("unexpected token request")
}

func TestLoginManualStateMismatchDoesNotExchangeCode(t *testing.T) {
	transport := &manualStateNoRequestTransport{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	credentials, err := Login(ctx, LoginOptions{
		Config: Config{
			CallbackHost: "127.0.0.1", CallbackPort: -1,
			HTTPClient: &http.Client{Transport: transport},
		},
		ManualCode: func(_ context.Context, prompt AuthPrompt) (string, error) {
			authorizeURL, err := url.Parse(prompt.AuthURL)
			if err != nil {
				return "", err
			}
			state := authorizeURL.Query().Get("state")
			return "code=synthetic&state=" + state + strings.Repeat("z", 1000), nil
		},
	})
	if err == nil || !errors.Is(err, ErrStateMismatch) {
		t.Fatalf("Login() = (%+v, %v), want zero credentials and state mismatch", credentials.Redacted(), err)
	}
	if len(err.Error()) > 400 {
		t.Fatalf("Login() diagnostic length = %d, want <= 400: %q", len(err.Error()), err)
	}
	if transport.requests.Load() != 0 {
		t.Fatalf("token request count = %d, want zero", transport.requests.Load())
	}
	if credentials != (auth.Credentials{}) {
		t.Fatalf("Login() credentials = %+v, want zero credentials", credentials.Redacted())
	}
}
