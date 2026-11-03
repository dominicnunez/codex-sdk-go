package appserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	sdk "github.com/dominicnunez/codex-sdk-go/appserver"
)

func TestRawListenerRunIsolation(t *testing.T) {
	for _, route := range []string{"run", "stream", "collector", "conversation", "conversation-stream"} {
		for _, mutate := range []bool{false, true} {
			mode := "unchanged"
			if mutate {
				mode = "mutating"
			}
			t.Run(route+"/"+mode, func(t *testing.T) {
				process, mock := mockProcess(t)
				defer process.Client.Close()
				remove := process.Client.AddNotificationListener("item/completed", func(_ context.Context, n sdk.Notification) {
					if !mutate {
						return
					}
					offset := bytes.Index(n.Params, []byte(`"original"`))
					if offset < 0 {
						t.Error("fixture text missing")
						return
					}
					copy(n.Params[offset+1:offset+9], []byte("modified"))
				})
				defer remove()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				var conversation *sdk.Conversation
				if route == "conversation" || route == "conversation-stream" {
					var err error
					conversation, err = process.StartConversation(ctx, sdk.ConversationOptions{})
					if err != nil {
						t.Fatal(err)
					}
					defer conversation.Close()
				}
				collector := sdk.NewStreamCollector()
				type outcome struct {
					result *sdk.RunResult
					err    error
				}
				done := make(chan outcome, 1)
				go func() {
					if route == "run" {
						r, err := process.Run(ctx, sdk.RunOptions{Prompt: "payload ownership"})
						done <- outcome{r, err}
						return
					}
					if route == "conversation" {
						r, err := conversation.Turn(ctx, sdk.TurnOptions{Prompt: "payload ownership"})
						done <- outcome{r, err}
						return
					}
					var stream *sdk.Stream
					switch route {
					case "stream":
						stream = process.RunStreamed(ctx, sdk.RunOptions{Prompt: "payload ownership"})
					case "collector":
						stream = process.RunStreamedWithCollector(ctx, sdk.RunOptions{Prompt: "payload ownership"}, collector)
					default:
						stream = conversation.TurnStreamed(ctx, sdk.TurnOptions{Prompt: "payload ownership"})
					}
					var resultError error
					for event, err := range stream.Events() {
						if err != nil {
							resultError = err
						}
						if completed, ok := event.(*sdk.ItemCompleted); ok {
							switch item := completed.Item.Value.(type) {
							case *sdk.PlanThreadItem:
								if item.Text != "original" {
									t.Error("raw listener changed emitted plan")
								}
							case *sdk.AgentMessageThreadItem:
								if item.Text != "original" {
									t.Error("raw listener changed emitted message")
								}
							}
						}
					}
					done <- outcome{stream.Result(), resultError}
				}()
				waitForMethodCallCount(t, mock, "turn/start", 1)
				mock.InjectServerNotification(ctx, sdk.Notification{Method: "item/completed", Params: json.RawMessage(`{"completedAtMs":1,"threadId":"thread-1","turnId":"turn-1","item":{"type":"plan","id":"plan","text":"original"}}`)})
				mock.InjectServerNotification(ctx, sdk.Notification{Method: "item/completed", Params: json.RawMessage(`{"completedAtMs":2,"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"message","text":"original"}}`)})
				mock.InjectServerNotification(ctx, sdk.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
				select {
				case out := <-done:
					if out.err != nil || out.result == nil {
						t.Fatalf("run failed: %v", out.err)
					}
					if out.result.Response != "original" || len(out.result.Items) != 2 || out.result.Items[0].Value.(*sdk.PlanThreadItem).Text != "original" {
						t.Fatal("raw listener changed run result")
					}
					if route == "collector" {
						summary := collector.Summary()
						if summary.LatestPlanText == nil || *summary.LatestPlanText != "original" {
							t.Fatal("raw listener changed collector state")
						}
					}
				case <-ctx.Done():
					t.Fatal("run did not finish")
				}
			})
		}
	}
}
