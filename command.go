package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// CommandExecTerminalSize represents a PTY size in character cells.
type CommandExecTerminalSize struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// CommandExecParams represents parameters for executing a command
type CommandExecParams struct {
	Command            []string                 `json:"command"`
	Cwd                *string                  `json:"cwd,omitempty"`
	DisableOutputCap   *bool                    `json:"disableOutputCap,omitempty"`
	DisableTimeout     *bool                    `json:"disableTimeout,omitempty"`
	Env                map[string]*string       `json:"env,omitempty"`
	OutputBytesCap     *uint64                  `json:"outputBytesCap,omitempty"`
	ProcessID          *string                  `json:"processId,omitempty"`
	SandboxPolicy      *SandboxPolicyWrapper    `json:"sandboxPolicy,omitempty"`
	Size               *CommandExecTerminalSize `json:"size,omitempty"`
	StreamStdin        *bool                    `json:"streamStdin,omitempty"`
	StreamStdoutStderr *bool                    `json:"streamStdoutStderr,omitempty"`
	TimeoutMs          *int64                   `json:"timeoutMs,omitempty"`
	TTY                *bool                    `json:"tty,omitempty"`
}

// CommandExecResponse represents the result of command execution
type CommandExecResponse struct {
	ExitCode int32  `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// CommandExecWriteParams writes stdin bytes to a running command/exec session.
type CommandExecWriteParams struct {
	ProcessID   string  `json:"processId"`
	CloseStdin  *bool   `json:"closeStdin,omitempty"`
	DeltaBase64 *string `json:"deltaBase64,omitempty"`
}

// CommandExecWriteResponse is the empty response from command/exec/write.
type CommandExecWriteResponse struct{}

// CommandExecTerminateParams terminates a running command/exec session.
type CommandExecTerminateParams struct {
	ProcessID string `json:"processId"`
}

// CommandExecTerminateResponse is the empty response from command/exec/terminate.
type CommandExecTerminateResponse struct{}

// CommandExecResizeParams resizes a running PTY-backed command/exec session.
type CommandExecResizeParams struct {
	ProcessID string                  `json:"processId"`
	Size      CommandExecTerminalSize `json:"size"`
}

// CommandExecResizeResponse is the empty response from command/exec/resize.
type CommandExecResizeResponse struct{}

// CommandExecOutputStream labels streamed command/exec output.
type CommandExecOutputStream string

const (
	CommandExecOutputStreamStdout CommandExecOutputStream = "stdout"
	CommandExecOutputStreamStderr CommandExecOutputStream = "stderr"
)

// CommandExecOutputDeltaNotification represents streamed output for standalone command/exec calls.
type CommandExecOutputDeltaNotification struct {
	CapReached  bool                    `json:"capReached"`
	DeltaBase64 string                  `json:"deltaBase64"`
	ProcessID   string                  `json:"processId"`
	Stream      CommandExecOutputStream `json:"stream"`
}

// CommandExecutionOutputDeltaNotification represents streaming output from command execution
type CommandExecutionOutputDeltaNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	ItemID   string `json:"itemId"`
	Delta    string `json:"delta"`
}

// CommandService provides command execution functionality
type CommandService struct {
	client *Client
}

func newCommandService(client *Client) *CommandService {
	return &CommandService{client: client}
}

// Exec executes a command and returns the result
func (s *CommandService) Exec(ctx context.Context, params CommandExecParams) (CommandExecResponse, error) {
	var response CommandExecResponse
	if err := s.client.sendRequest(ctx, methodCommandExec, params, &response); err != nil {
		return CommandExecResponse{}, err
	}
	return response, nil
}

// Write writes stdin bytes to a running command/exec session.
func (s *CommandService) Write(ctx context.Context, params CommandExecWriteParams) (CommandExecWriteResponse, error) {
	if err := s.client.sendRequest(ctx, methodCommandExecWrite, params, nil); err != nil {
		return CommandExecWriteResponse{}, err
	}
	return CommandExecWriteResponse{}, nil
}

// Terminate terminates a running command/exec session.
func (s *CommandService) Terminate(ctx context.Context, params CommandExecTerminateParams) (CommandExecTerminateResponse, error) {
	if err := s.client.sendRequest(ctx, methodCommandExecTerminate, params, nil); err != nil {
		return CommandExecTerminateResponse{}, err
	}
	return CommandExecTerminateResponse{}, nil
}

// Resize resizes a running PTY-backed command/exec session.
func (s *CommandService) Resize(ctx context.Context, params CommandExecResizeParams) (CommandExecResizeResponse, error) {
	if err := s.client.sendRequest(ctx, methodCommandExecResize, params, nil); err != nil {
		return CommandExecResizeResponse{}, err
	}
	return CommandExecResizeResponse{}, nil
}

// OnCommandExecutionOutputDelta registers a listener for command execution output delta notifications
func (c *Client) OnCommandExecutionOutputDelta(handler func(CommandExecutionOutputDeltaNotification)) {
	if handler == nil {
		c.OnNotification(notifyCommandExecutionOutputDelta, nil)
		return
	}
	c.OnNotification(notifyCommandExecutionOutputDelta, func(ctx context.Context, notif Notification) {
		var notification CommandExecutionOutputDeltaNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			c.reportHandlerError(notifyCommandExecutionOutputDelta, fmt.Errorf("unmarshal %s: %w", notifyCommandExecutionOutputDelta, err))
			return
		}
		handler(notification)
	})
}

// OnCommandExecOutputDelta registers a listener for standalone command/exec output streaming.
func (c *Client) OnCommandExecOutputDelta(handler func(CommandExecOutputDeltaNotification)) {
	if handler == nil {
		c.OnNotification(notifyCommandExecOutputDelta, nil)
		return
	}
	c.OnNotification(notifyCommandExecOutputDelta, func(ctx context.Context, notif Notification) {
		var notification CommandExecOutputDeltaNotification
		if err := json.Unmarshal(notif.Params, &notification); err != nil {
			c.reportHandlerError(notifyCommandExecOutputDelta, fmt.Errorf("unmarshal %s: %w", notifyCommandExecOutputDelta, err))
			return
		}
		handler(notification)
	})
}
