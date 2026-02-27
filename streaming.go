package codex

import (
	"context"
	"encoding/json"
)

// Streaming Notifications
// These are server→client notifications for streaming turn events.

// AgentMessageDeltaNotification is sent when agent message text is streamed.
// Method: agent/messageDelta
type AgentMessageDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

// FileChangeOutputDeltaNotification is sent when file change diff is streamed.
// Method: turn/fileChangeOutputDelta
type FileChangeOutputDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

// PlanDeltaNotification is sent when plan text is streamed.
// Method: turn/planDelta
// EXPERIMENTAL - proposed plan streaming deltas for plan items.
// Clients should not assume concatenated deltas match the completed plan item content.
type PlanDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

// ReasoningTextDeltaNotification is sent when reasoning content text is streamed.
// Method: turn/reasoningTextDelta
type ReasoningTextDeltaNotification struct {
	ContentIndex int64  `json:"contentIndex"` // Index within the reasoning content array
	Delta        string `json:"delta"`
	ItemID       string `json:"itemId"`
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
}

// ReasoningSummaryTextDeltaNotification is sent when reasoning summary text is streamed.
// Method: turn/reasoningSummaryTextDelta
type ReasoningSummaryTextDeltaNotification struct {
	Delta        string `json:"delta"`
	ItemID       string `json:"itemId"`
	SummaryIndex int64  `json:"summaryIndex"` // Index within the reasoning summary array
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
}

// ReasoningSummaryPartAddedNotification is sent when a new reasoning summary part is added.
// Method: turn/reasoningSummaryPartAdded
type ReasoningSummaryPartAddedNotification struct {
	ItemID       string `json:"itemId"`
	SummaryIndex int64  `json:"summaryIndex"` // Index of the newly added summary part
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
}

// ItemStartedNotification is sent when a thread item starts.
// Method: turn/itemStarted
// The Item field contains a ThreadItem discriminated union with many variants.
// For simplicity, we use json.RawMessage to avoid defining all variants here.
// Users can unmarshal Item to their own types if needed.
type ItemStartedNotification struct {
	Item     json.RawMessage `json:"item"` // ThreadItem discriminated union (userMessage, agentMessage, plan, reasoning, commandExecution, fileChange, mcpToolCall, dynamicToolCall, collabAgentToolCall, webSearch, imageView, enteredReviewMode, exitedReviewMode, contextCompaction)
	ThreadID string          `json:"threadId"`
	TurnID   string          `json:"turnId"`
}

// ItemCompletedNotification is sent when a thread item completes.
// Method: turn/itemCompleted
// The Item field contains a ThreadItem discriminated union with many variants.
type ItemCompletedNotification struct {
	Item     json.RawMessage `json:"item"` // ThreadItem discriminated union
	ThreadID string          `json:"threadId"`
	TurnID   string          `json:"turnId"`
}

// RawResponseItemCompletedNotification is sent when a raw response item completes.
// Method: turn/rawResponseItemCompleted
// The Item field contains a ResponseItem discriminated union from the Responses API.
// Response items include: message, reasoning, local_shell_call, function_call, function_call_output,
// custom_tool_call, custom_tool_call_output, web_search_call, ghost_snapshot, compaction, other.
type RawResponseItemCompletedNotification struct {
	Item     json.RawMessage `json:"item"` // ResponseItem discriminated union
	ThreadID string          `json:"threadId"`
	TurnID   string          `json:"turnId"`
}

// Listener registration methods on Client

// OnAgentMessageDelta registers a listener for agent/messageDelta notifications.
func (c *Client) OnAgentMessageDelta(handler func(AgentMessageDeltaNotification)) {
	c.OnNotification("item/agentMessage/delta", func(ctx context.Context, notif Notification) {
		var n AgentMessageDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			// Silently ignore unmarshal errors (notification is malformed)
			return
		}
		handler(n)
	})
}

// OnFileChangeOutputDelta registers a listener for turn/fileChangeOutputDelta notifications.
func (c *Client) OnFileChangeOutputDelta(handler func(FileChangeOutputDeltaNotification)) {
	c.OnNotification("item/fileChange/outputDelta", func(ctx context.Context, notif Notification) {
		var n FileChangeOutputDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnPlanDelta registers a listener for turn/planDelta notifications.
func (c *Client) OnPlanDelta(handler func(PlanDeltaNotification)) {
	c.OnNotification("item/plan/delta", func(ctx context.Context, notif Notification) {
		var n PlanDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnReasoningTextDelta registers a listener for turn/reasoningTextDelta notifications.
func (c *Client) OnReasoningTextDelta(handler func(ReasoningTextDeltaNotification)) {
	c.OnNotification("item/reasoning/textDelta", func(ctx context.Context, notif Notification) {
		var n ReasoningTextDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnReasoningSummaryTextDelta registers a listener for turn/reasoningSummaryTextDelta notifications.
func (c *Client) OnReasoningSummaryTextDelta(handler func(ReasoningSummaryTextDeltaNotification)) {
	c.OnNotification("item/reasoning/summaryTextDelta", func(ctx context.Context, notif Notification) {
		var n ReasoningSummaryTextDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnReasoningSummaryPartAdded registers a listener for turn/reasoningSummaryPartAdded notifications.
func (c *Client) OnReasoningSummaryPartAdded(handler func(ReasoningSummaryPartAddedNotification)) {
	c.OnNotification("item/reasoning/summaryPartAdded", func(ctx context.Context, notif Notification) {
		var n ReasoningSummaryPartAddedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnItemStarted registers a listener for turn/itemStarted notifications.
func (c *Client) OnItemStarted(handler func(ItemStartedNotification)) {
	c.OnNotification("item/started", func(ctx context.Context, notif Notification) {
		var n ItemStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnItemCompleted registers a listener for turn/itemCompleted notifications.
func (c *Client) OnItemCompleted(handler func(ItemCompletedNotification)) {
	c.OnNotification("item/completed", func(ctx context.Context, notif Notification) {
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnRawResponseItemCompleted registers a listener for turn/rawResponseItemCompleted notifications.
func (c *Client) OnRawResponseItemCompleted(handler func(RawResponseItemCompletedNotification)) {
	c.OnNotification("item/rawResponseItemCompleted", func(ctx context.Context, notif Notification) {
		var n RawResponseItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}
