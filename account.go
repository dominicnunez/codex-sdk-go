package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetAccountParams are parameters for the account/get method
type GetAccountParams struct {
	RefreshToken *bool `json:"refreshToken,omitempty"`
}

// GetAccountResponse is the response from account/get
type GetAccountResponse struct {
	Account            *AccountWrapper `json:"account"`
	RequiresOpenaiAuth bool            `json:"requiresOpenaiAuth"`
}

// AccountWrapper wraps the Account interface for JSON marshaling
type AccountWrapper struct {
	Value Account
}

// Account represents an account (apiKey or chatgpt)
type Account interface {
	accountType() string
}

// ApiKeyAccount represents an API key account
type ApiKeyAccount struct {
	Type string `json:"type"`
}

func (a *ApiKeyAccount) accountType() string { return "apiKey" }

// ChatgptAccount represents a ChatGPT account
type ChatgptAccount struct {
	Type     string   `json:"type"`
	Email    string   `json:"email"`
	PlanType PlanType `json:"planType"`
}

func (c *ChatgptAccount) accountType() string { return "chatgpt" }

// UnknownAccount represents an unrecognized account type from a newer protocol version.
type UnknownAccount struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (u *UnknownAccount) accountType() string { return u.Type }

func (u *UnknownAccount) MarshalJSON() ([]byte, error) {
	return u.Raw, nil
}

// PlanType represents the account plan tier
type PlanType string

const (
	PlanTypeFree       PlanType = "free"
	PlanTypeGo         PlanType = "go"
	PlanTypePlus       PlanType = "plus"
	PlanTypePro        PlanType = "pro"
	PlanTypeTeam       PlanType = "team"
	PlanTypeBusiness   PlanType = "business"
	PlanTypeEnterprise PlanType = "enterprise"
	PlanTypeEdu        PlanType = "edu"
	PlanTypeUnknown    PlanType = "unknown"
)

// UnmarshalJSON implements custom unmarshaling for AccountWrapper
func (a *AccountWrapper) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		a.Value = nil
		return nil
	}

	var typeCheck struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeCheck); err != nil {
		return err
	}

	switch typeCheck.Type {
	case "apiKey":
		var apiKey ApiKeyAccount
		if err := json.Unmarshal(data, &apiKey); err != nil {
			return err
		}
		a.Value = &apiKey
	case "chatgpt":
		var chatgpt ChatgptAccount
		if err := json.Unmarshal(data, &chatgpt); err != nil {
			return err
		}
		a.Value = &chatgpt
	default:
		a.Value = &UnknownAccount{Type: typeCheck.Type, Raw: append(json.RawMessage(nil), data...)}
	}

	return nil
}

// MarshalJSON implements custom marshaling for AccountWrapper
func (a *AccountWrapper) MarshalJSON() ([]byte, error) {
	if a == nil || a.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(a.Value)
}

// GetAccountRateLimitsResponse is the response from account/getRateLimits
type GetAccountRateLimitsResponse struct {
	RateLimits           RateLimitSnapshot             `json:"rateLimits"`
	RateLimitsByLimitId  map[string]*RateLimitSnapshot `json:"rateLimitsByLimitId,omitempty"`
}

// RateLimitSnapshot represents rate limit information
type RateLimitSnapshot struct {
	Credits   *CreditsSnapshot `json:"credits,omitempty"`
	LimitId   *string          `json:"limitId,omitempty"`
	LimitName *string          `json:"limitName,omitempty"`
	PlanType  *PlanType        `json:"planType,omitempty"`
	Primary   *RateLimitWindow `json:"primary,omitempty"`
	Secondary *RateLimitWindow `json:"secondary,omitempty"`
}

// CreditsSnapshot represents credit balance information
type CreditsSnapshot struct {
	Balance    *string `json:"balance,omitempty"`
	HasCredits bool    `json:"hasCredits"`
	Unlimited  bool    `json:"unlimited"`
}

// RateLimitWindow represents a rate limit time window
type RateLimitWindow struct {
	UsedPercent         int32  `json:"usedPercent"`
	ResetsAt            *int64 `json:"resetsAt,omitempty"`
	WindowDurationMins  *int64 `json:"windowDurationMins,omitempty"`
}

// LoginAccountParams is an interface for login parameter variants
type LoginAccountParams interface {
	loginParamsType() string
}

// ApiKeyLoginAccountParams represents API key login parameters
type ApiKeyLoginAccountParams struct {
	Type   string `json:"type"`
	ApiKey string `json:"apiKey"`
}

func (p *ApiKeyLoginAccountParams) loginParamsType() string { return "apiKey" }

// MarshalJSON redacts the API key to prevent accidental credential leaks
// via structured logging, debug serializers, or error payloads.
func (p *ApiKeyLoginAccountParams) MarshalJSON() ([]byte, error) {
	type redacted struct {
		Type   string `json:"type"`
		ApiKey string `json:"apiKey"`
	}
	return json.Marshal(redacted{
		Type:   p.Type,
		ApiKey: "[REDACTED]",
	})
}

func (p *ApiKeyLoginAccountParams) marshalWire() ([]byte, error) {
	type wire ApiKeyLoginAccountParams
	return json.Marshal((*wire)(p))
}

// String redacts the API key to prevent accidental credential leaks in logs.
func (p *ApiKeyLoginAccountParams) String() string {
	return fmt.Sprintf("ApiKeyLoginAccountParams{Type:%s, ApiKey:[REDACTED]}", p.Type)
}

// GoString implements fmt.GoStringer to redact credentials from %#v.
func (p *ApiKeyLoginAccountParams) GoString() string { return p.String() }

// Format implements fmt.Formatter to redact credentials from all format verbs.
func (p *ApiKeyLoginAccountParams) Format(f fmt.State, verb rune) {
	fmt.Fprint(f, p.String())
}

// ChatgptLoginAccountParams represents ChatGPT OAuth login parameters
type ChatgptLoginAccountParams struct {
	Type string `json:"type"`
}

func (p *ChatgptLoginAccountParams) loginParamsType() string { return "chatgpt" }

// ChatgptAuthTokensLoginAccountParams represents external auth token login parameters
type ChatgptAuthTokensLoginAccountParams struct {
	Type              string  `json:"type"`
	AccessToken       string  `json:"accessToken"`
	ChatgptAccountId  string  `json:"chatgptAccountId"`
	ChatgptPlanType   *string `json:"chatgptPlanType,omitempty"`
}

func (p *ChatgptAuthTokensLoginAccountParams) loginParamsType() string { return "chatgptAuthTokens" }

// MarshalJSON redacts the access token to prevent accidental credential leaks
// via structured logging, debug serializers, or error payloads.
func (p *ChatgptAuthTokensLoginAccountParams) MarshalJSON() ([]byte, error) {
	type redacted struct {
		Type             string  `json:"type"`
		AccessToken      string  `json:"accessToken"`
		ChatgptAccountId string  `json:"chatgptAccountId"`
		ChatgptPlanType  *string `json:"chatgptPlanType,omitempty"`
	}
	return json.Marshal(redacted{
		Type:             p.Type,
		AccessToken:      "[REDACTED]",
		ChatgptAccountId: p.ChatgptAccountId,
		ChatgptPlanType:  p.ChatgptPlanType,
	})
}

func (p *ChatgptAuthTokensLoginAccountParams) marshalWire() ([]byte, error) {
	type wire ChatgptAuthTokensLoginAccountParams
	return json.Marshal((*wire)(p))
}

// String redacts the access token to prevent accidental credential leaks in logs.
func (p *ChatgptAuthTokensLoginAccountParams) String() string {
	return fmt.Sprintf("ChatgptAuthTokensLoginAccountParams{Type:%s, AccessToken:[REDACTED], ChatgptAccountId:%s}", p.Type, p.ChatgptAccountId)
}

// GoString implements fmt.GoStringer to redact credentials from %#v.
func (p *ChatgptAuthTokensLoginAccountParams) GoString() string { return p.String() }

// Format implements fmt.Formatter to redact credentials from all format verbs.
func (p *ChatgptAuthTokensLoginAccountParams) Format(f fmt.State, verb rune) {
	fmt.Fprint(f, p.String())
}

// LoginAccountResponse is an interface for login response variants
type LoginAccountResponse interface {
	loginResponseType() string
}

// ApiKeyLoginAccountResponse represents API key login response
type ApiKeyLoginAccountResponse struct {
	Type string `json:"type"`
}

func (r *ApiKeyLoginAccountResponse) loginResponseType() string { return "apiKey" }

// ChatgptLoginAccountResponse represents ChatGPT OAuth login response
type ChatgptLoginAccountResponse struct {
	Type    string `json:"type"`
	AuthUrl string `json:"authUrl"`
	LoginId string `json:"loginId"`
}

func (r *ChatgptLoginAccountResponse) loginResponseType() string { return "chatgpt" }

// ChatgptAuthTokensLoginAccountResponse represents external auth token login response
type ChatgptAuthTokensLoginAccountResponse struct {
	Type string `json:"type"`
}

func (r *ChatgptAuthTokensLoginAccountResponse) loginResponseType() string {
	return "chatgptAuthTokens"
}

// UnmarshalLoginAccountResponse unmarshals a LoginAccountResponse from JSON
func UnmarshalLoginAccountResponse(data []byte) (LoginAccountResponse, error) {
	var typeCheck struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeCheck); err != nil {
		return nil, err
	}

	switch typeCheck.Type {
	case "apiKey":
		var resp ApiKeyLoginAccountResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	case "chatgpt":
		var resp ChatgptLoginAccountResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	case "chatgptAuthTokens":
		var resp ChatgptAuthTokensLoginAccountResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	default:
		return nil, fmt.Errorf("unknown login response type: %s", typeCheck.Type)
	}
}

// CancelLoginAccountParams are parameters for account/login/cancel
type CancelLoginAccountParams struct {
	LoginId string `json:"loginId"`
}

// CancelLoginAccountResponse is the response from account/login/cancel
type CancelLoginAccountResponse struct {
	Status CancelLoginAccountStatus `json:"status"`
}

// CancelLoginAccountStatus represents the cancellation status
type CancelLoginAccountStatus string

const (
	CancelLoginAccountStatusCanceled CancelLoginAccountStatus = "canceled"
	CancelLoginAccountStatusNotFound CancelLoginAccountStatus = "notFound"
)

// LogoutAccountResponse is the response from account/logout
type LogoutAccountResponse struct {
	// Empty response per spec
}

// AccountService provides account-related operations
type AccountService struct {
	client *Client
}

func newAccountService(client *Client) *AccountService {
	return &AccountService{client: client}
}

// Get retrieves the current account information
func (s *AccountService) Get(ctx context.Context, params GetAccountParams) (GetAccountResponse, error) {
	var resp GetAccountResponse
	if err := s.client.sendRequest(ctx, "account/read", params, &resp); err != nil {
		return GetAccountResponse{}, err
	}
	return resp, nil
}

// GetRateLimits retrieves the current rate limit information
func (s *AccountService) GetRateLimits(ctx context.Context) (GetAccountRateLimitsResponse, error) {
	var resp GetAccountRateLimitsResponse
	if err := s.client.sendRequest(ctx, "account/rateLimits/read", nil, &resp); err != nil {
		return GetAccountRateLimitsResponse{}, err
	}
	return resp, nil
}

// Login initiates an account login
func (s *AccountService) Login(ctx context.Context, params LoginAccountParams) (LoginAccountResponse, error) {
	respData, err := s.client.sendRequestRaw(ctx, "account/login/start", params)
	if err != nil {
		return nil, err
	}
	return UnmarshalLoginAccountResponse(respData)
}

// CancelLogin cancels an in-progress login
func (s *AccountService) CancelLogin(ctx context.Context, params CancelLoginAccountParams) (CancelLoginAccountResponse, error) {
	var resp CancelLoginAccountResponse
	if err := s.client.sendRequest(ctx, "account/login/cancel", params, &resp); err != nil {
		return CancelLoginAccountResponse{}, err
	}
	return resp, nil
}

// Logout logs out of the current account
func (s *AccountService) Logout(ctx context.Context) (LogoutAccountResponse, error) {
	var resp LogoutAccountResponse
	if err := s.client.sendRequest(ctx, "account/logout", nil, &resp); err != nil {
		return LogoutAccountResponse{}, err
	}
	return resp, nil
}
