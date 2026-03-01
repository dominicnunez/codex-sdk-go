package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestKnownNotificationDispatch verifies that known notification methods
// are dispatched to the correct registered listeners.
func TestKnownNotificationDispatch(t *testing.T) {
	ctx := context.Background()
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	tests := []struct {
		name         string
		method       string
		register     func(client *codex.Client, called *bool)
		params       interface{}
		expectCalled bool
	}{
		{
			name:   "thread/started",
			method: "thread/started",
			register: func(client *codex.Client, called *bool) {
				client.OnThreadStarted(func(n codex.ThreadStartedNotification) { *called = true })
			},
			params: map[string]interface{}{
				"thread": map[string]interface{}{
					"id":            "thread-123",
					"cliVersion":    "1.0.0",
					"createdAt":     float64(1704067200),
					"cwd":           "/home/user",
					"modelProvider": "anthropic",
					"preview":       "test thread",
					"source":        "cli",
					"status":        map[string]interface{}{"type": "idle"},
					"turns":         []interface{}{},
					"updatedAt":     float64(1704067200),
				},
			},
			expectCalled: true,
		},
		{
			name:   "thread/closed",
			method: "thread/closed",
			register: func(client *codex.Client, called *bool) {
				client.OnThreadClosed(func(n codex.ThreadClosedNotification) { *called = true })
			},
			params: map[string]interface{}{
				"threadId": "thread-123",
			},
			expectCalled: true,
		},
		{
			name:   "turn/started",
			method: "turn/started",
			register: func(client *codex.Client, called *bool) {
				client.OnTurnStarted(func(n codex.TurnStartedNotification) { *called = true })
			},
			params: map[string]interface{}{
				"threadId": "thread-123",
				"turnId":   float64(1),
			},
			expectCalled: true,
		},
		{
			name:   "turn/completed",
			method: "turn/completed",
			register: func(client *codex.Client, called *bool) {
				client.OnTurnCompleted(func(n codex.TurnCompletedNotification) { *called = true })
			},
			params: map[string]interface{}{
				"threadId": "thread-123",
				"turnId":   float64(1),
			},
			expectCalled: true,
		},
		{
			name:   "account/updated",
			method: "account/updated",
			register: func(client *codex.Client, called *bool) {
				client.OnAccountUpdated(func(n codex.AccountUpdatedNotification) { *called = true })
			},
			params: map[string]interface{}{
				"account": map[string]interface{}{
					"authMode": "apikey",
					"email":    "user@example.com",
					"id":       "user-123",
				},
			},
			expectCalled: true,
		},
		{
			name:   "configWarning",
			method: "configWarning",
			register: func(client *codex.Client, called *bool) {
				client.OnConfigWarning(func(n codex.ConfigWarningNotification) { *called = true })
			},
			params: map[string]interface{}{
				"summary": "Invalid config value",
			},
			expectCalled: true,
		},
		{
			name:   "model/rerouted",
			method: "model/rerouted",
			register: func(client *codex.Client, called *bool) {
				client.OnModelRerouted(func(n codex.ModelReroutedNotification) { *called = true })
			},
			params: map[string]interface{}{
				"threadId":  "thread-123",
				"turnId":    "turn-1",
				"fromModel": "claude-opus-4",
				"toModel":   "claude-sonnet-4",
				"reason":    "highRiskCyberActivity",
			},
			expectCalled: true,
		},
		{
			name:   "app/list/updated",
			method: "app/list/updated",
			register: func(client *codex.Client, called *bool) {
				client.OnAppListUpdated(func(n codex.AppListUpdatedNotification) { *called = true })
			},
			params: map[string]interface{}{
				"data": []interface{}{},
			},
			expectCalled: true,
		},
		{
			name:   "item/agentMessage/delta",
			method: "item/agentMessage/delta",
			register: func(client *codex.Client, called *bool) {
				client.OnAgentMessageDelta(func(n codex.AgentMessageDeltaNotification) { *called = true })
			},
			params: map[string]interface{}{
				"delta":    "Hello",
				"itemId":   "item-123",
				"threadId": "thread-123",
				"turnId":   "turn-1",
			},
			expectCalled: true,
		},
		{
			name:   "thread/realtime/started",
			method: "thread/realtime/started",
			register: func(client *codex.Client, called *bool) {
				client.OnThreadRealtimeStarted(func(n codex.ThreadRealtimeStartedNotification) { *called = true })
			},
			params: map[string]interface{}{
				"threadId": "thread-123",
			},
			expectCalled: true,
		},
		{
			name:   "error",
			method: "error",
			register: func(client *codex.Client, called *bool) {
				client.OnError(func(n codex.ErrorNotification) { *called = true })
			},
			params: map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Something went wrong",
				},
				"threadId": "thread-123",
				"turnId":   "turn-1",
			},
			expectCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.Reset()
			client = codex.NewClient(mock)

			called := false
			tt.register(client, &called)

			// Inject server notification
			paramsJSON, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("failed to marshal params: %v", err)
			}

			notif := codex.Notification{
				JSONRPC: "2.0",
				Method:  tt.method,
				Params:  json.RawMessage(paramsJSON),
			}

			mock.InjectServerNotification(ctx, notif)

			// Verify handler was called
			if called != tt.expectCalled {
				t.Errorf("expected called=%v, got %v", tt.expectCalled, called)
			}
		})
	}
}

// TestKnownApprovalHandlerDispatch verifies that known server→client request
// methods are dispatched to the correct approval handlers.
func TestKnownApprovalHandlerDispatch(t *testing.T) {
	ctx := context.Background()
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	tests := []struct {
		name         string
		method       string
		params       interface{}
		handler      func(*codex.ApprovalHandlers)
		expectCalled bool
	}{
		{
			name:   "applyPatchApproval",
			method: "applyPatchApproval",
			params: map[string]interface{}{
				"diff": "diff --git a/file.txt b/file.txt",
			},
			handler: func(ah *codex.ApprovalHandlers) {
				ah.OnApplyPatchApproval = func(ctx context.Context, p codex.ApplyPatchApprovalParams) (codex.ApplyPatchApprovalResponse, error) {
					return codex.ApplyPatchApprovalResponse{
						Decision: codex.ReviewDecisionWrapper{Value: "approved"},
					}, nil
				}
			},
			expectCalled: true,
		},
		{
			name:   "item/commandExecution/requestApproval",
			method: "item/commandExecution/requestApproval",
			params: map[string]interface{}{
				"command": []interface{}{"ls", "-la"},
			},
			handler: func(ah *codex.ApprovalHandlers) {
				ah.OnCommandExecutionRequestApproval = func(ctx context.Context, p codex.CommandExecutionRequestApprovalParams) (codex.CommandExecutionRequestApprovalResponse, error) {
					return codex.CommandExecutionRequestApprovalResponse{
						Decision: codex.CommandExecutionApprovalDecisionWrapper{Value: "accept"},
					}, nil
				}
			},
			expectCalled: true,
		},
		{
			name:   "execCommandApproval",
			method: "execCommandApproval",
			params: map[string]interface{}{
				"command": []interface{}{"echo", "hello"},
			},
			handler: func(ah *codex.ApprovalHandlers) {
				ah.OnExecCommandApproval = func(ctx context.Context, p codex.ExecCommandApprovalParams) (codex.ExecCommandApprovalResponse, error) {
					return codex.ExecCommandApprovalResponse{
						Decision: codex.ReviewDecisionWrapper{Value: "approved"},
					}, nil
				}
			},
			expectCalled: true,
		},
		{
			name:   "item/fileChange/requestApproval",
			method: "item/fileChange/requestApproval",
			params: map[string]interface{}{
				"changes": []interface{}{
					map[string]interface{}{
						"type": "add",
						"path": "/tmp/test.txt",
					},
				},
			},
			handler: func(ah *codex.ApprovalHandlers) {
				ah.OnFileChangeRequestApproval = func(ctx context.Context, p codex.FileChangeRequestApprovalParams) (codex.FileChangeRequestApprovalResponse, error) {
					return codex.FileChangeRequestApprovalResponse{Decision: "accept"}, nil
				}
			},
			expectCalled: true,
		},
		{
			name:   "item/tool/call",
			method: "item/tool/call",
			params: map[string]interface{}{
				"name": "test-tool",
			},
			handler: func(ah *codex.ApprovalHandlers) {
				ah.OnDynamicToolCall = func(ctx context.Context, p codex.DynamicToolCallParams) (codex.DynamicToolCallResponse, error) {
					return codex.DynamicToolCallResponse{
						Success:      true,
						ContentItems: []codex.DynamicToolCallOutputContentItemWrapper{},
					}, nil
				}
			},
			expectCalled: true,
		},
		{
			name:   "item/tool/requestUserInput",
			method: "item/tool/requestUserInput",
			params: map[string]interface{}{
				"prompt": "Enter value:",
			},
			handler: func(ah *codex.ApprovalHandlers) {
				ah.OnToolRequestUserInput = func(ctx context.Context, p codex.ToolRequestUserInputParams) (codex.ToolRequestUserInputResponse, error) {
					return codex.ToolRequestUserInputResponse{
						Answers: map[string]codex.ToolRequestUserInputAnswer{
							"q1": {Answers: []string{"test-value"}},
						},
					}, nil
				}
			},
			expectCalled: true,
		},
		{
			name:   "account/chatgptAuthTokens/refresh",
			method: "account/chatgptAuthTokens/refresh",
			params: map[string]interface{}{
				"authTokens": map[string]interface{}{
					"accessToken":  "access-123",
					"refreshToken": "refresh-456",
				},
			},
			handler: func(ah *codex.ApprovalHandlers) {
				ah.OnChatgptAuthTokensRefresh = func(ctx context.Context, p codex.ChatgptAuthTokensRefreshParams) (codex.ChatgptAuthTokensRefreshResponse, error) {
					return codex.ChatgptAuthTokensRefreshResponse{}, nil
				}
			},
			expectCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.Reset()
			client = codex.NewClient(mock)

			// Register approval handler
			var handlers codex.ApprovalHandlers
			tt.handler(&handlers)
			client.SetApprovalHandlers(handlers)

			// Inject server request
			paramsJSON, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("failed to marshal params: %v", err)
			}

			req := codex.Request{
				JSONRPC: "2.0",
				Method:  tt.method,
				ID:      codex.RequestID{Value: float64(1)},
				Params:  json.RawMessage(paramsJSON),
			}

			_, _ = mock.InjectServerRequest(ctx, req)

			// Verify response was sent
			if len(mock.GetSentResponses()) == 0 {
				t.Fatalf("expected response to be sent, got none")
			}

			resp := mock.GetSentResponses()[0]
			if resp.Error != nil {
				t.Errorf("expected no error in response, got: %v", resp.Error)
			}
		})
	}
}

// TestUnknownNotificationIgnored verifies that unknown notification methods
// are silently ignored and do not dispatch to any registered handler.
func TestUnknownNotificationIgnored(t *testing.T) {
	ctx := context.Background()
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Register a handler for a known method to verify it is NOT called
	// for an unknown method.
	called := false
	client.OnNotification("known/method", func(_ context.Context, _ codex.Notification) {
		called = true
	})

	// Inject unknown notification
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "unknown/notification",
		Params:  json.RawMessage(`{"data":"test"}`),
	}

	mock.InjectServerNotification(ctx, notif)

	if called {
		t.Error("known handler was called for an unknown notification method")
	}
}

// TestUnknownRequestReturnsMethodNotFound verifies that unknown server→client
// request methods return a JSON-RPC method-not-found error.
func TestUnknownRequestReturnsMethodNotFound(t *testing.T) {
	ctx := context.Background()
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Don't register any approval handlers
	client.SetApprovalHandlers(codex.ApprovalHandlers{})

	// Inject unknown server request
	req := codex.Request{
		JSONRPC: "2.0",
		Method:  "unknown/request",
		ID:      codex.RequestID{Value: float64(1)},
		Params:  json.RawMessage(`{"data":"test"}`),
	}

	_, _ = mock.InjectServerRequest(ctx, req)

	// Verify method-not-found error response was sent
	if len(mock.GetSentResponses()) == 0 {
		t.Fatalf("expected error response to be sent, got none")
	}

	resp := mock.GetSentResponses()[0]
	if resp.Error == nil {
		t.Fatalf("expected error in response, got nil")
	}

	if resp.Error.Code != codex.ErrCodeMethodNotFound {
		t.Errorf("expected error code %d (method not found), got %d", codex.ErrCodeMethodNotFound, resp.Error.Code)
	}

	if resp.Error.Message == "" {
		t.Errorf("expected error message, got empty string")
	}
}

// TestMultipleListenersForSameNotification verifies that multiple listeners
// can be registered for the same notification method (last one wins).
func TestMultipleListenersForSameNotification(t *testing.T) {
	ctx := context.Background()
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	firstCalled := false
	secondCalled := false

	// Register first listener
	client.OnThreadClosed(func(n codex.ThreadClosedNotification) {
		firstCalled = true
	})

	// Register second listener (should override first)
	client.OnThreadClosed(func(n codex.ThreadClosedNotification) {
		secondCalled = true
	})

	// Inject notification
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/closed",
		Params:  json.RawMessage(`{"threadId":"thread-123"}`),
	}

	mock.InjectServerNotification(ctx, notif)

	// Only second listener should be called (last one wins)
	if firstCalled {
		t.Errorf("expected first listener not to be called")
	}
	if !secondCalled {
		t.Errorf("expected second listener to be called")
	}
}

// TestApprovalHandlerMarshalFailureReturnsError verifies that when an
// approval handler returns a response that fails to marshal, the client's
// handleRequest propagates the error to the transport layer.
func TestApprovalHandlerMarshalFailureReturnsError(t *testing.T) {
	ctx := context.Background()
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Register handler that returns a response with an unmarshalable value.
	// ReviewDecisionWrapper.MarshalJSON returns an error for unknown types.
	client.SetApprovalHandlers(codex.ApprovalHandlers{
		OnApplyPatchApproval: func(ctx context.Context, p codex.ApplyPatchApprovalParams) (codex.ApplyPatchApprovalResponse, error) {
			return codex.ApplyPatchApprovalResponse{
				Decision: codex.ReviewDecisionWrapper{Value: 42}, // int triggers default error branch
			}, nil
		},
	})

	req := codex.Request{
		JSONRPC: "2.0",
		Method:  "applyPatchApproval",
		ID:      codex.RequestID{Value: float64(1)},
		Params:  json.RawMessage(`{"diff":"test"}`),
	}

	// The mock transport calls Client.handleRequest directly, which returns
	// the marshal error. In production, StdioTransport.handleRequest converts
	// this into an ErrCodeInternalError JSON-RPC response.
	_, err := mock.InjectServerRequest(ctx, req)
	if err == nil {
		t.Fatal("expected marshal error from handleRequest, got nil")
	}
}

// TestMissingApprovalHandlerReturnsMethodNotFound verifies that when an
// approval handler is not set, a method-not-found error is returned.
func TestMissingApprovalHandlerReturnsMethodNotFound(t *testing.T) {
	ctx := context.Background()
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Set approval handlers but leave OnApplyPatchApproval nil
	client.SetApprovalHandlers(codex.ApprovalHandlers{
		OnExecCommandApproval: func(ctx context.Context, p codex.ExecCommandApprovalParams) (codex.ExecCommandApprovalResponse, error) {
			return codex.ExecCommandApprovalResponse{
				Decision: codex.ReviewDecisionWrapper{Value: "approved"},
			}, nil
		},
	})

	// Inject server request for unhandled method
	req := codex.Request{
		JSONRPC: "2.0",
		Method:  "applyPatchApproval",
		ID:      codex.RequestID{Value: float64(1)},
		Params:  json.RawMessage(`{"diff":"test"}`),
	}

	_, _ = mock.InjectServerRequest(ctx, req)

	// Verify method-not-found error response
	if len(mock.GetSentResponses()) == 0 {
		t.Fatalf("expected error response to be sent, got none")
	}

	resp := mock.GetSentResponses()[0]
	if resp.Error == nil {
		t.Fatalf("expected error in response, got nil")
	}

	if resp.Error.Code != codex.ErrCodeMethodNotFound {
		t.Errorf("expected error code %d (method not found), got %d", codex.ErrCodeMethodNotFound, resp.Error.Code)
	}
}

// TestSetApprovalHandlersConcurrentWithRequests verifies that calling
// SetApprovalHandlers concurrently with incoming server requests does
// not race (run with -race).
func TestSetApprovalHandlersConcurrentWithRequests(t *testing.T) {
	ctx := context.Background()
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	const iterations = 100

	// Continuously swap approval handlers while injecting requests.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < iterations; i++ {
			client.SetApprovalHandlers(codex.ApprovalHandlers{
				OnApplyPatchApproval: func(_ context.Context, _ codex.ApplyPatchApprovalParams) (codex.ApplyPatchApprovalResponse, error) {
					return codex.ApplyPatchApprovalResponse{
						Decision: codex.ReviewDecisionWrapper{Value: "approved"},
					}, nil
				},
			})
		}
	}()

	for i := 0; i < iterations; i++ {
		req := codex.Request{
			JSONRPC: "2.0",
			Method:  "applyPatchApproval",
			ID:      codex.RequestID{Value: float64(i)},
			Params:  json.RawMessage(`{"diff":"test"}`),
		}
		_, _ = mock.InjectServerRequest(ctx, req)
	}

	<-done
}

// TestNilHandlerDeregistration verifies that passing nil to an On* notification
// method removes the previously registered handler so it no longer fires.
func TestNilHandlerDeregistration(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		method     string
		register   func(client *codex.Client, called *bool)
		deregister func(client *codex.Client)
		params     string
	}{
		{
			name:   "OnThreadStarted",
			method: "thread/started",
			register: func(client *codex.Client, called *bool) {
				client.OnThreadStarted(func(n codex.ThreadStartedNotification) {
					*called = true
				})
			},
			deregister: func(client *codex.Client) {
				client.OnThreadStarted(nil)
			},
			params: `{
				"thread": {
					"id": "thread-123",
					"cliVersion": "1.0.0",
					"createdAt": 1234567890,
					"cwd": "/home/user/project",
					"modelProvider": "openai",
					"preview": "Test",
					"source": "cli",
					"status": {"type": "idle"},
					"turns": [],
					"updatedAt": 1234567890
				}
			}`,
		},
		{
			name:   "OnThreadClosed",
			method: "thread/closed",
			register: func(client *codex.Client, called *bool) {
				client.OnThreadClosed(func(n codex.ThreadClosedNotification) {
					*called = true
				})
			},
			deregister: func(client *codex.Client) {
				client.OnThreadClosed(nil)
			},
			params: `{"threadId": "thread-123"}`,
		},
		{
			name:   "OnThreadArchived",
			method: "thread/archived",
			register: func(client *codex.Client, called *bool) {
				client.OnThreadArchived(func(n codex.ThreadArchivedNotification) {
					*called = true
				})
			},
			deregister: func(client *codex.Client) {
				client.OnThreadArchived(nil)
			},
			params: `{"threadId": "thread-123"}`,
		},
		{
			name:   "OnThreadUnarchived",
			method: "thread/unarchived",
			register: func(client *codex.Client, called *bool) {
				client.OnThreadUnarchived(func(n codex.ThreadUnarchivedNotification) {
					*called = true
				})
			},
			deregister: func(client *codex.Client) {
				client.OnThreadUnarchived(nil)
			},
			params: `{"threadId": "thread-123"}`,
		},
		{
			name:   "OnThreadStatusChanged",
			method: "thread/status/changed",
			register: func(client *codex.Client, called *bool) {
				client.OnThreadStatusChanged(func(n codex.ThreadStatusChangedNotification) {
					*called = true
				})
			},
			deregister: func(client *codex.Client) {
				client.OnThreadStatusChanged(nil)
			},
			params: `{"threadId": "thread-123", "status": {"type": "idle"}}`,
		},
		{
			name:   "OnError",
			method: "error",
			register: func(client *codex.Client, called *bool) {
				client.OnError(func(n codex.ErrorNotification) {
					*called = true
				})
			},
			deregister: func(client *codex.Client) {
				client.OnError(nil)
			},
			params: `{"error": {"message": "test error"}, "threadId": "thread-123", "turnId": "turn-1"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			called := false
			tt.register(client, &called)

			// Fire notification: handler should be called.
			mock.InjectServerNotification(ctx, codex.Notification{
				JSONRPC: "2.0",
				Method:  tt.method,
				Params:  json.RawMessage(tt.params),
			})

			if !called {
				t.Fatalf("handler was not called before deregistration")
			}

			// Deregister by passing nil.
			tt.deregister(client)

			// Reset flag and fire again: handler should NOT be called.
			called = false
			mock.InjectServerNotification(ctx, codex.Notification{
				JSONRPC: "2.0",
				Method:  tt.method,
				Params:  json.RawMessage(tt.params),
			})

			if called {
				t.Errorf("handler was called after nil deregistration")
			}
		})
	}
}
