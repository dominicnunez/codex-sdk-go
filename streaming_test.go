package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestAgentMessageDelta tests the agent/messageDelta notification
func TestAgentMessageDelta(t *testing.T) {
	tests := []struct {
		name string
		json string
		want codex.AgentMessageDeltaNotification
	}{
		{
			name: "basic delta",
			json: `{
				"delta": "Hello ",
				"itemId": "item-123",
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			want: codex.AgentMessageDeltaNotification{
				Delta:    "Hello ",
				ItemID:   "item-123",
				ThreadID: "thread-456",
				TurnID:   "turn-789",
			},
		},
		{
			name: "multiline delta",
			json: `{
				"delta": "Line 1\nLine 2\nLine 3",
				"itemId": "item-abc",
				"threadId": "thread-def",
				"turnId": "turn-ghi"
			}`,
			want: codex.AgentMessageDeltaNotification{
				Delta:    "Line 1\nLine 2\nLine 3",
				ItemID:   "item-abc",
				ThreadID: "thread-def",
				TurnID:   "turn-ghi",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.AgentMessageDeltaNotification
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if got.Delta != tt.want.Delta || got.ItemID != tt.want.ItemID || got.ThreadID != tt.want.ThreadID || got.TurnID != tt.want.TurnID {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.AgentMessageDeltaNotification
			client.OnAgentMessageDelta(func(notif codex.AgentMessageDeltaNotification) {
				received = &notif
			})

			// Inject notification
			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "agent/messageDelta",
				Params:  json.RawMessage(tt.json),
			})

			if received == nil {
				t.Fatal("listener not called")
			}
			if received.Delta != tt.want.Delta {
				t.Errorf("received delta %q, want %q", received.Delta, tt.want.Delta)
			}
		})
	}
}

// TestFileChangeOutputDelta tests the turn/fileChangeOutputDelta notification
func TestFileChangeOutputDelta(t *testing.T) {
	tests := []struct {
		name string
		json string
		want codex.FileChangeOutputDeltaNotification
	}{
		{
			name: "diff delta",
			json: `{
				"delta": "+added line\n-removed line",
				"itemId": "item-123",
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			want: codex.FileChangeOutputDeltaNotification{
				Delta:    "+added line\n-removed line",
				ItemID:   "item-123",
				ThreadID: "thread-456",
				TurnID:   "turn-789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.FileChangeOutputDeltaNotification
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.FileChangeOutputDeltaNotification
			client.OnFileChangeOutputDelta(func(notif codex.FileChangeOutputDeltaNotification) {
				received = &notif
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "turn/fileChangeOutputDelta",
				Params:  json.RawMessage(tt.json),
			})

			if received == nil {
				t.Fatal("listener not called")
			}
			if received.Delta != tt.want.Delta {
				t.Errorf("received delta %q, want %q", received.Delta, tt.want.Delta)
			}
		})
	}
}

// TestPlanDelta tests the turn/planDelta notification
func TestPlanDelta(t *testing.T) {
	tests := []struct {
		name string
		json string
		want codex.PlanDeltaNotification
	}{
		{
			name: "plan text delta",
			json: `{
				"delta": "1. First step\n",
				"itemId": "item-plan-1",
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			want: codex.PlanDeltaNotification{
				Delta:    "1. First step\n",
				ItemID:   "item-plan-1",
				ThreadID: "thread-456",
				TurnID:   "turn-789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.PlanDeltaNotification
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.PlanDeltaNotification
			client.OnPlanDelta(func(notif codex.PlanDeltaNotification) {
				received = &notif
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "turn/planDelta",
				Params:  json.RawMessage(tt.json),
			})

			if received == nil {
				t.Fatal("listener not called")
			}
			if received.Delta != tt.want.Delta {
				t.Errorf("received delta %q, want %q", received.Delta, tt.want.Delta)
			}
		})
	}
}

// TestReasoningTextDelta tests the turn/reasoningTextDelta notification
func TestReasoningTextDelta(t *testing.T) {
	tests := []struct {
		name string
		json string
		want codex.ReasoningTextDeltaNotification
	}{
		{
			name: "reasoning content delta",
			json: `{
				"contentIndex": 0,
				"delta": "Analyzing the problem...",
				"itemId": "item-reasoning-1",
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			want: codex.ReasoningTextDeltaNotification{
				ContentIndex: 0,
				Delta:        "Analyzing the problem...",
				ItemID:       "item-reasoning-1",
				ThreadID:     "thread-456",
				TurnID:       "turn-789",
			},
		},
		{
			name: "second content item",
			json: `{
				"contentIndex": 1,
				"delta": "Considering alternatives",
				"itemId": "item-reasoning-1",
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			want: codex.ReasoningTextDeltaNotification{
				ContentIndex: 1,
				Delta:        "Considering alternatives",
				ItemID:       "item-reasoning-1",
				ThreadID:     "thread-456",
				TurnID:       "turn-789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.ReasoningTextDeltaNotification
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.ReasoningTextDeltaNotification
			client.OnReasoningTextDelta(func(notif codex.ReasoningTextDeltaNotification) {
				received = &notif
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "turn/reasoningTextDelta",
				Params:  json.RawMessage(tt.json),
			})

			if received == nil {
				t.Fatal("listener not called")
			}
			if received.Delta != tt.want.Delta {
				t.Errorf("received delta %q, want %q", received.Delta, tt.want.Delta)
			}
			if received.ContentIndex != tt.want.ContentIndex {
				t.Errorf("received contentIndex %d, want %d", received.ContentIndex, tt.want.ContentIndex)
			}
		})
	}
}

// TestReasoningSummaryTextDelta tests the turn/reasoningSummaryTextDelta notification
func TestReasoningSummaryTextDelta(t *testing.T) {
	tests := []struct {
		name string
		json string
		want codex.ReasoningSummaryTextDeltaNotification
	}{
		{
			name: "summary delta",
			json: `{
				"delta": "The solution involves ",
				"itemId": "item-reasoning-1",
				"summaryIndex": 0,
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			want: codex.ReasoningSummaryTextDeltaNotification{
				Delta:        "The solution involves ",
				ItemID:       "item-reasoning-1",
				SummaryIndex: 0,
				ThreadID:     "thread-456",
				TurnID:       "turn-789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.ReasoningSummaryTextDeltaNotification
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.ReasoningSummaryTextDeltaNotification
			client.OnReasoningSummaryTextDelta(func(notif codex.ReasoningSummaryTextDeltaNotification) {
				received = &notif
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "turn/reasoningSummaryTextDelta",
				Params:  json.RawMessage(tt.json),
			})

			if received == nil {
				t.Fatal("listener not called")
			}
			if received.Delta != tt.want.Delta {
				t.Errorf("received delta %q, want %q", received.Delta, tt.want.Delta)
			}
			if received.SummaryIndex != tt.want.SummaryIndex {
				t.Errorf("received summaryIndex %d, want %d", received.SummaryIndex, tt.want.SummaryIndex)
			}
		})
	}
}

// TestReasoningSummaryPartAdded tests the turn/reasoningSummaryPartAdded notification
func TestReasoningSummaryPartAdded(t *testing.T) {
	tests := []struct {
		name string
		json string
		want codex.ReasoningSummaryPartAddedNotification
	}{
		{
			name: "new summary part",
			json: `{
				"itemId": "item-reasoning-1",
				"summaryIndex": 0,
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			want: codex.ReasoningSummaryPartAddedNotification{
				ItemID:       "item-reasoning-1",
				SummaryIndex: 0,
				ThreadID:     "thread-456",
				TurnID:       "turn-789",
			},
		},
		{
			name: "second summary part",
			json: `{
				"itemId": "item-reasoning-1",
				"summaryIndex": 1,
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			want: codex.ReasoningSummaryPartAddedNotification{
				ItemID:       "item-reasoning-1",
				SummaryIndex: 1,
				ThreadID:     "thread-456",
				TurnID:       "turn-789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.ReasoningSummaryPartAddedNotification
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.ReasoningSummaryPartAddedNotification
			client.OnReasoningSummaryPartAdded(func(notif codex.ReasoningSummaryPartAddedNotification) {
				received = &notif
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "turn/reasoningSummaryPartAdded",
				Params:  json.RawMessage(tt.json),
			})

			if received == nil {
				t.Fatal("listener not called")
			}
			if received.SummaryIndex != tt.want.SummaryIndex {
				t.Errorf("received summaryIndex %d, want %d", received.SummaryIndex, tt.want.SummaryIndex)
			}
		})
	}
}

// TestItemStarted tests the turn/itemStarted notification with simplified ThreadItem
func TestItemStarted(t *testing.T) {
	// Test with a simple JSON object representing a ThreadItem
	// We use json.RawMessage to avoid defining the full ThreadItem union in tests
	tests := []struct {
		name string
		json string
		checkFn func(*testing.T, codex.ItemStartedNotification)
	}{
		{
			name: "user message item",
			json: `{
				"item": {"type": "userMessage", "id": "item-123", "content": []},
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			checkFn: func(t *testing.T, notif codex.ItemStartedNotification) {
				if notif.ThreadID != "thread-456" {
					t.Errorf("got threadId %q, want %q", notif.ThreadID, "thread-456")
				}
				if notif.TurnID != "turn-789" {
					t.Errorf("got turnId %q, want %q", notif.TurnID, "turn-789")
				}
				// Item is json.RawMessage, just verify it's not empty
				if len(notif.Item) == 0 {
					t.Error("item is empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.ItemStartedNotification
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			tt.checkFn(t, got)

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.ItemStartedNotification
			client.OnItemStarted(func(notif codex.ItemStartedNotification) {
				received = &notif
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "turn/itemStarted",
				Params:  json.RawMessage(tt.json),
			})

			if received == nil {
				t.Fatal("listener not called")
			}
			if received.ThreadID != "thread-456" {
				t.Errorf("listener received threadId %q, want %q", received.ThreadID, "thread-456")
			}
		})
	}
}

// TestItemCompleted tests the turn/itemCompleted notification
func TestItemCompleted(t *testing.T) {
	tests := []struct {
		name string
		json string
		checkFn func(*testing.T, codex.ItemCompletedNotification)
	}{
		{
			name: "completed agent message",
			json: `{
				"item": {"type": "agentMessage", "id": "item-123", "text": "Done!"},
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			checkFn: func(t *testing.T, notif codex.ItemCompletedNotification) {
				if notif.ThreadID != "thread-456" {
					t.Errorf("got threadId %q, want %q", notif.ThreadID, "thread-456")
				}
				if notif.TurnID != "turn-789" {
					t.Errorf("got turnId %q, want %q", notif.TurnID, "turn-789")
				}
				if len(notif.Item) == 0 {
					t.Error("item is empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.ItemCompletedNotification
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			tt.checkFn(t, got)

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.ItemCompletedNotification
			client.OnItemCompleted(func(notif codex.ItemCompletedNotification) {
				received = &notif
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "turn/itemCompleted",
				Params:  json.RawMessage(tt.json),
			})

			if received == nil {
				t.Fatal("listener not called")
			}
			if received.ThreadID != "thread-456" {
				t.Errorf("listener received threadId %q, want %q", received.ThreadID, "thread-456")
			}
		})
	}
}

// TestRawResponseItemCompleted tests the turn/rawResponseItemCompleted notification
func TestRawResponseItemCompleted(t *testing.T) {
	tests := []struct {
		name string
		json string
		checkFn func(*testing.T, codex.RawResponseItemCompletedNotification)
	}{
		{
			name: "message response item",
			json: `{
				"item": {"type": "message", "id": "resp-123", "content": []},
				"threadId": "thread-456",
				"turnId": "turn-789"
			}`,
			checkFn: func(t *testing.T, notif codex.RawResponseItemCompletedNotification) {
				if notif.ThreadID != "thread-456" {
					t.Errorf("got threadId %q, want %q", notif.ThreadID, "thread-456")
				}
				if notif.TurnID != "turn-789" {
					t.Errorf("got turnId %q, want %q", notif.TurnID, "turn-789")
				}
				if len(notif.Item) == 0 {
					t.Error("item is empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.RawResponseItemCompletedNotification
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			tt.checkFn(t, got)

			// Test listener dispatch
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var received *codex.RawResponseItemCompletedNotification
			client.OnRawResponseItemCompleted(func(notif codex.RawResponseItemCompletedNotification) {
				received = &notif
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "turn/rawResponseItemCompleted",
				Params:  json.RawMessage(tt.json),
			})

			if received == nil {
				t.Fatal("listener not called")
			}
			if received.ThreadID != "thread-456" {
				t.Errorf("listener received threadId %q, want %q", received.ThreadID, "thread-456")
			}
		})
	}
}
