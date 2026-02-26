package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestThreadStartedNotification tests ThreadStartedNotification deserialization
func TestThreadStartedNotification(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		validate func(*testing.T, codex.ThreadStartedNotification)
	}{
		{
			name: "minimal thread started",
			jsonData: `{
				"thread": {
					"id": "thread-123",
					"cliVersion": "1.0.0",
					"createdAt": 1234567890,
					"cwd": "/home/user/project",
					"modelProvider": "openai",
					"preview": "Hello world",
					"source": "cli",
					"status": {"type": "idle"},
					"turns": [],
					"updatedAt": 1234567890
				}
			}`,
			validate: func(t *testing.T, n codex.ThreadStartedNotification) {
				if n.Thread.ID != "thread-123" {
					t.Errorf("expected thread ID 'thread-123', got %q", n.Thread.ID)
				}
				if n.Thread.CLIVersion != "1.0.0" {
					t.Errorf("expected CLI version '1.0.0', got %q", n.Thread.CLIVersion)
				}
			},
		},
		{
			name: "thread started with optional fields",
			jsonData: `{
				"thread": {
					"id": "thread-456",
					"cliVersion": "1.0.0",
					"createdAt": 1234567890,
					"cwd": "/home/user/project",
					"modelProvider": "openai",
					"preview": "Test thread",
					"source": "vscode",
					"status": {"type": "active", "activeFlags": ["waitingOnApproval"]},
					"turns": [],
					"updatedAt": 1234567890,
					"name": "My Thread",
					"agentNickname": "agent-1",
					"agentRole": "developer"
				}
			}`,
			validate: func(t *testing.T, n codex.ThreadStartedNotification) {
				if n.Thread.ID != "thread-456" {
					t.Errorf("expected thread ID 'thread-456', got %q", n.Thread.ID)
				}
				if n.Thread.Name == nil || *n.Thread.Name != "My Thread" {
					t.Errorf("expected thread name 'My Thread', got %v", n.Thread.Name)
				}
				if n.Thread.AgentNickname == nil || *n.Thread.AgentNickname != "agent-1" {
					t.Errorf("expected agent nickname 'agent-1', got %v", n.Thread.AgentNickname)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var notification codex.ThreadStartedNotification
			if err := json.Unmarshal([]byte(tt.jsonData), &notification); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			tt.validate(t, notification)
		})
	}
}

// TestThreadClosedNotification tests ThreadClosedNotification deserialization
func TestThreadClosedNotification(t *testing.T) {
	jsonData := `{"threadId": "thread-123"}`

	var notification codex.ThreadClosedNotification
	if err := json.Unmarshal([]byte(jsonData), &notification); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if notification.ThreadID != "thread-123" {
		t.Errorf("expected threadId 'thread-123', got %q", notification.ThreadID)
	}
}

// TestThreadArchivedNotification tests ThreadArchivedNotification deserialization
func TestThreadArchivedNotification(t *testing.T) {
	jsonData := `{"threadId": "thread-456"}`

	var notification codex.ThreadArchivedNotification
	if err := json.Unmarshal([]byte(jsonData), &notification); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if notification.ThreadID != "thread-456" {
		t.Errorf("expected threadId 'thread-456', got %q", notification.ThreadID)
	}
}

// TestThreadUnarchivedNotification tests ThreadUnarchivedNotification deserialization
func TestThreadUnarchivedNotification(t *testing.T) {
	jsonData := `{"threadId": "thread-789"}`

	var notification codex.ThreadUnarchivedNotification
	if err := json.Unmarshal([]byte(jsonData), &notification); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if notification.ThreadID != "thread-789" {
		t.Errorf("expected threadId 'thread-789', got %q", notification.ThreadID)
	}
}

// TestThreadNameUpdatedNotification tests ThreadNameUpdatedNotification deserialization
func TestThreadNameUpdatedNotification(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		validate func(*testing.T, codex.ThreadNameUpdatedNotification)
	}{
		{
			name:     "with thread name",
			jsonData: `{"threadId": "thread-123", "threadName": "Updated Name"}`,
			validate: func(t *testing.T, n codex.ThreadNameUpdatedNotification) {
				if n.ThreadID != "thread-123" {
					t.Errorf("expected threadId 'thread-123', got %q", n.ThreadID)
				}
				if n.ThreadName == nil || *n.ThreadName != "Updated Name" {
					t.Errorf("expected threadName 'Updated Name', got %v", n.ThreadName)
				}
			},
		},
		{
			name:     "with null thread name",
			jsonData: `{"threadId": "thread-456", "threadName": null}`,
			validate: func(t *testing.T, n codex.ThreadNameUpdatedNotification) {
				if n.ThreadID != "thread-456" {
					t.Errorf("expected threadId 'thread-456', got %q", n.ThreadID)
				}
				if n.ThreadName != nil {
					t.Errorf("expected threadName to be nil, got %v", *n.ThreadName)
				}
			},
		},
		{
			name:     "without thread name field",
			jsonData: `{"threadId": "thread-789"}`,
			validate: func(t *testing.T, n codex.ThreadNameUpdatedNotification) {
				if n.ThreadID != "thread-789" {
					t.Errorf("expected threadId 'thread-789', got %q", n.ThreadID)
				}
				if n.ThreadName != nil {
					t.Errorf("expected threadName to be nil, got %v", *n.ThreadName)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var notification codex.ThreadNameUpdatedNotification
			if err := json.Unmarshal([]byte(tt.jsonData), &notification); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			tt.validate(t, notification)
		})
	}
}

// TestThreadStatusChangedNotification tests ThreadStatusChangedNotification deserialization
func TestThreadStatusChangedNotification(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		validate func(*testing.T, codex.ThreadStatusChangedNotification)
	}{
		{
			name:     "idle status",
			jsonData: `{"threadId": "thread-123", "status": {"type": "idle"}}`,
			validate: func(t *testing.T, n codex.ThreadStatusChangedNotification) {
				if n.ThreadID != "thread-123" {
					t.Errorf("expected threadId 'thread-123', got %q", n.ThreadID)
				}
				// Validate that status unmarshaled successfully
				// Detailed status validation would require accessing the wrapper's value
			},
		},
		{
			name:     "active status with flags",
			jsonData: `{"threadId": "thread-456", "status": {"type": "active", "activeFlags": ["waitingOnApproval", "waitingOnUserInput"]}}`,
			validate: func(t *testing.T, n codex.ThreadStatusChangedNotification) {
				if n.ThreadID != "thread-456" {
					t.Errorf("expected threadId 'thread-456', got %q", n.ThreadID)
				}
			},
		},
		{
			name:     "system error status",
			jsonData: `{"threadId": "thread-789", "status": {"type": "systemError"}}`,
			validate: func(t *testing.T, n codex.ThreadStatusChangedNotification) {
				if n.ThreadID != "thread-789" {
					t.Errorf("expected threadId 'thread-789', got %q", n.ThreadID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var notification codex.ThreadStatusChangedNotification
			if err := json.Unmarshal([]byte(tt.jsonData), &notification); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			tt.validate(t, notification)
		})
	}
}

// TestThreadTokenUsageUpdatedNotification tests ThreadTokenUsageUpdatedNotification deserialization
func TestThreadTokenUsageUpdatedNotification(t *testing.T) {
	jsonData := `{
		"threadId": "thread-123",
		"turnId": "turn-456",
		"tokenUsage": {
			"last": {
				"cachedInputTokens": 100,
				"inputTokens": 500,
				"outputTokens": 200,
				"reasoningOutputTokens": 50,
				"totalTokens": 850
			},
			"total": {
				"cachedInputTokens": 300,
				"inputTokens": 1500,
				"outputTokens": 600,
				"reasoningOutputTokens": 150,
				"totalTokens": 2550
			},
			"modelContextWindow": 128000
		}
	}`

	var notification codex.ThreadTokenUsageUpdatedNotification
	if err := json.Unmarshal([]byte(jsonData), &notification); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if notification.ThreadID != "thread-123" {
		t.Errorf("expected threadId 'thread-123', got %q", notification.ThreadID)
	}
	if notification.TurnID != "turn-456" {
		t.Errorf("expected turnId 'turn-456', got %q", notification.TurnID)
	}
	if notification.TokenUsage.Last.InputTokens != 500 {
		t.Errorf("expected last inputTokens 500, got %d", notification.TokenUsage.Last.InputTokens)
	}
	if notification.TokenUsage.Total.TotalTokens != 2550 {
		t.Errorf("expected total totalTokens 2550, got %d", notification.TokenUsage.Total.TotalTokens)
	}
	if notification.TokenUsage.ModelContextWindow == nil || *notification.TokenUsage.ModelContextWindow != 128000 {
		t.Errorf("expected modelContextWindow 128000, got %v", notification.TokenUsage.ModelContextWindow)
	}
}

// TestThreadNotificationListenerRegistration tests that notification listeners can be registered and dispatched
func TestThreadNotificationListenerRegistration(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	ctx := context.Background()

	t.Run("ThreadStarted listener", func(t *testing.T) {
		called := false
		var receivedNotification codex.ThreadStartedNotification

		client.OnThreadStarted(func(n codex.ThreadStartedNotification) {
			called = true
			receivedNotification = n
		})

		// Inject a thread/started notification from the server
		notificationJSON := `{
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
		}`
		mock.InjectServerNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/started",
			Params:  json.RawMessage(notificationJSON),
		})

		if !called {
			t.Error("thread/started listener was not called")
		}
		if receivedNotification.Thread.ID != "thread-123" {
			t.Errorf("expected thread ID 'thread-123', got %q", receivedNotification.Thread.ID)
		}
	})

	t.Run("ThreadClosed listener", func(t *testing.T) {
		called := false
		var receivedNotification codex.ThreadClosedNotification

		client.OnThreadClosed(func(n codex.ThreadClosedNotification) {
			called = true
			receivedNotification = n
		})

		mock.InjectServerNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/closed",
			Params:  json.RawMessage(`{"threadId": "thread-456"}`),
		})

		if !called {
			t.Error("thread/closed listener was not called")
		}
		if receivedNotification.ThreadID != "thread-456" {
			t.Errorf("expected thread ID 'thread-456', got %q", receivedNotification.ThreadID)
		}
	})

	t.Run("ThreadArchived listener", func(t *testing.T) {
		called := false
		var receivedNotification codex.ThreadArchivedNotification

		client.OnThreadArchived(func(n codex.ThreadArchivedNotification) {
			called = true
			receivedNotification = n
		})

		mock.InjectServerNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/archived",
			Params:  json.RawMessage(`{"threadId": "thread-789"}`),
		})

		if !called {
			t.Error("thread/archived listener was not called")
		}
		if receivedNotification.ThreadID != "thread-789" {
			t.Errorf("expected thread ID 'thread-789', got %q", receivedNotification.ThreadID)
		}
	})

	t.Run("ThreadUnarchived listener", func(t *testing.T) {
		called := false
		var receivedNotification codex.ThreadUnarchivedNotification

		client.OnThreadUnarchived(func(n codex.ThreadUnarchivedNotification) {
			called = true
			receivedNotification = n
		})

		mock.InjectServerNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/unarchived",
			Params:  json.RawMessage(`{"threadId": "thread-101"}`),
		})

		if !called {
			t.Error("thread/unarchived listener was not called")
		}
		if receivedNotification.ThreadID != "thread-101" {
			t.Errorf("expected thread ID 'thread-101', got %q", receivedNotification.ThreadID)
		}
	})

	t.Run("ThreadNameUpdated listener", func(t *testing.T) {
		called := false
		var receivedNotification codex.ThreadNameUpdatedNotification

		client.OnThreadNameUpdated(func(n codex.ThreadNameUpdatedNotification) {
			called = true
			receivedNotification = n
		})

		mock.InjectServerNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/nameUpdated",
			Params:  json.RawMessage(`{"threadId": "thread-202", "threadName": "New Name"}`),
		})

		if !called {
			t.Error("thread/nameUpdated listener was not called")
		}
		if receivedNotification.ThreadID != "thread-202" {
			t.Errorf("expected thread ID 'thread-202', got %q", receivedNotification.ThreadID)
		}
	})

	t.Run("ThreadStatusChanged listener", func(t *testing.T) {
		called := false
		var receivedNotification codex.ThreadStatusChangedNotification

		client.OnThreadStatusChanged(func(n codex.ThreadStatusChangedNotification) {
			called = true
			receivedNotification = n
		})

		mock.InjectServerNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/statusChanged",
			Params:  json.RawMessage(`{"threadId": "thread-303", "status": {"type": "idle"}}`),
		})

		if !called {
			t.Error("thread/statusChanged listener was not called")
		}
		if receivedNotification.ThreadID != "thread-303" {
			t.Errorf("expected thread ID 'thread-303', got %q", receivedNotification.ThreadID)
		}
	})

	t.Run("ThreadTokenUsageUpdated listener", func(t *testing.T) {
		called := false
		var receivedNotification codex.ThreadTokenUsageUpdatedNotification

		client.OnThreadTokenUsageUpdated(func(n codex.ThreadTokenUsageUpdatedNotification) {
			called = true
			receivedNotification = n
		})

		notificationJSON := `{
			"threadId": "thread-404",
			"turnId": "turn-505",
			"tokenUsage": {
				"last": {
					"cachedInputTokens": 0,
					"inputTokens": 100,
					"outputTokens": 50,
					"reasoningOutputTokens": 0,
					"totalTokens": 150
				},
				"total": {
					"cachedInputTokens": 0,
					"inputTokens": 100,
					"outputTokens": 50,
					"reasoningOutputTokens": 0,
					"totalTokens": 150
				}
			}
		}`
		mock.InjectServerNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/tokenUsageUpdated",
			Params:  json.RawMessage(notificationJSON),
		})

		if !called {
			t.Error("thread/tokenUsageUpdated listener was not called")
		}
		if receivedNotification.ThreadID != "thread-404" {
			t.Errorf("expected thread ID 'thread-404', got %q", receivedNotification.ThreadID)
		}
		if receivedNotification.TurnID != "turn-505" {
			t.Errorf("expected turn ID 'turn-505', got %q", receivedNotification.TurnID)
		}
	})
}
