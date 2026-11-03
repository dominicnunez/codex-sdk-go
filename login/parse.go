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
// Recognized OAuth error responses return an error wrapping ErrAuthorizationFailed
// after the manual input state policy is applied.
func ParseAuthorizationInput(input string, expectedState string) (AuthorizationCode, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return AuthorizationCode{}, ErrMissingAuthorizationCode
	}

	values, err := parseAuthorizationInput(trimmed)
	if err != nil {
		return AuthorizationCode{}, err
	}
	code := strings.TrimSpace(values.Get("code"))
	state := strings.TrimSpace(values.Get("state"))
	failure := authorizationFailure(values)
	if code == "" && failure == nil {
		return AuthorizationCode{}, ErrMissingAuthorizationCode
	}
	if expectedState != "" && state != "" && state != expectedState {
		return AuthorizationCode{}, fmt.Errorf("%w: got %s", ErrStateMismatch, diagnostic.Display(state))
	}
	if failure != nil {
		return AuthorizationCode{}, failure
	}
	return AuthorizationCode{Code: code, State: state}, nil
}

func parseAuthorizationInput(input string) (url.Values, error) {
	if parsed, err := url.Parse(input); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parseAuthorizationQuery(parsed.RawQuery)
	}

	query := strings.TrimPrefix(input, "?")
	queryPrefix, _, _ := strings.Cut(query, "#")
	if strings.Contains(input, "code=") || strings.HasPrefix(queryPrefix, "error=") || strings.Contains(queryPrefix, "&error=") {
		return parseAuthorizationQuery(query)
	}

	if strings.Contains(input, "#") {
		code, state, _ := strings.Cut(input, "#")
		return url.Values{"code": {code}, "state": {state}}, nil
	}

	return url.Values{"code": {input}}, nil
}

func authorizationFailure(values url.Values) error {
	code := values.Get("error")
	if code == "" {
		return nil
	}
	if description := values.Get("error_description"); description != "" {
		return fmt.Errorf("%w: %s: %s", ErrAuthorizationFailed, diagnostic.Display(code), diagnostic.Display(description))
	}
	return fmt.Errorf("%w: %s", ErrAuthorizationFailed, diagnostic.Display(code))
}

func parseAuthorizationQuery(query string) (url.Values, error) {
	values, err := url.ParseQuery(query)
	if err != nil {
		return nil, fmt.Errorf("parse authorization query: %w", err)
	}
	return values, nil
}
