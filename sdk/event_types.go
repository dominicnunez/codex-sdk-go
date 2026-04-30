package codex

import (
	"encoding/json"
	"fmt"
)

// Shared Event Types
// These types are referenced by multiple streaming notifications and thread items.
// They are extracted from EventMsg.json and ItemStartedNotification definitions.

// ByteRange represents a range of bytes in UTF-8 encoded text.
// Start is inclusive, End is exclusive.
type ByteRange struct {
	Start uint `json:"start"` // Start byte offset (inclusive)
	End   uint `json:"end"`   // End byte offset (exclusive)
}

func (b *ByteRange) UnmarshalJSON(data []byte) error {
	type wire ByteRange
	var decoded wire
	if err := unmarshalRequiredEventObject(data, &decoded, "start", "end"); err != nil {
		return err
	}
	*b = ByteRange(decoded)
	return nil
}

// TextElement represents a span within text used to render or persist special elements.
// Used in UserInput to mark UI-defined elements within the text.
type TextElement struct {
	ByteRange   ByteRange `json:"byteRange"`             // Byte range in the parent text buffer
	Placeholder *string   `json:"placeholder,omitempty"` // Optional human-readable placeholder for UI
}

func (t *TextElement) UnmarshalJSON(data []byte) error {
	type wire TextElement
	var decoded wire
	if err := unmarshalRequiredEventObject(data, &decoded, "byteRange"); err != nil {
		return err
	}
	*t = TextElement(decoded)
	return nil
}

// MessagePhase classifies an assistant message as interim commentary or final answer text.
// Providers do not emit this consistently, so None/null should be treated as "phase unknown".
type MessagePhase string

const (
	MessagePhaseCommentary  MessagePhase = "commentary"   // Mid-turn assistant text (preamble/progress narration)
	MessagePhaseFinalAnswer MessagePhase = "final_answer" // Terminal answer text for the current turn
)

var validMessagePhases = map[MessagePhase]struct{}{
	MessagePhaseCommentary:  {},
	MessagePhaseFinalAnswer: {},
}

func (p *MessagePhase) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "agentMessage.phase", validMessagePhases, p)
}

// CommandExecutionStatus represents the status of a command execution.
type CommandExecutionStatus string

const (
	CommandExecutionStatusInProgress CommandExecutionStatus = "inProgress"
	CommandExecutionStatusCompleted  CommandExecutionStatus = "completed"
	CommandExecutionStatusFailed     CommandExecutionStatus = "failed"
	CommandExecutionStatusDeclined   CommandExecutionStatus = "declined"
)

var validCommandExecutionStatuses = map[CommandExecutionStatus]struct{}{
	CommandExecutionStatusInProgress: {},
	CommandExecutionStatusCompleted:  {},
	CommandExecutionStatusFailed:     {},
	CommandExecutionStatusDeclined:   {},
}

func (s *CommandExecutionStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "commandExecution.status", validCommandExecutionStatuses, s)
}

// PatchApplyStatus represents the status of applying a code patch.
type PatchApplyStatus string

const (
	PatchApplyStatusInProgress PatchApplyStatus = "inProgress"
	PatchApplyStatusCompleted  PatchApplyStatus = "completed"
	PatchApplyStatusFailed     PatchApplyStatus = "failed"
	PatchApplyStatusDeclined   PatchApplyStatus = "declined"
)

var validPatchApplyStatuses = map[PatchApplyStatus]struct{}{
	PatchApplyStatusInProgress: {},
	PatchApplyStatusCompleted:  {},
	PatchApplyStatusFailed:     {},
	PatchApplyStatusDeclined:   {},
}

func (s *PatchApplyStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "fileChange.status", validPatchApplyStatuses, s)
}

// McpToolCallStatus represents the status of an MCP tool call.
type McpToolCallStatus string

const (
	McpToolCallStatusInProgress McpToolCallStatus = "inProgress"
	McpToolCallStatusCompleted  McpToolCallStatus = "completed"
	McpToolCallStatusFailed     McpToolCallStatus = "failed"
)

var validMcpToolCallStatuses = map[McpToolCallStatus]struct{}{
	McpToolCallStatusInProgress: {},
	McpToolCallStatusCompleted:  {},
	McpToolCallStatusFailed:     {},
}

func (s *McpToolCallStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "mcpToolCall.status", validMcpToolCallStatuses, s)
}

// DynamicToolCallStatus represents the status of a dynamic tool call.
type DynamicToolCallStatus string

const (
	DynamicToolCallStatusInProgress DynamicToolCallStatus = "inProgress"
	DynamicToolCallStatusCompleted  DynamicToolCallStatus = "completed"
	DynamicToolCallStatusFailed     DynamicToolCallStatus = "failed"
)

var validDynamicToolCallStatuses = map[DynamicToolCallStatus]struct{}{
	DynamicToolCallStatusInProgress: {},
	DynamicToolCallStatusCompleted:  {},
	DynamicToolCallStatusFailed:     {},
}

func (s *DynamicToolCallStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "dynamicToolCall.status", validDynamicToolCallStatuses, s)
}

// CollabAgentStatus represents the status of a collaboration agent.
type CollabAgentStatus string

const (
	CollabAgentStatusPendingInit CollabAgentStatus = "pendingInit"
	CollabAgentStatusRunning     CollabAgentStatus = "running"
	CollabAgentStatusInterrupted CollabAgentStatus = "interrupted"
	CollabAgentStatusCompleted   CollabAgentStatus = "completed"
	CollabAgentStatusErrored     CollabAgentStatus = "errored"
	CollabAgentStatusShutdown    CollabAgentStatus = "shutdown"
	CollabAgentStatusNotFound    CollabAgentStatus = "notFound"
)

var validCollabAgentStatuses = map[CollabAgentStatus]struct{}{
	CollabAgentStatusPendingInit: {},
	CollabAgentStatusRunning:     {},
	CollabAgentStatusInterrupted: {},
	CollabAgentStatusCompleted:   {},
	CollabAgentStatusErrored:     {},
	CollabAgentStatusShutdown:    {},
	CollabAgentStatusNotFound:    {},
}

func (s *CollabAgentStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "collabAgentState.status", validCollabAgentStatuses, s)
}

// CollabAgentTool represents the type of collaboration tool being invoked.
type CollabAgentTool string

const (
	CollabAgentToolSpawnAgent  CollabAgentTool = "spawnAgent"
	CollabAgentToolSendInput   CollabAgentTool = "sendInput"
	CollabAgentToolResumeAgent CollabAgentTool = "resumeAgent"
	CollabAgentToolWait        CollabAgentTool = "wait"
	CollabAgentToolCloseAgent  CollabAgentTool = "closeAgent"
)

var validCollabAgentTools = map[CollabAgentTool]struct{}{
	CollabAgentToolSpawnAgent:  {},
	CollabAgentToolSendInput:   {},
	CollabAgentToolResumeAgent: {},
	CollabAgentToolWait:        {},
	CollabAgentToolCloseAgent:  {},
}

func (t *CollabAgentTool) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "collabAgentToolCall.tool", validCollabAgentTools, t)
}

// CollabAgentToolCallStatus represents the status of a collab agent tool call.
type CollabAgentToolCallStatus string

const (
	CollabAgentToolCallStatusInProgress CollabAgentToolCallStatus = "inProgress"
	CollabAgentToolCallStatusCompleted  CollabAgentToolCallStatus = "completed"
	CollabAgentToolCallStatusFailed     CollabAgentToolCallStatus = "failed"
)

var validCollabAgentToolCallStatuses = map[CollabAgentToolCallStatus]struct{}{
	CollabAgentToolCallStatusInProgress: {},
	CollabAgentToolCallStatusCompleted:  {},
	CollabAgentToolCallStatusFailed:     {},
}

func (s *CollabAgentToolCallStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "collabAgentToolCall.status", validCollabAgentToolCallStatuses, s)
}

// CollabAgentState represents the current state of a collaboration agent.
type CollabAgentState struct {
	Status  CollabAgentStatus `json:"status"`
	Message *string           `json:"message,omitempty"`
}

func (s *CollabAgentState) UnmarshalJSON(data []byte) error {
	type wire CollabAgentState
	var decoded wire
	if err := unmarshalRequiredEventObject(data, &decoded, "status"); err != nil {
		return err
	}
	*s = CollabAgentState(decoded)
	return nil
}

// MemoryCitationEntry identifies a cited memory location.
type MemoryCitationEntry struct {
	LineEnd   uint32 `json:"lineEnd"`
	LineStart uint32 `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

// MemoryCitation contains memory citation entries referenced by a message.
type MemoryCitation struct {
	Entries   []MemoryCitationEntry `json:"entries"`
	ThreadIDs []string              `json:"threadIds"`
}

// CommandExecutionSource identifies what initiated a command execution.
type CommandExecutionSource string

const (
	CommandExecutionSourceAgent                  CommandExecutionSource = "agent"
	CommandExecutionSourceUserShell              CommandExecutionSource = "userShell"
	CommandExecutionSourceUnifiedExecStartup     CommandExecutionSource = "unifiedExecStartup"
	CommandExecutionSourceUnifiedExecInteraction CommandExecutionSource = "unifiedExecInteraction"
)

// FileUpdateChange represents a file change with diff and kind.
type FileUpdateChange struct {
	Path string                 `json:"path"`
	Diff string                 `json:"diff"`
	Kind PatchChangeKindWrapper `json:"kind"`
}

func (c *FileUpdateChange) UnmarshalJSON(data []byte) error {
	type wire FileUpdateChange
	var decoded wire
	if err := unmarshalRequiredEventObject(data, &decoded, "path", "diff", "kind"); err != nil {
		return err
	}
	*c = FileUpdateChange(decoded)
	return nil
}

// PatchChangeKind is a discriminated union for patch change types.
type PatchChangeKind interface {
	patchChangeKind()
}

// AddPatchChangeKind represents adding a new file.
type AddPatchChangeKind struct{}

func (AddPatchChangeKind) patchChangeKind() {}

func (a *AddPatchChangeKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
	}{Type: "add"})
}

// DeletePatchChangeKind represents deleting a file.
type DeletePatchChangeKind struct{}

func (DeletePatchChangeKind) patchChangeKind() {}

func (d *DeletePatchChangeKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
	}{Type: "delete"})
}

// UpdatePatchChangeKind represents updating a file (optionally moving it).
type UpdatePatchChangeKind struct {
	MovePath *string `json:"move_path,omitempty"`
}

func (UpdatePatchChangeKind) patchChangeKind() {}

func (u *UpdatePatchChangeKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type     string  `json:"type"`
		MovePath *string `json:"move_path,omitempty"`
	}{Type: "update", MovePath: u.MovePath})
}

// UnknownPatchChangeKind represents an unrecognized patch change type from a newer protocol version.
type UnknownPatchChangeKind struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (UnknownPatchChangeKind) patchChangeKind() {}

func (u *UnknownPatchChangeKind) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// PatchChangeKindWrapper wraps the PatchChangeKind discriminated union.
type PatchChangeKindWrapper struct {
	Value PatchChangeKind
}

func (w *PatchChangeKindWrapper) UnmarshalJSON(data []byte) error {
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
	case "add":
		w.Value = &AddPatchChangeKind{}
		return nil
	case "delete":
		w.Value = &DeletePatchChangeKind{}
		return nil
	case "update":
		var u UpdatePatchChangeKind
		if err := json.Unmarshal(data, &u); err != nil {
			return err
		}
		w.Value = &u
		return nil
	default:
		w.Value = &UnknownPatchChangeKind{Type: typeCheck.Type, Raw: append(json.RawMessage(nil), data...)}
		return nil
	}
}

func (w PatchChangeKindWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

// McpToolCallResult represents the result of an MCP tool call.
type McpToolCallResult struct {
	Content           []interface{} `json:"content"`
	StructuredContent interface{}   `json:"structuredContent,omitempty"`
}

func (r *McpToolCallResult) UnmarshalJSON(data []byte) error {
	type wire McpToolCallResult
	var decoded wire
	if err := unmarshalRequiredEventObject(data, &decoded, "content"); err != nil {
		return err
	}
	*r = McpToolCallResult(decoded)
	return nil
}

// McpToolCallError represents an error from an MCP tool call.
type McpToolCallError struct {
	Message string `json:"message"`
}

func (e *McpToolCallError) UnmarshalJSON(data []byte) error {
	type wire McpToolCallError
	var decoded wire
	if err := unmarshalRequiredEventObject(data, &decoded, "message"); err != nil {
		return err
	}
	*e = McpToolCallError(decoded)
	return nil
}

// WebSearchAction is a discriminated union for web search actions.
type WebSearchAction interface {
	webSearchAction()
}

// SearchWebSearchAction represents a search query action.
type SearchWebSearchAction struct {
	Query   *string   `json:"query,omitempty"`
	Queries *[]string `json:"queries,omitempty"`
}

func (SearchWebSearchAction) webSearchAction() {}

func (s *SearchWebSearchAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type    string    `json:"type"`
		Query   *string   `json:"query,omitempty"`
		Queries *[]string `json:"queries,omitempty"`
	}{Type: "search", Query: s.Query, Queries: s.Queries})
}

// OpenPageWebSearchAction represents opening a page.
type OpenPageWebSearchAction struct {
	URL *string `json:"url,omitempty"`
}

func (OpenPageWebSearchAction) webSearchAction() {}

func (o *OpenPageWebSearchAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string  `json:"type"`
		URL  *string `json:"url,omitempty"`
	}{Type: "openPage", URL: o.URL})
}

// FindInPageWebSearchAction represents finding text in a page.
type FindInPageWebSearchAction struct {
	URL     *string `json:"url,omitempty"`
	Pattern *string `json:"pattern,omitempty"`
}

func (FindInPageWebSearchAction) webSearchAction() {}

func (f *FindInPageWebSearchAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type    string  `json:"type"`
		URL     *string `json:"url,omitempty"`
		Pattern *string `json:"pattern,omitempty"`
	}{Type: "findInPage", URL: f.URL, Pattern: f.Pattern})
}

// OtherWebSearchAction represents an unspecified web search action.
type OtherWebSearchAction struct{}

func (OtherWebSearchAction) webSearchAction() {}

func (o *OtherWebSearchAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
	}{Type: "other"})
}

// UnknownWebSearchAction represents an unrecognized web search action type from a newer protocol version.
type UnknownWebSearchAction struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (UnknownWebSearchAction) webSearchAction() {}

func (u *UnknownWebSearchAction) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// WebSearchActionWrapper wraps the WebSearchAction discriminated union.
type WebSearchActionWrapper struct {
	Value WebSearchAction
}

func (w *WebSearchActionWrapper) UnmarshalJSON(data []byte) error {
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
	case "search":
		var s SearchWebSearchAction
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		w.Value = &s
		return nil
	case "openPage":
		var o OpenPageWebSearchAction
		if err := json.Unmarshal(data, &o); err != nil {
			return err
		}
		w.Value = &o
		return nil
	case "findInPage":
		var f FindInPageWebSearchAction
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		w.Value = &f
		return nil
	case "other":
		w.Value = &OtherWebSearchAction{}
		return nil
	default:
		w.Value = &UnknownWebSearchAction{Type: typeCheck.Type, Raw: append(json.RawMessage(nil), data...)}
		return nil
	}
}

func (w WebSearchActionWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

func unmarshalRequiredEventObject(data []byte, dest interface{}, requiredFields ...string) error {
	if err := validateRequiredObjectFields(data, requiredFields...); err != nil {
		return err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("unmarshal required event object: %w", err)
	}
	return nil
}
