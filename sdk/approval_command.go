package codex

import (
	"encoding/json"
	"errors"
	"fmt"
)

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
