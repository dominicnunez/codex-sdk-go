package codex

import (
	"context"
	"encoding/json"
)

// Streaming Notifications
// These are server→client notifications for streaming turn events.

// AgentMessageDeltaNotification is sent when agent message text is streamed.
// Method: item/agentMessage/delta
type AgentMessageDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

// FileChangeOutputDeltaNotification is sent when file change diff is streamed.
// Method: item/fileChange/outputDelta
type FileChangeOutputDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

// PlanDeltaNotification is sent when plan text is streamed.
// Method: item/plan/delta
// EXPERIMENTAL - proposed plan streaming deltas for plan items.
// Clients should not assume concatenated deltas match the completed plan item content.
type PlanDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

// ReasoningTextDeltaNotification is sent when reasoning content text is streamed.
// Method: item/reasoning/textDelta
type ReasoningTextDeltaNotification struct {
	ContentIndex int64  `json:"contentIndex"` // Index within the reasoning content array
	Delta        string `json:"delta"`
	ItemID       string `json:"itemId"`
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
}

// ReasoningSummaryTextDeltaNotification is sent when reasoning summary text is streamed.
// Method: item/reasoning/summaryTextDelta
type ReasoningSummaryTextDeltaNotification struct {
	Delta        string `json:"delta"`
	ItemID       string `json:"itemId"`
	SummaryIndex int64  `json:"summaryIndex"` // Index within the reasoning summary array
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
}

// ReasoningSummaryPartAddedNotification is sent when a new reasoning summary part is added.
// Method: item/reasoning/summaryPartAdded
type ReasoningSummaryPartAddedNotification struct {
	ItemID       string `json:"itemId"`
	SummaryIndex int64  `json:"summaryIndex"` // Index of the newly added summary part
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
}

// ItemStartedNotification is sent when a thread item starts.
// Method: item/started
// The Item field contains a ThreadItem discriminated union with many variants.
// For simplicity, we use json.RawMessage to avoid defining all variants here.
// Users can unmarshal Item to their own types if needed.
type ItemStartedNotification struct {
	Item     json.RawMessage `json:"item"` // ThreadItem discriminated union (userMessage, agentMessage, plan, reasoning, commandExecution, fileChange, mcpToolCall, dynamicToolCall, collabAgentToolCall, webSearch, imageView, enteredReviewMode, exitedReviewMode, contextCompaction)
	ThreadID string          `json:"threadId"`
	TurnID   string          `json:"turnId"`
}

// ItemCompletedNotification is sent when a thread item completes.
// Method: item/completed
// The Item field contains a ThreadItem discriminated union with many variants.
type ItemCompletedNotification struct {
	Item     json.RawMessage `json:"item"` // ThreadItem discriminated union
	ThreadID string          `json:"threadId"`
	TurnID   string          `json:"turnId"`
}

// Listener registration methods on Client

// OnAgentMessageDelta registers a listener for item/agentMessage/delta notifications.
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

// OnFileChangeOutputDelta registers a listener for item/fileChange/outputDelta notifications.
func (c *Client) OnFileChangeOutputDelta(handler func(FileChangeOutputDeltaNotification)) {
	c.OnNotification("item/fileChange/outputDelta", func(ctx context.Context, notif Notification) {
		var n FileChangeOutputDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnPlanDelta registers a listener for item/plan/delta notifications.
func (c *Client) OnPlanDelta(handler func(PlanDeltaNotification)) {
	c.OnNotification("item/plan/delta", func(ctx context.Context, notif Notification) {
		var n PlanDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnReasoningTextDelta registers a listener for item/reasoning/textDelta notifications.
func (c *Client) OnReasoningTextDelta(handler func(ReasoningTextDeltaNotification)) {
	c.OnNotification("item/reasoning/textDelta", func(ctx context.Context, notif Notification) {
		var n ReasoningTextDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnReasoningSummaryTextDelta registers a listener for item/reasoning/summaryTextDelta notifications.
func (c *Client) OnReasoningSummaryTextDelta(handler func(ReasoningSummaryTextDeltaNotification)) {
	c.OnNotification("item/reasoning/summaryTextDelta", func(ctx context.Context, notif Notification) {
		var n ReasoningSummaryTextDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnReasoningSummaryPartAdded registers a listener for item/reasoning/summaryPartAdded notifications.
func (c *Client) OnReasoningSummaryPartAdded(handler func(ReasoningSummaryPartAddedNotification)) {
	c.OnNotification("item/reasoning/summaryPartAdded", func(ctx context.Context, notif Notification) {
		var n ReasoningSummaryPartAddedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnItemStarted registers a listener for item/started notifications.
func (c *Client) OnItemStarted(handler func(ItemStartedNotification)) {
	c.OnNotification("item/started", func(ctx context.Context, notif Notification) {
		var n ItemStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnItemCompleted registers a listener for item/completed notifications.
func (c *Client) OnItemCompleted(handler func(ItemCompletedNotification)) {
	c.OnNotification("item/completed", func(ctx context.Context, notif Notification) {
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

