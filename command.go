package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// CommandExecParams represents parameters for executing a command
type CommandExecParams struct {
	Command       []string              `json:"command"`
	Cwd           *string               `json:"cwd,omitempty"`
	SandboxPolicy *SandboxPolicyWrapper `json:"sandboxPolicy,omitempty"`
	TimeoutMs     *int64                `json:"timeoutMs,omitempty"`
}

// CommandExecResponse represents the result of command execution
type CommandExecResponse struct {
	ExitCode int32  `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
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
