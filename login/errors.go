package login

import "errors"

var (
	ErrMissingAuthorizationCode = errors.New("missing authorization code")
	ErrStateMismatch            = errors.New("state mismatch")
	// ErrAuthorizationFailed identifies a reported OAuth authorization failure.
	ErrAuthorizationFailed = errors.New("OAuth authorization failed")
)
