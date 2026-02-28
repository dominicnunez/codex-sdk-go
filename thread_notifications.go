package codex

import (
	"context"
	"encoding/json"
)

// ThreadStartedNotification is sent when a thread is started
type ThreadStartedNotification struct {
	Thread Thread `json:"thread"`
}

// ThreadClosedNotification is sent when a thread is closed
type ThreadClosedNotification struct {
	ThreadID string `json:"threadId"`
}

// ThreadArchivedNotification is sent when a thread is archived
type ThreadArchivedNotification struct {
	ThreadID string `json:"threadId"`
}

// ThreadUnarchivedNotification is sent when a thread is unarchived
type ThreadUnarchivedNotification struct {
	ThreadID string `json:"threadId"`
}

// ThreadNameUpdatedNotification is sent when a thread's name is updated
type ThreadNameUpdatedNotification struct {
	ThreadID   string  `json:"threadId"`
	ThreadName *string `json:"threadName,omitempty"`
}

// ThreadStatusChangedNotification is sent when a thread's status changes
type ThreadStatusChangedNotification struct {
	ThreadID string              `json:"threadId"`
	Status   ThreadStatusWrapper `json:"status"`
}

// TokenUsageBreakdown contains token usage metrics
type TokenUsageBreakdown struct {
	CachedInputTokens    int64 `json:"cachedInputTokens"`
	InputTokens          int64 `json:"inputTokens"`
	OutputTokens         int64 `json:"outputTokens"`
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
	TotalTokens          int64 `json:"totalTokens"`
}

// ThreadTokenUsage contains token usage information for a thread
type ThreadTokenUsage struct {
	Last                TokenUsageBreakdown `json:"last"`
	Total               TokenUsageBreakdown `json:"total"`
	ModelContextWindow  *int64              `json:"modelContextWindow,omitempty"`
}

// ThreadTokenUsageUpdatedNotification is sent when a thread's token usage is updated
type ThreadTokenUsageUpdatedNotification struct {
	ThreadID   string           `json:"threadId"`
	TurnID     string           `json:"turnId"`
	TokenUsage ThreadTokenUsage `json:"tokenUsage"`
}

// OnThreadStarted registers a listener for thread/started notifications
func (c *Client) OnThreadStarted(handler func(ThreadStartedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadStarted, nil)
		return
	}
	c.OnNotification(notifyThreadStarted, func(ctx context.Context, notif Notification) {
		var notification ThreadStartedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			return
		}
		handler(notification)
	})
}

// OnThreadClosed registers a listener for thread/closed notifications
func (c *Client) OnThreadClosed(handler func(ThreadClosedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadClosed, nil)
		return
	}
	c.OnNotification(notifyThreadClosed, func(ctx context.Context, notif Notification) {
		var notification ThreadClosedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			return
		}
		handler(notification)
	})
}

// OnThreadArchived registers a listener for thread/archived notifications
func (c *Client) OnThreadArchived(handler func(ThreadArchivedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadArchived, nil)
		return
	}
	c.OnNotification(notifyThreadArchived, func(ctx context.Context, notif Notification) {
		var notification ThreadArchivedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			return
		}
		handler(notification)
	})
}

// OnThreadUnarchived registers a listener for thread/unarchived notifications
func (c *Client) OnThreadUnarchived(handler func(ThreadUnarchivedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadUnarchived, nil)
		return
	}
	c.OnNotification(notifyThreadUnarchived, func(ctx context.Context, notif Notification) {
		var notification ThreadUnarchivedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			return
		}
		handler(notification)
	})
}

// OnThreadNameUpdated registers a listener for thread/nameUpdated notifications
func (c *Client) OnThreadNameUpdated(handler func(ThreadNameUpdatedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadNameUpdated, nil)
		return
	}
	c.OnNotification(notifyThreadNameUpdated, func(ctx context.Context, notif Notification) {
		var notification ThreadNameUpdatedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			return
		}
		handler(notification)
	})
}

// OnThreadStatusChanged registers a listener for thread/statusChanged notifications
func (c *Client) OnThreadStatusChanged(handler func(ThreadStatusChangedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadStatusChanged, nil)
		return
	}
	c.OnNotification(notifyThreadStatusChanged, func(ctx context.Context, notif Notification) {
		var notification ThreadStatusChangedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			return
		}
		handler(notification)
	})
}

// OnThreadTokenUsageUpdated registers a listener for thread/tokenUsageUpdated notifications
func (c *Client) OnThreadTokenUsageUpdated(handler func(ThreadTokenUsageUpdatedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadTokenUsageUpdated, nil)
		return
	}
	c.OnNotification(notifyThreadTokenUsageUpdated, func(ctx context.Context, notif Notification) {
		var notification ThreadTokenUsageUpdatedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			return
		}
		handler(notification)
	})
}
