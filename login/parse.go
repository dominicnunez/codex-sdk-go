package login

import (
	"fmt"
	"net/url"
	"strings"
)

type AuthorizationCode struct {
	Code  string
	State string
}

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
		return AuthorizationCode{}, fmt.Errorf("%w: got %s", ErrStateMismatch, state)
	}
	return AuthorizationCode{Code: code, State: state}, nil
}

func parseAuthorizationInput(input string) (string, string, error) {
	if parsed, err := url.Parse(input); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		q := parsed.Query()
		return q.Get("code"), q.Get("state"), nil
	}

	if strings.Contains(input, "code=") {
		query := strings.TrimPrefix(input, "?")
		values, err := url.ParseQuery(query)
		if err != nil {
			return "", "", fmt.Errorf("parse authorization query: %w", err)
		}
		return values.Get("code"), values.Get("state"), nil
	}

	if strings.Contains(input, "#") {
		code, state, _ := strings.Cut(input, "#")
		return code, state, nil
	}

	return input, "", nil
}
