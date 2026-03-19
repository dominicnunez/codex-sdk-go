package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// ThreadStartedNotification is sent when a thread is started
type ThreadStartedNotification struct {
	Thread Thread `json:"thread"`
}

func (n *ThreadStartedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadStartedNotification
	var decoded wire
	required := []string{"thread"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadStartedNotification(decoded)
	return nil
}

// ThreadClosedNotification is sent when a thread is closed
type ThreadClosedNotification struct {
	ThreadID string `json:"threadId"`
}

func (n *ThreadClosedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadClosedNotification
	var decoded wire
	required := []string{"threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadClosedNotification(decoded)
	return nil
}

// ThreadArchivedNotification is sent when a thread is archived
type ThreadArchivedNotification struct {
	ThreadID string `json:"threadId"`
}

func (n *ThreadArchivedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadArchivedNotification
	var decoded wire
	required := []string{"threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadArchivedNotification(decoded)
	return nil
}

// ThreadUnarchivedNotification is sent when a thread is unarchived
type ThreadUnarchivedNotification struct {
	ThreadID string `json:"threadId"`
}

func (n *ThreadUnarchivedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadUnarchivedNotification
	var decoded wire
	required := []string{"threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadUnarchivedNotification(decoded)
	return nil
}

// ThreadNameUpdatedNotification is sent when a thread's name is updated
type ThreadNameUpdatedNotification struct {
	ThreadID   string  `json:"threadId"`
	ThreadName *string `json:"threadName,omitempty"`
}

func (n *ThreadNameUpdatedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadNameUpdatedNotification
	var decoded wire
	required := []string{"threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadNameUpdatedNotification(decoded)
	return nil
}

// ThreadStatusChangedNotification is sent when a thread's status changes
type ThreadStatusChangedNotification struct {
	ThreadID string              `json:"threadId"`
	Status   ThreadStatusWrapper `json:"status"`
}

func (n *ThreadStatusChangedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadStatusChangedNotification
	var decoded wire
	required := []string{"status", "threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadStatusChangedNotification(decoded)
	return nil
}

// TokenUsageBreakdown contains token usage metrics
type TokenUsageBreakdown struct {
	CachedInputTokens     int64 `json:"cachedInputTokens"`
	InputTokens           int64 `json:"inputTokens"`
	OutputTokens          int64 `json:"outputTokens"`
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
	TotalTokens           int64 `json:"totalTokens"`
}

func (b *TokenUsageBreakdown) UnmarshalJSON(data []byte) error {
	type wire TokenUsageBreakdown
	var decoded wire
	required := []string{"cachedInputTokens", "inputTokens", "outputTokens", "reasoningOutputTokens", "totalTokens"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*b = TokenUsageBreakdown(decoded)
	return nil
}

// ThreadTokenUsage contains token usage information for a thread
type ThreadTokenUsage struct {
	Last               TokenUsageBreakdown `json:"last"`
	Total              TokenUsageBreakdown `json:"total"`
	ModelContextWindow *int64              `json:"modelContextWindow,omitempty"`
}

func (u *ThreadTokenUsage) UnmarshalJSON(data []byte) error {
	type wire ThreadTokenUsage
	var decoded wire
	required := []string{"last", "total"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*u = ThreadTokenUsage(decoded)
	return nil
}

// ThreadTokenUsageUpdatedNotification is sent when a thread's token usage is updated
type ThreadTokenUsageUpdatedNotification struct {
	ThreadID   string           `json:"threadId"`
	TurnID     string           `json:"turnId"`
	TokenUsage ThreadTokenUsage `json:"tokenUsage"`
}

func (n *ThreadTokenUsageUpdatedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadTokenUsageUpdatedNotification
	var decoded wire
	required := []string{"threadId", "tokenUsage", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadTokenUsageUpdatedNotification(decoded)
	return nil
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
			c.reportHandlerError(notifyThreadStarted, fmt.Errorf("unmarshal %s: %w", notifyThreadStarted, err))
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
			c.reportHandlerError(notifyThreadClosed, fmt.Errorf("unmarshal %s: %w", notifyThreadClosed, err))
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
			c.reportHandlerError(notifyThreadArchived, fmt.Errorf("unmarshal %s: %w", notifyThreadArchived, err))
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
			c.reportHandlerError(notifyThreadUnarchived, fmt.Errorf("unmarshal %s: %w", notifyThreadUnarchived, err))
			return
		}
		handler(notification)
	})
}

// OnThreadNameUpdated registers a listener for thread/name/updated notifications
func (c *Client) OnThreadNameUpdated(handler func(ThreadNameUpdatedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadNameUpdated, nil)
		return
	}
	c.OnNotification(notifyThreadNameUpdated, func(ctx context.Context, notif Notification) {
		var notification ThreadNameUpdatedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			c.reportHandlerError(notifyThreadNameUpdated, fmt.Errorf("unmarshal %s: %w", notifyThreadNameUpdated, err))
			return
		}
		handler(notification)
	})
}

// OnThreadStatusChanged registers a listener for thread/status/changed notifications
func (c *Client) OnThreadStatusChanged(handler func(ThreadStatusChangedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadStatusChanged, nil)
		return
	}
	c.OnNotification(notifyThreadStatusChanged, func(ctx context.Context, notif Notification) {
		var notification ThreadStatusChangedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			c.reportHandlerError(notifyThreadStatusChanged, fmt.Errorf("unmarshal %s: %w", notifyThreadStatusChanged, err))
			return
		}
		handler(notification)
	})
}

// ServerRequestResolvedNotification is sent when a server request is resolved
type ServerRequestResolvedNotification struct {
	RequestID RequestID `json:"requestId"`
	ThreadID  string    `json:"threadId"`
}

func (n *ServerRequestResolvedNotification) UnmarshalJSON(data []byte) error {
	type wire ServerRequestResolvedNotification
	var decoded wire
	required := []string{"requestId", "threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ServerRequestResolvedNotification(decoded)
	return nil
}

// OnServerRequestResolved registers a listener for serverRequest/resolved notifications
func (c *Client) OnServerRequestResolved(handler func(ServerRequestResolvedNotification)) {
	if handler == nil {
		c.OnNotification(notifyServerRequestResolved, nil)
		return
	}
	c.OnNotification(notifyServerRequestResolved, func(ctx context.Context, notif Notification) {
		var notification ServerRequestResolvedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			c.reportHandlerError(notifyServerRequestResolved, fmt.Errorf("unmarshal %s: %w", notifyServerRequestResolved, err))
			return
		}
		handler(notification)
	})
}

// OnThreadTokenUsageUpdated registers a listener for thread/tokenUsage/updated notifications
func (c *Client) OnThreadTokenUsageUpdated(handler func(ThreadTokenUsageUpdatedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadTokenUsageUpdated, nil)
		return
	}
	c.OnNotification(notifyThreadTokenUsageUpdated, func(ctx context.Context, notif Notification) {
		var notification ThreadTokenUsageUpdatedNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			c.reportHandlerError(notifyThreadTokenUsageUpdated, fmt.Errorf("unmarshal %s: %w", notifyThreadTokenUsageUpdated, err))
			return
		}
		handler(notification)
	})
}
