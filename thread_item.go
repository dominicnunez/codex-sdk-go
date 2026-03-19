package codex

import (
	"encoding/json"
)

// ThreadItem is a discriminated union for thread item variants.
// The "type" field determines which concrete variant is represented.
type ThreadItem interface {
	threadItem()
}

// UserMessageThreadItem represents a user message in a thread.
type UserMessageThreadItem struct {
	ID      string      `json:"id"`
	Content []UserInput `json:"content"`
}

func (UserMessageThreadItem) threadItem() {}

func (u *UserMessageThreadItem) MarshalJSON() ([]byte, error) {
	items := make([]json.RawMessage, len(u.Content))
	for i, input := range u.Content {
		b, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		items[i] = b
	}
	contentBytes, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Type    string          `json:"type"`
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
	}{
		Type:    "userMessage",
		ID:      u.ID,
		Content: contentBytes,
	})
}

// AgentMessageThreadItem represents an agent message in a thread.
type AgentMessageThreadItem struct {
	ID    string        `json:"id"`
	Text  string        `json:"text"`
	Phase *MessagePhase `json:"phase,omitempty"`
}

func (AgentMessageThreadItem) threadItem() {}

func (a *AgentMessageThreadItem) MarshalJSON() ([]byte, error) {
	type Alias AgentMessageThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "agentMessage",
		Alias: (*Alias)(a),
	})
}

// PlanThreadItem represents a plan in a thread.
type PlanThreadItem struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func (PlanThreadItem) threadItem() {}

func (p *PlanThreadItem) MarshalJSON() ([]byte, error) {
	type Alias PlanThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "plan",
		Alias: (*Alias)(p),
	})
}

// ReasoningThreadItem represents reasoning content in a thread.
type ReasoningThreadItem struct {
	ID      string   `json:"id"`
	Content []string `json:"content,omitempty"`
	Summary []string `json:"summary,omitempty"`
}

func (ReasoningThreadItem) threadItem() {}

func (r *ReasoningThreadItem) MarshalJSON() ([]byte, error) {
	type Alias ReasoningThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "reasoning",
		Alias: (*Alias)(r),
	})
}

// CommandExecutionThreadItem represents a command execution in a thread.
type CommandExecutionThreadItem struct {
	ID               string                 `json:"id"`
	Command          string                 `json:"command"`
	CommandActions   []CommandActionWrapper `json:"commandActions"`
	Cwd              string                 `json:"cwd"`
	Status           CommandExecutionStatus `json:"status"`
	AggregatedOutput *string                `json:"aggregatedOutput,omitempty"`
	DurationMs       *int64                 `json:"durationMs,omitempty"`
	ExitCode         *int32                 `json:"exitCode,omitempty"`
	ProcessId        *string                `json:"processId,omitempty"`
}

func (CommandExecutionThreadItem) threadItem() {}

func (c *CommandExecutionThreadItem) MarshalJSON() ([]byte, error) {
	type Alias CommandExecutionThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "commandExecution",
		Alias: (*Alias)(c),
	})
}

// FileChangeThreadItem represents a file change in a thread.
type FileChangeThreadItem struct {
	ID      string             `json:"id"`
	Changes []FileUpdateChange `json:"changes"`
	Status  PatchApplyStatus   `json:"status"`
}

func (FileChangeThreadItem) threadItem() {}

func (f *FileChangeThreadItem) MarshalJSON() ([]byte, error) {
	type Alias FileChangeThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "fileChange",
		Alias: (*Alias)(f),
	})
}

// McpToolCallThreadItem represents an MCP tool call in a thread.
type McpToolCallThreadItem struct {
	ID         string             `json:"id"`
	Server     string             `json:"server"`
	Tool       string             `json:"tool"`
	Status     McpToolCallStatus  `json:"status"`
	Arguments  interface{}        `json:"arguments"`
	Result     *McpToolCallResult `json:"result,omitempty"`
	Error      *McpToolCallError  `json:"error,omitempty"`
	DurationMs *int64             `json:"durationMs,omitempty"`
}

func (McpToolCallThreadItem) threadItem() {}

func (m *McpToolCallThreadItem) MarshalJSON() ([]byte, error) {
	type Alias McpToolCallThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "mcpToolCall",
		Alias: (*Alias)(m),
	})
}

// DynamicToolCallThreadItem represents a dynamic tool call in a thread.
type DynamicToolCallThreadItem struct {
	ID           string                                    `json:"id"`
	Tool         string                                    `json:"tool"`
	Status       DynamicToolCallStatus                     `json:"status"`
	Arguments    interface{}                               `json:"arguments"`
	ContentItems []DynamicToolCallOutputContentItemWrapper `json:"contentItems,omitempty"`
	Success      *bool                                     `json:"success,omitempty"`
	DurationMs   *int64                                    `json:"durationMs,omitempty"`
}

func (DynamicToolCallThreadItem) threadItem() {}

func (d *DynamicToolCallThreadItem) MarshalJSON() ([]byte, error) {
	type Alias DynamicToolCallThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "dynamicToolCall",
		Alias: (*Alias)(d),
	})
}

// CollabAgentToolCallThreadItem represents a collaboration agent tool call in a thread.
type CollabAgentToolCallThreadItem struct {
	ID                string                      `json:"id"`
	Tool              CollabAgentTool             `json:"tool"`
	Status            CollabAgentToolCallStatus   `json:"status"`
	AgentsStates      map[string]CollabAgentState `json:"agentsStates"`
	Model             *string                     `json:"model,omitempty"`
	ReceiverThreadIds []string                    `json:"receiverThreadIds"`
	ReasoningEffort   *ReasoningEffort            `json:"reasoningEffort,omitempty"`
	SenderThreadId    string                      `json:"senderThreadId"`
	Prompt            *string                     `json:"prompt,omitempty"`
}

func (CollabAgentToolCallThreadItem) threadItem() {}

func (c *CollabAgentToolCallThreadItem) MarshalJSON() ([]byte, error) {
	type Alias CollabAgentToolCallThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "collabAgentToolCall",
		Alias: (*Alias)(c),
	})
}

// WebSearchThreadItem represents a web search in a thread.
type WebSearchThreadItem struct {
	ID     string                  `json:"id"`
	Query  string                  `json:"query"`
	Action *WebSearchActionWrapper `json:"action,omitempty"`
}

func (WebSearchThreadItem) threadItem() {}

func (w *WebSearchThreadItem) MarshalJSON() ([]byte, error) {
	type Alias WebSearchThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "webSearch",
		Alias: (*Alias)(w),
	})
}

// ImageViewThreadItem represents an image view in a thread.
type ImageViewThreadItem struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func (ImageViewThreadItem) threadItem() {}

func (i *ImageViewThreadItem) MarshalJSON() ([]byte, error) {
	type Alias ImageViewThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "imageView",
		Alias: (*Alias)(i),
	})
}

// EnteredReviewModeThreadItem represents entering review mode in a thread.
type EnteredReviewModeThreadItem struct {
	ID     string `json:"id"`
	Review string `json:"review"`
}

func (EnteredReviewModeThreadItem) threadItem() {}

func (e *EnteredReviewModeThreadItem) MarshalJSON() ([]byte, error) {
	type Alias EnteredReviewModeThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "enteredReviewMode",
		Alias: (*Alias)(e),
	})
}

// ExitedReviewModeThreadItem represents exiting review mode in a thread.
type ExitedReviewModeThreadItem struct {
	ID     string `json:"id"`
	Review string `json:"review"`
}

func (ExitedReviewModeThreadItem) threadItem() {}

func (e *ExitedReviewModeThreadItem) MarshalJSON() ([]byte, error) {
	type Alias ExitedReviewModeThreadItem
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "exitedReviewMode",
		Alias: (*Alias)(e),
	})
}

// ContextCompactionThreadItem represents a context compaction event in a thread.
type ContextCompactionThreadItem struct {
	ID string `json:"id"`
}

func (ContextCompactionThreadItem) threadItem() {}

func (c *ContextCompactionThreadItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}{
		Type: "contextCompaction",
		ID:   c.ID,
	})
}

// UnmarshalErrorItemType is the Type value assigned to synthetic UnknownThreadItem
// entries created when a notification payload fails to unmarshal. Callers can check
// this value to detect items that represent parse failures rather than real thread items.
const UnmarshalErrorItemType = "unmarshal_error"

// UnknownThreadItem represents an unrecognized thread item type from a newer protocol version.
type UnknownThreadItem struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (UnknownThreadItem) threadItem() {}

func (u *UnknownThreadItem) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

type threadItemDecoder func([]byte) (ThreadItem, error)

var threadItemDecoders = map[string]threadItemDecoder{
	"userMessage":         decodeUserMessageThreadItem,
	"agentMessage":        decodeAgentMessageThreadItem,
	"plan":                decodePlanThreadItem,
	"reasoning":           decodeReasoningThreadItem,
	"commandExecution":    decodeCommandExecutionThreadItem,
	"fileChange":          decodeFileChangeThreadItem,
	"mcpToolCall":         decodeMcpToolCallThreadItem,
	"dynamicToolCall":     decodeDynamicToolCallThreadItem,
	"collabAgentToolCall": decodeCollabAgentToolCallThreadItem,
	"webSearch":           decodeWebSearchThreadItem,
	"imageView":           decodeImageViewThreadItem,
	"enteredReviewMode":   decodeEnteredReviewModeThreadItem,
	"exitedReviewMode":    decodeExitedReviewModeThreadItem,
	"contextCompaction":   decodeContextCompactionThreadItem,
}

func decodeThreadItemInto(data []byte, dest ThreadItem, requiredFields ...string) (ThreadItem, error) {
	return decodeThreadItemIntoWithValidation(data, dest, requiredFields, requiredFields)
}

func decodeThreadItemIntoWithValidation(
	data []byte,
	dest ThreadItem,
	requiredFields []string,
	nonNullFields []string,
) (ThreadItem, error) {
	if err := validateTaggedObjectFields(data, requiredFields, nonNullFields); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return nil, err
	}
	return dest, nil
}

func decodeUserMessageThreadItem(data []byte) (ThreadItem, error) {
	var raw struct {
		ID      string            `json:"id"`
		Content []json.RawMessage `json:"content"`
	}
	if err := validateRequiredTaggedObjectFields(data, "id", "content"); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	inputs, err := unmarshalUserInputSlice(raw.Content)
	if err != nil {
		return nil, err
	}
	return &UserMessageThreadItem{ID: raw.ID, Content: inputs}, nil
}

func decodeAgentMessageThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &AgentMessageThreadItem{}, "id", "text")
}

func decodePlanThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &PlanThreadItem{}, "id", "text")
}

func decodeReasoningThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &ReasoningThreadItem{}, "id")
}

func decodeCommandExecutionThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &CommandExecutionThreadItem{}, "command", "commandActions", "cwd", "id", "status")
}

func decodeFileChangeThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &FileChangeThreadItem{}, "changes", "id", "status")
}

func decodeMcpToolCallThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemIntoWithValidation(
		data,
		&McpToolCallThreadItem{},
		[]string{"arguments", "id", "server", "status", "tool"},
		[]string{"id", "server", "status", "tool"},
	)
}

func decodeDynamicToolCallThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemIntoWithValidation(
		data,
		&DynamicToolCallThreadItem{},
		[]string{"arguments", "id", "status", "tool"},
		[]string{"id", "status", "tool"},
	)
}

func decodeCollabAgentToolCallThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &CollabAgentToolCallThreadItem{}, "agentsStates", "id", "receiverThreadIds", "senderThreadId", "status", "tool")
}

func decodeWebSearchThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &WebSearchThreadItem{}, "id", "query")
}

func decodeImageViewThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &ImageViewThreadItem{}, "id", "path")
}

func decodeEnteredReviewModeThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &EnteredReviewModeThreadItem{}, "id", "review")
}

func decodeExitedReviewModeThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &ExitedReviewModeThreadItem{}, "id", "review")
}

func decodeContextCompactionThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &ContextCompactionThreadItem{}, "id")
}

// ThreadItemWrapper wraps the ThreadItem discriminated union for JSON marshaling/unmarshaling.
type ThreadItemWrapper struct {
	Value ThreadItem
}

func (w *ThreadItemWrapper) UnmarshalJSON(data []byte) error {
	typeField, err := decodeRequiredObjectTypeField(data, "thread item")
	if err != nil {
		return err
	}

	decoder, ok := threadItemDecoders[typeField]
	if !ok {
		w.Value = &UnknownThreadItem{Type: typeField, Raw: append(json.RawMessage(nil), data...)}
		return nil
	}

	value, err := decoder(data)
	if err != nil {
		return err
	}
	w.Value = value
	return nil
}

func (w ThreadItemWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

// IsCollabToolCall returns true if the item is a CollabAgentToolCallThreadItem.
func (w ThreadItemWrapper) IsCollabToolCall() bool {
	_, ok := w.Value.(*CollabAgentToolCallThreadItem)
	return ok
}

// CollabToolCall returns the underlying CollabAgentToolCallThreadItem, or nil.
func (w ThreadItemWrapper) CollabToolCall() *CollabAgentToolCallThreadItem {
	c, _ := w.Value.(*CollabAgentToolCallThreadItem)
	return c
}
