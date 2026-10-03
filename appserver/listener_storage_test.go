package appserver_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
)

// Called after lifecycle termination; registration and cleanup have stopped.
// Read-only reflection avoids adding a production API for retained ownership.
func assertListenerStorageReleased(t *testing.T, client *codex.Client) {
	t.Helper()
	registry := reflect.ValueOf(client).Elem().FieldByName("internalListeners")
	for _, method := range registry.MapKeys() {
		listeners := registry.MapIndex(method)
		if listeners.Len() == 0 {
			t.Errorf("empty registry retained for %s", method.String())
		}
		backing := listeners.Slice(0, listeners.Cap())
		for i := listeners.Len(); i < backing.Len(); i++ {
			if !backing.Index(i).FieldByName("handler").IsNil() {
				t.Errorf("removed listener retained for %s at slot %d", method.String(), i)
			}
		}
	}
}

func TestRunListenerStorageReleased(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		for _, outcome := range []string{"success", "cancel", "start-error"} {
			t.Run(outcome+map[bool]string{false: "/blocking", true: "/streamed"}[streamed], func(t *testing.T) {
				proc, mock := mockProcess(t)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if outcome == "start-error" {
					mock.SetResponse("turn/start", codex.Response{JSONRPC: "2.0", Error: &codex.Error{Code: -32600, Message: "test failure"}})
				}
				done := make(chan bool, 1)
				go func() {
					if streamed {
						stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "test"})
						failed := false
						for _, err := range stream.Events() {
							if err != nil {
								failed = true
							}
						}
						stream.Result()
						done <- failed
					} else {
						_, err := proc.Run(ctx, codex.RunOptions{Prompt: "test"})
						done <- err != nil
					}
				}()
				waitForMethodCallCount(t, mock, "turn/start", 1)
				switch outcome {
				case "success":
					mock.InjectServerNotification(ctx, codex.Notification{JSONRPC: "2.0", Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
				case "cancel":
					cancel()
				}
				select {
				case failed := <-done:
					if failed != (outcome != "success") {
						t.Fatalf("failed = %v for %s", failed, outcome)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("lifecycle did not finish")
				}
				assertListenerStorageReleased(t, proc.Client)
			})
		}
	}
}

func TestConversationCloseReleasesListenerStorage(t *testing.T) {
	proc, _ := mockProcess(t)
	first, err := proc.StartConversation(context.Background(), codex.ConversationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := proc.StartConversation(context.Background(), codex.ConversationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	registry := reflect.ValueOf(proc.Client).Elem().FieldByName("threadStateListeners")
	backing := registry.MapIndex(reflect.ValueOf("thread-1"))
	if backing.Len() != 2 {
		t.Fatalf("registrations = %d", backing.Len())
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if !backing.Index(1).FieldByName("onUpdate").IsNil() || !backing.Index(1).FieldByName("onClose").IsNil() {
		t.Fatal("closed conversation retained in backing tail")
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if registry.MapIndex(reflect.ValueOf("thread-1")).IsValid() {
		t.Fatal("empty conversation registry retained")
	}
}
