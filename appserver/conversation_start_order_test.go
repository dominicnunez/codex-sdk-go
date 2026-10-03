package appserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
)

type completingStartTransport struct{ *MockTransport }

func (m *completingStartTransport) Send(ctx context.Context, request codex.Request) (codex.Response, error) {
	response, err := m.MockTransport.Send(ctx, request)
	if err == nil && request.Method == "turn/start" {
		m.InjectServerNotification(ctx, codex.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
	}
	return response, err
}

func startOrderProcess(t *testing.T) (*codex.Process, *codex.Client, *MockTransport) {
	t.Helper()
	mock := NewMockTransport()
	for method, response := range map[string]any{
		"initialize":   validInitializeResponseData("synthetic"),
		"thread/start": validProcessThreadStartResponse(validProcessThreadPayload("thread-1")),
		"turn/start":   map[string]any{"turn": map[string]any{"id": "turn-1", "status": "inProgress", "items": []any{}}},
	} {
		if err := mock.SetResponseData(method, response); err != nil {
			t.Fatal(err)
		}
	}
	client := codex.NewClient(&completingStartTransport{mock})
	t.Cleanup(func() { _ = client.Close() })
	return codex.NewProcessFromClient(client), client, mock
}

func startOrderTurn(ctx context.Context, conversation *codex.Conversation, streamed bool) (*codex.RunResult, error) {
	if !streamed {
		return conversation.Turn(ctx, codex.TurnOptions{Prompt: "synthetic"})
	}
	stream := conversation.TurnStreamed(ctx, codex.TurnOptions{Prompt: "synthetic"})
	var streamErr error
	for _, err := range stream.Events() {
		if err != nil {
			streamErr = err
		}
	}
	return stream.Result(), streamErr
}

func TestConversationStartPinsStateBeforeCallbacks(t *testing.T) {
	for _, boundary := range []string{"eviction", "metadata", "close", "reopen"} {
		for _, streamed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/streamed=%v", boundary, streamed), func(t *testing.T) {
				process, client, mock := startOrderProcess(t)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				var remove func()
				remove = client.AddThreadStateListener("thread-1", func(thread codex.Thread) {
					remove()
					if boundary == "metadata" {
						thread.Preview = "latest"
						client.CacheThreadState(thread)
					}
					if boundary == "close" || boundary == "reopen" {
						mock.InjectServerNotification(ctx, codex.Notification{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread-1"}`)})
						if boundary == "reopen" {
							thread.Preview = "reopened"
							client.CacheThreadState(thread)
						}
					}
					for i := range 65 {
						client.CacheThreadState(codex.Thread{ID: fmt.Sprintf("pressure-%d", i)})
					}
				}, nil)
				defer remove()
				conversation, err := process.StartConversation(ctx, codex.ConversationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				defer conversation.Close()
				if conversation.Thread().ID != "thread-1" {
					t.Fatal("start response snapshot was lost")
				}
				if (boundary == "close" || boundary == "reopen") && conversation.Thread().Preview != "" {
					t.Fatal("closed startup handle adopted replacement incarnation")
				}
				if boundary == "metadata" && conversation.Thread().Preview != "latest" {
					t.Fatal("startup seed overwrote newer delivery")
				}
				result, turnErr := startOrderTurn(ctx, conversation, streamed)
				if boundary == "eviction" || boundary == "metadata" {
					if turnErr != nil || result == nil || mock.MethodCallCount("turn/start") != 1 {
						t.Fatalf("cache pressure returned unusable conversation: err=%v result=%v calls=%d", turnErr, result, mock.MethodCallCount("turn/start"))
					}
				} else if turnErr == nil || !strings.Contains(turnErr.Error(), "conversation is closed") || result != nil || mock.MethodCallCount("turn/start") != 0 {
					t.Fatalf("start closure was forgotten: err=%v result=%v calls=%d", turnErr, result, mock.MethodCallCount("turn/start"))
				}
			})
		}
	}
}

func TestRunHistoricalResultSurvivesStartCachePressure(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		t.Run(fmt.Sprintf("streamed=%v", streamed), func(t *testing.T) {
			process, client, _ := startOrderProcess(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var remove func()
			remove = client.AddThreadStateListener("thread-1", func(codex.Thread) {
				remove()
				for i := range 65 {
					client.CacheThreadState(codex.Thread{ID: fmt.Sprintf("pressure-%d", i)})
				}
			}, nil)
			defer remove()
			var result *codex.RunResult
			var err error
			if streamed {
				stream := process.RunStreamed(ctx, codex.RunOptions{Prompt: "synthetic"})
				for _, streamErr := range stream.Events() {
					if streamErr != nil {
						err = streamErr
					}
				}
				result = stream.Result()
			} else {
				result, err = process.Run(ctx, codex.RunOptions{Prompt: "synthetic"})
			}
			if err != nil || result == nil || result.Thread.ID != "thread-1" || len(result.Thread.Turns) != 1 {
				t.Fatalf("historical result lost: result=%v err=%v", result, err)
			}
			if _, present := client.ThreadStateSnapshot("thread-1"); present {
				t.Fatal("completion recreated evicted snapshot")
			}
		})
	}
}
