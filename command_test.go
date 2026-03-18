package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
					Value: codex.SandboxPolicyReadOnly{},
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
				return
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

func TestCommandExecOutputDeltaNotification(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	var received *codex.CommandExecOutputDeltaNotification
	client.OnCommandExecOutputDelta(func(notif codex.CommandExecOutputDeltaNotification) {
		received = &notif
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "command/exec/outputDelta",
		Params: json.RawMessage(`{
			"capReached": false,
			"deltaBase64": "aGVsbG8=",
			"processId": "proc-123",
			"stream": "stdout"
		}`),
	})

	if received == nil {
		t.Fatal("notification not received")
	}
	if received.ProcessID != "proc-123" {
		t.Fatalf("ProcessID = %q; want %q", received.ProcessID, "proc-123")
	}
	if received.DeltaBase64 != "aGVsbG8=" {
		t.Fatalf("DeltaBase64 = %q; want %q", received.DeltaBase64, "aGVsbG8=")
	}
	if received.Stream != codex.CommandExecOutputStreamStdout {
		t.Fatalf("Stream = %q; want %q", received.Stream, codex.CommandExecOutputStreamStdout)
	}

	client.OnCommandExecOutputDelta(nil)
	received = nil

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "command/exec/outputDelta",
		Params:  json.RawMessage(`{"capReached":true,"deltaBase64":"bW9yZQ==","processId":"proc-123","stream":"stderr"}`),
	})

	if received != nil {
		t.Fatal("notification handler should have been removed")
	}
}

func TestCommandExecOutputDeltaMalformedNotificationReportsHandlerError(t *testing.T) {
	mock := NewMockTransport()

	var (
		gotMethod string
		gotErr    error
	)
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
		gotMethod = method
		gotErr = err
	}))

	var called bool
	client.OnCommandExecOutputDelta(func(codex.CommandExecOutputDeltaNotification) {
		called = true
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "command/exec/outputDelta",
		Params:  json.RawMessage(`{"capReached":true,"deltaBase64":123,"processId":"proc-123","stream":"stdout"}`),
	})

	if called {
		t.Fatal("handler should not be called for malformed payload")
	}
	if gotMethod != "command/exec/outputDelta" {
		t.Fatalf("handler error method = %q; want %q", gotMethod, "command/exec/outputDelta")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "unmarshal command/exec/outputDelta") {
		t.Fatalf("handler error = %v; want unmarshal failure", gotErr)
	}
}

func TestCommandExec_RPCError_ReturnsRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	mock.SetResponse("command/exec", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    codex.ErrCodeInvalidParams,
			Message: "command array must not be empty",
		},
	})

	_, err := client.Command.Exec(context.Background(), codex.CommandExecParams{
		Command: []string{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rpcErr *codex.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected error to unwrap to *RPCError, got %T", err)
	}
	if rpcErr.RPCError().Code != codex.ErrCodeInvalidParams {
		t.Errorf("expected error code %d, got %d", codex.ErrCodeInvalidParams, rpcErr.RPCError().Code)
	}
}
