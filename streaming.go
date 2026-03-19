package codex

import (
	"context"
	"encoding/json"
	"fmt"
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

func (n *AgentMessageDeltaNotification) UnmarshalJSON(data []byte) error {
	type wire AgentMessageDeltaNotification
	var decoded wire
	required := []string{"delta", "itemId", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = AgentMessageDeltaNotification(decoded)
	return nil
}

// FileChangeOutputDeltaNotification is sent when file change diff is streamed.
// Method: item/fileChange/outputDelta
type FileChangeOutputDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

func (n *FileChangeOutputDeltaNotification) UnmarshalJSON(data []byte) error {
	type wire FileChangeOutputDeltaNotification
	var decoded wire
	required := []string{"delta", "itemId", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = FileChangeOutputDeltaNotification(decoded)
	return nil
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

func (n *PlanDeltaNotification) UnmarshalJSON(data []byte) error {
	type wire PlanDeltaNotification
	var decoded wire
	required := []string{"delta", "itemId", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = PlanDeltaNotification(decoded)
	return nil
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

func (n *ReasoningTextDeltaNotification) UnmarshalJSON(data []byte) error {
	type wire ReasoningTextDeltaNotification
	var decoded wire
	required := []string{"contentIndex", "delta", "itemId", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ReasoningTextDeltaNotification(decoded)
	return nil
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

func (n *ReasoningSummaryTextDeltaNotification) UnmarshalJSON(data []byte) error {
	type wire ReasoningSummaryTextDeltaNotification
	var decoded wire
	required := []string{"delta", "itemId", "summaryIndex", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ReasoningSummaryTextDeltaNotification(decoded)
	return nil
}

// ReasoningSummaryPartAddedNotification is sent when a new reasoning summary part is added.
// Method: item/reasoning/summaryPartAdded
type ReasoningSummaryPartAddedNotification struct {
	ItemID       string `json:"itemId"`
	SummaryIndex int64  `json:"summaryIndex"` // Index of the newly added summary part
	ThreadID     string `json:"threadId"`
	TurnID       string `json:"turnId"`
}

func (n *ReasoningSummaryPartAddedNotification) UnmarshalJSON(data []byte) error {
	type wire ReasoningSummaryPartAddedNotification
	var decoded wire
	required := []string{"itemId", "summaryIndex", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ReasoningSummaryPartAddedNotification(decoded)
	return nil
}

// ItemStartedNotification is sent when a thread item starts.
// Method: item/started
type ItemStartedNotification struct {
	Item     ThreadItemWrapper `json:"item"`
	ThreadID string            `json:"threadId"`
	TurnID   string            `json:"turnId"`
}

func (n *ItemStartedNotification) UnmarshalJSON(data []byte) error {
	type wire ItemStartedNotification
	var decoded wire
	required := []string{"item", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ItemStartedNotification(decoded)
	return nil
}

// ItemCompletedNotification is sent when a thread item completes.
// Method: item/completed
type ItemCompletedNotification struct {
	Item     ThreadItemWrapper `json:"item"`
	ThreadID string            `json:"threadId"`
	TurnID   string            `json:"turnId"`
}

func (n *ItemCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire ItemCompletedNotification
	var decoded wire
	required := []string{"item", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ItemCompletedNotification(decoded)
	return nil
}

// Listener registration methods on Client

// OnAgentMessageDelta registers a listener for item/agentMessage/delta notifications.
func (c *Client) OnAgentMessageDelta(handler func(AgentMessageDeltaNotification)) {
	if handler == nil {
		c.OnNotification(notifyAgentMessageDelta, nil)
		return
	}
	c.OnNotification(notifyAgentMessageDelta, func(ctx context.Context, notif Notification) {
		var n AgentMessageDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyAgentMessageDelta, fmt.Errorf("unmarshal %s: %w", notifyAgentMessageDelta, err))
			return
		}
		handler(n)
	})
}

// OnFileChangeOutputDelta registers a listener for item/fileChange/outputDelta notifications.
func (c *Client) OnFileChangeOutputDelta(handler func(FileChangeOutputDeltaNotification)) {
	if handler == nil {
		c.OnNotification(notifyFileChangeOutputDelta, nil)
		return
	}
	c.OnNotification(notifyFileChangeOutputDelta, func(ctx context.Context, notif Notification) {
		var n FileChangeOutputDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyFileChangeOutputDelta, fmt.Errorf("unmarshal %s: %w", notifyFileChangeOutputDelta, err))
			return
		}
		handler(n)
	})
}

// OnPlanDelta registers a listener for item/plan/delta notifications.
func (c *Client) OnPlanDelta(handler func(PlanDeltaNotification)) {
	if handler == nil {
		c.OnNotification(notifyPlanDelta, nil)
		return
	}
	c.OnNotification(notifyPlanDelta, func(ctx context.Context, notif Notification) {
		var n PlanDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyPlanDelta, fmt.Errorf("unmarshal %s: %w", notifyPlanDelta, err))
			return
		}
		handler(n)
	})
}

// OnReasoningTextDelta registers a listener for item/reasoning/textDelta notifications.
func (c *Client) OnReasoningTextDelta(handler func(ReasoningTextDeltaNotification)) {
	if handler == nil {
		c.OnNotification(notifyReasoningTextDelta, nil)
		return
	}
	c.OnNotification(notifyReasoningTextDelta, func(ctx context.Context, notif Notification) {
		var n ReasoningTextDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyReasoningTextDelta, fmt.Errorf("unmarshal %s: %w", notifyReasoningTextDelta, err))
			return
		}
		handler(n)
	})
}

// OnReasoningSummaryTextDelta registers a listener for item/reasoning/summaryTextDelta notifications.
func (c *Client) OnReasoningSummaryTextDelta(handler func(ReasoningSummaryTextDeltaNotification)) {
	if handler == nil {
		c.OnNotification(notifyReasoningSummaryTextDelta, nil)
		return
	}
	c.OnNotification(notifyReasoningSummaryTextDelta, func(ctx context.Context, notif Notification) {
		var n ReasoningSummaryTextDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyReasoningSummaryTextDelta, fmt.Errorf("unmarshal %s: %w", notifyReasoningSummaryTextDelta, err))
			return
		}
		handler(n)
	})
}

// OnReasoningSummaryPartAdded registers a listener for item/reasoning/summaryPartAdded notifications.
func (c *Client) OnReasoningSummaryPartAdded(handler func(ReasoningSummaryPartAddedNotification)) {
	if handler == nil {
		c.OnNotification(notifyReasoningSummaryPartAdded, nil)
		return
	}
	c.OnNotification(notifyReasoningSummaryPartAdded, func(ctx context.Context, notif Notification) {
		var n ReasoningSummaryPartAddedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyReasoningSummaryPartAdded, fmt.Errorf("unmarshal %s: %w", notifyReasoningSummaryPartAdded, err))
			return
		}
		handler(n)
	})
}

// OnItemStarted registers a listener for item/started notifications.
func (c *Client) OnItemStarted(handler func(ItemStartedNotification)) {
	if handler == nil {
		c.OnNotification(notifyItemStarted, nil)
		return
	}
	c.OnNotification(notifyItemStarted, func(ctx context.Context, notif Notification) {
		var n ItemStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyItemStarted, fmt.Errorf("unmarshal %s: %w", notifyItemStarted, err))
			return
		}
		handler(n)
	})
}

// OnItemCompleted registers a listener for item/completed notifications.
func (c *Client) OnItemCompleted(handler func(ItemCompletedNotification)) {
	if handler == nil {
		c.OnNotification(notifyItemCompleted, nil)
		return
	}
	c.OnNotification(notifyItemCompleted, func(ctx context.Context, notif Notification) {
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyItemCompleted, fmt.Errorf("unmarshal %s: %w", notifyItemCompleted, err))
			return
		}
		handler(n)
	})
}

// OnCollabToolCallStarted registers an internal listener for item/started notifications
// that fires only when the item is a CollabAgentToolCallThreadItem. Uses
// addNotificationListener so it does not clobber existing OnItemStarted handlers.
// Returns a function that removes the listener.
func (c *Client) OnCollabToolCallStarted(handler func(ItemStartedNotification, *CollabAgentToolCallThreadItem)) func() {
	if handler == nil {
		return func() {}
	}
	return c.addNotificationListener(notifyItemStarted, func(_ context.Context, notif Notification) {
		var n ItemStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyItemStarted, fmt.Errorf("unmarshal %s: %w", notifyItemStarted, err))
			return
		}
		if collab, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
			handler(n, collab)
		}
	})
}

// OnCollabToolCallCompleted registers an internal listener for item/completed notifications
// that fires only when the item is a CollabAgentToolCallThreadItem. Uses
// addNotificationListener so it does not clobber existing OnItemCompleted handlers.
// Returns a function that removes the listener.
func (c *Client) OnCollabToolCallCompleted(handler func(ItemCompletedNotification, *CollabAgentToolCallThreadItem)) func() {
	if handler == nil {
		return func() {}
	}
	return c.addNotificationListener(notifyItemCompleted, func(_ context.Context, notif Notification) {
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyItemCompleted, fmt.Errorf("unmarshal %s: %w", notifyItemCompleted, err))
			return
		}
		if collab, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
			handler(n, collab)
		}
	})
}
