package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// HookEventName identifies when a hook ran.
type HookEventName string

const (
	HookEventNameSessionStart     HookEventName = "sessionStart"
	HookEventNameUserPromptSubmit HookEventName = "userPromptSubmit"
	HookEventNameStop             HookEventName = "stop"
)

// HookExecutionMode identifies whether a hook ran synchronously or asynchronously.
type HookExecutionMode string

const (
	HookExecutionModeSync  HookExecutionMode = "sync"
	HookExecutionModeAsync HookExecutionMode = "async"
)

// HookHandlerType identifies the type of hook handler.
type HookHandlerType string

const (
	HookHandlerTypeCommand HookHandlerType = "command"
	HookHandlerTypePrompt  HookHandlerType = "prompt"
	HookHandlerTypeAgent   HookHandlerType = "agent"
)

// HookOutputEntryKind identifies the kind of hook output entry.
type HookOutputEntryKind string

const (
	HookOutputEntryKindWarning  HookOutputEntryKind = "warning"
	HookOutputEntryKindStop     HookOutputEntryKind = "stop"
	HookOutputEntryKindFeedback HookOutputEntryKind = "feedback"
	HookOutputEntryKindContext  HookOutputEntryKind = "context"
	HookOutputEntryKindError    HookOutputEntryKind = "error"
)

// HookRunStatus identifies the lifecycle state of a hook execution.
type HookRunStatus string

const (
	HookRunStatusRunning   HookRunStatus = "running"
	HookRunStatusCompleted HookRunStatus = "completed"
	HookRunStatusFailed    HookRunStatus = "failed"
	HookRunStatusBlocked   HookRunStatus = "blocked"
	HookRunStatusStopped   HookRunStatus = "stopped"
)

// HookScope identifies whether a hook ran at thread or turn scope.
type HookScope string

const (
	HookScopeThread HookScope = "thread"
	HookScopeTurn   HookScope = "turn"
)

// HookOutputEntry is a single line of hook output.
type HookOutputEntry struct {
	Kind HookOutputEntryKind `json:"kind"`
	Text string              `json:"text"`
}

func (e *HookOutputEntry) UnmarshalJSON(data []byte) error {
	type wire HookOutputEntry
	var decoded wire
	required := []string{"kind", "text"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*e = HookOutputEntry(decoded)
	return nil
}

// HookRunSummary describes a hook execution.
type HookRunSummary struct {
	CompletedAt   *int64            `json:"completedAt,omitempty"`
	DisplayOrder  int64             `json:"displayOrder"`
	DurationMs    *int64            `json:"durationMs,omitempty"`
	Entries       []HookOutputEntry `json:"entries"`
	EventName     HookEventName     `json:"eventName"`
	ExecutionMode HookExecutionMode `json:"executionMode"`
	HandlerType   HookHandlerType   `json:"handlerType"`
	ID            string            `json:"id"`
	Scope         HookScope         `json:"scope"`
	SourcePath    string            `json:"sourcePath"`
	StartedAt     int64             `json:"startedAt"`
	Status        HookRunStatus     `json:"status"`
	StatusMessage *string           `json:"statusMessage,omitempty"`
}

func (s *HookRunSummary) UnmarshalJSON(data []byte) error {
	type wire HookRunSummary
	var decoded wire
	required := []string{
		"displayOrder",
		"entries",
		"eventName",
		"executionMode",
		"handlerType",
		"id",
		"scope",
		"sourcePath",
		"startedAt",
		"status",
	}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*s = HookRunSummary(decoded)
	return nil
}

// HookStartedNotification is sent when a hook run starts.
type HookStartedNotification struct {
	Run      HookRunSummary `json:"run"`
	ThreadID string         `json:"threadId"`
	TurnID   *string        `json:"turnId,omitempty"`
}

func (n *HookStartedNotification) UnmarshalJSON(data []byte) error {
	type wire HookStartedNotification
	var decoded wire
	required := []string{"run", "threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = HookStartedNotification(decoded)
	return nil
}

// HookCompletedNotification is sent when a hook run completes.
type HookCompletedNotification struct {
	Run      HookRunSummary `json:"run"`
	ThreadID string         `json:"threadId"`
	TurnID   *string        `json:"turnId,omitempty"`
}

func (n *HookCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire HookCompletedNotification
	var decoded wire
	required := []string{"run", "threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = HookCompletedNotification(decoded)
	return nil
}

// GuardianApprovalReviewStatus is the lifecycle status of a guardian review.
type GuardianApprovalReviewStatus string

const (
	GuardianApprovalReviewStatusInProgress GuardianApprovalReviewStatus = "inProgress"
	GuardianApprovalReviewStatusApproved   GuardianApprovalReviewStatus = "approved"
	GuardianApprovalReviewStatusDenied     GuardianApprovalReviewStatus = "denied"
	GuardianApprovalReviewStatusAborted    GuardianApprovalReviewStatus = "aborted"
)

var validGuardianApprovalReviewStatuses = map[GuardianApprovalReviewStatus]struct{}{
	GuardianApprovalReviewStatusInProgress: {},
	GuardianApprovalReviewStatusApproved:   {},
	GuardianApprovalReviewStatusDenied:     {},
	GuardianApprovalReviewStatusAborted:    {},
}

func (s *GuardianApprovalReviewStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "guardian.review.status", validGuardianApprovalReviewStatuses, s)
}

// GuardianRiskLevel is the risk level assigned by guardian review.
type GuardianRiskLevel string

const (
	GuardianRiskLevelLow    GuardianRiskLevel = "low"
	GuardianRiskLevelMedium GuardianRiskLevel = "medium"
	GuardianRiskLevelHigh   GuardianRiskLevel = "high"
)

var validGuardianRiskLevels = map[GuardianRiskLevel]struct{}{
	GuardianRiskLevelLow:    {},
	GuardianRiskLevelMedium: {},
	GuardianRiskLevelHigh:   {},
}

func (l *GuardianRiskLevel) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "guardian.review.riskLevel", validGuardianRiskLevels, l)
}

// GuardianApprovalReview contains the guardian review payload.
type GuardianApprovalReview struct {
	Rationale *string                      `json:"rationale,omitempty"`
	RiskLevel *GuardianRiskLevel           `json:"riskLevel,omitempty"`
	RiskScore *uint8                       `json:"riskScore,omitempty"`
	Status    GuardianApprovalReviewStatus `json:"status"`
}

func (r *GuardianApprovalReview) UnmarshalJSON(data []byte) error {
	type wire GuardianApprovalReview
	var decoded wire
	required := []string{"status"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*r = GuardianApprovalReview(decoded)
	return nil
}

// ItemGuardianApprovalReviewStartedNotification is sent when guardian review begins.
type ItemGuardianApprovalReviewStartedNotification struct {
	Action       interface{}            `json:"action,omitempty"`
	Review       GuardianApprovalReview `json:"review"`
	TargetItemID string                 `json:"targetItemId"`
	ThreadID     string                 `json:"threadId"`
	TurnID       string                 `json:"turnId"`
}

func (n *ItemGuardianApprovalReviewStartedNotification) UnmarshalJSON(data []byte) error {
	type wire ItemGuardianApprovalReviewStartedNotification
	var decoded wire
	required := []string{"review", "targetItemId", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ItemGuardianApprovalReviewStartedNotification(decoded)
	return nil
}

// ItemGuardianApprovalReviewCompletedNotification is sent when guardian review finishes.
type ItemGuardianApprovalReviewCompletedNotification struct {
	Action       interface{}            `json:"action,omitempty"`
	Review       GuardianApprovalReview `json:"review"`
	TargetItemID string                 `json:"targetItemId"`
	ThreadID     string                 `json:"threadId"`
	TurnID       string                 `json:"turnId"`
}

func (n *ItemGuardianApprovalReviewCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire ItemGuardianApprovalReviewCompletedNotification
	var decoded wire
	required := []string{"review", "targetItemId", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ItemGuardianApprovalReviewCompletedNotification(decoded)
	return nil
}

// OnHookStarted registers a listener for hook/started notifications.
func (c *Client) OnHookStarted(handler func(HookStartedNotification)) {
	if handler == nil {
		c.OnNotification(notifyHookStarted, nil)
		return
	}
	c.OnNotification(notifyHookStarted, func(ctx context.Context, notif Notification) {
		var params HookStartedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyHookStarted, fmt.Errorf("unmarshal %s: %w", notifyHookStarted, err))
			return
		}
		handler(params)
	})
}

// OnHookCompleted registers a listener for hook/completed notifications.
func (c *Client) OnHookCompleted(handler func(HookCompletedNotification)) {
	if handler == nil {
		c.OnNotification(notifyHookCompleted, nil)
		return
	}
	c.OnNotification(notifyHookCompleted, func(ctx context.Context, notif Notification) {
		var params HookCompletedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyHookCompleted, fmt.Errorf("unmarshal %s: %w", notifyHookCompleted, err))
			return
		}
		handler(params)
	})
}

// OnItemGuardianApprovalReviewStarted registers a listener for guardian review start notifications.
func (c *Client) OnItemGuardianApprovalReviewStarted(handler func(ItemGuardianApprovalReviewStartedNotification)) {
	if handler == nil {
		c.OnNotification(notifyItemGuardianApprovalReviewStarted, nil)
		return
	}
	c.OnNotification(notifyItemGuardianApprovalReviewStarted, func(ctx context.Context, notif Notification) {
		var params ItemGuardianApprovalReviewStartedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyItemGuardianApprovalReviewStarted, fmt.Errorf("unmarshal %s: %w", notifyItemGuardianApprovalReviewStarted, err))
			return
		}
		handler(params)
	})
}

// OnItemGuardianApprovalReviewCompleted registers a listener for guardian review completion notifications.
func (c *Client) OnItemGuardianApprovalReviewCompleted(handler func(ItemGuardianApprovalReviewCompletedNotification)) {
	if handler == nil {
		c.OnNotification(notifyItemGuardianApprovalReviewCompleted, nil)
		return
	}
	c.OnNotification(notifyItemGuardianApprovalReviewCompleted, func(ctx context.Context, notif Notification) {
		var params ItemGuardianApprovalReviewCompletedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyItemGuardianApprovalReviewCompleted, fmt.Errorf("unmarshal %s: %w", notifyItemGuardianApprovalReviewCompleted, err))
			return
		}
		handler(params)
	})
}
