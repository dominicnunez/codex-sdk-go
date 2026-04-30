package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ApprovalHandlers contains optional callback functions for all server→client approval requests.
// If a handler is not set, the client will return a JSON-RPC method-not-found error when
// the server sends that request type.
type ApprovalHandlers struct {
	OnApplyPatchApproval              func(context.Context, ApplyPatchApprovalParams) (ApplyPatchApprovalResponse, error)
	OnCommandExecutionRequestApproval func(context.Context, CommandExecutionRequestApprovalParams) (CommandExecutionRequestApprovalResponse, error)
	OnExecCommandApproval             func(context.Context, ExecCommandApprovalParams) (ExecCommandApprovalResponse, error)
	OnFileChangeRequestApproval       func(context.Context, FileChangeRequestApprovalParams) (FileChangeRequestApprovalResponse, error)
	OnPermissionsRequestApproval      func(context.Context, PermissionsRequestApprovalParams) (PermissionsRequestApprovalResponse, error)
	OnDynamicToolCall                 func(context.Context, DynamicToolCallParams) (DynamicToolCallResponse, error)
	OnToolRequestUserInput            func(context.Context, ToolRequestUserInputParams) (ToolRequestUserInputResponse, error)
	OnChatgptAuthTokensRefresh        func(context.Context, ChatgptAuthTokensRefreshParams) (ChatgptAuthTokensRefreshResponse, error)
	OnMcpServerElicitationRequest     func(context.Context, McpServerElicitationRequestParams) (McpServerElicitationRequestResponse, error)
}

// SetApprovalHandlers registers approval handlers on the client for server→client requests.
func (c *Client) SetApprovalHandlers(handlers ApprovalHandlers) {
	c.approvalMu.Lock()
	defer c.approvalMu.Unlock()
	c.approvalHandlers = handlers
}

// ========== ApplyPatchApproval (DEPRECATED - Legacy API) ==========

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

// ========== CommandExecutionRequestApproval (NEW - turn/start API) ==========

// CommandExecutionRequestApprovalParams represents parameters for command execution approval.
type CommandExecutionRequestApprovalParams struct {
	ItemID                          string                    `json:"itemId"`
	ThreadID                        string                    `json:"threadId"`
	TurnID                          string                    `json:"turnId"`
	ApprovalID                      *string                   `json:"approvalId,omitempty"`
	Command                         *string                   `json:"command,omitempty"`
	Cwd                             *string                   `json:"cwd,omitempty"`
	CommandActions                  *[]CommandActionWrapper   `json:"commandActions,omitempty"`
	NetworkApprovalContext          *NetworkApprovalContext   `json:"networkApprovalContext,omitempty"`
	ProposedExecpolicyAmendment     *[]string                 `json:"proposedExecpolicyAmendment,omitempty"`
	ProposedNetworkPolicyAmendments *[]NetworkPolicyAmendment `json:"proposedNetworkPolicyAmendments,omitempty"`
	Reason                          *string                   `json:"reason,omitempty"`
}

func (p *CommandExecutionRequestApprovalParams) UnmarshalJSON(data []byte) error {
	type wire CommandExecutionRequestApprovalParams
	var decoded wire
	required := []string{"itemId", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	var err error
	decoded.Cwd, err = validateInboundAbsolutePathPointerField("cwd", decoded.Cwd)
	if err != nil {
		return err
	}
	if decoded.CommandActions != nil {
		for i := range *decoded.CommandActions {
			if err := validateCommandActionWrapperPaths(&(*decoded.CommandActions)[i], decoded.Cwd, fmt.Sprintf("commandActions[%d]", i)); err != nil {
				return err
			}
		}
	}
	if decoded.ProposedNetworkPolicyAmendments != nil {
		for i, amendment := range *decoded.ProposedNetworkPolicyAmendments {
			if err := validateNetworkPolicyAmendment(amendment); err != nil {
				return fmt.Errorf("proposedNetworkPolicyAmendments[%d]: %w", i, err)
			}
		}
	}
	*p = CommandExecutionRequestApprovalParams(decoded)
	return nil
}

// CommandAction is a discriminated union for parsed command actions.
type CommandAction interface {
	commandAction()
}

// CommandActionWrapper wraps a CommandAction for JSON marshaling/unmarshaling.
type CommandActionWrapper struct {
	Value CommandAction
}

// ReadCommandAction represents a file read command.
type ReadCommandAction struct {
	Command string `json:"command"`
	Name    string `json:"name"`
	Path    string `json:"path"`
}

func (r *ReadCommandAction) commandAction() {}

func (r *ReadCommandAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Name    string `json:"name"`
		Path    string `json:"path"`
	}{
		Type:    "read",
		Command: r.Command,
		Name:    r.Name,
		Path:    r.Path,
	})
}

// ListFilesCommandAction represents a directory listing command.
type ListFilesCommandAction struct {
	Command string  `json:"command"`
	Path    *string `json:"path,omitempty"`
}

func (l *ListFilesCommandAction) commandAction() {}

func (l *ListFilesCommandAction) MarshalJSON() ([]byte, error) {
	type listJSON struct {
		Type    string  `json:"type"`
		Command string  `json:"command"`
		Path    *string `json:"path,omitempty"`
	}
	return json.Marshal(listJSON{
		Type:    "listFiles",
		Command: l.Command,
		Path:    l.Path,
	})
}

// SearchCommandAction represents a search command.
type SearchCommandAction struct {
	Command string  `json:"command"`
	Path    *string `json:"path,omitempty"`
	Query   *string `json:"query,omitempty"`
}

func (s *SearchCommandAction) commandAction() {}

func (s *SearchCommandAction) MarshalJSON() ([]byte, error) {
	type searchJSON struct {
		Type    string  `json:"type"`
		Command string  `json:"command"`
		Path    *string `json:"path,omitempty"`
		Query   *string `json:"query,omitempty"`
	}
	return json.Marshal(searchJSON{
		Type:    "search",
		Command: s.Command,
		Path:    s.Path,
		Query:   s.Query,
	})
}

// UnknownCommandAction represents an unparseable command.
type UnknownCommandAction struct {
	Command string `json:"command"`
}

func (u *UnknownCommandAction) commandAction() {}

func (u *UnknownCommandAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}{
		Type:    "unknown",
		Command: u.Command,
	})
}

// UnmarshalJSON implements custom unmarshaling for CommandActionWrapper.
func (w *CommandActionWrapper) UnmarshalJSON(data []byte) error {
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
	case "read":
		if err := validateRequiredObjectFields(data, "command", "name", "path", "type"); err != nil {
			return err
		}
		var read ReadCommandAction
		if err := json.Unmarshal(data, &read); err != nil {
			return err
		}
		w.Value = &read
	case "listFiles":
		if err := validateRequiredObjectFields(data, "command", "type"); err != nil {
			return err
		}
		var list ListFilesCommandAction
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		w.Value = &list
	case "search":
		if err := validateRequiredObjectFields(data, "command", "type"); err != nil {
			return err
		}
		var search SearchCommandAction
		if err := json.Unmarshal(data, &search); err != nil {
			return err
		}
		w.Value = &search
	default:
		if err := validateRequiredObjectFields(data, "command", "type"); err != nil {
			return err
		}
		var unknown UnknownCommandAction
		if err := json.Unmarshal(data, &unknown); err != nil {
			return err
		}
		w.Value = &unknown
	}

	return nil
}

func validateCommandActionWrapperPaths(w *CommandActionWrapper, cwd *string, field string) error {
	switch value := w.Value.(type) {
	case *ReadCommandAction:
		path, err := validateApprovalPathField(value.Path, cwd, field+".path")
		if err != nil {
			return err
		}
		value.Path = path
	case *ListFilesCommandAction:
		path, err := validateApprovalPathPointerField(value.Path, cwd, field+".path")
		if err != nil {
			return err
		}
		value.Path = path
	case *SearchCommandAction:
		path, err := validateApprovalPathPointerField(value.Path, cwd, field+".path")
		if err != nil {
			return err
		}
		value.Path = path
	}
	return nil
}

// MarshalJSON implements custom marshaling for CommandActionWrapper.
func (w CommandActionWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

// NetworkApprovalContext provides context for network-related approval requests.
type NetworkApprovalContext struct {
	Host     string                  `json:"host"`
	Protocol NetworkApprovalProtocol `json:"protocol"`
}

func (c *NetworkApprovalContext) UnmarshalJSON(data []byte) error {
	type wire NetworkApprovalContext
	var decoded wire
	required := []string{"host", "protocol"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	if err := validateNetworkApprovalProtocolField("protocol", decoded.Protocol); err != nil {
		return err
	}
	*c = NetworkApprovalContext(decoded)
	return nil
}

// CommandExecutionRequestApprovalResponse represents the response to a command execution approval request.
type CommandExecutionRequestApprovalResponse struct {
	Decision CommandExecutionApprovalDecisionWrapper `json:"decision"`
}

func (r CommandExecutionRequestApprovalResponse) validate() error {
	return validateCommandExecutionApprovalDecisionWrapper(r.Decision)
}

// CommandExecutionApprovalDecisionWrapper wraps the decision for command execution approval.
// String values should use the CommandExecutionApprovalDecision* constants.
type CommandExecutionApprovalDecisionWrapper struct {
	Value interface{} // string or object
}

// String constants for CommandExecutionApprovalDecision.
const (
	CommandExecutionApprovalDecisionAccept           = "accept"
	CommandExecutionApprovalDecisionAcceptForSession = "acceptForSession"
	CommandExecutionApprovalDecisionDecline          = "decline"
	CommandExecutionApprovalDecisionCancel           = "cancel"
)

// AcceptWithExecpolicyAmendmentDecision represents acceptance with execpolicy amendment.
type AcceptWithExecpolicyAmendmentDecision struct {
	ExecpolicyAmendment []string `json:"execpolicy_amendment"`
}

// ApplyNetworkPolicyAmendmentDecision represents acceptance with network policy amendment.
type ApplyNetworkPolicyAmendmentDecision struct {
	NetworkPolicyAmendment NetworkPolicyAmendment `json:"network_policy_amendment"`
}

// UnknownCommandExecutionApprovalDecision represents an unrecognized command execution approval decision
// variant from a newer protocol version.
type UnknownCommandExecutionApprovalDecision struct {
	Raw json.RawMessage `json:"-"`
}

func (u UnknownCommandExecutionApprovalDecision) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// UnmarshalJSON implements custom unmarshaling for CommandExecutionApprovalDecisionWrapper.
func (w *CommandExecutionApprovalDecisionWrapper) UnmarshalJSON(data []byte) error {
	// Try string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		w.Value = str
		return nil
	}

	// Dispatch on which key is present in the JSON object
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("unable to unmarshal CommandExecutionApprovalDecision: %w", err)
	}

	if raw, ok := keys["acceptWithExecpolicyAmendment"]; ok {
		var inner AcceptWithExecpolicyAmendmentDecision
		if err := json.Unmarshal(raw, &inner); err != nil {
			return fmt.Errorf("unable to unmarshal acceptWithExecpolicyAmendment: %w", err)
		}
		w.Value = inner
		return nil
	}

	if raw, ok := keys["applyNetworkPolicyAmendment"]; ok {
		var inner struct {
			NetworkPolicyAmendment NetworkPolicyAmendment `json:"network_policy_amendment"`
		}
		if err := json.Unmarshal(raw, &inner); err != nil {
			return fmt.Errorf("unable to unmarshal applyNetworkPolicyAmendment: %w", err)
		}
		w.Value = ApplyNetworkPolicyAmendmentDecision{NetworkPolicyAmendment: inner.NetworkPolicyAmendment}
		return nil
	}

	w.Value = UnknownCommandExecutionApprovalDecision{Raw: append(json.RawMessage(nil), data...)}
	return nil
}

// MarshalJSON implements custom marshaling for CommandExecutionApprovalDecisionWrapper.
func (w CommandExecutionApprovalDecisionWrapper) MarshalJSON() ([]byte, error) {
	switch v := normalizeCommandExecutionApprovalDecisionValue(w.Value).(type) {
	case nil:
		return []byte("null"), nil
	case string:
		return json.Marshal(v)
	case AcceptWithExecpolicyAmendmentDecision:
		return json.Marshal(struct {
			AcceptWithExecpolicyAmendment struct {
				ExecpolicyAmendment []string `json:"execpolicy_amendment"`
			} `json:"acceptWithExecpolicyAmendment"`
		}{
			AcceptWithExecpolicyAmendment: struct {
				ExecpolicyAmendment []string `json:"execpolicy_amendment"`
			}{
				ExecpolicyAmendment: v.ExecpolicyAmendment,
			},
		})
	case ApplyNetworkPolicyAmendmentDecision:
		return json.Marshal(struct {
			ApplyNetworkPolicyAmendment struct {
				NetworkPolicyAmendment NetworkPolicyAmendment `json:"network_policy_amendment"`
			} `json:"applyNetworkPolicyAmendment"`
		}{
			ApplyNetworkPolicyAmendment: struct {
				NetworkPolicyAmendment NetworkPolicyAmendment `json:"network_policy_amendment"`
			}{
				NetworkPolicyAmendment: v.NetworkPolicyAmendment,
			},
		})
	case UnknownCommandExecutionApprovalDecision:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown decision type: %T", v)
	}
}

func normalizeCommandExecutionApprovalDecisionValue(value interface{}) interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case *string:
		if v == nil {
			return nil
		}
		return *v
	case *AcceptWithExecpolicyAmendmentDecision:
		if v == nil {
			return nil
		}
		return *v
	case *ApplyNetworkPolicyAmendmentDecision:
		if v == nil {
			return nil
		}
		return *v
	case *UnknownCommandExecutionApprovalDecision:
		if v == nil {
			return nil
		}
		return *v
	default:
		return value
	}
}

func validateCommandExecutionApprovalDecisionWrapper(decision CommandExecutionApprovalDecisionWrapper) error {
	switch value := normalizeCommandExecutionApprovalDecisionValue(decision.Value).(type) {
	case nil:
		return errors.New("missing decision")
	case string:
		switch value {
		case CommandExecutionApprovalDecisionAccept,
			CommandExecutionApprovalDecisionAcceptForSession,
			CommandExecutionApprovalDecisionDecline,
			CommandExecutionApprovalDecisionCancel:
			return nil
		default:
			return fmt.Errorf("invalid decision %q", value)
		}
	case AcceptWithExecpolicyAmendmentDecision:
		if value.ExecpolicyAmendment == nil {
			return errors.New("acceptWithExecpolicyAmendment.execpolicy_amendment: missing array")
		}
		return nil
	case ApplyNetworkPolicyAmendmentDecision:
		return validateNetworkPolicyAmendment(value.NetworkPolicyAmendment)
	case UnknownCommandExecutionApprovalDecision:
		return errors.New("missing decision")
	default:
		return fmt.Errorf("invalid decision type %T", decision.Value)
	}
}

// ========== ExecCommandApproval (DEPRECATED - Legacy API) ==========

// ExecCommandApprovalParams represents parameters for exec command approval (legacy).
//
// Deprecated: Use CommandExecutionRequestApprovalParams instead.
type ExecCommandApprovalParams struct {
	CallID         string                 `json:"callId"`
	Command        []string               `json:"command"`
	ConversationID string                 `json:"conversationId"`
	Cwd            string                 `json:"cwd"`
	ParsedCmd      []ParsedCommandWrapper `json:"parsedCmd"`
	ApprovalID     *string                `json:"approvalId,omitempty"`
	Reason         *string                `json:"reason,omitempty"`
}

func (p *ExecCommandApprovalParams) UnmarshalJSON(data []byte) error {
	type wire ExecCommandApprovalParams
	var decoded wire
	required := []string{"callId", "command", "conversationId", "cwd", "parsedCmd"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	validatedCwd, err := validateInboundAbsolutePathField("cwd", decoded.Cwd)
	if err != nil {
		return err
	}
	decoded.Cwd = validatedCwd
	for i := range decoded.ParsedCmd {
		if err := validateParsedCommandWrapperPaths(&decoded.ParsedCmd[i], decoded.Cwd, fmt.Sprintf("parsedCmd[%d]", i)); err != nil {
			return err
		}
	}
	*p = ExecCommandApprovalParams(decoded)
	return nil
}

// ParsedCommand is a discriminated union for legacy parsed commands.
type ParsedCommand interface {
	parsedCommand()
}

// ParsedCommandWrapper wraps a ParsedCommand for JSON marshaling/unmarshaling.
type ParsedCommandWrapper struct {
	Value ParsedCommand
}

// ReadParsedCommand represents a file read command (legacy).
type ReadParsedCommand struct {
	Cmd  string `json:"cmd"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func (r *ReadParsedCommand) parsedCommand() {}

func (r *ReadParsedCommand) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
		Cmd  string `json:"cmd"`
		Name string `json:"name"`
		Path string `json:"path"`
	}{
		Type: "read",
		Cmd:  r.Cmd,
		Name: r.Name,
		Path: r.Path,
	})
}

// ListFilesParsedCommand represents a directory listing command (legacy).
type ListFilesParsedCommand struct {
	Cmd  string  `json:"cmd"`
	Path *string `json:"path,omitempty"`
}

func (l *ListFilesParsedCommand) parsedCommand() {}

func (l *ListFilesParsedCommand) MarshalJSON() ([]byte, error) {
	type listJSON struct {
		Type string  `json:"type"`
		Cmd  string  `json:"cmd"`
		Path *string `json:"path,omitempty"`
	}
	return json.Marshal(listJSON{
		Type: "list_files",
		Cmd:  l.Cmd,
		Path: l.Path,
	})
}

// SearchParsedCommand represents a search command (legacy).
type SearchParsedCommand struct {
	Cmd   string  `json:"cmd"`
	Path  *string `json:"path,omitempty"`
	Query *string `json:"query,omitempty"`
}

func (s *SearchParsedCommand) parsedCommand() {}

func (s *SearchParsedCommand) MarshalJSON() ([]byte, error) {
	type searchJSON struct {
		Type  string  `json:"type"`
		Cmd   string  `json:"cmd"`
		Path  *string `json:"path,omitempty"`
		Query *string `json:"query,omitempty"`
	}
	return json.Marshal(searchJSON{
		Type:  "search",
		Cmd:   s.Cmd,
		Path:  s.Path,
		Query: s.Query,
	})
}

// UnknownParsedCommand represents an unparseable command (legacy).
type UnknownParsedCommand struct {
	Cmd string `json:"cmd"`
}

func (u *UnknownParsedCommand) parsedCommand() {}

func (u *UnknownParsedCommand) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
		Cmd  string `json:"cmd"`
	}{
		Type: "unknown",
		Cmd:  u.Cmd,
	})
}

// UnmarshalJSON implements custom unmarshaling for ParsedCommandWrapper.
func (w *ParsedCommandWrapper) UnmarshalJSON(data []byte) error {
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
	case "read":
		if err := validateRequiredObjectFields(data, "cmd", "name", "path", "type"); err != nil {
			return err
		}
		var read ReadParsedCommand
		if err := json.Unmarshal(data, &read); err != nil {
			return err
		}
		w.Value = &read
	case "list_files":
		if err := validateRequiredObjectFields(data, "cmd", "type"); err != nil {
			return err
		}
		var list ListFilesParsedCommand
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		w.Value = &list
	case "search":
		if err := validateRequiredObjectFields(data, "cmd", "type"); err != nil {
			return err
		}
		var search SearchParsedCommand
		if err := json.Unmarshal(data, &search); err != nil {
			return err
		}
		w.Value = &search
	default:
		if err := validateRequiredObjectFields(data, "cmd", "type"); err != nil {
			return err
		}
		var unknown UnknownParsedCommand
		if err := json.Unmarshal(data, &unknown); err != nil {
			return err
		}
		w.Value = &unknown
	}

	return nil
}

func validateParsedCommandWrapperPaths(w *ParsedCommandWrapper, cwd string, field string) error {
	switch value := w.Value.(type) {
	case *ReadParsedCommand:
		path, err := validateApprovalPathField(value.Path, &cwd, field+".path")
		if err != nil {
			return err
		}
		value.Path = path
	case *ListFilesParsedCommand:
		path, err := validateApprovalPathPointerField(value.Path, &cwd, field+".path")
		if err != nil {
			return err
		}
		value.Path = path
	case *SearchParsedCommand:
		path, err := validateApprovalPathPointerField(value.Path, &cwd, field+".path")
		if err != nil {
			return err
		}
		value.Path = path
	}
	return nil
}

// MarshalJSON implements custom marshaling for ParsedCommandWrapper.
func (w ParsedCommandWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

// ExecCommandApprovalResponse represents the response to an exec command approval request (legacy).
// Uses the same ReviewDecision as ApplyPatchApproval.
//
// Deprecated: Use CommandExecutionRequestApprovalResponse instead.
type ExecCommandApprovalResponse struct {
	Decision ReviewDecisionWrapper `json:"decision"`
}

func (r ExecCommandApprovalResponse) validate() error {
	return validateReviewDecisionWrapper(r.Decision)
}

// ========== FileChangeRequestApproval (NEW - turn/start API) ==========

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

// ========== DynamicToolCall (NEW - Direct Tool Execution) ==========

// DynamicToolCallParams represents parameters for a dynamic tool call.
type DynamicToolCallParams struct {
	Tool      string      `json:"tool"`
	Arguments interface{} `json:"arguments"` // any JSON structure
	CallID    string      `json:"callId"`
	Namespace *string     `json:"namespace,omitempty"`
	ThreadID  string      `json:"threadId"`
	TurnID    string      `json:"turnId"`
}

func (p *DynamicToolCallParams) UnmarshalJSON(data []byte) error {
	type wire DynamicToolCallParams
	var decoded wire
	required := []string{"arguments", "callId", "threadId", "tool", "turnId"}
	nonNull := []string{"callId", "threadId", "tool", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, nonNull); err != nil {
		return err
	}
	*p = DynamicToolCallParams(decoded)
	return nil
}

// DynamicToolCallResponse represents the response to a dynamic tool call.
type DynamicToolCallResponse struct {
	Success      bool                                      `json:"success"`
	ContentItems []DynamicToolCallOutputContentItemWrapper `json:"contentItems"`
}

func (r DynamicToolCallResponse) validate() error {
	if r.ContentItems == nil {
		return errors.New("missing contentItems")
	}
	for i, item := range r.ContentItems {
		if err := item.validateForResponse(); err != nil {
			return fmt.Errorf("contentItems[%d]: %w", i, err)
		}
	}
	return nil
}

// DynamicToolCallOutputContentItem is a discriminated union for tool output content.
type DynamicToolCallOutputContentItem interface {
	dynamicToolCallOutputContentItem()
}

// UnknownDynamicToolCallOutputContentItem represents an unrecognized tool output content type from a newer protocol version.
type UnknownDynamicToolCallOutputContentItem struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (u *UnknownDynamicToolCallOutputContentItem) dynamicToolCallOutputContentItem() {}

func (u *UnknownDynamicToolCallOutputContentItem) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// DynamicToolCallOutputContentItemWrapper wraps output content items for JSON marshaling.
type DynamicToolCallOutputContentItemWrapper struct {
	Value DynamicToolCallOutputContentItem
}

// InputTextDynamicToolCallOutputContentItem represents text output.
type InputTextDynamicToolCallOutputContentItem struct {
	Text string `json:"text"`
}

func (i *InputTextDynamicToolCallOutputContentItem) dynamicToolCallOutputContentItem() {}

func (i *InputTextDynamicToolCallOutputContentItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}{
		Type: "inputText",
		Text: i.Text,
	})
}

func (i *InputTextDynamicToolCallOutputContentItem) UnmarshalJSON(data []byte) error {
	type wire struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"type", "text"}, []string{"type", "text"}); err != nil {
		return err
	}
	if decoded.Type != "inputText" {
		return fmt.Errorf("invalid dynamic tool output content item type %q", decoded.Type)
	}
	i.Text = decoded.Text
	return nil
}

// InputImageDynamicToolCallOutputContentItem represents image output.
type InputImageDynamicToolCallOutputContentItem struct {
	ImageURL string `json:"imageUrl"`
}

func (i *InputImageDynamicToolCallOutputContentItem) dynamicToolCallOutputContentItem() {}

func (i *InputImageDynamicToolCallOutputContentItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type     string `json:"type"`
		ImageURL string `json:"imageUrl"`
	}{
		Type:     "inputImage",
		ImageURL: i.ImageURL,
	})
}

func (i *InputImageDynamicToolCallOutputContentItem) UnmarshalJSON(data []byte) error {
	type wire struct {
		Type     string `json:"type"`
		ImageURL string `json:"imageUrl"`
	}
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"imageUrl", "type"}, []string{"imageUrl", "type"}); err != nil {
		return err
	}
	if decoded.Type != "inputImage" {
		return fmt.Errorf("invalid dynamic tool output content item type %q", decoded.Type)
	}
	i.ImageURL = decoded.ImageURL
	return nil
}

// UnmarshalJSON implements custom unmarshaling for DynamicToolCallOutputContentItemWrapper.
func (w *DynamicToolCallOutputContentItemWrapper) UnmarshalJSON(data []byte) error {
	itemType, err := decodeRequiredObjectTypeField(data, "dynamic tool output content item")
	if err != nil {
		return err
	}

	switch itemType {
	case "inputText":
		var text InputTextDynamicToolCallOutputContentItem
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		w.Value = &text
	case "inputImage":
		var image InputImageDynamicToolCallOutputContentItem
		if err := json.Unmarshal(data, &image); err != nil {
			return err
		}
		w.Value = &image
	default:
		w.Value = &UnknownDynamicToolCallOutputContentItem{Type: itemType, Raw: append(json.RawMessage(nil), data...)}
	}

	return nil
}

// MarshalJSON implements custom marshaling for DynamicToolCallOutputContentItemWrapper.
func (w DynamicToolCallOutputContentItemWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

func (w DynamicToolCallOutputContentItemWrapper) validateForResponse() error {
	switch value := w.Value.(type) {
	case nil:
		return errors.New("missing content item")
	case *InputTextDynamicToolCallOutputContentItem:
		if value == nil {
			return errors.New("missing content item")
		}
		return nil
	case *InputImageDynamicToolCallOutputContentItem:
		if value == nil {
			return errors.New("missing content item")
		}
		return nil
	case *UnknownDynamicToolCallOutputContentItem:
		if value == nil {
			return errors.New("missing content item")
		}
		return fmt.Errorf("unsupported content item type %q", value.Type)
	default:
		return fmt.Errorf("unsupported content item type %T", w.Value)
	}
}

// ========== ToolRequestUserInput (EXPERIMENTAL) ==========

// ToolRequestUserInputParams represents parameters for requesting user input for a tool.
type ToolRequestUserInputParams struct {
	ItemID    string                         `json:"itemId"`
	ThreadID  string                         `json:"threadId"`
	TurnID    string                         `json:"turnId"`
	Questions []ToolRequestUserInputQuestion `json:"questions"`
}

func (p *ToolRequestUserInputParams) UnmarshalJSON(data []byte) error {
	type wire ToolRequestUserInputParams
	var decoded wire
	required := []string{"itemId", "questions", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*p = ToolRequestUserInputParams(decoded)
	return nil
}

// ToolRequestUserInputQuestion represents a question to ask the user.
type ToolRequestUserInputQuestion struct {
	ID       string                        `json:"id"`
	Header   string                        `json:"header"`
	Question string                        `json:"question"`
	IsSecret bool                          `json:"isSecret"`
	IsOther  bool                          `json:"isOther"`
	Options  *[]ToolRequestUserInputOption `json:"options,omitempty"`
}

func (q *ToolRequestUserInputQuestion) UnmarshalJSON(data []byte) error {
	type wire ToolRequestUserInputQuestion
	var decoded wire
	required := []string{"header", "id", "question"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*q = ToolRequestUserInputQuestion(decoded)
	return nil
}

// ToolRequestUserInputOption represents a selectable option for a question.
type ToolRequestUserInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

func (o *ToolRequestUserInputOption) UnmarshalJSON(data []byte) error {
	type wire ToolRequestUserInputOption
	var decoded wire
	required := []string{"description", "label"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*o = ToolRequestUserInputOption(decoded)
	return nil
}

// ToolRequestUserInputResponse represents the response containing user's answers.
type ToolRequestUserInputResponse struct {
	Answers map[string]ToolRequestUserInputAnswer `json:"answers"` // question ID → answer
}

func (r ToolRequestUserInputResponse) validate() error {
	if r.Answers == nil {
		return errors.New("missing answers")
	}
	for questionID, answer := range r.Answers {
		if answer.Answers == nil {
			return fmt.Errorf("answers[%q].answers: missing answers", questionID)
		}
	}
	return nil
}

// ToolRequestUserInputAnswer represents an answer to a question.
type ToolRequestUserInputAnswer struct {
	Answers []string `json:"answers"`
}

// ========== ChatgptAuthTokensRefresh (Authentication Token Refresh) ==========

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

func validateNetworkPolicyAmendment(amendment NetworkPolicyAmendment) error {
	if err := validateNetworkPolicyRuleAction(amendment.Action); err != nil {
		return err
	}
	if amendment.Host == "" {
		return errors.New("network policy amendment missing host")
	}
	return nil
}

// GoString implements fmt.GoStringer to redact credentials from %#v.
func (r ChatgptAuthTokensRefreshResponse) GoString() string { return r.String() }

// Format implements fmt.Formatter to redact credentials from all format verbs.
func (r ChatgptAuthTokensRefreshResponse) Format(f fmt.State, verb rune) {
	_, _ = fmt.Fprint(f, r.String())
}
