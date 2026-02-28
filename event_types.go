package codex

import (
	"encoding/json"
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

// TextElement represents a span within text used to render or persist special elements.
// Used in UserInput to mark UI-defined elements within the text.
type TextElement struct {
	ByteRange   ByteRange `json:"byteRange"`             // Byte range in the parent text buffer
	Placeholder *string   `json:"placeholder,omitempty"` // Optional human-readable placeholder for UI
}

// MessagePhase classifies an assistant message as interim commentary or final answer text.
// Providers do not emit this consistently, so None/null should be treated as "phase unknown".
type MessagePhase string

const (
	MessagePhaseCommentary  MessagePhase = "commentary"   // Mid-turn assistant text (preamble/progress narration)
	MessagePhaseFinalAnswer MessagePhase = "final_answer" // Terminal answer text for the current turn
)

// CommandExecutionStatus represents the status of a command execution.
type CommandExecutionStatus string

const (
	CommandExecutionStatusInProgress CommandExecutionStatus = "inProgress"
	CommandExecutionStatusCompleted  CommandExecutionStatus = "completed"
	CommandExecutionStatusFailed     CommandExecutionStatus = "failed"
	CommandExecutionStatusDeclined   CommandExecutionStatus = "declined"
)

// PatchApplyStatus represents the status of applying a code patch.
type PatchApplyStatus string

const (
	PatchApplyStatusInProgress PatchApplyStatus = "inProgress"
	PatchApplyStatusCompleted  PatchApplyStatus = "completed"
	PatchApplyStatusFailed     PatchApplyStatus = "failed"
	PatchApplyStatusDeclined   PatchApplyStatus = "declined"
)

// McpToolCallStatus represents the status of an MCP tool call.
type McpToolCallStatus string

const (
	McpToolCallStatusInProgress McpToolCallStatus = "inProgress"
	McpToolCallStatusCompleted  McpToolCallStatus = "completed"
	McpToolCallStatusFailed     McpToolCallStatus = "failed"
)

// DynamicToolCallStatus represents the status of a dynamic tool call.
type DynamicToolCallStatus string

const (
	DynamicToolCallStatusInProgress DynamicToolCallStatus = "inProgress"
	DynamicToolCallStatusCompleted  DynamicToolCallStatus = "completed"
	DynamicToolCallStatusFailed     DynamicToolCallStatus = "failed"
)

// CollabAgentStatus represents the status of a collaboration agent.
type CollabAgentStatus string

const (
	CollabAgentStatusPendingInit CollabAgentStatus = "pendingInit"
	CollabAgentStatusRunning     CollabAgentStatus = "running"
	CollabAgentStatusCompleted   CollabAgentStatus = "completed"
	CollabAgentStatusErrored     CollabAgentStatus = "errored"
	CollabAgentStatusShutdown    CollabAgentStatus = "shutdown"
	CollabAgentStatusNotFound    CollabAgentStatus = "notFound"
)

// CollabAgentTool represents the type of collaboration tool being invoked.
type CollabAgentTool string

const (
	CollabAgentToolSpawnAgent  CollabAgentTool = "spawnAgent"
	CollabAgentToolSendInput   CollabAgentTool = "sendInput"
	CollabAgentToolResumeAgent CollabAgentTool = "resumeAgent"
	CollabAgentToolWait        CollabAgentTool = "wait"
	CollabAgentToolCloseAgent  CollabAgentTool = "closeAgent"
)

// CollabAgentToolCallStatus represents the status of a collab agent tool call.
type CollabAgentToolCallStatus string

const (
	CollabAgentToolCallStatusInProgress CollabAgentToolCallStatus = "inProgress"
	CollabAgentToolCallStatusCompleted  CollabAgentToolCallStatus = "completed"
	CollabAgentToolCallStatusFailed     CollabAgentToolCallStatus = "failed"
)

// CollabAgentState represents the current state of a collaboration agent.
type CollabAgentState struct {
	Status  CollabAgentStatus `json:"status"`
	Message *string           `json:"message,omitempty"`
}

// FileUpdateChange represents a file change with diff and kind.
type FileUpdateChange struct {
	Path string                 `json:"path"`
	Diff string                 `json:"diff"`
	Kind PatchChangeKindWrapper `json:"kind"`
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

// McpToolCallError represents an error from an MCP tool call.
type McpToolCallError struct {
	Message string `json:"message"`
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
