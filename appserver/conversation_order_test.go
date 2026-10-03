package appserver_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
)

func TestConversationCompletionPreservesNewerState(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		name := "blocking"
		if streamed {
			name = "streamed"
		}
		t.Run(name, func(t *testing.T) {
			proc, mock := mockProcess(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer conv.Close()
			thread := conv.Thread()
			thread.Preview = "outer"
			proc.Client.CacheThreadState(thread)
			var armed atomic.Bool
			armed.Store(true)
			// Register after Conversation to distinguish the independent local
			// completion writer from the earlier-listener delivery regression.
			remove := proc.Client.AddThreadStateListener(conv.ThreadID(), func(thread codex.Thread) {
				if len(thread.Turns) != 0 && armed.Swap(false) {
					thread.Preview = "inner"
					proc.Client.CacheThreadState(thread)
				}
			}, nil)
			defer remove()
			type outcome struct {
				result *codex.RunResult
				err    error
			}
			done := make(chan outcome, 1)
			go func() {
				if streamed {
					stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "synthetic"})
					var streamErr error
					for _, err := range stream.Events() {
						if err != nil {
							streamErr = err
						}
					}
					done <- outcome{stream.Result(), streamErr}
					return
				}
				result, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "synthetic"})
				done <- outcome{result, err}
			}()
			waitForMethodCallCount(t, mock, "turn/start", 1)
			mock.InjectServerNotification(ctx, codex.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
			got := <-done
			if got.err != nil || got.result == nil {
				t.Fatalf("completion: %+v", got)
			}
			cached, ok := proc.Client.ThreadStateSnapshot(conv.ThreadID())
			if !ok || cached.Preview != "inner" || conv.Thread().Preview != "inner" {
				t.Fatalf("cache=%q present=%v conversation=%q", cached.Preview, ok, conv.Thread().Preview)
			}
			if got.result.Thread.Preview != "outer" || len(got.result.Thread.Turns) != 1 {
				t.Fatalf("historical result changed: %+v", got.result.Thread)
			}
		})
	}
}

func TestConversationRejectsCommittedCloseBeforeItsCallback(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		name := "blocking"
		if streamed {
			name = "streamed"
		}
		t.Run(name, func(t *testing.T) {
			proc, mock := mockProcess(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var conv *codex.Conversation
			var turnErr error
			remove := proc.Client.AddThreadStateListener("thread-1", nil, func() {
				if streamed {
					stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "synthetic"})
					for _, err := range stream.Events() {
						if err != nil {
							turnErr = err
						}
					}
				} else {
					_, turnErr = conv.Turn(ctx, codex.TurnOptions{Prompt: "synthetic"})
				}
			})
			defer remove()
			var err error
			conv, err = proc.StartConversation(ctx, codex.ConversationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer conv.Close()
			mock.InjectServerNotification(ctx, codex.Notification{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread-1"}`)})
			if turnErr == nil || !strings.Contains(turnErr.Error(), "conversation is closed") || mock.MethodCallCount("turn/start") != 0 {
				t.Fatalf("committed close admission: err=%v turn/start=%d", turnErr, mock.MethodCallCount("turn/start"))
			}
		})
	}
}

func TestConversationCompletionDoesNotCrossClosure(t *testing.T) {
	for _, reopen := range []bool{false, true} {
		name := "closed"
		if reopen {
			name = "reopened"
		}
		t.Run(name, func(t *testing.T) {
			proc, mock := mockProcess(t)
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
				result, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "synthetic"})
				done <- outcome{result, err}
			}()
			waitForMethodCallCount(t, mock, "turn/start", 1)
			mock.InjectServerNotification(ctx, codex.Notification{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread-1"}`)})
			if reopen {
				thread := conv.Thread()
				thread.Preview = "new incarnation"
				proc.Client.CacheThreadState(thread)
			}
			mock.InjectServerNotification(ctx, codex.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
			got := <-done
			if got.err != nil || got.result == nil || len(got.result.Thread.Turns) != 1 {
				t.Fatalf("admitted operation lost historical result: %+v", got)
			}
			cached, present := proc.Client.ThreadStateSnapshot(conv.ThreadID())
			if present != reopen || (reopen && (cached.Preview != "new incarnation" || len(cached.Turns) != 0)) {
				t.Fatalf("old completion crossed closure: present=%v thread=%+v", present, cached)
			}
			if len(conv.Thread().Turns) != 0 {
				t.Fatal("closed Conversation changed after completion")
			}
		})
	}
}

func TestConversationInFlightCloseKeepsHistoricalResult(t *testing.T) {
	for _, test := range []struct {
		name     string
		streamed bool
		boundary string
	}{
		{"blocking-local", false, "local"}, {"streamed-local", true, "local"},
		{"streamed-closed", true, "closed"}, {"streamed-reopened", true, "reopened"},
	} {
		t.Run(test.name, func(t *testing.T) {
			proc, mock := mockProcess(t)
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
				if !test.streamed {
					result, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "synthetic"})
					done <- outcome{result, err}
					return
				}
				stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "synthetic"})
				var streamErr error
				for _, err := range stream.Events() {
					if err != nil {
						streamErr = err
					}
				}
				done <- outcome{stream.Result(), streamErr}
			}()
			waitForMethodCallCount(t, mock, "turn/start", 1)
			if test.boundary == "local" {
				if err := conv.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				mock.InjectServerNotification(ctx, codex.Notification{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread-1"}`)})
				if test.boundary == "reopened" {
					thread := conv.Thread()
					thread.Preview = "new incarnation"
					proc.Client.CacheThreadState(thread)
				}
			}
			mock.InjectServerNotification(ctx, codex.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
			got := <-done
			if got.err != nil || got.result == nil || len(got.result.Thread.Turns) != 1 {
				t.Fatalf("historical completion=%+v", got)
			}
			if len(conv.Thread().Turns) != 0 {
				t.Fatal("closed local snapshot changed")
			}
			cached, present := proc.Client.ThreadStateSnapshot(conv.ThreadID())
			switch test.boundary {
			case "closed":
				if present {
					t.Fatal("completion reopened closed cache")
				}
			case "reopened":
				if !present || cached.Preview != "new incarnation" || len(cached.Turns) != 0 {
					t.Fatal("completion altered new incarnation")
				}
			case "local":
				if !present || len(cached.Turns) != 1 {
					t.Fatal("local Close incorrectly stopped admitted cache completion")
				}
			}
		})
	}
}
