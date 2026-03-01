package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// WindowsSandboxSetupMode represents the sandbox setup mode
type WindowsSandboxSetupMode string

const (
	WindowsSandboxSetupModeElevated   WindowsSandboxSetupMode = "elevated"
	WindowsSandboxSetupModeUnelevated WindowsSandboxSetupMode = "unelevated"
)

// --- Client→Server Request Types ---

// WindowsSandboxSetupStartParams are the parameters for windowsSandbox/setupStart request
type WindowsSandboxSetupStartParams struct {
	Mode WindowsSandboxSetupMode `json:"mode"`
}

// WindowsSandboxSetupStartResponse is the response from windowsSandbox/setupStart
type WindowsSandboxSetupStartResponse struct {
	Started bool `json:"started"`
}

// --- Server→Client Notification Types ---

// WindowsSandboxSetupCompletedNotification is sent when sandbox setup completes
type WindowsSandboxSetupCompletedNotification struct {
	Mode    WindowsSandboxSetupMode `json:"mode"`
	Success bool                    `json:"success"`
	Error   *string                 `json:"error,omitempty"`
}

// WindowsWorldWritableWarningNotification warns about world-writable files
type WindowsWorldWritableWarningNotification struct {
	ExtraCount  uint     `json:"extraCount"`
	FailedScan  bool     `json:"failedScan"`
	SamplePaths []string `json:"samplePaths"`
}

// Deprecated: Use ContextCompaction item type instead.
// ContextCompactedNotification is sent when context is compacted.
type ContextCompactedNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

// DeprecationNoticeNotification informs about deprecated features
type DeprecationNoticeNotification struct {
	Summary string  `json:"summary"`
	Details *string `json:"details,omitempty"`
}

// ErrorNotification is sent when a system error occurs
type ErrorNotification struct {
	Error     TurnError `json:"error"`
	ThreadID  string    `json:"threadId"`
	TurnID    string    `json:"turnId"`
	WillRetry bool      `json:"willRetry"`
}

// TerminalInteractionNotification is sent for terminal stdin interactions
type TerminalInteractionNotification struct {
	ItemID    string `json:"itemId"`
	ProcessID string `json:"processId"`
	Stdin     string `json:"stdin"`
	ThreadID  string `json:"threadId"`
	TurnID    string `json:"turnId"`
}

// --- SystemService ---

// SystemService provides system-level operations
type SystemService struct {
	client *Client
}

func newSystemService(client *Client) *SystemService {
	return &SystemService{client: client}
}

// WindowsSandboxSetupStart initiates Windows sandbox setup
func (s *SystemService) WindowsSandboxSetupStart(ctx context.Context, params WindowsSandboxSetupStartParams) (WindowsSandboxSetupStartResponse, error) {
	var resp WindowsSandboxSetupStartResponse
	if err := s.client.sendRequest(ctx, methodWindowsSandboxSetupStart, params, &resp); err != nil {
		return WindowsSandboxSetupStartResponse{}, err
	}
	return resp, nil
}

// --- Client Notification Listeners ---

// OnWindowsSandboxSetupCompleted registers a listener for windowsSandbox/setupCompleted notifications
func (c *Client) OnWindowsSandboxSetupCompleted(handler func(WindowsSandboxSetupCompletedNotification)) {
	if handler == nil {
		c.OnNotification(notifyWindowsSandboxSetupCompleted, nil)
		return
	}
	c.OnNotification(notifyWindowsSandboxSetupCompleted, func(ctx context.Context, notif Notification) {
		var params WindowsSandboxSetupCompletedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyWindowsSandboxSetupCompleted, fmt.Errorf("unmarshal %s: %w", notifyWindowsSandboxSetupCompleted, err))
			return
		}
		handler(params)
	})
}

// OnWindowsWorldWritableWarning registers a listener for windows/worldWritableWarning notifications
func (c *Client) OnWindowsWorldWritableWarning(handler func(WindowsWorldWritableWarningNotification)) {
	if handler == nil {
		c.OnNotification(notifyWindowsWorldWritableWarning, nil)
		return
	}
	c.OnNotification(notifyWindowsWorldWritableWarning, func(ctx context.Context, notif Notification) {
		var params WindowsWorldWritableWarningNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyWindowsWorldWritableWarning, fmt.Errorf("unmarshal %s: %w", notifyWindowsWorldWritableWarning, err))
			return
		}
		handler(params)
	})
}

// Deprecated: Use ContextCompaction item type instead.
// OnContextCompacted registers a listener for thread/compacted notifications.
func (c *Client) OnContextCompacted(handler func(ContextCompactedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadCompacted, nil)
		return
	}
	c.OnNotification(notifyThreadCompacted, func(ctx context.Context, notif Notification) {
		var params ContextCompactedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyThreadCompacted, fmt.Errorf("unmarshal %s: %w", notifyThreadCompacted, err))
			return
		}
		handler(params)
	})
}

// OnDeprecationNotice registers a listener for deprecationNotice notifications
func (c *Client) OnDeprecationNotice(handler func(DeprecationNoticeNotification)) {
	if handler == nil {
		c.OnNotification(notifyDeprecationNotice, nil)
		return
	}
	c.OnNotification(notifyDeprecationNotice, func(ctx context.Context, notif Notification) {
		var params DeprecationNoticeNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyDeprecationNotice, fmt.Errorf("unmarshal %s: %w", notifyDeprecationNotice, err))
			return
		}
		handler(params)
	})
}

// OnError registers a listener for error notifications
func (c *Client) OnError(handler func(ErrorNotification)) {
	if handler == nil {
		c.OnNotification(notifyError, nil)
		return
	}
	c.OnNotification(notifyError, func(ctx context.Context, notif Notification) {
		var params ErrorNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyError, fmt.Errorf("unmarshal %s: %w", notifyError, err))
			return
		}
		handler(params)
	})
}

// OnTerminalInteraction registers a listener for item/commandExecution/terminalInteraction notifications
func (c *Client) OnTerminalInteraction(handler func(TerminalInteractionNotification)) {
	if handler == nil {
		c.OnNotification(notifyTerminalInteraction, nil)
		return
	}
	c.OnNotification(notifyTerminalInteraction, func(ctx context.Context, notif Notification) {
		var params TerminalInteractionNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyTerminalInteraction, fmt.Errorf("unmarshal %s: %w", notifyTerminalInteraction, err))
			return
		}
		handler(params)
	})
}
