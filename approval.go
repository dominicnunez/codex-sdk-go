package codex

import (
	"context"
	"encoding/json"
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
	OnSkillRequestApproval            func(context.Context, SkillRequestApprovalParams) (SkillRequestApprovalResponse, error)
	OnDynamicToolCall                 func(context.Context, DynamicToolCallParams) (DynamicToolCallResponse, error)
	OnToolRequestUserInput            func(context.Context, ToolRequestUserInputParams) (ToolRequestUserInputResponse, error)
	OnChatgptAuthTokensRefresh        func(context.Context, ChatgptAuthTokensRefreshParams) (ChatgptAuthTokensRefreshResponse, error)
}

// SetApprovalHandlers registers approval handlers on the client for server→client requests.
func (c *Client) SetApprovalHandlers(handlers ApprovalHandlers) {
	c.approvalMu.Lock()
	defer c.approvalMu.Unlock()
	c.approvalHandlers = handlers
}

// ========== ApplyPatchApproval (DEPRECATED - Legacy API) ==========

// ApplyPatchApprovalParams represents the parameters for a server→client applyPatchApproval request.
type ApplyPatchApprovalParams struct {
	CallID         string                     `json:"callId"`
	ConversationID string                     `json:"conversationId"`
	FileChanges    map[string]FileChangeWrapper `json:"fileChanges"`
	GrantRoot      *string                    `json:"grantRoot,omitempty"`
	Reason         *string                    `json:"reason,omitempty"`
}

// FileChange is a discriminated union for file changes (add/delete/update).
type FileChange interface {
	fileChange()
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
		Type:    "add",
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
		Type:    "delete",
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
		Type:        "update",
		UnifiedDiff: u.UnifiedDiff,
		MovePath:    u.MovePath,
	})
}

// UnmarshalJSON implements custom unmarshaling for FileChangeWrapper.
func (w *FileChangeWrapper) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw.Type {
	case "add":
		var add AddFileChange
		if err := json.Unmarshal(data, &add); err != nil {
			return err
		}
		w.Value = &add
	case "delete":
		var del DeleteFileChange
		if err := json.Unmarshal(data, &del); err != nil {
			return err
		}
		w.Value = &del
	case "update":
		var upd UpdateFileChange
		if err := json.Unmarshal(data, &upd); err != nil {
			return err
		}
		w.Value = &upd
	default:
		return fmt.Errorf("unknown file change type: %s", raw.Type)
	}

	return nil
}

// MarshalJSON implements custom marshaling for FileChangeWrapper.
func (w FileChangeWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

// ApplyPatchApprovalResponse represents the response to an applyPatchApproval request.
type ApplyPatchApprovalResponse struct {
	Decision ReviewDecisionWrapper `json:"decision"`
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

// Valid string values for ReviewDecision per spec.
var validReviewDecisions = map[string]bool{
	"approved":             true,
	"approved_for_session": true,
	"denied":               true,
	"abort":                true,
}

// UnmarshalJSON implements custom unmarshaling for ReviewDecisionWrapper.
func (w *ReviewDecisionWrapper) UnmarshalJSON(data []byte) error {
	// Try string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if !validReviewDecisions[str] {
			return fmt.Errorf("unknown review decision: %s", str)
		}
		w.Value = str
		return nil
	}

	// Dispatch on which key is present in the JSON object
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("unable to unmarshal ReviewDecision")
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

	return fmt.Errorf("unable to unmarshal ReviewDecision")
}

// MarshalJSON implements custom marshaling for ReviewDecisionWrapper.
func (w ReviewDecisionWrapper) MarshalJSON() ([]byte, error) {
	switch v := w.Value.(type) {
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
			NetworkPolicyAmendment NetworkPolicyAmendment `json:"network_policy_amendment"`
		}{
			NetworkPolicyAmendment: v.NetworkPolicyAmendment,
		})
	default:
		return nil, fmt.Errorf("unknown decision type: %T", v)
	}
}

// ========== CommandExecutionRequestApproval (NEW - turn/start API) ==========

// CommandExecutionRequestApprovalParams represents parameters for command execution approval.
type CommandExecutionRequestApprovalParams struct {
	ItemID                           string                          `json:"itemId"`
	ThreadID                         string                          `json:"threadId"`
	TurnID                           string                          `json:"turnId"`
	ApprovalID                       *string                         `json:"approvalId,omitempty"`
	Command                          *string                         `json:"command,omitempty"`
	Cwd                              *string                         `json:"cwd,omitempty"`
	CommandActions                   *[]CommandActionWrapper         `json:"commandActions,omitempty"`
	NetworkApprovalContext           *NetworkApprovalContext         `json:"networkApprovalContext,omitempty"`
	ProposedExecpolicyAmendment      *[]string                       `json:"proposedExecpolicyAmendment,omitempty"`
	ProposedNetworkPolicyAmendments  *[]NetworkPolicyAmendment       `json:"proposedNetworkPolicyAmendments,omitempty"`
	Reason                           *string                         `json:"reason,omitempty"`
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
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw.Type {
	case "read":
		var read ReadCommandAction
		if err := json.Unmarshal(data, &read); err != nil {
			return err
		}
		w.Value = &read
	case "listFiles":
		var list ListFilesCommandAction
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		w.Value = &list
	case "search":
		var search SearchCommandAction
		if err := json.Unmarshal(data, &search); err != nil {
			return err
		}
		w.Value = &search
	case "unknown":
		var unknown UnknownCommandAction
		if err := json.Unmarshal(data, &unknown); err != nil {
			return err
		}
		w.Value = &unknown
	default:
		return fmt.Errorf("unknown command action type: %s", raw.Type)
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

// CommandExecutionRequestApprovalResponse represents the response to a command execution approval request.
type CommandExecutionRequestApprovalResponse struct {
	Decision CommandExecutionApprovalDecisionWrapper `json:"decision"`
}

// CommandExecutionApprovalDecisionWrapper wraps the decision for command execution approval.
type CommandExecutionApprovalDecisionWrapper struct {
	Value interface{} // string or object
}

// AcceptWithExecpolicyAmendmentDecision represents acceptance with execpolicy amendment.
type AcceptWithExecpolicyAmendmentDecision struct {
	ExecpolicyAmendment []string `json:"execpolicy_amendment"`
}

// ApplyNetworkPolicyAmendmentDecision represents acceptance with network policy amendment.
type ApplyNetworkPolicyAmendmentDecision struct {
	NetworkPolicyAmendment NetworkPolicyAmendment `json:"network_policy_amendment"`
}

// Valid string values for CommandExecutionApprovalDecision per spec.
var validCommandExecutionDecisions = map[string]bool{
	"accept":           true,
	"acceptForSession": true,
	"decline":          true,
	"cancel":           true,
}

// UnmarshalJSON implements custom unmarshaling for CommandExecutionApprovalDecisionWrapper.
func (w *CommandExecutionApprovalDecisionWrapper) UnmarshalJSON(data []byte) error {
	// Try string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if !validCommandExecutionDecisions[str] {
			return fmt.Errorf("unknown command execution approval decision: %s", str)
		}
		w.Value = str
		return nil
	}

	// Dispatch on which key is present in the JSON object
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("unable to unmarshal CommandExecutionApprovalDecision")
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

	return fmt.Errorf("unable to unmarshal CommandExecutionApprovalDecision")
}

// MarshalJSON implements custom marshaling for CommandExecutionApprovalDecisionWrapper.
func (w CommandExecutionApprovalDecisionWrapper) MarshalJSON() ([]byte, error) {
	switch v := w.Value.(type) {
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
	default:
		return nil, fmt.Errorf("unknown decision type: %T", v)
	}
}

// ========== ExecCommandApproval (DEPRECATED - Legacy API) ==========

// ExecCommandApprovalParams represents parameters for exec command approval (legacy).
type ExecCommandApprovalParams struct {
	CallID         string                 `json:"callId"`
	Command        []string               `json:"command"`
	ConversationID string                 `json:"conversationId"`
	Cwd            string                 `json:"cwd"`
	ParsedCmd      []ParsedCommandWrapper `json:"parsedCmd"`
	ApprovalID     *string                `json:"approvalId,omitempty"`
	Reason         *string                `json:"reason,omitempty"`
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
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw.Type {
	case "read":
		var read ReadParsedCommand
		if err := json.Unmarshal(data, &read); err != nil {
			return err
		}
		w.Value = &read
	case "list_files":
		var list ListFilesParsedCommand
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		w.Value = &list
	case "search":
		var search SearchParsedCommand
		if err := json.Unmarshal(data, &search); err != nil {
			return err
		}
		w.Value = &search
	case "unknown":
		var unknown UnknownParsedCommand
		if err := json.Unmarshal(data, &unknown); err != nil {
			return err
		}
		w.Value = &unknown
	default:
		return fmt.Errorf("unknown parsed command type: %s", raw.Type)
	}

	return nil
}

// MarshalJSON implements custom marshaling for ParsedCommandWrapper.
func (w ParsedCommandWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

// ExecCommandApprovalResponse represents the response to an exec command approval request (legacy).
// Uses the same ReviewDecision as ApplyPatchApproval.
type ExecCommandApprovalResponse struct {
	Decision ReviewDecisionWrapper `json:"decision"`
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

// FileChangeRequestApprovalResponse represents the response to a file change approval request.
type FileChangeRequestApprovalResponse struct {
	Decision FileChangeApprovalDecision `json:"decision"`
}

// ========== SkillRequestApproval (NEW - turn/start API) ==========

// SkillRequestApprovalParams represents parameters for skill approval.
type SkillRequestApprovalParams struct {
	ItemID    string `json:"itemId"`
	SkillName string `json:"skillName"`
}

// SkillRequestApprovalResponse represents the response to a skill approval request.
type SkillRequestApprovalResponse struct {
	Decision SkillApprovalDecision `json:"decision"`
}

// ========== DynamicToolCall (NEW - Direct Tool Execution) ==========

// DynamicToolCallParams represents parameters for a dynamic tool call.
type DynamicToolCallParams struct {
	Tool      string      `json:"tool"`
	Arguments interface{} `json:"arguments"` // any JSON structure
	CallID    string      `json:"callId"`
	ThreadID  string      `json:"threadId"`
	TurnID    string      `json:"turnId"`
}

// DynamicToolCallResponse represents the response to a dynamic tool call.
type DynamicToolCallResponse struct {
	Success      bool                                        `json:"success"`
	ContentItems []DynamicToolCallOutputContentItemWrapper  `json:"contentItems"`
}

// DynamicToolCallOutputContentItem is a discriminated union for tool output content.
type DynamicToolCallOutputContentItem interface {
	dynamicToolCallOutputContentItem()
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

// UnmarshalJSON implements custom unmarshaling for DynamicToolCallOutputContentItemWrapper.
func (w *DynamicToolCallOutputContentItemWrapper) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw.Type {
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
		return fmt.Errorf("unknown tool output content type: %s", raw.Type)
	}

	return nil
}

// MarshalJSON implements custom marshaling for DynamicToolCallOutputContentItemWrapper.
func (w DynamicToolCallOutputContentItemWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

// ========== ToolRequestUserInput (EXPERIMENTAL) ==========

// ToolRequestUserInputParams represents parameters for requesting user input for a tool.
type ToolRequestUserInputParams struct {
	ItemID    string                         `json:"itemId"`
	ThreadID  string                         `json:"threadId"`
	TurnID    string                         `json:"turnId"`
	Questions []ToolRequestUserInputQuestion `json:"questions"`
}

// ToolRequestUserInputQuestion represents a question to ask the user.
type ToolRequestUserInputQuestion struct {
	ID       string                          `json:"id"`
	Header   string                          `json:"header"`
	Question string                          `json:"question"`
	IsSecret bool                            `json:"isSecret,omitempty"`
	IsOther  bool                            `json:"isOther,omitempty"`
	Options  *[]ToolRequestUserInputOption   `json:"options,omitempty"`
}

// ToolRequestUserInputOption represents a selectable option for a question.
type ToolRequestUserInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// ToolRequestUserInputResponse represents the response containing user's answers.
type ToolRequestUserInputResponse struct {
	Answers map[string]ToolRequestUserInputAnswer `json:"answers"` // question ID → answer
}

// ToolRequestUserInputAnswer represents an answer to a question.
type ToolRequestUserInputAnswer struct {
	Answers []string `json:"answers"`
}

// ========== ChatgptAuthTokensRefresh (Authentication Token Refresh) ==========

// ChatgptAuthTokensRefreshParams represents parameters for ChatGPT auth token refresh.
type ChatgptAuthTokensRefreshParams struct {
	Reason            ChatgptAuthTokensRefreshReason `json:"reason"`
	PreviousAccountID *string `json:"previousAccountId,omitempty"`
}

// ChatgptAuthTokensRefreshResponse represents the response containing new auth tokens.
type ChatgptAuthTokensRefreshResponse struct {
	AccessToken       string  `json:"accessToken"`
	ChatgptAccountID  string  `json:"chatgptAccountId"`
	ChatgptPlanType   *string `json:"chatgptPlanType,omitempty"`
}

// String redacts the access token to prevent accidental credential leaks in logs.
func (r *ChatgptAuthTokensRefreshResponse) String() string {
	return fmt.Sprintf("ChatgptAuthTokensRefreshResponse{AccessToken:[REDACTED], ChatgptAccountID:%s}", r.ChatgptAccountID)
}

// GoString implements fmt.GoStringer to redact credentials from %#v.
func (r *ChatgptAuthTokensRefreshResponse) GoString() string { return r.String() }

// Format implements fmt.Formatter to redact credentials from all format verbs.
func (r *ChatgptAuthTokensRefreshResponse) Format(f fmt.State, verb rune) {
	fmt.Fprint(f, r.String())
}
