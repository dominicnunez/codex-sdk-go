package codex

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ApplyPatchApprovalParams represents the parameters for a server→client applyPatchApproval request.
//
// Deprecated: Use FileChangeRequestApprovalParams instead.
type ApplyPatchApprovalParams struct {
	CallID         string                       `json:"callId"`
	ConversationID string                       `json:"conversationId"`
	FileChanges    map[string]FileChangeWrapper `json:"fileChanges"`
	GrantRoot      *string                      `json:"grantRoot,omitempty"`
	Reason         *string                      `json:"reason,omitempty"`
}

func (p *ApplyPatchApprovalParams) UnmarshalJSON(data []byte) error {
	type wire ApplyPatchApprovalParams
	var decoded wire
	required := []string{"callId", "conversationId", "fileChanges"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	var err error
	decoded.GrantRoot, err = validateInboundAbsolutePathPointerField("grantRoot", decoded.GrantRoot)
	if err != nil {
		return err
	}
	*p = ApplyPatchApprovalParams(decoded)
	return nil
}

// FileChange is a discriminated union for file changes (add/delete/update).
type FileChange interface {
	fileChange()
}

const (
	fileChangeTypeAdd    = "add"
	fileChangeTypeDelete = "delete"
	fileChangeTypeUpdate = "update"
)

// UnknownFileChange represents an unrecognized file change type from a newer protocol version.
type UnknownFileChange struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (u *UnknownFileChange) fileChange() {}

func (u *UnknownFileChange) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// FileChangeWrapper wraps a FileChange for JSON marshaling/unmarshaling.
type FileChangeWrapper struct {
	Value FileChange
}

// AddFileChange represents adding a new file.
type AddFileChange struct {
	Content string `json:"content"`
}

func (a *AddFileChange) fileChange() {}

func (a *AddFileChange) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}{
		Type:    fileChangeTypeAdd,
		Content: a.Content,
	})
}

// DeleteFileChange represents deleting a file.
type DeleteFileChange struct {
	Content string `json:"content"`
}

func (d *DeleteFileChange) fileChange() {}

func (d *DeleteFileChange) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}{
		Type:    fileChangeTypeDelete,
		Content: d.Content,
	})
}

// UpdateFileChange represents updating an existing file with a unified diff.
type UpdateFileChange struct {
	UnifiedDiff string  `json:"unified_diff"`
	MovePath    *string `json:"move_path,omitempty"`
}

func (u *UpdateFileChange) fileChange() {}

func (u *UpdateFileChange) MarshalJSON() ([]byte, error) {
	type updateJSON struct {
		Type        string  `json:"type"`
		UnifiedDiff string  `json:"unified_diff"`
		MovePath    *string `json:"move_path,omitempty"`
	}
	return json.Marshal(updateJSON{
		Type:        fileChangeTypeUpdate,
		UnifiedDiff: u.UnifiedDiff,
		MovePath:    u.MovePath,
	})
}

// UnmarshalJSON implements custom unmarshaling for FileChangeWrapper.
func (w *FileChangeWrapper) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "type"); err != nil {
		return err
	}

	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw.Type {
	case fileChangeTypeAdd:
		if err := validateRequiredObjectFields(data, "type", "content"); err != nil {
			return err
		}
		var add AddFileChange
		if err := json.Unmarshal(data, &add); err != nil {
			return err
		}
		w.Value = &add
	case fileChangeTypeDelete:
		if err := validateRequiredObjectFields(data, "type", "content"); err != nil {
			return err
		}
		var del DeleteFileChange
		if err := json.Unmarshal(data, &del); err != nil {
			return err
		}
		w.Value = &del
	case fileChangeTypeUpdate:
		if err := validateRequiredObjectFields(data, "type", "unified_diff"); err != nil {
			return err
		}
		var upd UpdateFileChange
		if err := json.Unmarshal(data, &upd); err != nil {
			return err
		}
		w.Value = &upd
	default:
		w.Value = &UnknownFileChange{Type: raw.Type, Raw: append(json.RawMessage(nil), data...)}
	}

	return nil
}

// MarshalJSON implements custom marshaling for FileChangeWrapper.
func (w FileChangeWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

// ApplyPatchApprovalResponse represents the response to an applyPatchApproval request.
//
// Deprecated: Use FileChangeRequestApprovalResponse instead.
type ApplyPatchApprovalResponse struct {
	Decision ReviewDecisionWrapper `json:"decision"`
}

func (r ApplyPatchApprovalResponse) validate() error {
	return validateReviewDecisionWrapper(r.Decision)
}

// ReviewDecision is the user's decision on the patch approval request.
// Can be: "approved", "approved_for_session", "denied", "abort",
// or objects for amendment decisions.
type ReviewDecisionWrapper struct {
	Value interface{} // string or object
}

// ApprovedExecpolicyAmendmentDecision represents approval with execpolicy amendment.
type ApprovedExecpolicyAmendmentDecision struct {
	ProposedExecpolicyAmendment []string `json:"proposed_execpolicy_amendment"`
}

// NetworkPolicyAmendmentDecision represents approval with network policy amendment.
type NetworkPolicyAmendmentDecision struct {
	NetworkPolicyAmendment NetworkPolicyAmendment `json:"network_policy_amendment"`
}

// NetworkPolicyAmendment defines a network policy rule.
type NetworkPolicyAmendment struct {
	Action NetworkPolicyRuleAction `json:"action"`
	Host   string                  `json:"host"`
}

// UnknownReviewDecision represents an unrecognized review decision variant from a newer protocol version.
type UnknownReviewDecision struct {
	Raw json.RawMessage `json:"-"`
}

func (u UnknownReviewDecision) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// UnmarshalJSON implements custom unmarshaling for ReviewDecisionWrapper.
// The network_policy_amendment variant uses double-nested JSON to match the spec:
//
//	{"network_policy_amendment": {"network_policy_amendment": {...}}}
//
// See specs/ApplyPatchApprovalResponse.json and ExecCommandApprovalResponse.json.
func (w *ReviewDecisionWrapper) UnmarshalJSON(data []byte) error {
	// Try string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		w.Value = str
		return nil
	}

	// Dispatch on which key is present in the JSON object
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("unable to unmarshal ReviewDecision: %w", err)
	}

	if raw, ok := keys["approved_execpolicy_amendment"]; ok {
		var inner ApprovedExecpolicyAmendmentDecision
		if err := json.Unmarshal(raw, &inner); err != nil {
			return fmt.Errorf("unable to unmarshal approved_execpolicy_amendment: %w", err)
		}
		w.Value = inner
		return nil
	}

	if raw, ok := keys["network_policy_amendment"]; ok {
		var inner NetworkPolicyAmendmentDecision
		if err := json.Unmarshal(raw, &inner); err != nil {
			return fmt.Errorf("unable to unmarshal network_policy_amendment: %w", err)
		}
		w.Value = inner
		return nil
	}

	w.Value = UnknownReviewDecision{Raw: append(json.RawMessage(nil), data...)}
	return nil
}

// MarshalJSON implements custom marshaling for ReviewDecisionWrapper.
// The double-nested anonymous structs match the spec schema structure:
//   - ApplyPatchApprovalResponse.json / ExecCommandApprovalResponse.json define
//     {"approved_execpolicy_amendment": {"proposed_execpolicy_amendment": [...]}}
//   - Both specs define {"network_policy_amendment": {"network_policy_amendment": {...}}}
func (w ReviewDecisionWrapper) MarshalJSON() ([]byte, error) {
	switch v := normalizeReviewDecisionValue(w.Value).(type) {
	case nil:
		return []byte("null"), nil
	case string:
		return json.Marshal(v)
	case ApprovedExecpolicyAmendmentDecision:
		return json.Marshal(struct {
			ApprovedExecpolicyAmendment struct {
				ProposedExecpolicyAmendment []string `json:"proposed_execpolicy_amendment"`
			} `json:"approved_execpolicy_amendment"`
		}{
			ApprovedExecpolicyAmendment: struct {
				ProposedExecpolicyAmendment []string `json:"proposed_execpolicy_amendment"`
			}{
				ProposedExecpolicyAmendment: v.ProposedExecpolicyAmendment,
			},
		})
	case NetworkPolicyAmendmentDecision:
		return json.Marshal(struct {
			NetworkPolicyAmendment struct {
				NetworkPolicyAmendment NetworkPolicyAmendment `json:"network_policy_amendment"`
			} `json:"network_policy_amendment"`
		}{
			NetworkPolicyAmendment: struct {
				NetworkPolicyAmendment NetworkPolicyAmendment `json:"network_policy_amendment"`
			}{
				NetworkPolicyAmendment: v.NetworkPolicyAmendment,
			},
		})
	case UnknownReviewDecision:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown decision type: %T", v)
	}
}

func normalizeReviewDecisionValue(value interface{}) interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case *string:
		if v == nil {
			return nil
		}
		return *v
	case *ApprovedExecpolicyAmendmentDecision:
		if v == nil {
			return nil
		}
		return *v
	case *NetworkPolicyAmendmentDecision:
		if v == nil {
			return nil
		}
		return *v
	case *UnknownReviewDecision:
		if v == nil {
			return nil
		}
		return *v
	default:
		return value
	}
}

func validateReviewDecisionWrapper(decision ReviewDecisionWrapper) error {
	switch value := normalizeReviewDecisionValue(decision.Value).(type) {
	case nil:
		return errors.New("missing decision")
	case string:
		switch value {
		case "approved", "approved_for_session", "denied", "abort":
			return nil
		default:
			return fmt.Errorf("invalid decision %q", value)
		}
	case ApprovedExecpolicyAmendmentDecision:
		if value.ProposedExecpolicyAmendment == nil {
			return errors.New("approved_execpolicy_amendment.proposed_execpolicy_amendment: missing array")
		}
		return nil
	case NetworkPolicyAmendmentDecision:
		return validateNetworkPolicyAmendment(value.NetworkPolicyAmendment)
	case UnknownReviewDecision:
		return errors.New("missing decision")
	default:
		return fmt.Errorf("invalid decision type %T", decision.Value)
	}
}

// FileChangeRequestApprovalParams represents parameters for file change approval.
type FileChangeRequestApprovalParams struct {
	ItemID    string  `json:"itemId"`
	ThreadID  string  `json:"threadId"`
	TurnID    string  `json:"turnId"`
	GrantRoot *string `json:"grantRoot,omitempty"`
	Reason    *string `json:"reason,omitempty"`
}

func (p *FileChangeRequestApprovalParams) UnmarshalJSON(data []byte) error {
	type wire FileChangeRequestApprovalParams
	var decoded wire
	required := []string{"itemId", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	var err error
	decoded.GrantRoot, err = validateInboundAbsolutePathPointerField("grantRoot", decoded.GrantRoot)
	if err != nil {
		return err
	}
	*p = FileChangeRequestApprovalParams(decoded)
	return nil
}

func validateApprovalPathField(value string, cwd *string, field string) (string, error) {
	if cwd == nil {
		return validateInboundAbsolutePathField(field, value)
	}
	return validateInboundPathFieldWithBase(field, value, *cwd)
}

func validateApprovalPathPointerField(value *string, cwd *string, field string) (*string, error) {
	if value == nil {
		return nil, nil //nolint:nilnil // nil pointer is the valid absence case for optional approval path fields.
	}
	if cwd == nil {
		return validateInboundAbsolutePathPointerField(field, value)
	}
	return validateInboundPathPointerFieldWithBase(field, value, *cwd)
}

// FileChangeRequestApprovalResponse represents the response to a file change approval request.
type FileChangeRequestApprovalResponse struct {
	Decision FileChangeApprovalDecision `json:"decision"`
}

func (r FileChangeRequestApprovalResponse) validate() error {
	return validateFileChangeApprovalDecisionField("decision", r.Decision)
}

func validateNetworkPolicyAmendment(amendment NetworkPolicyAmendment) error {
	if err := validateNetworkPolicyRuleAction(amendment.Action); err != nil {
		return err
	}
	if amendment.Host == "" {
		return errors.New("network policy amendment missing host")
	}
	return nil
}
