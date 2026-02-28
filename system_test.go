package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestWindowsSandboxSetupStart tests the client→server WindowsSandboxSetupStart request
func TestWindowsSandboxSetupStart(t *testing.T) {
	tests := []struct {
		name     string
		params   codex.WindowsSandboxSetupStartParams
		response map[string]interface{}
	}{
		{
			name: "elevated mode",
			params: codex.WindowsSandboxSetupStartParams{
				Mode: codex.WindowsSandboxSetupModeElevated,
			},
			response: map[string]interface{}{},
		},
		{
			name: "unelevated mode",
			params: codex.WindowsSandboxSetupStartParams{
				Mode: codex.WindowsSandboxSetupModeUnelevated,
			},
			response: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			_ = mock.SetResponseData("windowsSandbox/setupStart", tt.response)

			resp, err := client.System.WindowsSandboxSetupStart(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("WindowsSandboxSetupStart failed: %v", err)
			}

			_ = resp

			req := mock.GetSentRequest(0)
			if req.Method != "windowsSandbox/setupStart" {
				t.Errorf("Expected method 'windowsSandbox/setupStart', got %q", req.Method)
			}

			var sentParams codex.WindowsSandboxSetupStartParams
			if err := json.Unmarshal(req.Params, &sentParams); err != nil {
				t.Fatalf("Failed to unmarshal params: %v", err)
			}
			if sentParams.Mode != tt.params.Mode {
				t.Errorf("Expected mode %q, got %q", tt.params.Mode, sentParams.Mode)
			}
		})
	}
}

// TestWindowsSandboxSetupCompletedNotification tests the server→client notification
func TestWindowsSandboxSetupCompletedNotification(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected codex.WindowsSandboxSetupCompletedNotification
	}{
		{
			name: "success elevated",
			params: map[string]interface{}{
				"mode":    "elevated",
				"success": true,
			},
			expected: codex.WindowsSandboxSetupCompletedNotification{
				Mode:    codex.WindowsSandboxSetupModeElevated,
				Success: true,
				Error:   nil,
			},
		},
		{
			name: "failure with error",
			params: map[string]interface{}{
				"mode":    "unelevated",
				"success": false,
				"error":   "Failed to create sandbox",
			},
			expected: codex.WindowsSandboxSetupCompletedNotification{
				Mode:    codex.WindowsSandboxSetupModeUnelevated,
				Success: false,
				Error:   ptr("Failed to create sandbox"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON deserialization
			paramsJSON, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("Failed to marshal params: %v", err)
			}

			var notif codex.WindowsSandboxSetupCompletedNotification
			if err := json.Unmarshal(paramsJSON, &notif); err != nil {
				t.Fatalf("Failed to unmarshal notification: %v", err)
			}

			if notif.Mode != tt.expected.Mode {
				t.Errorf("Expected mode %q, got %q", tt.expected.Mode, notif.Mode)
			}
			if notif.Success != tt.expected.Success {
				t.Errorf("Expected success %v, got %v", tt.expected.Success, notif.Success)
			}
			if (notif.Error == nil) != (tt.expected.Error == nil) {
				t.Errorf("Expected error nil=%v, got nil=%v", tt.expected.Error == nil, notif.Error == nil)
			}
			if notif.Error != nil && tt.expected.Error != nil && *notif.Error != *tt.expected.Error {
				t.Errorf("Expected error %q, got %q", *tt.expected.Error, *notif.Error)
			}

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.WindowsSandboxSetupCompletedNotification
			client.OnWindowsSandboxSetupCompleted(func(n codex.WindowsSandboxSetupCompletedNotification) {
				received = &n
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "windowsSandbox/setupCompleted",
				Params:  paramsJSON,
			})

			if received == nil {
				t.Fatal("Listener not called")
			}
			if received.Mode != tt.expected.Mode {
				t.Errorf("Listener received mode %q, expected %q", received.Mode, tt.expected.Mode)
			}
		})
	}
}

// TestWindowsWorldWritableWarningNotification tests the world-writable warning notification
func TestWindowsWorldWritableWarningNotification(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected codex.WindowsWorldWritableWarningNotification
	}{
		{
			name: "no extra files",
			params: map[string]interface{}{
				"extraCount":  float64(0),
				"failedScan":  false,
				"samplePaths": []interface{}{"/tmp/file1", "/tmp/file2"},
			},
			expected: codex.WindowsWorldWritableWarningNotification{
				ExtraCount:  0,
				FailedScan:  false,
				SamplePaths: []string{"/tmp/file1", "/tmp/file2"},
			},
		},
		{
			name: "with extra files and failed scan",
			params: map[string]interface{}{
				"extraCount":  float64(5),
				"failedScan":  true,
				"samplePaths": []interface{}{"/tmp/writable"},
			},
			expected: codex.WindowsWorldWritableWarningNotification{
				ExtraCount:  5,
				FailedScan:  true,
				SamplePaths: []string{"/tmp/writable"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON deserialization
			paramsJSON, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("Failed to marshal params: %v", err)
			}

			var notif codex.WindowsWorldWritableWarningNotification
			if err := json.Unmarshal(paramsJSON, &notif); err != nil {
				t.Fatalf("Failed to unmarshal notification: %v", err)
			}

			if notif.ExtraCount != tt.expected.ExtraCount {
				t.Errorf("Expected extraCount %d, got %d", tt.expected.ExtraCount, notif.ExtraCount)
			}
			if notif.FailedScan != tt.expected.FailedScan {
				t.Errorf("Expected failedScan %v, got %v", tt.expected.FailedScan, notif.FailedScan)
			}
			if len(notif.SamplePaths) != len(tt.expected.SamplePaths) {
				t.Fatalf("Expected %d sample paths, got %d", len(tt.expected.SamplePaths), len(notif.SamplePaths))
			}

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.WindowsWorldWritableWarningNotification
			client.OnWindowsWorldWritableWarning(func(n codex.WindowsWorldWritableWarningNotification) {
				received = &n
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "windows/worldWritableWarning",
				Params:  paramsJSON,
			})

			if received == nil {
				t.Fatal("Listener not called")
			}
			if received.ExtraCount != tt.expected.ExtraCount {
				t.Errorf("Listener received extraCount %d, expected %d", received.ExtraCount, tt.expected.ExtraCount)
			}
		})
	}
}

// TestContextCompactedNotification tests the deprecated context compaction notification
func TestContextCompactedNotification(t *testing.T) {
	params := map[string]interface{}{
		"threadId": "thread-123",
		"turnId":   "turn-456",
	}

	// Test JSON deserialization
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal params: %v", err)
	}

	var notif codex.ContextCompactedNotification
	if err := json.Unmarshal(paramsJSON, &notif); err != nil {
		t.Fatalf("Failed to unmarshal notification: %v", err)
	}

	if notif.ThreadID != "thread-123" {
		t.Errorf("Expected threadId 'thread-123', got %q", notif.ThreadID)
	}
	if notif.TurnID != "turn-456" {
		t.Errorf("Expected turnId 'turn-456', got %q", notif.TurnID)
	}

	// Test listener dispatch
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	var received *codex.ContextCompactedNotification
	client.OnContextCompacted(func(n codex.ContextCompactedNotification) {
		received = &n
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/compacted",
		Params:  paramsJSON,
	})

	if received == nil {
		t.Fatal("Listener not called")
	}
	if received.ThreadID != "thread-123" {
		t.Errorf("Listener received threadId %q, expected 'thread-123'", received.ThreadID)
	}
}

// TestDeprecationNoticeNotification tests the deprecation notice notification
func TestDeprecationNoticeNotification(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected codex.DeprecationNoticeNotification
	}{
		{
			name: "summary only",
			params: map[string]interface{}{
				"summary": "Feature X is deprecated",
			},
			expected: codex.DeprecationNoticeNotification{
				Summary: "Feature X is deprecated",
				Details: nil,
			},
		},
		{
			name: "with details",
			params: map[string]interface{}{
				"summary": "Feature X is deprecated",
				"details": "Use Feature Y instead. Migration guide: https://...",
			},
			expected: codex.DeprecationNoticeNotification{
				Summary: "Feature X is deprecated",
				Details: ptr("Use Feature Y instead. Migration guide: https://..."),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON deserialization
			paramsJSON, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("Failed to marshal params: %v", err)
			}

			var notif codex.DeprecationNoticeNotification
			if err := json.Unmarshal(paramsJSON, &notif); err != nil {
				t.Fatalf("Failed to unmarshal notification: %v", err)
			}

			if notif.Summary != tt.expected.Summary {
				t.Errorf("Expected summary %q, got %q", tt.expected.Summary, notif.Summary)
			}
			if (notif.Details == nil) != (tt.expected.Details == nil) {
				t.Errorf("Expected details nil=%v, got nil=%v", tt.expected.Details == nil, notif.Details == nil)
			}
			if notif.Details != nil && tt.expected.Details != nil && *notif.Details != *tt.expected.Details {
				t.Errorf("Expected details %q, got %q", *tt.expected.Details, *notif.Details)
			}

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.DeprecationNoticeNotification
			client.OnDeprecationNotice(func(n codex.DeprecationNoticeNotification) {
				received = &n
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "deprecationNotice",
				Params:  paramsJSON,
			})

			if received == nil {
				t.Fatal("Listener not called")
			}
			if received.Summary != tt.expected.Summary {
				t.Errorf("Listener received summary %q, expected %q", received.Summary, tt.expected.Summary)
			}
		})
	}
}

// TestErrorNotification tests the system error notification
func TestErrorNotification(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected codex.ErrorNotification
	}{
		{
			name: "simple string error",
			params: map[string]interface{}{
				"threadId":  "thread-123",
				"turnId":    "turn-456",
				"willRetry": true,
				"error": map[string]interface{}{
					"message": "Server error occurred",
				},
			},
			expected: codex.ErrorNotification{
				ThreadID:  "thread-123",
				TurnID:    "turn-456",
				WillRetry: true,
			},
		},
		{
			name: "with context window exceeded",
			params: map[string]interface{}{
				"threadId":  "thread-123",
				"turnId":    "turn-456",
				"willRetry": false,
				"error": map[string]interface{}{
					"message":        "Context limit exceeded",
					"codexErrorInfo": "contextWindowExceeded",
				},
			},
			expected: codex.ErrorNotification{
				ThreadID:  "thread-123",
				TurnID:    "turn-456",
				WillRetry: false,
			},
		},
		{
			name: "with http connection failed",
			params: map[string]interface{}{
				"threadId":  "thread-123",
				"turnId":    "turn-456",
				"willRetry": true,
				"error": map[string]interface{}{
					"message": "Connection failed",
					"codexErrorInfo": map[string]interface{}{
						"httpConnectionFailed": map[string]interface{}{
							"httpStatusCode": float64(503),
						},
					},
					"additionalDetails": "Retry in 5s",
				},
			},
			expected: codex.ErrorNotification{
				ThreadID:  "thread-123",
				TurnID:    "turn-456",
				WillRetry: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON deserialization
			paramsJSON, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("Failed to marshal params: %v", err)
			}

			var notif codex.ErrorNotification
			if err := json.Unmarshal(paramsJSON, &notif); err != nil {
				t.Fatalf("Failed to unmarshal notification: %v", err)
			}

			if notif.ThreadID != tt.expected.ThreadID {
				t.Errorf("Expected threadId %q, got %q", tt.expected.ThreadID, notif.ThreadID)
			}
			if notif.TurnID != tt.expected.TurnID {
				t.Errorf("Expected turnId %q, got %q", tt.expected.TurnID, notif.TurnID)
			}
			if notif.WillRetry != tt.expected.WillRetry {
				t.Errorf("Expected willRetry %v, got %v", tt.expected.WillRetry, notif.WillRetry)
			}

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.ErrorNotification
			client.OnError(func(n codex.ErrorNotification) {
				received = &n
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "error",
				Params:  paramsJSON,
			})

			if received == nil {
				t.Fatal("Listener not called")
			}
			if received.ThreadID != tt.expected.ThreadID {
				t.Errorf("Listener received threadId %q, expected %q", received.ThreadID, tt.expected.ThreadID)
			}
		})
	}
}

// TestTerminalInteractionNotification tests the terminal interaction notification
func TestTerminalInteractionNotification(t *testing.T) {
	params := map[string]interface{}{
		"threadId":  "thread-123",
		"turnId":    "turn-456",
		"itemId":    "item-789",
		"processId": "process-abc",
		"stdin":     "ls -la\n",
	}

	// Test JSON deserialization
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal params: %v", err)
	}

	var notif codex.TerminalInteractionNotification
	if err := json.Unmarshal(paramsJSON, &notif); err != nil {
		t.Fatalf("Failed to unmarshal notification: %v", err)
	}

	if notif.ThreadID != "thread-123" {
		t.Errorf("Expected threadId 'thread-123', got %q", notif.ThreadID)
	}
	if notif.TurnID != "turn-456" {
		t.Errorf("Expected turnId 'turn-456', got %q", notif.TurnID)
	}
	if notif.ItemID != "item-789" {
		t.Errorf("Expected itemId 'item-789', got %q", notif.ItemID)
	}
	if notif.ProcessID != "process-abc" {
		t.Errorf("Expected processId 'process-abc', got %q", notif.ProcessID)
	}
	if notif.Stdin != "ls -la\n" {
		t.Errorf("Expected stdin 'ls -la\\n', got %q", notif.Stdin)
	}

	// Test listener dispatch
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	var received *codex.TerminalInteractionNotification
	client.OnTerminalInteraction(func(n codex.TerminalInteractionNotification) {
		received = &n
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/commandExecution/terminalInteraction",
		Params:  paramsJSON,
	})

	if received == nil {
		t.Fatal("Listener not called")
	}
	if received.ThreadID != "thread-123" {
		t.Errorf("Listener received threadId %q, expected 'thread-123'", received.ThreadID)
	}
	if received.Stdin != "ls -la\n" {
		t.Errorf("Listener received stdin %q, expected 'ls -la\\n'", received.Stdin)
	}
}

// TestSystemServiceMethodSignatures verifies all SystemService methods exist with correct signatures
func TestSystemServiceMethodSignatures(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Verify SystemService exists
	if client.System == nil {
		t.Fatal("Expected client.System to be non-nil")
	}

	// Test WindowsSandboxSetupStart method signature
	_ = mock.SetResponseData("windowsSandbox/setupStart", map[string]interface{}{})
	_, err := client.System.WindowsSandboxSetupStart(context.Background(), codex.WindowsSandboxSetupStartParams{
		Mode: codex.WindowsSandboxSetupModeElevated,
	})
	if err != nil {
		t.Errorf("WindowsSandboxSetupStart failed: %v", err)
	}
}
