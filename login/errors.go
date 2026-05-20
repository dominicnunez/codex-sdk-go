package login

import "errors"

var (
	ErrMissingAuthorizationCode = errors.New("missing authorization code")
	ErrStateMismatch            = errors.New("state mismatch")
)
