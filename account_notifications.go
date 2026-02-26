package codex

import (
	"context"
	"encoding/json"
)

// AccountUpdatedNotification is sent when account information changes
type AccountUpdatedNotification struct {
	AuthMode *string `json:"authMode,omitempty"`
}

// AccountLoginCompletedNotification is sent when a login attempt completes
type AccountLoginCompletedNotification struct {
	Success bool    `json:"success"`
	LoginId *string `json:"loginId,omitempty"`
	Error   *string `json:"error,omitempty"`
}

// AccountRateLimitsUpdatedNotification is sent when rate limits are updated
type AccountRateLimitsUpdatedNotification struct {
	RateLimits RateLimitSnapshot `json:"rateLimits"`
}

// OnAccountUpdated registers a listener for account/updated notifications
func (c *Client) OnAccountUpdated(handler func(AccountUpdatedNotification)) {
	c.OnNotification("account/updated", func(ctx context.Context, notif Notification) {
		var n AccountUpdatedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnAccountLoginCompleted registers a listener for account/loginCompleted notifications
func (c *Client) OnAccountLoginCompleted(handler func(AccountLoginCompletedNotification)) {
	c.OnNotification("account/loginCompleted", func(ctx context.Context, notif Notification) {
		var n AccountLoginCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}

// OnAccountRateLimitsUpdated registers a listener for account/rateLimitsUpdated notifications
func (c *Client) OnAccountRateLimitsUpdated(handler func(AccountRateLimitsUpdatedNotification)) {
	c.OnNotification("account/rateLimitsUpdated", func(ctx context.Context, notif Notification) {
		var n AccountRateLimitsUpdatedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}
