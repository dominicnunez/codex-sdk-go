package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestCommandExec(t *testing.T) {
	tests := []struct {
		name     string
		params   codex.CommandExecParams
		response map[string]interface{}
		verify   func(*testing.T, codex.CommandExecResponse)
	}{
		{
			name: "minimal command",
			params: codex.CommandExecParams{
				Command: []string{"echo", "hello"},
			},
			response: map[string]interface{}{
				"exitCode": 0,
				"stdout":   "hello\n",
				"stderr":   "",
			},
			verify: func(t *testing.T, resp codex.CommandExecResponse) {
				if resp.ExitCode != 0 {
					t.Errorf("expected exit code 0, got %d", resp.ExitCode)
				}
				if resp.Stdout != "hello\n" {
					t.Errorf("expected stdout 'hello\\n', got %q", resp.Stdout)
				}
				if resp.Stderr != "" {
					t.Errorf("expected empty stderr, got %q", resp.Stderr)
				}
			},
		},
		{
			name: "command with cwd and timeout",
			params: codex.CommandExecParams{
				Command:   []string{"ls", "-la"},
				Cwd:       ptr("/home/user"),
				TimeoutMs: ptr(int64(5000)),
			},
			response: map[string]interface{}{
				"exitCode": 0,
				"stdout":   "total 0\n",
				"stderr":   "",
			},
			verify: func(t *testing.T, resp codex.CommandExecResponse) {
				if resp.ExitCode != 0 {
					t.Errorf("expected exit code 0, got %d", resp.ExitCode)
				}
			},
		},
		{
			name: "command with sandbox policy",
			params: codex.CommandExecParams{
				Command: []string{"cat", "/etc/passwd"},
				SandboxPolicy: &codex.SandboxPolicyWrapper{
					Value: codex.SandboxPolicyReadOnly{
						Type: "readOnly",
					},
				},
			},
			response: map[string]interface{}{
				"exitCode": 1,
				"stdout":   "",
				"stderr":   "Permission denied\n",
			},
			verify: func(t *testing.T, resp codex.CommandExecResponse) {
				if resp.ExitCode != 1 {
					t.Errorf("expected exit code 1, got %d", resp.ExitCode)
				}
				if resp.Stderr != "Permission denied\n" {
					t.Errorf("expected stderr 'Permission denied\\n', got %q", resp.Stderr)
				}
			},
		},
		{
			name: "command with non-zero exit",
			params: codex.CommandExecParams{
				Command: []string{"false"},
			},
			response: map[string]interface{}{
				"exitCode": 1,
				"stdout":   "",
				"stderr":   "",
			},
			verify: func(t *testing.T, resp codex.CommandExecResponse) {
				if resp.ExitCode != 1 {
					t.Errorf("expected exit code 1, got %d", resp.ExitCode)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("command/exec", tt.response)

			client := codex.NewClient(mock)
			resp, err := client.Command.Exec(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("Command.Exec() failed: %v", err)
			}

			tt.verify(t, resp)

			// Verify the sent request
			req := mock.GetSentRequest(0)
			if req == nil {
				t.Fatal("no request sent")
			}
			if req.Method != "command/exec" {
				t.Errorf("expected method 'command/exec', got %q", req.Method)
			}

			// Verify params serialization
			var sentParams codex.CommandExecParams
			if err := json.Unmarshal(req.Params, &sentParams); err != nil {
				t.Fatalf("failed to unmarshal sent params: %v", err)
			}
			if len(sentParams.Command) != len(tt.params.Command) {
				t.Errorf("expected command length %d, got %d", len(tt.params.Command), len(sentParams.Command))
			}
		})
	}
}

func TestCommandExecutionOutputDeltaNotification(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	var received *codex.CommandExecutionOutputDeltaNotification
	client.OnCommandExecutionOutputDelta(func(notif codex.CommandExecutionOutputDeltaNotification) {
		received = &notif
	})

	// Inject server notification
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/commandExecution/outputDelta",
		Params: json.RawMessage(`{
			"threadId": "thread-123",
			"turnId": "turn-456",
			"itemId": "item-789",
			"delta": "output line\n"
		}`),
	}
	mock.InjectServerNotification(context.Background(), notif)

	// Verify notification was dispatched
	if received == nil {
		t.Fatal("notification not received")
	}
	if received.ThreadID != "thread-123" {
		t.Errorf("expected threadId 'thread-123', got %q", received.ThreadID)
	}
	if received.TurnID != "turn-456" {
		t.Errorf("expected turnId 'turn-456', got %q", received.TurnID)
	}
	if received.ItemID != "item-789" {
		t.Errorf("expected itemId 'item-789', got %q", received.ItemID)
	}
	if received.Delta != "output line\n" {
		t.Errorf("expected delta 'output line\\n', got %q", received.Delta)
	}
}

func TestCommandServiceMethodSignatures(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Compile-time verification that all methods exist
	var _ func(context.Context, codex.CommandExecParams) (codex.CommandExecResponse, error) = client.Command.Exec
}
