package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// GetAccountParams are parameters for the account/read method.
type GetAccountParams struct {
	RefreshToken *bool `json:"refreshToken,omitempty"`
}

// GetAccountResponse is the response from account/read.
type GetAccountResponse struct {
	Account            *AccountWrapper `json:"account"`
	RequiresOpenaiAuth bool            `json:"requiresOpenaiAuth"`
}

func (r *GetAccountResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "requiresOpenaiAuth"); err != nil {
		return err
	}
	type wire GetAccountResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = GetAccountResponse(decoded)
	return nil
}

// AccountWrapper wraps the Account interface for JSON marshaling
type AccountWrapper struct {
	Value Account
}

// Account represents an account (apiKey or chatgpt)
type Account interface {
	isAccount()
}

// ApiKeyAccount represents an API key account
type ApiKeyAccount struct {
	Type string `json:"type"`
}

func (*ApiKeyAccount) isAccount() {}

func (a *ApiKeyAccount) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "type"); err != nil {
		return err
	}
	type wire ApiKeyAccount
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*a = ApiKeyAccount(decoded)
	return nil
}

// MarshalJSON injects the type discriminator, matching the pattern used by all
// other discriminated union variants in the codebase.
func (a *ApiKeyAccount) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
	}{Type: "apiKey"})
}

// ChatgptAccount represents a ChatGPT account
type ChatgptAccount struct {
	Type     string   `json:"type"`
	Email    string   `json:"email"`
	PlanType PlanType `json:"planType"`
}

func (*ChatgptAccount) isAccount() {}

func (c *ChatgptAccount) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "type", "email", "planType"); err != nil {
		return err
	}
	type wire ChatgptAccount
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validatePlanTypeField("account.planType", decoded.PlanType); err != nil {
		return err
	}
	*c = ChatgptAccount(decoded)
	return nil
}

// MarshalJSON injects the type discriminator, matching the pattern used by all
// other discriminated union variants in the codebase.
func (c *ChatgptAccount) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type     string   `json:"type"`
		Email    string   `json:"email"`
		PlanType PlanType `json:"planType"`
	}{Type: "chatgpt", Email: c.Email, PlanType: c.PlanType})
}

// UnknownAccount represents an unrecognized account type from a newer protocol version.
type UnknownAccount struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (*UnknownAccount) isAccount() {}

func (u *UnknownAccount) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// PlanType represents the account plan tier
type PlanType string

const (
	PlanTypeFree                        PlanType = "free"
	PlanTypeGo                          PlanType = "go"
	PlanTypePlus                        PlanType = "plus"
	PlanTypePro                         PlanType = "pro"
	PlanTypeProLite                     PlanType = "prolite"
	PlanTypeTeam                        PlanType = "team"
	PlanTypeBusiness                    PlanType = "business"
	PlanTypeEnterprise                  PlanType = "enterprise"
	PlanTypeEdu                         PlanType = "edu"
	PlanTypeSelfServeBusinessUsageBased PlanType = "self_serve_business_usage_based"
	PlanTypeEnterpriseCBPUsageBased     PlanType = "enterprise_cbp_usage_based"
	PlanTypeUnknown                     PlanType = "unknown"
)

var validPlanTypes = map[PlanType]struct{}{
	PlanTypeFree:                        {},
	PlanTypeGo:                          {},
	PlanTypePlus:                        {},
	PlanTypePro:                         {},
	PlanTypeProLite:                     {},
	PlanTypeTeam:                        {},
	PlanTypeBusiness:                    {},
	PlanTypeEnterprise:                  {},
	PlanTypeEdu:                         {},
	PlanTypeSelfServeBusinessUsageBased: {},
	PlanTypeEnterpriseCBPUsageBased:     {},
	PlanTypeUnknown:                     {},
}

func validatePlanTypeField(field string, value PlanType) error {
	return validateEnumValue(field, value, validPlanTypes)
}

func validateOptionalPlanTypeField(field string, value *PlanType) error {
	return validateOptionalEnumValue(field, value, validPlanTypes)
}

// UnmarshalJSON implements custom unmarshaling for AccountWrapper
func (a *AccountWrapper) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		a.Value = nil
		return nil
	}

	if err := validateRequiredObjectFields(data, "type"); err != nil {
		return err
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

// GetAccountRateLimitsResponse is the response from account/rateLimits/read.
type GetAccountRateLimitsResponse struct {
	RateLimits          RateLimitSnapshot             `json:"rateLimits"`
	RateLimitsByLimitId map[string]*RateLimitSnapshot `json:"rateLimitsByLimitId,omitempty"`
}

func (r *GetAccountRateLimitsResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "rateLimits"); err != nil {
		return err
	}
	type wire GetAccountRateLimitsResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = GetAccountRateLimitsResponse(decoded)
	return nil
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

func (r *RateLimitSnapshot) UnmarshalJSON(data []byte) error {
	type wire RateLimitSnapshot
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validateOptionalPlanTypeField("rateLimits.planType", decoded.PlanType); err != nil {
		return err
	}
	*r = RateLimitSnapshot(decoded)
	return nil
}

// CreditsSnapshot represents credit balance information
type CreditsSnapshot struct {
	Balance    *string `json:"balance,omitempty"`
	HasCredits bool    `json:"hasCredits"`
	Unlimited  bool    `json:"unlimited"`
}

func (c *CreditsSnapshot) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "hasCredits", "unlimited"); err != nil {
		return err
	}
	type wire CreditsSnapshot
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = CreditsSnapshot(decoded)
	return nil
}

// RateLimitWindow represents a rate limit time window
type RateLimitWindow struct {
	UsedPercent        int32  `json:"usedPercent"`
	ResetsAt           *int64 `json:"resetsAt,omitempty"`
	WindowDurationMins *int64 `json:"windowDurationMins,omitempty"`
}

// AddCreditsNudgeCreditType identifies which credit category should be nudged.
type AddCreditsNudgeCreditType string

const (
	AddCreditsNudgeCreditTypeCredits    AddCreditsNudgeCreditType = "credits"
	AddCreditsNudgeCreditTypeUsageLimit AddCreditsNudgeCreditType = "usage_limit"
)

var validAddCreditsNudgeCreditTypes = map[AddCreditsNudgeCreditType]struct{}{
	AddCreditsNudgeCreditTypeCredits:    {},
	AddCreditsNudgeCreditTypeUsageLimit: {},
}

func validateAddCreditsNudgeCreditTypeField(field string, value AddCreditsNudgeCreditType) error {
	return validateEnumValue(field, value, validAddCreditsNudgeCreditTypes)
}

// SendAddCreditsNudgeEmailParams sends an add-credits nudge email.
type SendAddCreditsNudgeEmailParams struct {
	CreditType AddCreditsNudgeCreditType `json:"creditType"`
}

func (p SendAddCreditsNudgeEmailParams) prepareRequest() (interface{}, error) {
	if err := validateAddCreditsNudgeCreditTypeField("creditType", p.CreditType); err != nil {
		return nil, invalidParamsError("%v", err)
	}
	return p, nil
}

// AddCreditsNudgeEmailStatus is the result of sending an add-credits nudge email.
type AddCreditsNudgeEmailStatus string

const (
	AddCreditsNudgeEmailStatusSent           AddCreditsNudgeEmailStatus = "sent"
	AddCreditsNudgeEmailStatusCooldownActive AddCreditsNudgeEmailStatus = "cooldown_active"
)

var validAddCreditsNudgeEmailStatuses = map[AddCreditsNudgeEmailStatus]struct{}{
	AddCreditsNudgeEmailStatusSent:           {},
	AddCreditsNudgeEmailStatusCooldownActive: {},
}

func validateAddCreditsNudgeEmailStatusField(field string, value AddCreditsNudgeEmailStatus) error {
	return validateEnumValue(field, value, validAddCreditsNudgeEmailStatuses)
}

// SendAddCreditsNudgeEmailResponse is the response from account/sendAddCreditsNudgeEmail.
type SendAddCreditsNudgeEmailResponse struct {
	Status AddCreditsNudgeEmailStatus `json:"status"`
}

func (r *SendAddCreditsNudgeEmailResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "status"); err != nil {
		return err
	}
	type wire SendAddCreditsNudgeEmailResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validateAddCreditsNudgeEmailStatusField("addCreditsNudge.status", decoded.Status); err != nil {
		return err
	}
	*r = SendAddCreditsNudgeEmailResponse(decoded)
	return nil
}

func (w *RateLimitWindow) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "usedPercent"); err != nil {
		return err
	}
	type wire RateLimitWindow
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*w = RateLimitWindow(decoded)
	return nil
}

// Login account param type discriminators (spec-defined).
const (
	loginTypeApiKey            = "apiKey"
	loginTypeChatgpt           = "chatgpt"
	loginTypeChatgptAuthTokens = "chatgptAuthTokens"
)

const (
	loginFieldApiKey           = "apiKey"
	loginFieldAccessToken      = "accessToken"
	loginFieldChatgptAccountID = "chatgptAccountId"
)

var errNilLoginAccountParams = errors.New("login params cannot be nil")
var errEmptyLoginAccountResponse = errors.New("login response must be a non-null object")
var errMissingLoginAccountResponseType = errors.New("login response missing type")

// LoginAccountParams is an interface for login parameter variants
type LoginAccountParams interface {
	isLoginAccountParams()
}

// ApiKeyLoginAccountParams represents API key login parameters
type ApiKeyLoginAccountParams struct {
	Type   string `json:"type"`
	ApiKey string `json:"apiKey"`
}

func (*ApiKeyLoginAccountParams) isLoginAccountParams() {}

// MarshalJSON redacts the API key to prevent accidental credential leaks
// via structured logging, debug serializers, or error payloads.
func (p *ApiKeyLoginAccountParams) MarshalJSON() ([]byte, error) {
	type redacted struct {
		Type   string `json:"type"`
		ApiKey string `json:"apiKey"`
	}
	return json.Marshal(redacted{
		Type:   loginTypeApiKey,
		ApiKey: "[REDACTED]",
	})
}

func (p *ApiKeyLoginAccountParams) marshalWire() ([]byte, error) {
	if p == nil {
		return nil, errNilLoginAccountParams
	}
	if err := validateRequiredNonEmptyStringField(loginFieldApiKey, p.ApiKey); err != nil {
		return nil, err
	}
	w := ApiKeyLoginAccountParams{
		Type:   loginTypeApiKey,
		ApiKey: p.ApiKey,
	}
	return json.Marshal(w)
}

// String redacts the API key to prevent accidental credential leaks in logs.
func (p *ApiKeyLoginAccountParams) String() string {
	return fmt.Sprintf("ApiKeyLoginAccountParams{Type:%s, ApiKey:[REDACTED]}", loginTypeApiKey)
}

// GoString implements fmt.GoStringer to redact credentials from %#v.
func (p *ApiKeyLoginAccountParams) GoString() string { return p.String() }

// Format implements fmt.Formatter to redact credentials from all format verbs.
func (p *ApiKeyLoginAccountParams) Format(f fmt.State, verb rune) {
	_, _ = fmt.Fprint(f, p.String())
}

// ChatgptLoginAccountParams represents ChatGPT OAuth login parameters
type ChatgptLoginAccountParams struct {
	Type string `json:"type"`
}

func (*ChatgptLoginAccountParams) isLoginAccountParams() {}

func (p *ChatgptLoginAccountParams) marshalWire() ([]byte, error) {
	if p == nil {
		return nil, errNilLoginAccountParams
	}
	w := ChatgptLoginAccountParams{
		Type: loginTypeChatgpt,
	}
	return json.Marshal(w)
}

// ChatgptAuthTokensLoginAccountParams represents external auth token login parameters
type ChatgptAuthTokensLoginAccountParams struct {
	Type             string  `json:"type"`
	AccessToken      string  `json:"accessToken"`
	ChatgptAccountId string  `json:"chatgptAccountId"`
	ChatgptPlanType  *string `json:"chatgptPlanType,omitempty"`
}

func (*ChatgptAuthTokensLoginAccountParams) isLoginAccountParams() {}

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
		Type:             loginTypeChatgptAuthTokens,
		AccessToken:      "[REDACTED]",
		ChatgptAccountId: p.ChatgptAccountId,
		ChatgptPlanType:  p.ChatgptPlanType,
	})
}

func (p *ChatgptAuthTokensLoginAccountParams) marshalWire() ([]byte, error) {
	if p == nil {
		return nil, errNilLoginAccountParams
	}
	if err := validateRequiredNonEmptyStringField(loginFieldAccessToken, p.AccessToken); err != nil {
		return nil, err
	}
	if err := validateRequiredNonEmptyStringField(loginFieldChatgptAccountID, p.ChatgptAccountId); err != nil {
		return nil, err
	}
	w := ChatgptAuthTokensLoginAccountParams{
		Type:             loginTypeChatgptAuthTokens,
		AccessToken:      p.AccessToken,
		ChatgptAccountId: p.ChatgptAccountId,
		ChatgptPlanType:  p.ChatgptPlanType,
	}
	return json.Marshal(w)
}

// String redacts the access token to prevent accidental credential leaks in logs.
func (p *ChatgptAuthTokensLoginAccountParams) String() string {
	return fmt.Sprintf("ChatgptAuthTokensLoginAccountParams{Type:%s, AccessToken:[REDACTED], ChatgptAccountId:%s}", loginTypeChatgptAuthTokens, p.ChatgptAccountId)
}

// GoString implements fmt.GoStringer to redact credentials from %#v.
func (p *ChatgptAuthTokensLoginAccountParams) GoString() string { return p.String() }

// Format implements fmt.Formatter to redact credentials from all format verbs.
func (p *ChatgptAuthTokensLoginAccountParams) Format(f fmt.State, verb rune) {
	_, _ = fmt.Fprint(f, p.String())
}

// LoginAccountResponse is an interface for login response variants
type LoginAccountResponse interface {
	isLoginAccountResponse()
}

// ApiKeyLoginAccountResponse represents API key login response
type ApiKeyLoginAccountResponse struct {
	Type string `json:"type"`
}

func (*ApiKeyLoginAccountResponse) isLoginAccountResponse() {}

func (r *ApiKeyLoginAccountResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "type"); err != nil {
		return err
	}
	type wire ApiKeyLoginAccountResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ApiKeyLoginAccountResponse(decoded)
	return nil
}

// ChatgptLoginAccountResponse represents ChatGPT OAuth login response
type ChatgptLoginAccountResponse struct {
	Type    string `json:"type"`
	AuthUrl string `json:"authUrl"`
	LoginId string `json:"loginId"`
}

func (*ChatgptLoginAccountResponse) isLoginAccountResponse() {}

func (r *ChatgptLoginAccountResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "type", "authUrl", "loginId"); err != nil {
		return err
	}
	type wire ChatgptLoginAccountResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ChatgptLoginAccountResponse(decoded)
	return nil
}

// ChatgptAuthTokensLoginAccountResponse represents external auth token login response
type ChatgptAuthTokensLoginAccountResponse struct {
	Type string `json:"type"`
}

func (*ChatgptAuthTokensLoginAccountResponse) isLoginAccountResponse() {}

func (r *ChatgptAuthTokensLoginAccountResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "type"); err != nil {
		return err
	}
	type wire ChatgptAuthTokensLoginAccountResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ChatgptAuthTokensLoginAccountResponse(decoded)
	return nil
}

// UnknownLoginAccountResponse represents an unrecognized login response type from a newer protocol version.
type UnknownLoginAccountResponse struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (*UnknownLoginAccountResponse) isLoginAccountResponse() {}

func (u *UnknownLoginAccountResponse) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// UnmarshalLoginAccountResponse unmarshals a LoginAccountResponse from JSON
func UnmarshalLoginAccountResponse(data []byte) (LoginAccountResponse, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, errEmptyLoginAccountResponse
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, err
	}
	rawType, ok := envelope["type"]
	if !ok {
		return nil, errMissingLoginAccountResponseType
	}

	var typeCheck string
	if err := json.Unmarshal(rawType, &typeCheck); err != nil {
		return nil, fmt.Errorf("invalid login response type: %w", err)
	}

	switch typeCheck {
	case "apiKey":
		var resp ApiKeyLoginAccountResponse
		if err := json.Unmarshal(trimmed, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	case "chatgpt":
		var resp ChatgptLoginAccountResponse
		if err := json.Unmarshal(trimmed, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	case "chatgptAuthTokens":
		var resp ChatgptAuthTokensLoginAccountResponse
		if err := json.Unmarshal(trimmed, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	default:
		return &UnknownLoginAccountResponse{Type: typeCheck, Raw: append(json.RawMessage(nil), trimmed...)}, nil
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

func (r *CancelLoginAccountResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "status"); err != nil {
		return err
	}
	type wire CancelLoginAccountResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	switch decoded.Status {
	case CancelLoginAccountStatusCanceled, CancelLoginAccountStatusNotFound:
	default:
		return fmt.Errorf("invalid status %q", decoded.Status)
	}
	*r = CancelLoginAccountResponse(decoded)
	return nil
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
	if err := s.client.sendRequest(ctx, methodAccountRead, params, &resp); err != nil {
		return GetAccountResponse{}, err
	}
	return resp, nil
}

// GetRateLimits retrieves the current rate limit information
func (s *AccountService) GetRateLimits(ctx context.Context) (GetAccountRateLimitsResponse, error) {
	var resp GetAccountRateLimitsResponse
	if err := s.client.sendRequest(ctx, methodAccountRateLimitsRead, nil, &resp); err != nil {
		return GetAccountRateLimitsResponse{}, err
	}
	return resp, nil
}

// SendAddCreditsNudgeEmail sends an add-credits nudge email.
func (s *AccountService) SendAddCreditsNudgeEmail(ctx context.Context, params SendAddCreditsNudgeEmailParams) (SendAddCreditsNudgeEmailResponse, error) {
	var resp SendAddCreditsNudgeEmailResponse
	if err := s.client.sendRequest(ctx, methodAccountSendAddCreditsNudgeEmail, params, &resp); err != nil {
		return SendAddCreditsNudgeEmailResponse{}, err
	}
	return resp, nil
}

// Login initiates an account login
func (s *AccountService) Login(ctx context.Context, params LoginAccountParams) (LoginAccountResponse, error) {
	if isNilLoginParams(params) {
		return nil, errNilLoginAccountParams
	}
	respData, err := s.client.sendRequestRaw(ctx, methodAccountLoginStart, params)
	if err != nil {
		return nil, err
	}
	resp, err := UnmarshalLoginAccountResponse(respData)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", methodAccountLoginStart, err)
	}
	return resp, nil
}

func isNilLoginParams(params LoginAccountParams) bool {
	if params == nil {
		return true
	}
	rv := reflect.ValueOf(params)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// CancelLogin cancels an in-progress login
func (s *AccountService) CancelLogin(ctx context.Context, params CancelLoginAccountParams) (CancelLoginAccountResponse, error) {
	var resp CancelLoginAccountResponse
	if err := s.client.sendRequest(ctx, methodAccountLoginCancel, params, &resp); err != nil {
		return CancelLoginAccountResponse{}, err
	}
	return resp, nil
}

// Logout logs out of the current account
func (s *AccountService) Logout(ctx context.Context) (LogoutAccountResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodAccountLogout, nil); err != nil {
		return LogoutAccountResponse{}, err
	}
	return LogoutAccountResponse{}, nil
}
