package protocol

import (
	"context"
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
)

// ProcessOutputStream labels process output streams.
type ProcessOutputStream string

const (
	ProcessOutputStreamStdout ProcessOutputStream = "stdout"
	ProcessOutputStreamStderr ProcessOutputStream = "stderr"
)

var validProcessOutputStreams = map[ProcessOutputStream]struct{}{
	ProcessOutputStreamStdout: {},
	ProcessOutputStreamStderr: {},
}

func validateProcessOutputStream(stream ProcessOutputStream) error {
	return validateEnumValue("stream", stream, validProcessOutputStreams)
}

// ProcessOutputDeltaNotification is streamed output for process/spawn.
type ProcessOutputDeltaNotification struct {
	CapReached    bool                `json:"capReached"`
	DeltaBase64   string              `json:"deltaBase64"`
	ProcessHandle string              `json:"processHandle"`
	Stream        ProcessOutputStream `json:"stream"`
}

func (n *ProcessOutputDeltaNotification) UnmarshalJSON(data []byte) error {
	type wire ProcessOutputDeltaNotification
	var decoded wire
	required := []string{"capReached", "deltaBase64", "processHandle", "stream"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	if err := validateProcessOutputStream(decoded.Stream); err != nil {
		return err
	}
	if err := validateInboundBase64Field("deltaBase64", decoded.DeltaBase64); err != nil {
		return err
	}
	if err := validateOutputStreamOccurrences(data); err != nil {
		return err
	}
	*n = ProcessOutputDeltaNotification(decoded)
	return nil
}

// Both output notification records recognize exact decoded field names. Keep
// their original errors first, then reject invalid streams hidden by duplicates.
func validateOutputStreamOccurrences(data []byte) error {
	return validateFields(data, func(_ string, raw []byte) error {
		var stream ProcessOutputStream
		return unmarshalEnumString(raw, "stream", validProcessOutputStreams, &stream)
	}, false, "stream")
}

// ProcessExitedNotification is the final exit notification for process/spawn.
type ProcessExitedNotification struct {
	ExitCode         int32  `json:"exitCode"`
	ProcessHandle    string `json:"processHandle"`
	Stderr           string `json:"stderr"`
	StderrCapReached bool   `json:"stderrCapReached"`
	Stdout           string `json:"stdout"`
	StdoutCapReached bool   `json:"stdoutCapReached"`
}

func (n *ProcessExitedNotification) UnmarshalJSON(data []byte) error {
	type wire ProcessExitedNotification
	var decoded wire
	required := []string{"exitCode", "processHandle", "stderr", "stderrCapReached", "stdout", "stdoutCapReached"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ProcessExitedNotification(decoded)
	return nil
}

// OnProcessOutputDelta registers a listener for process/outputDelta notifications.
func (c *Client) OnProcessOutputDelta(handler func(ProcessOutputDeltaNotification)) {
	if handler == nil {
		c.OnNotification(notifyProcessOutputDelta, nil)
		return
	}
	c.OnNotification(notifyProcessOutputDelta, func(ctx context.Context, notif Notification) {
		var notification ProcessOutputDeltaNotification
		if err := jsondecode.Unmarshal(notif.Params, &notification); err != nil {
			c.reportHandlerError(notifyProcessOutputDelta, fmt.Errorf("unmarshal %s: %w", notifyProcessOutputDelta, err))
			return
		}
		handler(notification)
	})
}

// OnProcessExited registers a listener for process/exited notifications.
func (c *Client) OnProcessExited(handler func(ProcessExitedNotification)) {
	if handler == nil {
		c.OnNotification(notifyProcessExited, nil)
		return
	}
	c.OnNotification(notifyProcessExited, func(ctx context.Context, notif Notification) {
		var notification ProcessExitedNotification
		if err := jsondecode.Unmarshal(notif.Params, &notification); err != nil {
			c.reportHandlerError(notifyProcessExited, fmt.Errorf("unmarshal %s: %w", notifyProcessExited, err))
			return
		}
		handler(notification)
	})
}
