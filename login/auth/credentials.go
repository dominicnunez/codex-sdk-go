package auth

import (
	"fmt"
	"strings"
	"time"
)

type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	AccountID    string    `json:"account_id"`
	PlanType     *string   `json:"plan_type,omitempty"`
}

type RedactedCredentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	AccountID    string    `json:"account_id"`
	PlanType     *string   `json:"plan_type,omitempty"`
}

func (c Credentials) Validate() error {
	if strings.TrimSpace(c.AccessToken) == "" {
		return fmt.Errorf("%w: access token is required", ErrInvalidCredentials)
	}
	if strings.TrimSpace(c.RefreshToken) == "" {
		return fmt.Errorf("%w: refresh token is required", ErrInvalidCredentials)
	}
	if c.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: expiration is required", ErrInvalidCredentials)
	}
	if strings.TrimSpace(c.AccountID) == "" {
		return fmt.Errorf("%w: account ID is required", ErrInvalidCredentials)
	}
	return nil
}

func (c Credentials) Redacted() RedactedCredentials {
	return RedactedCredentials{
		AccessToken:  redactedValue(c.AccessToken),
		RefreshToken: redactedValue(c.RefreshToken),
		ExpiresAt:    c.ExpiresAt,
		AccountID:    c.AccountID,
		PlanType:     c.PlanType,
	}
}

func (c Credentials) String() string {
	return fmt.Sprintf("Credentials{AccessToken:%s RefreshToken:%s ExpiresAt:%s AccountID:%s}", redactedValue(c.AccessToken), redactedValue(c.RefreshToken), c.ExpiresAt.Format(time.RFC3339), c.AccountID)
}

func (c Credentials) GoString() string {
	return c.String()
}

func (c Credentials) Format(f fmt.State, _ rune) {
	_, _ = fmt.Fprint(f, c.String())
}

func redactedValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "[REDACTED]"
}
