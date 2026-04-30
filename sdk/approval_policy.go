package codex

import (
	"encoding/json"
	"errors"
	"fmt"
)

// AskForApproval represents approval policy for operations
type AskForApproval interface {
	isAskForApproval()
}

// Approval policy literals
type approvalPolicyLiteral string

func (approvalPolicyLiteral) isAskForApproval() {}

const (
	ApprovalPolicyUntrusted approvalPolicyLiteral = "untrusted"
	ApprovalPolicyOnFailure approvalPolicyLiteral = "on-failure"
	ApprovalPolicyOnRequest approvalPolicyLiteral = "on-request"
	ApprovalPolicyNever     approvalPolicyLiteral = "never"
)

// ApprovalPolicyGranular represents granular approval policy overrides.
type ApprovalPolicyGranular struct {
	Granular struct {
		MCPElicitations    bool  `json:"mcp_elicitations"`
		RequestPermissions *bool `json:"request_permissions,omitempty"`
		Rules              bool  `json:"rules"`
		SandboxApproval    bool  `json:"sandbox_approval"`
		SkillApproval      *bool `json:"skill_approval,omitempty"`
	} `json:"granular"`
}

func (ApprovalPolicyGranular) isAskForApproval() {}

func (a *ApprovalPolicyGranular) UnmarshalJSON(data []byte) error {
	var raw struct {
		Granular json.RawMessage `json:"granular"`
	}
	if err := unmarshalInboundObject(data, &raw, []string{"granular"}, []string{"granular"}); err != nil {
		return err
	}
	if err := validateRequiredObjectFields(raw.Granular, "mcp_elicitations", "rules", "sandbox_approval"); err != nil {
		return err
	}
	type wire ApprovalPolicyGranular
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*a = ApprovalPolicyGranular(decoded)
	return nil
}

// UnknownAskForApproval represents an unrecognized approval policy shape from a newer protocol version.
type UnknownAskForApproval struct {
	Raw json.RawMessage `json:"-"`
}

func (UnknownAskForApproval) isAskForApproval() {}

func (u UnknownAskForApproval) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// AskForApprovalWrapper wraps AskForApproval for JSON marshaling
type AskForApprovalWrapper struct {
	Value AskForApproval
}

// UnmarshalJSON for AskForApprovalWrapper handles the union type
func (a *AskForApprovalWrapper) UnmarshalJSON(data []byte) error {
	// Try string literal first
	var literal string
	if err := json.Unmarshal(data, &literal); err == nil {
		a.Value = approvalPolicyLiteral(literal)
		return nil
	}

	// Try granular object — validate that the discriminating "granular" key
	// is present, otherwise any JSON object would silently match
	var rawObj map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawObj); err == nil {
		if _, hasKey := rawObj["granular"]; hasKey {
			var granular ApprovalPolicyGranular
			if err := json.Unmarshal(data, &granular); err != nil {
				return fmt.Errorf("unmarshal approval policy granular: %w", err)
			}
			a.Value = granular
			return nil
		}
		return errors.New("approval policy: missing discriminator")
	}

	return fmt.Errorf("unable to unmarshal approval policy from: %.200s", data)
}

// MarshalJSON for AskForApprovalWrapper
func (a AskForApprovalWrapper) MarshalJSON() ([]byte, error) {
	value := normalizeAskForApproval(a.Value)
	if value == nil {
		return []byte("null"), nil
	}
	switch v := value.(type) {
	case approvalPolicyLiteral:
		return json.Marshal(string(v))
	case ApprovalPolicyGranular:
		return json.Marshal(v)
	case UnknownAskForApproval:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown AskForApproval type: %T", v)
	}
}

func normalizeAskForApproval(value AskForApproval) AskForApproval {
	switch v := value.(type) {
	case nil:
		return nil
	case *approvalPolicyLiteral:
		if v == nil {
			return nil
		}
		return *v
	case *ApprovalPolicyGranular:
		if v == nil {
			return nil
		}
		return *v
	case *UnknownAskForApproval:
		if v == nil {
			return nil
		}
		return *v
	default:
		return value
	}
}
