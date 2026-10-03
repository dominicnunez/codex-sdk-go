package appserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
)

func TestNumericDiagnosticRunCompletion(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	large := "1" + strings.Repeat("0", 1<<18)
	for attempt, duration := range []string{large, "1"} {
		type outcome struct {
			result *codex.RunResult
			err    error
		}
		finished := make(chan outcome, 1)
		go func() {
			result, err := proc.Run(ctx, codex.RunOptions{Prompt: "synthetic completion"})
			finished <- outcome{result, err}
		}()
		waitForMethodCallCount(t, mock, "turn/start", attempt+1)
		mock.InjectServerNotification(ctx, codex.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[],"durationMs":` + duration + `}}`)})
		result := <-finished
		if attempt == 0 {
			var typed *codex.TurnError
			if !errors.As(result.err, &typed) || result.result != nil || len(result.err.Error()) > 4096 || len(typed.Message) > 4096 || !strings.Contains(typed.Message, "bytes omitted") {
				t.Fatal("failed completion retained oversized fallback diagnostics or published success")
			}
		} else if result.err != nil || result.result == nil {
			t.Fatal("valid run did not recover after numeric rejection")
		}
	}
}

func TestNumericDiagnosticStreamAndCollector(t *testing.T) {
	for _, failedCompletion := range []bool{false, true} {
		name := "notification"
		if failedCompletion {
			name = "completion"
		}
		t.Run(name, func(t *testing.T) {
			proc, mock := mockProcess(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			collector := codex.NewStreamCollector()
			stream := proc.RunStreamedWithCollector(ctx, codex.RunOptions{Prompt: "synthetic numeric rejection"}, collector)
			waitForRunStreamedReady(t, mock)
			large := "1" + strings.Repeat("0", 1<<18)
			duration := "1"
			if failedCompletion {
				duration = large
			} else {
				usage := `{"cachedInputTokens":0,"inputTokens":` + large + `,"outputTokens":0,"reasoningOutputTokens":0,"totalTokens":0}`
				mock.InjectServerNotification(ctx, codex.Notification{Method: "thread/tokenUsage/updated", Params: json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","tokenUsage":{"last":` + usage + `,"total":` + usage + `}}`)})
			}
			mock.InjectServerNotification(ctx, codex.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[],"durationMs":` + duration + `}}`)})
			var streamErr error
			for _, err := range stream.Events() {
				if err != nil {
					streamErr = err
					break
				}
			}
			result := stream.Result()
			if failedCompletion {
				var typed *codex.TurnError
				if !errors.As(streamErr, &typed) || result != nil || len(typed.Message) > 4096 || !strings.Contains(typed.Message, "bytes omitted") {
					t.Fatal("stream fallback lost bounded failed completion")
				}
			} else if streamErr != nil || result == nil {
				t.Fatal("malformed usage prevented a valid completion")
			}
			summary := collector.Summary()
			// Malformed convenience notifications are reported through the client
			// hook, not converted into collector events. Failed completions do feed
			// the fallback error into the collector.
			if (failedCompletion && len(summary.NormalizedErrors) == 0) || (!failedCompletion && len(summary.NormalizedErrors) != 0) || summary.LatestTokenUsage != nil {
				t.Fatal("collector lost rejection or published invalid usage")
			}
			for _, diagnostic := range summary.NormalizedErrors {
				if len(diagnostic.Message) > 4096 || !strings.Contains(diagnostic.Message, "bytes omitted") {
					t.Fatal("collector retained an oversized diagnostic copy")
				}
			}
		})
	}
}
