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

var validWindowsSandboxSetupModes = map[WindowsSandboxSetupMode]struct{}{
	WindowsSandboxSetupModeElevated:   {},
	WindowsSandboxSetupModeUnelevated: {},
}

func (m *WindowsSandboxSetupMode) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "windowsSandbox.mode", validWindowsSandboxSetupModes, m)
}

func (m WindowsSandboxSetupMode) MarshalJSON() ([]byte, error) {
	return marshalEnumString("windowsSandbox.mode", m, validWindowsSandboxSetupModes)
}

// --- Client→Server Request Types ---

// WindowsSandboxSetupStartParams are the parameters for windowsSandbox/setupStart request
type WindowsSandboxSetupStartParams struct {
	Cwd  *string                 `json:"cwd,omitempty"`
	Mode WindowsSandboxSetupMode `json:"mode"`
}

// WindowsSandboxSetupStartResponse is the response from windowsSandbox/setupStart
type WindowsSandboxSetupStartResponse struct {
	Started bool `json:"started"`
}

func (r *WindowsSandboxSetupStartResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "started"); err != nil {
		return err
	}
	type wire WindowsSandboxSetupStartResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = WindowsSandboxSetupStartResponse(decoded)
	return nil
}

// --- Server→Client Notification Types ---

// WindowsSandboxSetupCompletedNotification is sent when sandbox setup completes
type WindowsSandboxSetupCompletedNotification struct {
	Mode    WindowsSandboxSetupMode `json:"mode"`
	Success bool                    `json:"success"`
	Error   *string                 `json:"error,omitempty"`
}

func (n *WindowsSandboxSetupCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire WindowsSandboxSetupCompletedNotification
	var decoded wire
	required := []string{"mode", "success"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = WindowsSandboxSetupCompletedNotification(decoded)
	return nil
}

// WindowsWorldWritableWarningNotification warns about world-writable files
type WindowsWorldWritableWarningNotification struct {
	ExtraCount  uint     `json:"extraCount"`
	FailedScan  bool     `json:"failedScan"`
	SamplePaths []string `json:"samplePaths"`
}

func (n *WindowsWorldWritableWarningNotification) UnmarshalJSON(data []byte) error {
	type wire WindowsWorldWritableWarningNotification
	var decoded wire
	required := []string{"extraCount", "failedScan", "samplePaths"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = WindowsWorldWritableWarningNotification(decoded)
	return nil
}

// Deprecated: Use ContextCompaction item type instead.
// ContextCompactedNotification is sent when context is compacted.
type ContextCompactedNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

func (n *ContextCompactedNotification) UnmarshalJSON(data []byte) error {
	type wire ContextCompactedNotification
	var decoded wire
	required := []string{"threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ContextCompactedNotification(decoded)
	return nil
}

// DeprecationNoticeNotification informs about deprecated features
type DeprecationNoticeNotification struct {
	Summary string  `json:"summary"`
	Details *string `json:"details,omitempty"`
}

func (n *DeprecationNoticeNotification) UnmarshalJSON(data []byte) error {
	type wire DeprecationNoticeNotification
	var decoded wire
	required := []string{"summary"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = DeprecationNoticeNotification(decoded)
	return nil
}

// ErrorNotification is sent when a system error occurs
type ErrorNotification struct {
	Error     TurnError       `json:"error"`
	ThreadID  string          `json:"threadId"`
	TurnID    string          `json:"turnId"`
	WillRetry bool            `json:"willRetry"`
	Raw       json.RawMessage `json:"-"`
}

func (n *ErrorNotification) UnmarshalJSON(data []byte) error {
	type wire ErrorNotification
	var decoded wire
	required := []string{"error", "threadId", "turnId", "willRetry"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	decoded.Raw = append(json.RawMessage(nil), data...)
	*n = ErrorNotification(decoded)
	return nil
}

// TerminalInteractionNotification is sent for terminal stdin interactions
type TerminalInteractionNotification struct {
	ItemID    string `json:"itemId"`
	ProcessID string `json:"processId"`
	Stdin     string `json:"stdin"`
	ThreadID  string `json:"threadId"`
	TurnID    string `json:"turnId"`
}

// WarningNotification reports a non-fatal warning.
type WarningNotification struct {
	Message  string  `json:"message"`
	ThreadID *string `json:"threadId,omitempty"`
}

func (n *WarningNotification) UnmarshalJSON(data []byte) error {
	type wire WarningNotification
	var decoded wire
	required := []string{"message"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = WarningNotification(decoded)
	return nil
}

// GuardianWarningNotification reports a guardian warning for a thread.
type GuardianWarningNotification struct {
	Message  string `json:"message"`
	ThreadID string `json:"threadId"`
}

func (n *GuardianWarningNotification) UnmarshalJSON(data []byte) error {
	type wire GuardianWarningNotification
	var decoded wire
	required := []string{"message", "threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = GuardianWarningNotification(decoded)
	return nil
}

// RemoteControlConnectionStatus is the remote-control connection state.
type RemoteControlConnectionStatus string

const (
	RemoteControlConnectionStatusDisabled   RemoteControlConnectionStatus = "disabled"
	RemoteControlConnectionStatusConnecting RemoteControlConnectionStatus = "connecting"
	RemoteControlConnectionStatusConnected  RemoteControlConnectionStatus = "connected"
	RemoteControlConnectionStatusErrored    RemoteControlConnectionStatus = "errored"
)

// RemoteControlStatusChangedNotification reports remote-control connection status.
type RemoteControlStatusChangedNotification struct {
	EnvironmentID *string                       `json:"environmentId,omitempty"`
	Status        RemoteControlConnectionStatus `json:"status"`
}

func (n *RemoteControlStatusChangedNotification) UnmarshalJSON(data []byte) error {
	type wire RemoteControlStatusChangedNotification
	var decoded wire
	required := []string{"status"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = RemoteControlStatusChangedNotification(decoded)
	return nil
}

func (n *TerminalInteractionNotification) UnmarshalJSON(data []byte) error {
	type wire TerminalInteractionNotification
	var decoded wire
	required := []string{"itemId", "processId", "stdin", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = TerminalInteractionNotification(decoded)
	return nil
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

// OnWarning registers a listener for warning notifications.
func (c *Client) OnWarning(handler func(WarningNotification)) {
	if handler == nil {
		c.OnNotification(notifyWarning, nil)
		return
	}
	c.OnNotification(notifyWarning, func(ctx context.Context, notif Notification) {
		var params WarningNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyWarning, fmt.Errorf("unmarshal %s: %w", notifyWarning, err))
			return
		}
		handler(params)
	})
}

// OnGuardianWarning registers a listener for guardianWarning notifications.
func (c *Client) OnGuardianWarning(handler func(GuardianWarningNotification)) {
	if handler == nil {
		c.OnNotification(notifyGuardianWarning, nil)
		return
	}
	c.OnNotification(notifyGuardianWarning, func(ctx context.Context, notif Notification) {
		var params GuardianWarningNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyGuardianWarning, fmt.Errorf("unmarshal %s: %w", notifyGuardianWarning, err))
			return
		}
		handler(params)
	})
}

// OnRemoteControlStatusChanged registers a listener for remote-control status changes.
func (c *Client) OnRemoteControlStatusChanged(handler func(RemoteControlStatusChangedNotification)) {
	if handler == nil {
		c.OnNotification(notifyRemoteControlStatusChanged, nil)
		return
	}
	c.OnNotification(notifyRemoteControlStatusChanged, func(ctx context.Context, notif Notification) {
		var params RemoteControlStatusChangedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyRemoteControlStatusChanged, fmt.Errorf("unmarshal %s: %w", notifyRemoteControlStatusChanged, err))
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
