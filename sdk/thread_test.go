package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/sdk"
)

// TestThreadStart tests the ThreadService.Start method
func TestThreadStart(t *testing.T) {
	t.Run("start thread with minimal params", func(t *testing.T) {
		transport := NewMockTransport()
		defer func() { _ = transport.Close() }()

		client := codex.NewClient(transport)

		_ = transport.SetResponseData("thread/start", validThreadLifecycleResponse(map[string]interface{}{
			"id":            "thread-123",
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/test/dir",
			"ephemeral":     false,
			"modelProvider": "openai",
			"preview":       "test preview",
			"source":        "cli",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     int64(1234567890),
		}))

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
			return
		}
		if req.Method != "thread/start" {
			t.Errorf("expected method 'thread/start', got %q", req.Method)
		}
	})

	t.Run("start thread with all optional params", func(t *testing.T) {
		transport := NewMockTransport()
		defer func() { _ = transport.Close() }()

		client := codex.NewClient(transport)

		fixture := validThreadLifecycleResponse(map[string]interface{}{
			"id":            "thread-456",
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/custom/dir",
			"ephemeral":     true,
			"modelProvider": "anthropic",
			"preview":       "test preview",
			"source":        "cli",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     int64(1234567890),
		})
		fixture["approvalPolicy"] = "never"
		fixture["cwd"] = "/custom/dir"
		fixture["model"] = "claude-4"
		fixture["modelProvider"] = "anthropic"
		_ = transport.SetResponseData("thread/start", fixture)

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

	t.Run("rejects response missing thread id", func(t *testing.T) {
		transport := NewMockTransport()
		defer func() { _ = transport.Close() }()

		client := codex.NewClient(transport)
		_ = transport.SetResponseData("thread/start", validThreadLifecycleResponse(map[string]interface{}{
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/test/dir",
			"ephemeral":     false,
			"modelProvider": "openai",
			"preview":       "test preview",
			"source":        "cli",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     int64(1234567890),
		}))

		_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "missing thread.id") {
			t.Fatalf("error = %q, want missing thread.id", err.Error())
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
				"ephemeral":     false,
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
			return
		}
		if req.Method != "thread/read" {
			t.Errorf("expected method 'thread/read', got %q", req.Method)
		}
	})
}

func TestThreadRequestsRejectEmptyRequiredIDs(t *testing.T) {
	tests := []struct {
		name    string
		call    func(*codex.Client) error
		wantErr string
	}{
		{
			name: "read rejects empty thread id",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{})
				return err
			},
			wantErr: "threadId must not be empty",
		},
		{
			name: "resume rejects empty thread id",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{})
				return err
			},
			wantErr: "threadId must not be empty",
		},
		{
			name: "fork rejects empty thread id",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{})
				return err
			},
			wantErr: "threadId must not be empty",
		},
		{
			name: "rollback rejects empty thread id",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Rollback(context.Background(), codex.ThreadRollbackParams{
					NumTurns: 1,
				})
				return err
			},
			wantErr: "threadId must not be empty",
		},
		{
			name: "set name rejects empty thread id",
			call: func(client *codex.Client) error {
				_, err := client.Thread.SetName(context.Background(), codex.ThreadSetNameParams{
					Name: "name",
				})
				return err
			},
			wantErr: "threadId must not be empty",
		},
		{
			name: "metadata update rejects empty thread id",
			call: func(client *codex.Client) error {
				_, err := client.Thread.MetadataUpdate(context.Background(), codex.ThreadMetadataUpdateParams{})
				return err
			},
			wantErr: "threadId must not be empty",
		},
		{
			name: "archive rejects empty thread id",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Archive(context.Background(), codex.ThreadArchiveParams{})
				return err
			},
			wantErr: "threadId must not be empty",
		},
		{
			name: "unarchive rejects empty thread id",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Unarchive(context.Background(), codex.ThreadUnarchiveParams{})
				return err
			},
			wantErr: "threadId must not be empty",
		},
		{
			name: "unsubscribe rejects empty thread id",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Unsubscribe(context.Background(), codex.ThreadUnsubscribeParams{})
				return err
			},
			wantErr: "threadId must not be empty",
		},
		{
			name: "compact start rejects empty thread id",
			call: func(client *codex.Client) error {
				_, err := client.Thread.CompactStart(context.Background(), codex.ThreadCompactStartParams{})
				return err
			},
			wantErr: "threadId must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected invalid params error")
			}
			if !strings.Contains(err.Error(), "invalid params") {
				t.Fatalf("error = %v, want invalid params context", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
			if transport.CallCount() != 0 {
				t.Fatalf("CallCount() = %d, want 0", transport.CallCount())
			}
		})
	}
}

func TestThreadResponsesRejectMissingRequiredThreadFields(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		payload map[string]interface{}
		call    func(*codex.Client) error
		wantErr string
	}{
		{
			name:   "read rejects empty thread object",
			method: "thread/read",
			payload: map[string]interface{}{
				"thread": map[string]interface{}{},
			},
			call: func(c *codex.Client) error {
				_, err := c.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "thread-123"})
				return err
			},
			wantErr: "missing thread.id",
		},
		{
			name:    "resume rejects empty thread object",
			method:  "thread/resume",
			payload: validThreadLifecycleResponse(map[string]interface{}{}),
			call: func(c *codex.Client) error {
				_, err := c.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "thread-123"})
				return err
			},
			wantErr: "missing thread.id",
		},
		{
			name:    "metadata update rejects missing thread field",
			method:  "thread/metadata/update",
			payload: map[string]interface{}{},
			call: func(c *codex.Client) error {
				_, err := c.Thread.MetadataUpdate(context.Background(), codex.ThreadMetadataUpdateParams{ThreadID: "thread-123"})
				return err
			},
			wantErr: "missing thread.id",
		},
		{
			name:    "unarchive rejects missing thread field",
			method:  "thread/unarchive",
			payload: map[string]interface{}{},
			call: func(c *codex.Client) error {
				_, err := c.Thread.Unarchive(context.Background(), codex.ThreadUnarchiveParams{ThreadID: "thread-123"})
				return err
			},
			wantErr: "missing thread.id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			defer func() { _ = transport.Close() }()

			client := codex.NewClient(transport)
			_ = transport.SetResponseData(tt.method, tt.payload)

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestThreadMetadataWrappersRejectMissingOrEmptyType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		target  interface{}
		wantErr error
	}{
		{
			name:    "thread status missing type",
			input:   `{}`,
			target:  &codex.ThreadStatusWrapper{},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:    "thread status empty type",
			input:   `{"type":""}`,
			target:  &codex.ThreadStatusWrapper{},
			wantErr: nil,
		},
		{
			name:    "read only access missing type",
			input:   `{}`,
			target:  &codex.ReadOnlyAccessWrapper{},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:    "sandbox policy missing type",
			input:   `{}`,
			target:  &codex.SandboxPolicyWrapper{},
			wantErr: codex.ErrMissingResultField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := json.Unmarshal([]byte(tt.input), tt.target)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v; want %v", err, tt.wantErr)
				}
				return
			}
			if err.Error() != "thread status: missing or empty type field" {
				t.Fatalf("error = %v; want empty type failure", err)
			}
		})
	}
}

func TestThreadMetadataWrappersValidateRequiredFieldsAndUnknownTypes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		target  interface{}
		wantErr error
	}{
		{
			name:    "active thread status missing active flags",
			input:   `{"type":"active"}`,
			target:  &codex.ThreadStatusWrapper{},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:    "unknown thread status remains forward compatible",
			input:   `{"type":"futureStatus","extra":true}`,
			target:  &codex.ThreadStatusWrapper{},
			wantErr: nil,
		},
		{
			name:    "unknown read only access remains forward compatible",
			input:   `{"type":"futureAccess","extra":true}`,
			target:  &codex.ReadOnlyAccessWrapper{},
			wantErr: nil,
		},
		{
			name:    "unknown sandbox policy remains forward compatible",
			input:   `{"type":"futureSandbox","extra":true}`,
			target:  &codex.SandboxPolicyWrapper{},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := json.Unmarshal([]byte(tt.input), tt.target)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v; want %v", err, tt.wantErr)
				}
				return
			}
			if tt.name == "active thread status missing active flags" {
				if err == nil || !strings.Contains(err.Error(), "missing required field") {
					t.Fatalf("error = %v; want missing required field failure", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestSessionSourceWrapperRejectsMalformedSubAgentVariants(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     error
		wantContain string
	}{
		{
			name:        "sub-agent object requires discriminator",
			input:       `{"subAgent":{}}`,
			wantContain: "sub-agent source: missing discriminator",
		},
		{
			name:    "thread_spawn requires nested fields",
			input:   `{"subAgent":{"thread_spawn":{}}}`,
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:    "thread_spawn requires parent thread id",
			input:   `{"subAgent":{"thread_spawn":{"depth":1}}}`,
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:    "other requires non-null string value",
			input:   `{"subAgent":{"other":null}}`,
			wantErr: codex.ErrNullResultField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wrapper codex.SessionSourceWrapper
			err := json.Unmarshal([]byte(tt.input), &wrapper)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantContain != "" && !strings.Contains(err.Error(), tt.wantContain) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantContain)
			}
		})
	}
}

func TestThreadLifecycleAndReadResponsesRejectMalformedUnionPayloads(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		payload     map[string]interface{}
		call        func(*codex.Client) error
		wantErr     error
		wantContain string
	}{
		{
			name:   "start rejects approval policy object without discriminator",
			method: "thread/start",
			payload: func() map[string]interface{} {
				response := validThreadLifecycleResponse(validThreadPayload("thread-start"))
				response["approvalPolicy"] = map[string]interface{}{}
				return response
			}(),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{})
				return err
			},
			wantContain: "approval policy: missing discriminator",
		},
		{
			name:   "resume rejects granular approval policy missing required fields",
			method: "thread/resume",
			payload: func() map[string]interface{} {
				response := validThreadLifecycleResponse(validThreadPayload("thread-resume"))
				response["approvalPolicy"] = map[string]interface{}{
					"granular": map[string]interface{}{},
				}
				return response
			}(),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "thread-resume"})
				return err
			},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:   "fork rejects sandbox missing type",
			method: "thread/fork",
			payload: func() map[string]interface{} {
				response := validThreadLifecycleResponse(validThreadPayload("thread-fork"))
				response["sandbox"] = map[string]interface{}{}
				return response
			}(),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: "thread-fork"})
				return err
			},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:   "start rejects nested read only access missing type",
			method: "thread/start",
			payload: func() map[string]interface{} {
				response := validThreadLifecycleResponse(validThreadPayload("thread-start-nested"))
				response["sandbox"] = map[string]interface{}{
					"type":           "workspaceWrite",
					"readOnlyAccess": map[string]interface{}{},
				}
				return response
			}(),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{})
				return err
			},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:   "read rejects status missing type",
			method: "thread/read",
			payload: func() map[string]interface{} {
				thread := validThreadPayload("thread-read")
				thread["status"] = map[string]interface{}{}
				return map[string]interface{}{"thread": thread}
			}(),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "thread-read"})
				return err
			},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:   "read rejects active status missing flags",
			method: "thread/read",
			payload: func() map[string]interface{} {
				thread := validThreadPayload("thread-read-active")
				thread["status"] = map[string]interface{}{"type": "active"}
				return map[string]interface{}{"thread": thread}
			}(),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "thread-read-active"})
				return err
			},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:   "read rejects sub-agent source without discriminator",
			method: "thread/read",
			payload: func() map[string]interface{} {
				thread := validThreadPayload("thread-read-source")
				thread["source"] = map[string]interface{}{
					"subAgent": map[string]interface{}{},
				}
				return map[string]interface{}{"thread": thread}
			}(),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "thread-read-source"})
				return err
			},
			wantContain: "sub-agent source: missing discriminator",
		},
		{
			name:   "read rejects sub-agent other with null value",
			method: "thread/read",
			payload: func() map[string]interface{} {
				thread := validThreadPayload("thread-read-source-other")
				thread["source"] = map[string]interface{}{
					"subAgent": map[string]interface{}{
						"other": nil,
					},
				}
				return map[string]interface{}{"thread": thread}
			}(),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "thread-read-source-other"})
				return err
			},
			wantErr: codex.ErrNullResultField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			defer func() { _ = transport.Close() }()

			client := codex.NewClient(transport)
			_ = transport.SetResponseData(tt.method, tt.payload)

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantContain != "" && !strings.Contains(err.Error(), tt.wantContain) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantContain)
			}
		})
	}
}

func TestThreadLifecycleResponsesRequireApprovalsReviewer(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		payload func() map[string]interface{}
		call    func(*codex.Client) error
		wantErr error
	}{
		{
			name:   "start rejects omitted approvals reviewer",
			method: "thread/start",
			payload: func() map[string]interface{} {
				response := validThreadLifecycleResponse(validThreadPayload("thread-start"))
				delete(response, "approvalsReviewer")
				return response
			},
			call: func(client *codex.Client) error {
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{})
				return err
			},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:   "start rejects null approvals reviewer",
			method: "thread/start",
			payload: func() map[string]interface{} {
				response := validThreadLifecycleResponse(validThreadPayload("thread-start"))
				response["approvalsReviewer"] = nil
				return response
			},
			call: func(client *codex.Client) error {
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{})
				return err
			},
			wantErr: codex.ErrNullResultField,
		},
		{
			name:   "resume rejects omitted approvals reviewer",
			method: "thread/resume",
			payload: func() map[string]interface{} {
				response := validThreadLifecycleResponse(validThreadPayload("thread-resume"))
				delete(response, "approvalsReviewer")
				return response
			},
			call: func(client *codex.Client) error {
				_, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "thread-resume"})
				return err
			},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:   "resume rejects null approvals reviewer",
			method: "thread/resume",
			payload: func() map[string]interface{} {
				response := validThreadLifecycleResponse(validThreadPayload("thread-resume"))
				response["approvalsReviewer"] = nil
				return response
			},
			call: func(client *codex.Client) error {
				_, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "thread-resume"})
				return err
			},
			wantErr: codex.ErrNullResultField,
		},
		{
			name:   "fork rejects omitted approvals reviewer",
			method: "thread/fork",
			payload: func() map[string]interface{} {
				response := validThreadLifecycleResponse(validThreadPayload("thread-fork"))
				delete(response, "approvalsReviewer")
				return response
			},
			call: func(client *codex.Client) error {
				_, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: "thread-fork"})
				return err
			},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:   "fork rejects null approvals reviewer",
			method: "thread/fork",
			payload: func() map[string]interface{} {
				response := validThreadLifecycleResponse(validThreadPayload("thread-fork"))
				response["approvalsReviewer"] = nil
				return response
			},
			call: func(client *codex.Client) error {
				_, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: "thread-fork"})
				return err
			},
			wantErr: codex.ErrNullResultField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			defer func() { _ = transport.Close() }()

			client := codex.NewClient(transport)
			_ = transport.SetResponseData(tt.method, tt.payload())

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestThreadLifecycleResponsesRejectInvalidApprovalsReviewer(t *testing.T) {
	tests := []struct {
		name   string
		method string
		call   func(*codex.Client) error
	}{
		{
			name:   "start",
			method: "thread/start",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{})
				return err
			},
		},
		{
			name:   "resume",
			method: "thread/resume",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "thread-resume"})
				return err
			},
		},
		{
			name:   "fork",
			method: "thread/fork",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: "thread-fork"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			defer func() { _ = transport.Close() }()

			client := codex.NewClient(transport)
			response := validThreadLifecycleResponse(validThreadPayload("thread-invalid-reviewer"))
			response["approvalsReviewer"] = "bot"
			_ = transport.SetResponseData(tt.method, response)

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), `invalid approvalsReviewer "bot"`) {
				t.Fatalf("error = %q, want invalid approvalsReviewer", err.Error())
			}
		})
	}
}

func TestThreadResponseRequiredFieldValidation(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		response map[string]interface{}
		call     func(*codex.Client) error
		wantErr  string
	}{
		{
			name:     "read rejects thread missing ephemeral",
			method:   "thread/read",
			response: threadResponseWithMissingField("thread-ephemeral", "ephemeral"),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "thread-ephemeral"})
				return err
			},
			wantErr: "missing thread.ephemeral",
		},
		{
			name:     "read rejects thread missing cliVersion",
			method:   "thread/read",
			response: threadResponseWithMissingField("thread", "cliVersion"),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "thread-123"})
				return err
			},
			wantErr: "missing thread.cliVersion",
		},
		{
			name:     "resume rejects thread missing cliVersion",
			method:   "thread/resume",
			response: threadLifecycleResponseWithMissingField("thread-resume", "cliVersion"),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "thread-resume"})
				return err
			},
			wantErr: "missing thread.cliVersion",
		},
		{
			name:     "fork rejects thread missing cliVersion",
			method:   "thread/fork",
			response: threadLifecycleResponseWithMissingField("thread-forked", "cliVersion"),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: "thread-original"})
				return err
			},
			wantErr: "missing thread.cliVersion",
		},
		{
			name:     "rollback rejects thread missing cliVersion",
			method:   "thread/rollback",
			response: threadResponseWithMissingField("thread-rollback", "cliVersion"),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Rollback(context.Background(), codex.ThreadRollbackParams{ThreadID: "thread-rollback", NumTurns: 1})
				return err
			},
			wantErr: "missing thread.cliVersion",
		},
		{
			name:     "metadata update rejects thread missing cliVersion",
			method:   "thread/metadata/update",
			response: threadResponseWithMissingField("thread-metadata", "cliVersion"),
			call: func(client *codex.Client) error {
				_, err := client.Thread.MetadataUpdate(context.Background(), codex.ThreadMetadataUpdateParams{ThreadID: "thread-metadata"})
				return err
			},
			wantErr: "missing thread.cliVersion",
		},
		{
			name:     "unarchive rejects thread missing cliVersion",
			method:   "thread/unarchive",
			response: threadResponseWithMissingField("thread-unarchived", "cliVersion"),
			call: func(client *codex.Client) error {
				_, err := client.Thread.Unarchive(context.Background(), codex.ThreadUnarchiveParams{ThreadID: "thread-unarchived"})
				return err
			},
			wantErr: "missing thread.cliVersion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			defer func() { _ = transport.Close() }()

			client := codex.NewClient(transport)
			_ = transport.SetResponseData(tt.method, tt.response)

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
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
					"ephemeral":     false,
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
					"ephemeral":     false,
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
		t.Errorf("expected method 'thread/loaded/list', got %q", req.Method)
	}
}

func TestThreadListRequiresData(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]interface{}
		wantErr error
	}{
		{
			name:    "missing data",
			payload: map[string]interface{}{},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:    "null data",
			payload: map[string]interface{}{"data": nil},
			wantErr: codex.ErrNullResultField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			defer func() { _ = transport.Close() }()

			client := codex.NewClient(transport)
			if err := transport.SetResponseData("thread/list", tt.payload); err != nil {
				t.Fatalf("SetResponseData(thread/list): %v", err)
			}

			_, err := client.Thread.List(context.Background(), codex.ThreadListParams{})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Thread.List error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestThreadLoadedListRequiresData(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]interface{}
		wantErr error
	}{
		{
			name:    "missing data",
			payload: map[string]interface{}{},
			wantErr: codex.ErrMissingResultField,
		},
		{
			name:    "null data",
			payload: map[string]interface{}{"data": nil},
			wantErr: codex.ErrNullResultField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			defer func() { _ = transport.Close() }()

			client := codex.NewClient(transport)
			if err := transport.SetResponseData("thread/loaded/list", tt.payload); err != nil {
				t.Fatalf("SetResponseData(thread/loaded/list): %v", err)
			}

			_, err := client.Thread.LoadedList(context.Background(), codex.ThreadLoadedListParams{})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Thread.LoadedList error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestThreadResume tests the ThreadService.Resume method
func TestThreadResume(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/resume", map[string]interface{}{
		"approvalPolicy":    "untrusted",
		"approvalsReviewer": "user",
		"cwd":               "/test/dir",
		"model":             "gpt-4",
		"modelProvider":     "openai",
		"sandbox":           map[string]interface{}{"type": "dangerFullAccess"},
		"thread": map[string]interface{}{
			"id":            "thread-resume",
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/test/dir",
			"ephemeral":     false,
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
		"approvalPolicy":    "untrusted",
		"approvalsReviewer": "user",
		"cwd":               "/test/dir",
		"model":             "gpt-4",
		"modelProvider":     "openai",
		"sandbox":           map[string]interface{}{"type": "dangerFullAccess"},
		"thread": map[string]interface{}{
			"id":            "thread-forked",
			"cliVersion":    "1.0.0",
			"createdAt":     int64(1234567890),
			"cwd":           "/test/dir",
			"ephemeral":     false,
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

func TestThreadLifecycleRejectsInvalidServiceTier(t *testing.T) {
	tests := []struct {
		name   string
		method string
		call   func(client *codex.Client) error
	}{
		{
			name:   "start",
			method: "thread/start",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{})
				return err
			},
		},
		{
			name:   "resume",
			method: "thread/resume",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "thread-123"})
				return err
			},
		},
		{
			name:   "fork",
			method: "thread/fork",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: "thread-123"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			defer func() { _ = transport.Close() }()

			client := codex.NewClient(transport)
			_ = transport.SetResponseData(tt.method, map[string]interface{}{
				"approvalPolicy":    "untrusted",
				"approvalsReviewer": "user",
				"cwd":               "/test/dir",
				"model":             "gpt-4",
				"modelProvider":     "openai",
				"sandbox":           map[string]interface{}{"type": "dangerFullAccess"},
				"serviceTier":       "totally-invalid",
				"thread": map[string]interface{}{
					"id":            "thread-123",
					"cliVersion":    "1.0.0",
					"createdAt":     int64(1234567890),
					"cwd":           "/test/dir",
					"ephemeral":     false,
					"modelProvider": "openai",
					"preview":       "thread preview",
					"source":        "cli",
					"status":        map[string]interface{}{"type": "idle"},
					"turns":         []interface{}{},
					"updatedAt":     int64(1234567890),
				},
			})

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), `invalid serviceTier "totally-invalid"`) {
				t.Fatalf("error = %q, want invalid serviceTier", err.Error())
			}
		})
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
			"ephemeral":     false,
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

	_ = transport.SetResponseData("thread/name/set", map[string]interface{}{})

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
		t.Errorf("expected method 'thread/name/set', got %q", req.Method)
	}
}

// TestThreadArchive tests the ThreadService.Archive method
func TestThreadArchive(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/archive", map[string]interface{}{})

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
			"ephemeral":     false,
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

func threadResponseWithMissingField(threadID string, missingField string) map[string]interface{} {
	thread := validThreadPayload(threadID)
	delete(thread, missingField)
	return map[string]interface{}{
		"thread": thread,
	}
}

func threadLifecycleResponseWithMissingField(threadID string, missingField string) map[string]interface{} {
	response := validThreadLifecycleResponse(validThreadPayload(threadID))
	thread := response["thread"].(map[string]interface{})
	delete(thread, missingField)
	return response
}

func validThreadPayload(threadID string) map[string]interface{} {
	return map[string]interface{}{
		"id":            threadID,
		"cliVersion":    "1.0.0",
		"createdAt":     int64(1234567890),
		"cwd":           "/test/dir",
		"ephemeral":     false,
		"modelProvider": "openai",
		"preview":       "test preview",
		"source":        "cli",
		"status":        map[string]interface{}{"type": "idle"},
		"turns":         []interface{}{},
		"updatedAt":     int64(1234567890),
	}
}

func TestThreadReadRejectsInvalidInboundEnums(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]interface{})
		wantErr string
	}{
		{
			name: "invalid active flag",
			mutate: func(thread map[string]interface{}) {
				thread["status"] = map[string]interface{}{
					"type":        "active",
					"activeFlags": []interface{}{"bogus"},
				}
			},
			wantErr: `invalid thread.status.activeFlags "bogus"`,
		},
		{
			name: "invalid turn status",
			mutate: func(thread map[string]interface{}) {
				thread["turns"] = []interface{}{
					map[string]interface{}{
						"id":     "turn-1",
						"status": "bogus",
						"items":  []interface{}{},
					},
				}
			},
			wantErr: `invalid turn.status "bogus"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			defer func() { _ = transport.Close() }()

			client := codex.NewClient(transport)
			thread := validThreadPayload("thread-invalid-enum")
			tt.mutate(thread)
			_ = transport.SetResponseData("thread/read", map[string]interface{}{
				"thread": thread,
			})

			_, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{
				ThreadID: "thread-invalid-enum",
			})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v; want substring %q", err, tt.wantErr)
			}
		})
	}
}

// TestThreadUnsubscribe tests the ThreadService.Unsubscribe method
func TestThreadUnsubscribe(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/unsubscribe", map[string]interface{}{
		"status": "unsubscribed",
	})

	params := codex.ThreadUnsubscribeParams{
		ThreadID: "thread-unsub",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Thread.Unsubscribe(ctx, params)
	if err != nil {
		t.Fatalf("Thread.Unsubscribe failed: %v", err)
	}

	if response.Status != codex.ThreadUnsubscribeStatusUnsubscribed {
		t.Errorf("expected status %q, got %q", codex.ThreadUnsubscribeStatusUnsubscribed, response.Status)
	}

	req := transport.GetSentRequest(0)
	if req.Method != "thread/unsubscribe" {
		t.Errorf("expected method 'thread/unsubscribe', got %q", req.Method)
	}
}

func TestThreadUnsubscribeRejectsInvalidStatus(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/unsubscribe", map[string]interface{}{
		"status": "bogus",
	})

	_, err := client.Thread.Unsubscribe(context.Background(), codex.ThreadUnsubscribeParams{
		ThreadID: "thread-unsub",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid status "bogus"`) {
		t.Fatalf("error = %v, want invalid status", err)
	}
}

// TestThreadCompactStart tests the ThreadService.CompactStart method
func TestThreadCompactStart(t *testing.T) {
	transport := NewMockTransport()
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	_ = transport.SetResponseData("thread/compact/start", map[string]interface{}{})

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
		t.Errorf("expected method 'thread/compact/start', got %q", req.Method)
	}
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

func TestThreadServiceRPCError(t *testing.T) {
	tests := []struct {
		name   string
		method string
		call   func(client *codex.Client) error
	}{
		{"Start", "thread/start", func(c *codex.Client) error {
			_, err := c.Thread.Start(context.Background(), codex.ThreadStartParams{})
			return err
		}},
		{"Read", "thread/read", func(c *codex.Client) error {
			_, err := c.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "t"})
			return err
		}},
		{"List", "thread/list", func(c *codex.Client) error {
			_, err := c.Thread.List(context.Background(), codex.ThreadListParams{})
			return err
		}},
		{"LoadedList", "thread/loaded/list", func(c *codex.Client) error {
			_, err := c.Thread.LoadedList(context.Background(), codex.ThreadLoadedListParams{})
			return err
		}},
		{"Resume", "thread/resume", func(c *codex.Client) error {
			_, err := c.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "t"})
			return err
		}},
		{"Fork", "thread/fork", func(c *codex.Client) error {
			_, err := c.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: "t"})
			return err
		}},
		{"Rollback", "thread/rollback", func(c *codex.Client) error {
			_, err := c.Thread.Rollback(context.Background(), codex.ThreadRollbackParams{ThreadID: "t", NumTurns: 1})
			return err
		}},
		{"SetName", "thread/name/set", func(c *codex.Client) error {
			_, err := c.Thread.SetName(context.Background(), codex.ThreadSetNameParams{ThreadID: "t", Name: "n"})
			return err
		}},
		{"Archive", "thread/archive", func(c *codex.Client) error {
			_, err := c.Thread.Archive(context.Background(), codex.ThreadArchiveParams{ThreadID: "t"})
			return err
		}},
		{"Unarchive", "thread/unarchive", func(c *codex.Client) error {
			_, err := c.Thread.Unarchive(context.Background(), codex.ThreadUnarchiveParams{ThreadID: "t"})
			return err
		}},
		{"Unsubscribe", "thread/unsubscribe", func(c *codex.Client) error {
			_, err := c.Thread.Unsubscribe(context.Background(), codex.ThreadUnsubscribeParams{ThreadID: "t"})
			return err
		}},
		{"CompactStart", "thread/compact/start", func(c *codex.Client) error {
			_, err := c.Thread.CompactStart(context.Background(), codex.ThreadCompactStartParams{ThreadID: "t"})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			mock.SetResponse(tt.method, codex.Response{
				JSONRPC: "2.0",
				Error: &codex.Error{
					Code:    codex.ErrCodeInternalError,
					Message: tt.method + " failed",
				},
			})

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var rpcErr *codex.RPCError
			if !errors.As(err, &rpcErr) {
				t.Fatalf("expected error to unwrap to *RPCError, got %T", err)
			}
			if rpcErr.RPCError().Code != codex.ErrCodeInternalError {
				t.Errorf("expected error code %d, got %d", codex.ErrCodeInternalError, rpcErr.RPCError().Code)
			}
		})
	}
}

// TestThreadReadMalformedResult verifies that a valid JSON-RPC response
// envelope with an unexpected result shape returns a wrapped unmarshal error.
func TestThreadReadMalformedResult(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Return a result that cannot be deserialized into ThreadReadResponse
	// (e.g. thread field is a string instead of an object).
	mock.SetResponse("thread/read", codex.Response{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{"thread":"not-an-object","approvalPolicy":123}`),
	})

	_, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "t"})
	if err == nil {
		t.Fatal("expected unmarshal error for malformed result, got nil")
	}

	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected error to mention unmarshal, got: %v", err)
	}
}

func validThreadLifecycleResponse(thread map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"approvalPolicy":    "untrusted",
		"approvalsReviewer": "user",
		"cwd":               "/test/dir",
		"model":             "gpt-4",
		"modelProvider":     "openai",
		"sandbox":           map[string]interface{}{"type": "dangerFullAccess"},
		"thread":            thread,
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
