package appserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	c "github.com/dominicnunez/codex-sdk-go/appserver"
)

type pendingStartTransport struct {
	*MockTransport
	boundary    string
	before      bool
	observed    func()
	otherStarts int
}

func (m *pendingStartTransport) notify(ctx context.Context) {
	switch m.boundary {
	case "name":
		m.InjectServerNotification(ctx, c.Notification{Method: "thread/name/updated", Params: json.RawMessage(`{"threadId":"thread-1","threadName":"latest"}`)})
	case "settings":
		m.InjectServerNotification(ctx, c.Notification{Method: "thread/settings/updated", Params: json.RawMessage(`{"threadId":"thread-1","threadSettings":{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/tmp","model":"model","modelProvider":"openai","sandboxPolicy":{"type":"readOnly"},"collaborationMode":{"mode":"default","settings":{"model":"model"}},"disabledPluginIds":["latest"]}}`)})
	case "close":
		m.InjectServerNotification(ctx, c.Notification{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread-1"}`)})
	}
}
func (m *pendingStartTransport) Send(ctx context.Context, req c.Request) (c.Response, error) {
	resp, err := m.MockTransport.Send(ctx, req)
	if err == nil && req.Method == "thread/start" && m.before {
		for i := range m.otherStarts {
			thread, _ := json.Marshal(validProcessThreadPayload(fmt.Sprintf("other-%d", i)))
			m.InjectServerNotification(ctx, c.Notification{Method: "thread/started", Params: json.RawMessage(`{"thread":` + string(thread) + `}`)})
		}
		thread, _ := json.Marshal(validProcessThreadPayload("thread-1"))
		m.InjectServerNotification(ctx, c.Notification{Method: "thread/started", Params: json.RawMessage(`{"thread":` + string(thread) + `}`)})
		m.notify(ctx)
		if m.observed != nil {
			m.observed()
		}
	}
	if err == nil && req.Method == "turn/start" {
		m.InjectServerNotification(ctx, c.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
	}
	return resp, err
}
func TestConversationStartPreservesPendingNotifications(t *testing.T) {
	for _, boundary := range []string{"name", "settings", "close"} {
		for _, before := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/before=%v", boundary, before), func(t *testing.T) {
				mock := NewMockTransport()
				for method, body := range map[string]any{"initialize": validInitializeResponseData("synthetic"), "thread/start": validProcessThreadStartResponse(validProcessThreadPayload("thread-1")), "turn/start": map[string]any{"turn": map[string]any{"id": "turn-1", "status": "inProgress", "items": []any{}}}} {
					if err := mock.SetResponseData(method, body); err != nil {
						t.Fatal(err)
					}
				}
				transport := &pendingStartTransport{MockTransport: mock, boundary: boundary, before: before}
				client := c.NewClient(transport, c.WithHandlerErrorCallback(func(method string, err error) { t.Errorf("unexpected notification error %s: %v", method, err) }))
				transport.observed = func() {
					snapshot, present := client.ThreadStateSnapshot("thread-1")
					switch boundary {
					case "name":
						if !present || snapshot.Name == nil || *snapshot.Name != "latest" {
							t.Fatal("name precondition not admitted")
						}
					case "settings":
						if !present || !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"latest"}) {
							t.Fatal("settings precondition not admitted")
						}
					case "close":
						if present || client.ThreadStateGeneration("thread-1") != 0 {
							t.Fatal("close precondition not admitted")
						}
					}
				}
				t.Cleanup(func() { _ = client.Close() })
				process := c.NewProcessFromClient(client)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				conversation, err := process.StartConversation(ctx, c.ConversationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				defer conversation.Close()
				if !before {
					transport.notify(ctx)
				}
				thread := conversation.Thread()
				switch boundary {
				case "name":
					if thread.Name == nil || *thread.Name != "latest" {
						t.Errorf("newer name lost: %+v", thread.Name)
					}
				case "settings":
					if !reflect.DeepEqual(thread.DisabledPluginIDs, []string{"latest"}) {
						t.Errorf("newer settings lost: %#v", thread.DisabledPluginIDs)
					}
				case "close":
					if _, ok := client.ThreadStateSnapshot("thread-1"); ok {
						t.Error("closed cache reopened by start response")
					}
					result, turnErr := conversation.Turn(ctx, c.TurnOptions{Prompt: "local mock"})
					if turnErr == nil || !strings.Contains(turnErr.Error(), "conversation is closed") || result != nil || mock.MethodCallCount("turn/start") != 0 {
						t.Errorf("closure lost: err=%v result=%v turn requests=%d", turnErr, result, mock.MethodCallCount("turn/start"))
					}
					stream := conversation.TurnStreamed(ctx, c.TurnOptions{Prompt: "local mock"})
					var streamedErr error
					for _, err := range stream.Events() {
						if err != nil {
							streamedErr = err
						}
					}
					if stream.Result() != nil || streamedErr == nil || !strings.Contains(streamedErr.Error(), "conversation is closed") || mock.MethodCallCount("turn/start") != 0 {
						t.Error("streamed turn dispatched after admitted startup closure")
					}
				}
			})
		}
	}
}

func TestRuntimeStartObservationFailureDoesNotDispatchTurn(t *testing.T) {
	for _, route := range []string{"run", "stream", "collector", "conversation"} {
		t.Run(route, func(t *testing.T) {
			mock := NewMockTransport()
			for method, body := range map[string]any{"initialize": validInitializeResponseData("synthetic"), "thread/start": validProcessThreadStartResponse(validProcessThreadPayload("thread-1"))} {
				if err := mock.SetResponseData(method, body); err != nil {
					t.Fatal(err)
				}
			}
			tr := &pendingStartTransport{MockTransport: mock, before: true, boundary: "close", otherStarts: 65}
			client := c.NewClient(tr)
			defer client.Close()
			process := c.NewProcessFromClient(client)
			ctx := context.Background()
			var result *c.RunResult
			var err error
			switch route {
			case "run":
				result, err = process.Run(ctx, c.RunOptions{Prompt: "local mock"})
			case "conversation":
				var conversation *c.Conversation
				conversation, err = process.StartConversation(ctx, c.ConversationOptions{})
				if conversation != nil {
					_ = conversation.Close()
					t.Error("lost observation returned a conversation")
				}
			default:
				var stream *c.Stream
				if route == "collector" {
					stream = process.RunStreamedWithCollector(ctx, c.RunOptions{Prompt: "local mock"}, c.NewStreamCollector())
				} else {
					stream = process.RunStreamed(ctx, c.RunOptions{Prompt: "local mock"})
				}
				for _, failure := range stream.Events() {
					if failure != nil {
						err = failure
					}
				}
				result = stream.Result()
			}
			if !errors.Is(err, c.ErrThreadStartObservationLost) || result != nil || mock.MethodCallCount("turn/start") != 0 || mock.MethodCallCount("thread/start") != 1 {
				t.Fatalf("lost startup observation dispatched/retried: result=%v err=%v starts=%d turns=%d", result, err, mock.MethodCallCount("thread/start"), mock.MethodCallCount("turn/start"))
			}
		})
	}
}
