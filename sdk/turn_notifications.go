package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// ===== Turn Started Notification =====

// TurnStartedNotification is the notification when a turn starts
type TurnStartedNotification struct {
	ThreadID string `json:"threadId"`
	Turn     Turn   `json:"turn"`
}

func (n *TurnStartedNotification) UnmarshalJSON(data []byte) error {
	type wire TurnStartedNotification
	var decoded wire
	required := []string{"threadId", "turn"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = TurnStartedNotification(decoded)
	return nil
}

// OnTurnStarted registers a listener for turn/started notifications
func (c *Client) OnTurnStarted(handler func(TurnStartedNotification)) {
	if handler == nil {
		c.OnNotification(notifyTurnStarted, nil)
		return
	}
	c.OnNotification(notifyTurnStarted, func(ctx context.Context, notif Notification) {
		var params TurnStartedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyTurnStarted, fmt.Errorf("unmarshal %s: %w", notifyTurnStarted, err))
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

func (n *TurnCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire TurnCompletedNotification
	var decoded wire
	required := []string{"threadId", "turn"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = TurnCompletedNotification(decoded)
	return nil
}

// OnTurnCompleted registers a listener for turn/completed notifications
func (c *Client) OnTurnCompleted(handler func(TurnCompletedNotification)) {
	if handler == nil {
		c.OnNotification(notifyTurnCompleted, nil)
		return
	}
	c.OnNotification(notifyTurnCompleted, func(ctx context.Context, notif Notification) {
		var params TurnCompletedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyTurnCompleted, fmt.Errorf("unmarshal %s: %w", notifyTurnCompleted, err))
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

func (n *TurnPlanUpdatedNotification) UnmarshalJSON(data []byte) error {
	type wire TurnPlanUpdatedNotification
	var decoded wire
	required := []string{"plan", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = TurnPlanUpdatedNotification(decoded)
	return nil
}

// TurnPlanStepStatus represents the status of a plan step.
type TurnPlanStepStatus string

const (
	TurnPlanStepStatusPending    TurnPlanStepStatus = "pending"
	TurnPlanStepStatusInProgress TurnPlanStepStatus = "inProgress"
	TurnPlanStepStatusCompleted  TurnPlanStepStatus = "completed"
)

var validTurnPlanStepStatuses = map[TurnPlanStepStatus]struct{}{
	TurnPlanStepStatusPending:    {},
	TurnPlanStepStatusInProgress: {},
	TurnPlanStepStatusCompleted:  {},
}

func (s *TurnPlanStepStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "turn.plan.status", validTurnPlanStepStatuses, s)
}

// TurnPlanStep represents a step in a turn plan
type TurnPlanStep struct {
	Step   string             `json:"step"`
	Status TurnPlanStepStatus `json:"status"`
}

func (s *TurnPlanStep) UnmarshalJSON(data []byte) error {
	type wire TurnPlanStep
	var decoded wire
	required := []string{"status", "step"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*s = TurnPlanStep(decoded)
	return nil
}

// OnTurnPlanUpdated registers a listener for turn/plan/updated notifications
func (c *Client) OnTurnPlanUpdated(handler func(TurnPlanUpdatedNotification)) {
	if handler == nil {
		c.OnNotification(notifyTurnPlanUpdated, nil)
		return
	}
	c.OnNotification(notifyTurnPlanUpdated, func(ctx context.Context, notif Notification) {
		var params TurnPlanUpdatedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyTurnPlanUpdated, fmt.Errorf("unmarshal %s: %w", notifyTurnPlanUpdated, err))
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

func (n *TurnDiffUpdatedNotification) UnmarshalJSON(data []byte) error {
	type wire TurnDiffUpdatedNotification
	var decoded wire
	required := []string{"diff", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = TurnDiffUpdatedNotification(decoded)
	return nil
}

// OnTurnDiffUpdated registers a listener for turn/diff/updated notifications
func (c *Client) OnTurnDiffUpdated(handler func(TurnDiffUpdatedNotification)) {
	if handler == nil {
		c.OnNotification(notifyTurnDiffUpdated, nil)
		return
	}
	c.OnNotification(notifyTurnDiffUpdated, func(ctx context.Context, notif Notification) {
		var params TurnDiffUpdatedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyTurnDiffUpdated, fmt.Errorf("unmarshal %s: %w", notifyTurnDiffUpdated, err))
			return
		}
		handler(params)
	})
}
