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

func TestConversationClosesAfterEarlierStateListenerPanics(t *testing.T) {
	proc, mock := mockProcess(t)
	defer proc.Close()
	unsubscribe := proc.Client.AddThreadStateListener("thread-1", nil, func() {
		panic("application close listener")
	})
	defer unsubscribe()
	conv, err := proc.StartConversation(context.Background(), codex.ConversationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer conv.Close()
	mock.InjectServerNotification(context.Background(), codex.Notification{
		Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread-1"}`),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = conv.Turn(ctx, codex.TurnOptions{Prompt: "next turn"})
	if err == nil || !strings.Contains(err.Error(), "conversation is closed") || mock.MethodCallCount("turn/start") != 0 {
		t.Fatalf("missed closure: err=%v turn/start calls=%d", err, mock.MethodCallCount("turn/start"))
	}
	stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "next streamed turn"})
	streamErrors := 0
	for _, err := range stream.Events() {
		if err == nil || !strings.Contains(err.Error(), "conversation is closed") {
			t.Fatalf("streamed closure: %v", err)
		}
		streamErrors++
	}
	if streamErrors != 1 || stream.Result() != nil || mock.MethodCallCount("turn/start") != 0 {
		t.Fatal("closed conversation admitted streamed turn")
	}
}

func TestConversationCompletionSurvivesStateListenerPanic(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		t.Run(fmt.Sprint("streamed=", streamed), func(t *testing.T) {
			proc, mock := mockProcess(t)
			defer proc.Close()
			unsubscribe := proc.Client.AddThreadStateListener("thread-1", func(codex.Thread) { panic("application state listener") }, nil)
			defer unsubscribe()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer conv.Close()
			type outcome struct {
				result *codex.RunResult
				err    error
			}
			done := make(chan outcome, 1)
			go func() {
				if streamed {
					stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "finish"})
					for _, err := range stream.Events() {
						if err != nil {
							done <- outcome{err: err}
							return
						}
					}
					done <- outcome{result: stream.Result()}
				} else {
					result, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "finish"})
					done <- outcome{result: result, err: err}
				}
			}()
			waitForMethodCallCount(t, mock, "turn/start", 1)
			mock.InjectServerNotification(ctx, codex.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
			completed := <-done
			if completed.err != nil || completed.result == nil {
				t.Fatalf("completion=%+v", completed)
			}
			if len(completed.result.Thread.Turns) != 1 || len(conv.Thread().Turns) != 1 {
				t.Fatal("turn completion did not reach result and Conversation")
			}
		})
	}
}
