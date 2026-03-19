package codex_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

func TestThreadRealtimeStartedNotification(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
	}{
		{
			name: "with sessionId",
			jsonData: `{
				"threadId": "thread_abc123",
				"sessionId": "session_xyz789",
				"version": "v2"
			}`,
		},
		{
			name: "without sessionId",
			jsonData: `{
				"threadId": "thread_abc123",
				"version": "v1"
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var notif codex.ThreadRealtimeStartedNotification
			if err := json.Unmarshal([]byte(tt.jsonData), &notif); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if notif.ThreadID != "thread_abc123" {
				t.Errorf("expected ThreadID thread_abc123, got %s", notif.ThreadID)
			}

			// Re-marshal and verify round-trip
			data, err := json.Marshal(notif)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			var notif2 codex.ThreadRealtimeStartedNotification
			if err := json.Unmarshal(data, &notif2); err != nil {
				t.Fatalf("failed to unmarshal round-trip: %v", err)
			}

			if notif.ThreadID != notif2.ThreadID {
				t.Error("ThreadID mismatch after round-trip")
			}
			if notif.Version != notif2.Version {
				t.Error("Version mismatch after round-trip")
			}
		})
	}

	// Test listener dispatch
	t.Run("listener dispatch", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		var received *codex.ThreadRealtimeStartedNotification
		client.OnThreadRealtimeStarted(func(notif codex.ThreadRealtimeStartedNotification) {
			received = &notif
		})

		// Inject server notification
		mock.InjectServerNotification(context.Background(), codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/realtime/started",
			Params: json.RawMessage(`{
				"threadId": "thread_test",
				"sessionId": "session_test",
				"version": "v2"
			}`),
		})

		if received == nil {
			t.Fatal("notification handler not called")
		}
		if received.ThreadID != "thread_test" {
			t.Errorf("expected ThreadID thread_test, got %s", received.ThreadID)
		}
		if received.SessionID == nil || *received.SessionID != "session_test" {
			t.Error("expected SessionID session_test")
		}
		if received.Version != codex.RealtimeConversationVersionV2 {
			t.Errorf("expected Version v2, got %q", received.Version)
		}
	})
}

func TestThreadRealtimeStartedMissingRequiredFieldReportsHandlerError(t *testing.T) {
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
	client.OnThreadRealtimeStarted(func(codex.ThreadRealtimeStartedNotification) {
		called = true
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/realtime/started",
		Params:  json.RawMessage(`{"threadId":"thread_test"}`),
	})

	if called {
		t.Fatal("handler should not be called for malformed payload")
	}
	if gotMethod != "thread/realtime/started" {
		t.Fatalf("handler error method = %q; want %q", gotMethod, "thread/realtime/started")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "missing required field") {
		t.Fatalf("handler error = %v; want missing required field failure", gotErr)
	}
}

func TestThreadRealtimeStartedRejectsInvalidVersion(t *testing.T) {
	var notif codex.ThreadRealtimeStartedNotification
	err := json.Unmarshal([]byte(`{"threadId":"thread_test","version":"v3"}`), &notif)
	if err == nil || !strings.Contains(err.Error(), "invalid thread.realtime.version") {
		t.Fatalf("json.Unmarshal error = %v; want invalid version failure", err)
	}
}

func TestThreadRealtimeStartedInvalidVersionReportsHandlerError(t *testing.T) {
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
	client.OnThreadRealtimeStarted(func(codex.ThreadRealtimeStartedNotification) {
		called = true
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/realtime/started",
		Params:  json.RawMessage(`{"threadId":"thread_test","version":"v3"}`),
	})

	if called {
		t.Fatal("handler should not be called for invalid version")
	}
	if gotMethod != "thread/realtime/started" {
		t.Fatalf("handler error method = %q; want %q", gotMethod, "thread/realtime/started")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "invalid thread.realtime.version") {
		t.Fatalf("handler error = %v; want invalid version failure", gotErr)
	}
}

func TestRealtimeNotificationTypesRejectMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		target  interface{}
	}{
		{
			name:    "closed requires threadId",
			payload: `{"reason":"timeout"}`,
			target:  &codex.ThreadRealtimeClosedNotification{},
		},
		{
			name:    "error requires message",
			payload: `{"threadId":"thread-1"}`,
			target:  &codex.ThreadRealtimeErrorNotification{},
		},
		{
			name:    "item added requires item",
			payload: `{"threadId":"thread-1"}`,
			target:  &codex.ThreadRealtimeItemAddedNotification{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := json.Unmarshal([]byte(tt.payload), tt.target)
			if err == nil || !strings.Contains(err.Error(), "missing required field") {
				t.Fatalf("json.Unmarshal error = %v; want missing required field failure", err)
			}
		})
	}
}

func TestThreadRealtimeClosedNotification(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
	}{
		{
			name: "with reason",
			jsonData: `{
				"threadId": "thread_abc123",
				"reason": "user_disconnect"
			}`,
		},
		{
			name: "without reason",
			jsonData: `{
				"threadId": "thread_abc123"
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var notif codex.ThreadRealtimeClosedNotification
			if err := json.Unmarshal([]byte(tt.jsonData), &notif); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if notif.ThreadID != "thread_abc123" {
				t.Errorf("expected ThreadID thread_abc123, got %s", notif.ThreadID)
			}

			// Re-marshal and verify round-trip
			data, err := json.Marshal(notif)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			var notif2 codex.ThreadRealtimeClosedNotification
			if err := json.Unmarshal(data, &notif2); err != nil {
				t.Fatalf("failed to unmarshal round-trip: %v", err)
			}

			if notif.ThreadID != notif2.ThreadID {
				t.Error("ThreadID mismatch after round-trip")
			}
		})
	}

	// Test listener dispatch
	t.Run("listener dispatch", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		var received *codex.ThreadRealtimeClosedNotification
		client.OnThreadRealtimeClosed(func(notif codex.ThreadRealtimeClosedNotification) {
			received = &notif
		})

		// Inject server notification
		mock.InjectServerNotification(context.Background(), codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/realtime/closed",
			Params: json.RawMessage(`{
				"threadId": "thread_test",
				"reason": "timeout"
			}`),
		})

		if received == nil {
			t.Fatal("notification handler not called")
		}
		if received.ThreadID != "thread_test" {
			t.Errorf("expected ThreadID thread_test, got %s", received.ThreadID)
		}
		if received.Reason == nil || *received.Reason != "timeout" {
			t.Error("expected Reason timeout")
		}
	})
}

func TestThreadRealtimeClosedMissingRequiredFieldReportsHandlerError(t *testing.T) {
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
	client.OnThreadRealtimeClosed(func(codex.ThreadRealtimeClosedNotification) {
		called = true
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/realtime/closed",
		Params:  json.RawMessage(`{"reason":"timeout"}`),
	})

	if called {
		t.Fatal("handler should not be called for malformed payload")
	}
	if gotMethod != "thread/realtime/closed" {
		t.Fatalf("handler error method = %q; want %q", gotMethod, "thread/realtime/closed")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "missing required field") {
		t.Fatalf("handler error = %v; want missing required field failure", gotErr)
	}
}

func TestThreadRealtimeErrorNotification(t *testing.T) {
	jsonData := `{
		"threadId": "thread_abc123",
		"message": "Audio processing failed: invalid sample rate"
	}`

	var notif codex.ThreadRealtimeErrorNotification
	if err := json.Unmarshal([]byte(jsonData), &notif); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if notif.ThreadID != "thread_abc123" {
		t.Errorf("expected ThreadID thread_abc123, got %s", notif.ThreadID)
	}
	if notif.Message != "Audio processing failed: invalid sample rate" {
		t.Errorf("unexpected Message: %s", notif.Message)
	}

	// Re-marshal and verify round-trip
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var notif2 codex.ThreadRealtimeErrorNotification
	if err := json.Unmarshal(data, &notif2); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if notif.ThreadID != notif2.ThreadID || notif.Message != notif2.Message {
		t.Error("mismatch after round-trip")
	}

	// Test listener dispatch
	t.Run("listener dispatch", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		var received *codex.ThreadRealtimeErrorNotification
		client.OnThreadRealtimeError(func(notif codex.ThreadRealtimeErrorNotification) {
			received = &notif
		})

		// Inject server notification
		mock.InjectServerNotification(context.Background(), codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/realtime/error",
			Params: json.RawMessage(`{
				"threadId": "thread_test",
				"message": "test error"
			}`),
		})

		if received == nil {
			t.Fatal("notification handler not called")
		}
		if received.ThreadID != "thread_test" {
			t.Errorf("expected ThreadID thread_test, got %s", received.ThreadID)
		}
		if received.Message != "test error" {
			t.Errorf("expected Message 'test error', got %s", received.Message)
		}
	})
}

func TestThreadRealtimeErrorMissingRequiredFieldReportsHandlerError(t *testing.T) {
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
	client.OnThreadRealtimeError(func(codex.ThreadRealtimeErrorNotification) {
		called = true
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/realtime/error",
		Params:  json.RawMessage(`{"threadId":"thread_test"}`),
	})

	if called {
		t.Fatal("handler should not be called for malformed payload")
	}
	if gotMethod != "thread/realtime/error" {
		t.Fatalf("handler error method = %q; want %q", gotMethod, "thread/realtime/error")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "missing required field") {
		t.Fatalf("handler error = %v; want missing required field failure", gotErr)
	}
}

func TestThreadRealtimeItemAddedNotification(t *testing.T) {
	jsonData := `{
		"threadId": "thread_abc123",
		"item": {
			"type": "function_call",
			"id": "item_123",
			"name": "get_weather"
		}
	}`

	var notif codex.ThreadRealtimeItemAddedNotification
	if err := json.Unmarshal([]byte(jsonData), &notif); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if notif.ThreadID != "thread_abc123" {
		t.Errorf("expected ThreadID thread_abc123, got %s", notif.ThreadID)
	}

	// Verify Item is non-nil json.RawMessage
	if notif.Item == nil {
		t.Fatal("Item should not be nil")
	}

	// Re-marshal and verify round-trip
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var notif2 codex.ThreadRealtimeItemAddedNotification
	if err := json.Unmarshal(data, &notif2); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if notif.ThreadID != notif2.ThreadID {
		t.Error("ThreadID mismatch after round-trip")
	}

	// Test listener dispatch
	t.Run("listener dispatch", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		var received *codex.ThreadRealtimeItemAddedNotification
		client.OnThreadRealtimeItemAdded(func(notif codex.ThreadRealtimeItemAddedNotification) {
			received = &notif
		})

		// Inject server notification
		mock.InjectServerNotification(context.Background(), codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/realtime/itemAdded",
			Params: json.RawMessage(`{
				"threadId": "thread_test",
				"item": {"type": "test"}
			}`),
		})

		if received == nil {
			t.Fatal("notification handler not called")
		}
		if received.ThreadID != "thread_test" {
			t.Errorf("expected ThreadID thread_test, got %s", received.ThreadID)
		}
		if received.Item == nil {
			t.Error("expected Item to be non-nil")
		}
	})
}

func TestThreadRealtimeItemAddedMissingRequiredFieldReportsHandlerError(t *testing.T) {
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
	client.OnThreadRealtimeItemAdded(func(codex.ThreadRealtimeItemAddedNotification) {
		called = true
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/realtime/itemAdded",
		Params:  json.RawMessage(`{"threadId":"thread_test"}`),
	})

	if called {
		t.Fatal("handler should not be called for malformed payload")
	}
	if gotMethod != "thread/realtime/itemAdded" {
		t.Fatalf("handler error method = %q; want %q", gotMethod, "thread/realtime/itemAdded")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "missing required field") {
		t.Fatalf("handler error = %v; want missing required field failure", gotErr)
	}
}

func TestThreadRealtimeOutputAudioDeltaNotification(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
	}{
		{
			name: "with samplesPerChannel",
			jsonData: `{
				"threadId": "thread_abc123",
				"audio": {
					"data": "AAABAAIAAQ",
					"itemId": "item-123",
					"numChannels": 1,
					"sampleRate": 16000,
					"samplesPerChannel": 1024
				}
			}`,
		},
		{
			name: "without samplesPerChannel",
			jsonData: `{
				"threadId": "thread_abc123",
				"audio": {
					"data": "AAABAAIAAQ",
					"numChannels": 2,
					"sampleRate": 48000
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var notif codex.ThreadRealtimeOutputAudioDeltaNotification
			if err := json.Unmarshal([]byte(tt.jsonData), &notif); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if notif.ThreadID != "thread_abc123" {
				t.Errorf("expected ThreadID thread_abc123, got %s", notif.ThreadID)
			}

			if notif.Audio.Data != "AAABAAIAAQ" {
				t.Errorf("unexpected audio data: %s", notif.Audio.Data)
			}
			if tt.name == "with samplesPerChannel" && (notif.Audio.ItemID == nil || *notif.Audio.ItemID != "item-123") {
				t.Fatalf("Audio.ItemID = %v, want item-123", notif.Audio.ItemID)
			}

			// Re-marshal and verify round-trip
			data, err := json.Marshal(notif)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			var notif2 codex.ThreadRealtimeOutputAudioDeltaNotification
			if err := json.Unmarshal(data, &notif2); err != nil {
				t.Fatalf("failed to unmarshal round-trip: %v", err)
			}

			if notif.ThreadID != notif2.ThreadID {
				t.Error("ThreadID mismatch after round-trip")
			}
			if notif.Audio.Data != notif2.Audio.Data {
				t.Error("Audio.Data mismatch after round-trip")
			}
			if notif.Audio.NumChannels != notif2.Audio.NumChannels {
				t.Error("Audio.NumChannels mismatch after round-trip")
			}
			if notif.Audio.SampleRate != notif2.Audio.SampleRate {
				t.Error("Audio.SampleRate mismatch after round-trip")
			}
			if tt.name == "with samplesPerChannel" && (notif2.Audio.ItemID == nil || *notif2.Audio.ItemID != "item-123") {
				t.Fatalf("round-trip Audio.ItemID = %v, want item-123", notif2.Audio.ItemID)
			}
		})
	}

	// Test listener dispatch
	t.Run("listener dispatch", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		var received *codex.ThreadRealtimeOutputAudioDeltaNotification
		client.OnThreadRealtimeOutputAudioDelta(func(notif codex.ThreadRealtimeOutputAudioDeltaNotification) {
			received = &notif
		})

		// Inject server notification
		mock.InjectServerNotification(context.Background(), codex.Notification{
			JSONRPC: "2.0",
			Method:  "thread/realtime/outputAudio/delta",
			Params: json.RawMessage(`{
				"threadId": "thread_test",
				"audio": {
					"data": "test_data",
					"numChannels": 1,
					"sampleRate": 24000
				}
			}`),
		})

		if received == nil {
			t.Fatal("notification handler not called")
		}
		if received.ThreadID != "thread_test" {
			t.Errorf("expected ThreadID thread_test, got %s", received.ThreadID)
		}
		if received.Audio.Data != "test_data" {
			t.Errorf("expected audio data 'test_data', got %s", received.Audio.Data)
		}
		if received.Audio.NumChannels != 1 {
			t.Errorf("expected NumChannels 1, got %d", received.Audio.NumChannels)
		}
		if received.Audio.SampleRate != 24000 {
			t.Errorf("expected SampleRate 24000, got %d", received.Audio.SampleRate)
		}
	})
}
