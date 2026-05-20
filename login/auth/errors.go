package auth

import "errors"

var (
	ErrMissingTokenFields = errors.New("token response missing fields")
	ErrMissingAccountID   = errors.New("access token missing ChatGPT account ID")
	ErrInvalidCredentials = errors.New("invalid credentials")
)
