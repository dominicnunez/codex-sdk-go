package auth

import (
	"fmt"
	"strings"
)

const LoginTypeChatGPTAuthTokens = "chatgptAuthTokens"

type AuthTokensLoginParams struct {
	Type             string  `json:"type"`
	AccessToken      string  `json:"accessToken"`
	ChatGPTAccountID string  `json:"chatgptAccountId"`
	ChatGPTPlanType  *string `json:"chatgptPlanType,omitempty"`
}

type AuthTokensRefreshResponse struct {
	AccessToken      string  `json:"accessToken"`
	ChatGPTAccountID string  `json:"chatgptAccountId"`
	ChatGPTPlanType  *string `json:"chatgptPlanType"`
}

func NewAuthTokensLoginParams(creds Credentials) (AuthTokensLoginParams, error) {
	if err := creds.Validate(); err != nil {
		return AuthTokensLoginParams{}, err
	}
	return AuthTokensLoginParams{
		Type:             LoginTypeChatGPTAuthTokens,
		AccessToken:      creds.AccessToken,
		ChatGPTAccountID: creds.AccountID,
		ChatGPTPlanType:  creds.PlanType,
	}, nil
}

func (p AuthTokensLoginParams) Validate() error {
	if p.Type != LoginTypeChatGPTAuthTokens {
		return fmt.Errorf("invalid Codex login type: %s", p.Type)
	}
	if strings.TrimSpace(p.AccessToken) == "" {
		return fmt.Errorf("access token is required")
	}
	if strings.TrimSpace(p.ChatGPTAccountID) == "" {
		return fmt.Errorf("ChatGPT account ID is required")
	}
	return nil
}

func (p AuthTokensLoginParams) String() string {
	return fmt.Sprintf("AuthTokensLoginParams{Type:%s AccessToken:%s ChatGPTAccountID:%s}", p.Type, redactedValue(p.AccessToken), p.ChatGPTAccountID)
}

func (p AuthTokensLoginParams) GoString() string {
	return p.String()
}

func (p AuthTokensLoginParams) Format(f fmt.State, _ rune) {
	_, _ = fmt.Fprint(f, p.String())
}

func NewAuthTokensRefreshResponse(creds Credentials) (AuthTokensRefreshResponse, error) {
	if err := creds.Validate(); err != nil {
		return AuthTokensRefreshResponse{}, err
	}
	return AuthTokensRefreshResponse{
		AccessToken:      creds.AccessToken,
		ChatGPTAccountID: creds.AccountID,
		ChatGPTPlanType:  creds.PlanType,
	}, nil
}

func (r AuthTokensRefreshResponse) String() string {
	return fmt.Sprintf("AuthTokensRefreshResponse{AccessToken:%s ChatGPTAccountID:%s}", redactedValue(r.AccessToken), r.ChatGPTAccountID)
}

func (r AuthTokensRefreshResponse) GoString() string {
	return r.String()
}

func (r AuthTokensRefreshResponse) Format(f fmt.State, _ rune) {
	_, _ = fmt.Fprint(f, r.String())
}
