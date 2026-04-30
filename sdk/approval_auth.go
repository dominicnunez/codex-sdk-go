package codex

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ChatgptAuthTokensRefreshParams represents parameters for ChatGPT auth token refresh.
type ChatgptAuthTokensRefreshParams struct {
	Reason            ChatgptAuthTokensRefreshReason `json:"reason"`
	PreviousAccountID *string                        `json:"previousAccountId,omitempty"`
}

func (p *ChatgptAuthTokensRefreshParams) UnmarshalJSON(data []byte) error {
	type wire ChatgptAuthTokensRefreshParams
	var decoded wire
	required := []string{"reason"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	if err := validateChatgptAuthTokensRefreshReasonField("reason", decoded.Reason); err != nil {
		return err
	}
	*p = ChatgptAuthTokensRefreshParams(decoded)
	return nil
}

// ChatgptAuthTokensRefreshResponse represents the response containing new auth tokens.
type ChatgptAuthTokensRefreshResponse struct {
	AccessToken      string  `json:"accessToken"`
	ChatgptAccountID string  `json:"chatgptAccountId"`
	ChatgptPlanType  *string `json:"chatgptPlanType,omitempty"`
}

func (r ChatgptAuthTokensRefreshResponse) validate() error {
	switch {
	case r.AccessToken == "":
		return errors.New("missing accessToken")
	case r.ChatgptAccountID == "":
		return errors.New("missing chatgptAccountId")
	default:
		return nil
	}
}

// MarshalJSON redacts the access token to prevent accidental credential leaks
// via structured logging, debug serializers, or error payloads.
// Use marshalWire for intentional wire-protocol serialization.
func (r ChatgptAuthTokensRefreshResponse) MarshalJSON() ([]byte, error) {
	type redacted struct {
		AccessToken      string  `json:"accessToken"`
		ChatgptAccountID string  `json:"chatgptAccountId"`
		ChatgptPlanType  *string `json:"chatgptPlanType,omitempty"`
	}
	return json.Marshal(redacted{
		AccessToken:      "[REDACTED]",
		ChatgptAccountID: r.ChatgptAccountID,
		ChatgptPlanType:  r.ChatgptPlanType,
	})
}

func (r ChatgptAuthTokensRefreshResponse) marshalWire() ([]byte, error) {
	type wire ChatgptAuthTokensRefreshResponse
	w := wire(r)
	return json.Marshal(w)
}

// String redacts the access token to prevent accidental credential leaks in logs.
func (r ChatgptAuthTokensRefreshResponse) String() string {
	return fmt.Sprintf("ChatgptAuthTokensRefreshResponse{AccessToken:[REDACTED], ChatgptAccountID:%s}", r.ChatgptAccountID)
}

// GoString implements fmt.GoStringer to redact credentials from %#v.
func (r ChatgptAuthTokensRefreshResponse) GoString() string { return r.String() }

// Format implements fmt.Formatter to redact credentials from all format verbs.
func (r ChatgptAuthTokensRefreshResponse) Format(f fmt.State, verb rune) {
	_, _ = fmt.Fprint(f, r.String())
}
