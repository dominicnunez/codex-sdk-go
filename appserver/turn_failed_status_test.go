package appserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	c "github.com/dominicnunez/codex-sdk-go/appserver"
)

func TestRuntimeFailedTurnWithoutDetails(t *testing.T) {
	for _, route := range []string{"run", "stream", "collector", "conversation", "conversation-stream"} {
		for _, mode := range []string{"completed", "interrupted", "failed-absent", "failed-null", "failed-details", "failed-empty-details"} {
			t.Run(route+"/"+mode, func(t *testing.T) {
				process, mock := mockProcess(t)
				defer process.Client.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				var conv *c.Conversation
				if route == "conversation" || route == "conversation-stream" {
					var e error
					conv, e = process.StartConversation(ctx, c.ConversationOptions{})
					if e != nil {
						t.Fatal(e)
					}
					defer conv.Close()
				}
				type outcome struct {
					result   *c.RunResult
					err      error
					terminal *c.TurnCompleted
				}
				collector := c.NewStreamCollector()
				done := make(chan outcome, 1)
				go func() {
					if route == "run" {
						r, e := process.Run(ctx, c.RunOptions{Prompt: "local fixture"})
						done <- outcome{result: r, err: e}
						return
					}
					if route == "conversation" {
						r, e := conv.Turn(ctx, c.TurnOptions{Prompt: "local fixture"})
						done <- outcome{result: r, err: e}
						return
					}
					var stream *c.Stream
					switch route {
					case "stream":
						stream = process.RunStreamed(ctx, c.RunOptions{Prompt: "local fixture"})
					case "collector":
						stream = process.RunStreamedWithCollector(ctx, c.RunOptions{Prompt: "local fixture"}, collector)
					default:
						stream = conv.TurnStreamed(ctx, c.TurnOptions{Prompt: "local fixture"})
					}
					var err error
					var terminal *c.TurnCompleted
					for event, e := range stream.Events() {
						if completed, ok := event.(*c.TurnCompleted); ok {
							if err != nil {
								t.Error("terminal event followed the error")
							}
							terminal = completed
						}
						if e != nil {
							err = e
						}
					}
					done <- outcome{result: stream.Result(), err: err, terminal: terminal}
				}()
				waitForMethodCallCount(t, mock, "turn/start", 1)
				// A failed completion from another turn must not terminate this one.
				mock.InjectServerNotification(ctx, c.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"foreign","status":"failed","items":[]}}`)})
				mock.InjectServerNotification(ctx, c.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"foreign","turn":{"id":"turn-1","status":"failed","items":[]}}`)})
				mock.InjectServerNotification(ctx, c.Notification{Method: "item/completed", Params: json.RawMessage(`{"completedAtMs":1,"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"partial","text":"partial output"}}`)})
				status := "failed"
				extra := ""
				if mode == "completed" {
					status = "completed"
				}
				if mode == "interrupted" {
					status = "interrupted"
				}
				if mode == "failed-null" {
					extra = `,"error":null`
				}
				if mode == "failed-details" {
					extra = `,"error":{"message":"synthetic-failure"}`
				}
				if mode == "failed-empty-details" {
					extra = `,"error":{"message":""}`
				}
				mock.InjectServerNotification(ctx, c.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"` + status + `","items":[]` + extra + `}}`)})
				select {
				case got := <-done:
					if !strings.HasPrefix(mode, "failed") {
						if got.err != nil || got.result == nil || got.result.Turn.ID != "turn-1" || string(got.result.Turn.Status) != status {
							t.Fatalf("valid completion changed: result=%+v error=%v", got.result, got.err)
						}
					} else if got.err == nil || got.result != nil {
						observed := "no result"
						if got.result != nil {
							observed = string(got.result.Turn.Status)
						}
						t.Fatalf("failed turn reported no failure: status=%s result-present=%v error=%v", observed, got.result != nil, got.err)
					}
					var details *c.TurnError
					withDetails := mode == "failed-details" || mode == "failed-empty-details"
					if route == "collector" {
						assertFailedTurnCollector(t, collector.Summary(), got.err, withDetails)
					}
					if errors.As(got.err, &details) != withDetails {
						t.Fatalf("error detail ownership changed: %v", got.err)
					}
					if withDetails && (details == nil || got.err.Error() != "turn error: "+details.Message || len(details.Raw) == 0) {
						t.Fatal("detailed error wrapping changed")
					}
					if route == "stream" || route == "collector" || route == "conversation-stream" {
						if got.terminal == nil || got.terminal.Turn.ID != "turn-1" || string(got.terminal.Turn.Status) != status || (got.terminal.Turn.Error != nil) != withDetails {
							t.Fatal("terminal event lost or fabricated error details")
						}
					}
					thread, present := process.Client.ThreadStateSnapshot("thread-1")
					if conv != nil {
						thread = conv.Thread()
					}
					if !present || len(thread.Turns) != 1 || thread.Turns[0].ID != "turn-1" || string(thread.Turns[0].Status) != status || len(thread.Turns[0].Items) != 1 || (thread.Turns[0].Error != nil) != withDetails {
						t.Fatalf("terminal history bookkeeping changed: %+v", thread)
					}
				case <-ctx.Done():
					t.Fatal("runtime did not finish")
				}
			})
		}
	}
}

func assertFailedTurnCollector(t *testing.T, summary c.StreamSummary, failure error, withDetails bool) {
	t.Helper()
	counts := map[string]int{}
	for _, normalized := range summary.NormalizedErrors {
		counts[normalized.Kind]++
		if normalized.Kind == "stream_error" && (failure == nil || normalized.Message != failure.Error()) {
			t.Fatal("collector lost the runtime failure")
		}
	}
	wantStream, wantTurn := 0, 0
	if failure != nil {
		wantStream = 1
	}
	if withDetails {
		wantTurn = 1
	}
	if counts["stream_error"] != wantStream || counts["turn_error"] != wantTurn {
		t.Fatalf("collector failure counts = %v, want stream=%d turn=%d", counts, wantStream, wantTurn)
	}
}
