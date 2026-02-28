package codex_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestThreadStart tests the ThreadService.Start method
func TestThreadStart(t *testing.T) {
	t.Run("start thread with minimal params", func(t *testing.T) {
		transport := NewMockTransport()
		defer func() { _ = transport.Close() }()

		client := codex.NewClient(transport)

		// Inject a mock response (simplified for now - full implementation will parse ThreadStartResponse.json)
		_ = transport.SetResponseData("thread/start", map[string]interface{}{
			"thread": map[string]interface{}{
				"id":            "thread-123",
				"cliVersion":    "1.0.0",
				"createdAt":     int64(1234567890),
				"cwd":           "/test/dir",
				"modelProvider": "openai",
				"preview":       "test preview",
				"source":        "cli",
				"status":        map[string]interface{}{"type": "idle"},
				"turns":         []interface{}{},
				"updatedAt":     int64(1234567890),
			},
			"approvalPolicy": "untrusted",
			"cwd":            "/test/dir",
			"model":          "gpt-4",
			"modelProvider":  "openai",
			"sandbox":        map[string]interface{}{"type": "dangerFullAccess"},
		})

		params := codex.ThreadStartParams{
			// All fields optional per spec
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		response, err := client.Thread.Start(ctx, params)
		if err != nil {
			t.Fatalf("Thread.Start failed: %v", err)
		}

		// Verify response has required fields
		if response.Thread.ID == "" {
			t.Error("expected response.Thread.ID to be non-empty")
		}

		// Verify correct JSON-RPC method was sent
		req := transport.GetSentRequest(0)
		if req == nil {
			t.Fatal("expected request to be sent")
		}
		if req.Method != "thread/start" {
			t.Errorf("expected method 'thread/start', got %q", req.Method)
		}
	})

	t.Run("start thread with all optional params", func(t *testing.T) {
		transport := NewMockTransport()
		defer func() { _ = transport.Close() }()

		client := codex.NewClient(transport)

		_ = transport.SetResponseData("thread/start", map[string]interface{}{
			"thread": map[string]interface{}{
				"id":            "thread-456",
				"cliVersion":    "1.0.0",
				"createdAt":     int64(1234567890),
				"cwd":           "/custom/dir",
				"modelProvider": "anthropic",
				"preview":       "test preview",
				"source":        "cli",
				"status":        map[string]interface{}{"type": "idle"},
				"turns":         []interface{}{},
				"updatedAt":     int64(1234567890),
			},
			"approvalPolicy": "never",
			"cwd":            "/custom/dir",
			"model":          "claude-4",
			"modelProvider":  "anthropic",
			"sandbox":        map[string]interface{}{"type": "dangerFullAccess"},
		})

		params := codex.ThreadStartParams{
			Cwd:                   strPtr("/custom/dir"),
			Model:                 strPtr("claude-4"),
			ModelProvider:         strPtr("anthropic"),
			BaseInstructions:      strPtr("You are a helpful assistant"),
			DeveloperInstructions: strPtr("Follow best practices"),
			Ephemeral:             boolPtr(true),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		response, err := client.Thread.Start(ctx, params)
		if err != nil {
			t.Fatalf("Thread.Start failed: %v", err)
		}

		if response.Thread.ID != "thread-456" {
			t.Errorf("expected thread ID 'thread-456', got %q", response.Thread.ID)
		}
	})
}

// TestThreadRead tests the ThreadService.Read method
func TestThreadRead(t *testing.T) {
	t.Run("read thread without turns", func(t *testing.T) {
		transport := NewMockTransport()
		defer func() { _ = transport.Close() }()

		client := codex.NewClient(transport)

		_ = transport.SetResponseData("thread/read", map[string]interface{}{
			"approvalPolicy": "untrusted",
			"cwd":            "/test/dir",
			"model":          "gpt-4",
			"modelProvider":  "openai",
			"sandbox":        map[string]interface{}{"type": "dangerFullAccess"},
			"thread": map[string]interface{}{
				"id":            "thread-123",
				"cliVersion":    "1.0.0",
				"createdAt":     int64(1234567890),
				"cwd":           "/test/dir",
				"modelProvider": "openai",
				"preview":       "test preview",
				"source":        "cli",
				"status":        map[string]interface{}{"type": "idle"},
				"turns":         []interface{}{},
				"updatedAt":     int64(1234567890),
			},
		})

		params := codex.ThreadReadParams{
			ThreadID:     "thread-123",
			IncludeTurns: boolPtr(false),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		response, err := client.Thread.Read(ctx, params)
		if err != nil {
			t.Fatalf("Thread.Read failed: %v", err)
		}

		if response.Thread.ID != "thread-123" {
			t.Errorf("expected thread ID 'thread-123', got %q", response.Thread.ID)
		}

		// Verify correct method
		req := transport.GetSentRequest(0)
		if req == nil {
			t.Fatal("expected request to be sent")
		}
		if req.Method != "thread/read" {
			t.Errorf("expected method 'thread/read', got %q", req.Method)
		}
	})
}

// TestThreadList tests the ThreadService.List method
func TestThreadList(t *testing.T) {
	t.Run("list all threads", func(t *testing.T) {
		transport := NewMockTransport()
		defer func() { _ = transport.Close() }()

		client := codex.NewClient(transport)

		_ = transport.SetResponseData("thread/list", map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{
					"id":            "thread-1",
					"cliVersion":    "1.0.0",
					"createdAt":     int64(1234567890),
					"cwd":           "/test/dir",
					"modelProvider": "openai",
					"preview":       "preview 1",
					"source":        "cli",
					"status":        map[string]interface{}{"type": "idle"},
					"turns":         []interface{}{},
					"updatedAt":     int64(1234567890),
				},
				map[string]interface{}{
					"id":            "thread-2",
					"cliVersion":    "1.0.0",
					"createdAt":     int64(1234567891),
					"cwd":           "/test/dir2",
					"modelProvider": "anthropic",
					"preview":       "preview 2",
					"source":        "vscode",
					"status":        map[string]interface{}{"type": "idle"},
					"turns":         []interface{}{},
					"updatedAt":     int64(1234567891),
				},
			},
		})

		params := codex.ThreadListParams{}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		response, err := client.Thread.List(ctx, params)
		if err != nil {
			t.Fatalf("Thread.List failed: %v", err)
		}

		if len(response.Data) != 2 {
			t.Errorf("expected 2 threads, got %d", len(response.Data))
		}

		// Verify method
		req := transport.GetSentRequest(0)
		if req.Method != "thread/list" {
			t.Errorf("expected method 'thread/list', got %q", req.Method)
		}
	})
}

// TestThreadLoadedList tests the ThreadService.LoadedList method
func TestThreadLoadedList(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/loaded/list", map[string]interface{}{
		"data": []interface{}{
			"thread-loaded",
		},
	})

	params := codex.ThreadLoadedListParams{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Thread.LoadedList(ctx, params)
	if err != nil {
		t.Fatalf("Thread.LoadedList failed: %v", err)
	}

	if len(response.Data) != 1 {
		t.Errorf("expected 1 thread ID, got %d", len(response.Data))
	}

	req := transport.GetSentRequest(0)
	if req.Method != "thread/loaded/list" {
		t.Errorf("expected method 'thread/loadedList', got %q", req.Method)
	}
}

// TestThreadResume tests the ThreadService.Resume method
func TestThreadResume(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/resume", map[string]interface{}{
		"approvalPolicy": "untrusted",
		"cwd":            "/test/dir",
		"model":          "gpt-4",
		"modelProvider":  "openai",
		"sandbox":        map[string]interface{}{"type": "dangerFullAccess"},
		"thread": map[string]interface{}{
			"id":            "thread-resume",
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/test/dir",
			"modelProvider": "openai",
			"preview":       "resumed thread",
			"source":        "cli",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     int64(1234567890),
		},
	})

	params := codex.ThreadResumeParams{
		ThreadID: "thread-resume",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Thread.Resume(ctx, params)
	if err != nil {
		t.Fatalf("Thread.Resume failed: %v", err)
	}

	if response.Thread.ID != "thread-resume" {
		t.Errorf("expected thread ID 'thread-resume', got %q", response.Thread.ID)
	}

	req := transport.GetSentRequest(0)
	if req.Method != "thread/resume" {
		t.Errorf("expected method 'thread/resume', got %q", req.Method)
	}
}

// TestThreadFork tests the ThreadService.Fork method
func TestThreadFork(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/fork", map[string]interface{}{
		"approvalPolicy": "untrusted",
		"cwd":            "/test/dir",
		"model":          "gpt-4",
		"modelProvider":  "openai",
		"sandbox":        map[string]interface{}{"type": "dangerFullAccess"},
		"thread": map[string]interface{}{
			"id":            "thread-forked",
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/test/dir",
			"modelProvider": "openai",
			"preview":       "forked thread",
			"source":        "cli",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     int64(1234567890),
		},
	})

	params := codex.ThreadForkParams{
		ThreadID: "thread-original",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Thread.Fork(ctx, params)
	if err != nil {
		t.Fatalf("Thread.Fork failed: %v", err)
	}

	if response.Thread.ID != "thread-forked" {
		t.Errorf("expected thread ID 'thread-forked', got %q", response.Thread.ID)
	}

	req := transport.GetSentRequest(0)
	if req.Method != "thread/fork" {
		t.Errorf("expected method 'thread/fork', got %q", req.Method)
	}
}

// TestThreadRollback tests the ThreadService.Rollback method
func TestThreadRollback(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/rollback", map[string]interface{}{
		"approvalPolicy": "untrusted",
		"cwd":            "/test/dir",
		"model":          "gpt-4",
		"modelProvider":  "openai",
		"sandbox":        map[string]interface{}{"type": "dangerFullAccess"},
		"thread": map[string]interface{}{
			"id":            "thread-rollback",
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/test/dir",
			"modelProvider": "openai",
			"preview":       "rolled back thread",
			"source":        "cli",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     int64(1234567890),
		},
	})

	params := codex.ThreadRollbackParams{
		ThreadID: "thread-rollback",
		NumTurns: 3,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Thread.Rollback(ctx, params)
	if err != nil {
		t.Fatalf("Thread.Rollback failed: %v", err)
	}

	if response.Thread.ID != "thread-rollback" {
		t.Errorf("expected thread ID 'thread-rollback', got %q", response.Thread.ID)
	}

	req := transport.GetSentRequest(0)
	if req.Method != "thread/rollback" {
		t.Errorf("expected method 'thread/rollback', got %q", req.Method)
	}
}

// TestThreadSetName tests the ThreadService.SetName method
func TestThreadSetName(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/name/set", map[string]interface{}{
		"thread": map[string]interface{}{
			"id":            "thread-123",
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/test/dir",
			"modelProvider": "openai",
			"name":          "My Custom Thread Name",
			"preview":       "test preview",
			"source":        "cli",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     int64(1234567891),
		},
	})

	params := codex.ThreadSetNameParams{
		ThreadID: "thread-123",
		Name:     "My Custom Thread Name",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Thread.SetName(ctx, params)
	if err != nil {
		t.Fatalf("Thread.SetName failed: %v", err)
	}

	// SetName returns empty response per spec
	_ = response

	req := transport.GetSentRequest(0)
	if req.Method != "thread/name/set" {
		t.Errorf("expected method 'thread/setName', got %q", req.Method)
	}
}

// TestThreadArchive tests the ThreadService.Archive method
func TestThreadArchive(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/archive", map[string]interface{}{
		"thread": map[string]interface{}{
			"id":            "thread-archived",
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/test/dir",
			"modelProvider": "openai",
			"preview":       "archived thread",
			"source":        "cli",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     int64(1234567890),
		},
	})

	params := codex.ThreadArchiveParams{
		ThreadID: "thread-archived",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Thread.Archive(ctx, params)
	if err != nil {
		t.Fatalf("Thread.Archive failed: %v", err)
	}

	// Archive returns empty response per spec
	_ = response

	req := transport.GetSentRequest(0)
	if req.Method != "thread/archive" {
		t.Errorf("expected method 'thread/archive', got %q", req.Method)
	}
}

// TestThreadUnarchive tests the ThreadService.Unarchive method
func TestThreadUnarchive(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/unarchive", map[string]interface{}{
		"thread": map[string]interface{}{
			"id":            "thread-unarchived",
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/test/dir",
			"modelProvider": "openai",
			"preview":       "unarchived thread",
			"source":        "cli",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     int64(1234567890),
		},
	})

	params := codex.ThreadUnarchiveParams{
		ThreadID: "thread-unarchived",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Thread.Unarchive(ctx, params)
	if err != nil {
		t.Fatalf("Thread.Unarchive failed: %v", err)
	}

	if response.Thread.ID != "thread-unarchived" {
		t.Errorf("expected thread ID 'thread-unarchived', got %q", response.Thread.ID)
	}

	req := transport.GetSentRequest(0)
	if req.Method != "thread/unarchive" {
		t.Errorf("expected method 'thread/unarchive', got %q", req.Method)
	}
}

// TestThreadUnsubscribe tests the ThreadService.Unsubscribe method
func TestThreadUnsubscribe(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/unsubscribe", map[string]interface{}{})

	params := codex.ThreadUnsubscribeParams{
		ThreadID: "thread-unsub",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Thread.Unsubscribe(ctx, params)
	if err != nil {
		t.Fatalf("Thread.Unsubscribe failed: %v", err)
	}

	// ThreadUnsubscribeResponse should be an empty struct
	_ = response

	req := transport.GetSentRequest(0)
	if req.Method != "thread/unsubscribe" {
		t.Errorf("expected method 'thread/unsubscribe', got %q", req.Method)
	}
}

// TestThreadCompactStart tests the ThreadService.CompactStart method
func TestThreadCompactStart(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/compact/start", map[string]interface{}{
		"threadId": "compact-thread-id",
	})

	params := codex.ThreadCompactStartParams{
		ThreadID: "thread-to-compact",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Thread.CompactStart(ctx, params)
	if err != nil {
		t.Fatalf("Thread.CompactStart failed: %v", err)
	}

	// CompactStart returns empty response per spec
	_ = response

	req := transport.GetSentRequest(0)
	if req.Method != "thread/compact/start" {
		t.Errorf("expected method 'thread/compactStart', got %q", req.Method)
	}
}

// TestThreadServiceMethodSignatures ensures all methods exist on ThreadService
func TestThreadServiceMethodSignatures(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	// This test will fail to compile if any method is missing
	var service = client.Thread

	if service == nil {
		t.Fatal("Thread service should not be nil")
	}

	// Verify methods exist (compile-time check)
	_ = service.Start
	_ = service.Read
	_ = service.List
	_ = service.LoadedList
	_ = service.Resume
	_ = service.Fork
	_ = service.Rollback
	_ = service.SetName
	_ = service.Archive
	_ = service.Unarchive
	_ = service.Unsubscribe
	_ = service.CompactStart
}

// TestThreadParamsSerialization tests that params serialize correctly to JSON
func TestThreadParamsSerialization(t *testing.T) {
	t.Run("ThreadStartParams with complex nested types", func(t *testing.T) {
		params := codex.ThreadStartParams{
			Cwd:                   strPtr("/test"),
			Model:                 strPtr("gpt-4"),
			ModelProvider:         strPtr("openai"),
			BaseInstructions:      strPtr("base"),
			DeveloperInstructions: strPtr("dev"),
			Ephemeral:             boolPtr(true),
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("failed to marshal ThreadStartParams: %v", err)
		}

		var decoded map[string]interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		// Verify optional fields are present
		if decoded["cwd"] != "/test" {
			t.Error("expected cwd field in JSON")
		}
		if decoded["model"] != "gpt-4" {
			t.Error("expected model field in JSON")
		}
	})

	t.Run("ThreadReadParams", func(t *testing.T) {
		params := codex.ThreadReadParams{
			ThreadID:     "thread-123",
			IncludeTurns: boolPtr(true),
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("failed to marshal ThreadReadParams: %v", err)
		}

		var decoded map[string]interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if decoded["threadId"] != "thread-123" {
			t.Error("expected threadId field")
		}
		if decoded["includeTurns"] != true {
			t.Error("expected includeTurns to be true")
		}
	})

	t.Run("ThreadForkParams", func(t *testing.T) {
		params := codex.ThreadForkParams{
			ThreadID: "thread-original",
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("failed to marshal ThreadForkParams: %v", err)
		}

		var decoded map[string]interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if decoded["threadId"] != "thread-original" {
			t.Error("expected threadId field")
		}
	})
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
