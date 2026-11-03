package login

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dominicnunez/codex-sdk-go/internal/diagnostic"
)

type AuthorizationCode struct {
	Code  string
	State string
}

// ParseAuthorizationInput accepts a redirect URL, query, code/state pair or
// trusted bare code. Malformed query pairs fail before the missing-state
// exception is considered; no partial authorization values are returned.
func ParseAuthorizationInput(input string, expectedState string) (AuthorizationCode, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return AuthorizationCode{}, ErrMissingAuthorizationCode
	}

	code, state, err := parseAuthorizationInput(trimmed)
	if err != nil {
		return AuthorizationCode{}, err
	}
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" {
		return AuthorizationCode{}, ErrMissingAuthorizationCode
	}
	if expectedState != "" && state != "" && state != expectedState {
		return AuthorizationCode{}, fmt.Errorf("%w: got %s", ErrStateMismatch, diagnostic.Display(state))
	}
	return AuthorizationCode{Code: code, State: state}, nil
}

func parseAuthorizationInput(input string) (string, string, error) {
	if parsed, err := url.Parse(input); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		values, err := parseAuthorizationQuery(parsed.RawQuery)
		if err != nil {
			return "", "", err
		}
		return values.Get("code"), values.Get("state"), nil
	}

	if strings.Contains(input, "code=") {
		query := strings.TrimPrefix(input, "?")
		values, err := parseAuthorizationQuery(query)
		if err != nil {
			return "", "", err
		}
		return values.Get("code"), values.Get("state"), nil
	}

	if strings.Contains(input, "#") {
		code, state, _ := strings.Cut(input, "#")
		return code, state, nil
	}

	return input, "", nil
}

func parseAuthorizationQuery(query string) (url.Values, error) {
	values, err := url.ParseQuery(query)
	if err != nil {
		return nil, fmt.Errorf("parse authorization query: %w", err)
	}
	return values, nil
}
