package codex

import (
	"context"
	"encoding/json"
)

// ===== Turn Started Notification =====

// TurnStartedNotification is the notification when a turn starts
type TurnStartedNotification struct {
	ThreadID string `json:"threadId"`
	Turn     Turn   `json:"turn"`
}

// OnTurnStarted registers a listener for turn/started notifications
func (c *Client) OnTurnStarted(handler func(TurnStartedNotification)) {
	c.OnNotification("turn/started", func(ctx context.Context, notif Notification) {
		var params TurnStartedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			return
		}
		handler(params)
	})
}

// ===== Turn Completed Notification =====

// TurnCompletedNotification is the notification when a turn completes
type TurnCompletedNotification struct {
	ThreadID string `json:"threadId"`
	Turn     Turn   `json:"turn"`
}

// OnTurnCompleted registers a listener for turn/completed notifications
func (c *Client) OnTurnCompleted(handler func(TurnCompletedNotification)) {
	c.OnNotification("turn/completed", func(ctx context.Context, notif Notification) {
		var params TurnCompletedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			return
		}
		handler(params)
	})
}

// ===== Turn Plan Updated Notification =====

// TurnPlanUpdatedNotification is the notification when a turn's plan updates
type TurnPlanUpdatedNotification struct {
	ThreadID    string         `json:"threadId"`
	TurnID      string         `json:"turnId"`
	Plan        []TurnPlanStep `json:"plan"`
	Explanation *string        `json:"explanation,omitempty"`
}

// TurnPlanStep represents a step in a turn plan
type TurnPlanStep struct {
	Step   string `json:"step"`
	Status string `json:"status"` // "pending" | "inProgress" | "completed"
}

// OnTurnPlanUpdated registers a listener for turn/planUpdated notifications
func (c *Client) OnTurnPlanUpdated(handler func(TurnPlanUpdatedNotification)) {
	c.OnNotification("turn/planUpdated", func(ctx context.Context, notif Notification) {
		var params TurnPlanUpdatedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			return
		}
		handler(params)
	})
}

// ===== Turn Diff Updated Notification =====

// TurnDiffUpdatedNotification is the notification when a turn's diff updates
type TurnDiffUpdatedNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	Diff     string `json:"diff"`
}

// OnTurnDiffUpdated registers a listener for turn/diffUpdated notifications
func (c *Client) OnTurnDiffUpdated(handler func(TurnDiffUpdatedNotification)) {
	c.OnNotification("turn/diffUpdated", func(ctx context.Context, notif Notification) {
		var params TurnDiffUpdatedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			return
		}
		handler(params)
	})
}
