package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestThreadStateUpdateListenerIsolation(t *testing.T) {
	for _, method := range []string{"CacheThreadState", "thread/started", "thread/name/updated", "thread/project/updated", "thread/status/changed", "thread/settings/updated"} {
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			var contexts []string
			failure := errors.New("state listener failed")
			client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(context string, err error) {
				contexts = append(contexts, context)
				if !errors.Is(err, failure) {
					t.Errorf("panic identity changed: %v", err)
				}
				panic("reporter failed too")
			}))
			defer client.Close()
			client.CacheThreadState(codex.Thread{ID: "thread", DisabledPluginIDs: []string{"original"}})
			first := client.AddThreadStateListener("thread", func(thread codex.Thread) {
				thread.DisabledPluginIDs[0] = "listener mutation"
				panic(failure)
			}, nil)
			defer first()
			second := client.AddThreadStateListener("thread", func(codex.Thread) { panic(failure) }, nil)
			defer second()
			var received codex.Thread
			calls := 0
			last := client.AddThreadStateListener("thread", func(thread codex.Thread) { received = thread; calls++ }, nil)
			defer last()
			contexts, calls = nil, 0
			var payload any
			switch method {
			case "CacheThreadState":
				client.CacheThreadState(codex.Thread{ID: "thread", DisabledPluginIDs: []string{"updated"}})
			case "thread/started":
				payload = map[string]any{"thread": validThreadPayload("thread")}
			case "thread/name/updated":
				payload = map[string]any{"threadId": "thread", "threadName": "updated"}
			case "thread/project/updated":
				payload = map[string]any{"threadId": "thread", "projectId": "updated"}
			case "thread/status/changed":
				payload = map[string]any{"threadId": "thread", "status": map[string]any{"type": "notLoaded"}}
			case "thread/settings/updated":
				payload = map[string]any{"threadId": "thread", "threadSettings": map[string]any{
					"approvalPolicy": "never", "approvalsReviewer": "user", "cwd": "/tmp", "model": "model", "modelProvider": "openai",
					"sandboxPolicy":     map[string]any{"type": "readOnly"},
					"collaborationMode": map[string]any{"mode": "default", "settings": map[string]any{"model": "model"}}, "disabledPluginIds": []string{"updated"},
				}}
			}
			if payload != nil {
				data, err := json.Marshal(payload)
				if err != nil {
					t.Fatal(err)
				}
				mock.InjectServerNotification(context.Background(), codex.Notification{Method: method, Params: data})
			}
			cached, ok := client.ThreadStateSnapshot("thread")
			if calls != 1 || !ok || !reflect.DeepEqual(received, cached) || !reflect.DeepEqual(contexts, []string{method, method}) {
				t.Fatalf("delivery=%d contexts=%v received=%+v cached=%+v", calls, contexts, received, cached)
			}
			received.DisabledPluginIDs[0] = "recipient mutation"
			fresh, _ := client.ThreadStateSnapshot("thread")
			if fresh.DisabledPluginIDs[0] == "recipient mutation" {
				t.Fatal("healthy listener aliases cache")
			}
		})
	}
}

func TestThreadStateInitialReplayReturnsUnsubscribe(t *testing.T) {
	for _, closed := range []bool{false, true} {
		mock := NewMockTransport()
		contexts := []string{}
		client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(context string, _ error) { contexts = append(contexts, context) }))
		defer client.Close()
		client.CacheThreadState(codex.Thread{ID: "thread"})
		if closed {
			mock.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread"}`)})
		}
		calls := 0
		fail := func() { calls++; panic("initial replay failed") }
		unsubscribe := client.AddThreadStateListener("thread", func(codex.Thread) { fail() }, fail)
		if calls != 1 || !reflect.DeepEqual(contexts, []string{"AddThreadStateListener"}) {
			t.Fatalf("initial calls=%d contexts=%v", calls, contexts)
		}
		unsubscribe()
		unsubscribe()
		client.CacheThreadState(codex.Thread{ID: "thread"})
		mock.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread"}`)})
		if calls != 1 {
			t.Fatal("unsubscribe after initial replay failure did not remove registration")
		}
	}
}

func TestThreadStateCloseReentrantListenerIsolation(t *testing.T) {
	mock := NewMockTransport()
	var contexts []string
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, _ error) { contexts = append(contexts, method) }))
	defer client.Close()
	client.CacheThreadState(codex.Thread{ID: "thread"})
	var removeLater func()
	first := client.AddThreadStateListener("thread", nil, func() {
		if _, exists := client.ThreadStateSnapshot("thread"); exists {
			t.Error("closure was not committed before listener")
		}
		removeLater()
		panic("closure listener failed")
	})
	defer first()
	calls := 0
	removeLater = client.AddThreadStateListener("thread", nil, func() { calls++ })
	for range 2 {
		mock.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread"}`)})
	}
	if calls != 1 || !reflect.DeepEqual(contexts, []string{"thread/closed"}) {
		t.Fatalf("admitted closure delivery=%d contexts=%v", calls, contexts)
	}
}

func TestThreadStateServiceListenerFailures(t *testing.T) {
	for _, method := range []string{"thread/start", "thread/read", "thread/list", "thread/resume", "thread/fork", "thread/rollback", "thread/metadata/update", "thread/unarchive", "thread/revert"} {
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			var contexts []string
			client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(context string, _ error) { contexts = append(contexts, context) }))
			defer client.Close()
			unsub := client.AddThreadStateListener("thread", func(codex.Thread) { panic("service state listener") }, nil)
			defer unsub()
			calls := 0
			healthy := client.AddThreadStateListener("thread", func(codex.Thread) { calls++ }, nil)
			defer healthy()
			fixture := validThreadLifecycleResponse(validThreadPayload("thread"))
			if method == "thread/list" {
				fixture = map[string]any{"data": []any{validThreadPayload("thread"), validThreadPayload("later")}}
			}
			if err := mock.SetResponseData(method, fixture); err != nil {
				t.Fatal(err)
			}
			var err error
			ctx := context.Background()
			switch method {
			case "thread/start":
				_, err = client.Thread.Start(ctx, codex.ThreadStartParams{})
			case "thread/read":
				_, err = client.Thread.Read(ctx, codex.ThreadReadParams{ThreadID: "thread"})
			case "thread/list":
				_, err = client.Thread.List(ctx, codex.ThreadListParams{})
			case "thread/resume":
				_, err = client.Thread.Resume(ctx, codex.ThreadResumeParams{ThreadID: "thread"})
			case "thread/fork":
				_, err = client.Thread.Fork(ctx, codex.ThreadForkParams{ThreadID: "thread"})
			case "thread/rollback":
				_, err = client.Thread.Rollback(ctx, codex.ThreadRollbackParams{ThreadID: "thread", NumTurns: 1})
			case "thread/metadata/update":
				_, err = client.Thread.MetadataUpdate(ctx, codex.ThreadMetadataUpdateParams{ThreadID: "thread"})
			case "thread/unarchive":
				_, err = client.Thread.Unarchive(ctx, codex.ThreadUnarchiveParams{ThreadID: "thread"})
			case "thread/revert":
				_, err = client.Thread.Revert(ctx, codex.ThreadRevertParams{ThreadID: "thread", BeforeTurnID: "turn"})
			}
			if err != nil || calls != 1 || !reflect.DeepEqual(contexts, []string{method}) {
				t.Fatalf("service=%v delivery=%d contexts=%v", err, calls, contexts)
			}
			if _, ok := client.ThreadStateSnapshot("thread"); !ok {
				t.Fatal("successful service state was not cached")
			}
			if method == "thread/list" {
				if _, ok := client.ThreadStateSnapshot("later"); !ok {
					t.Fatal("first listener failure skipped later returned thread")
				}
			}
		})
	}
}

func TestThreadStateErrorReporterCanUnsubscribe(t *testing.T) {
	mock := NewMockTransport()
	var client *codex.Client
	var removeLater func()
	client = codex.NewClient(mock, codex.WithHandlerErrorCallback(func(_ string, _ error) {
		if _, ok := client.ThreadStateSnapshot("thread"); !ok {
			t.Error("update not committed before reporting")
		}
		removeLater()
	}))
	defer client.Close()
	removeFirst := client.AddThreadStateListener("thread", func(codex.Thread) { panic("update listener") }, nil)
	defer removeFirst()
	calls := 0
	removeLater = client.AddThreadStateListener("thread", func(codex.Thread) { calls++ }, nil)
	client.CacheThreadState(codex.Thread{ID: "thread"})
	client.CacheThreadState(codex.Thread{ID: "thread"})
	if calls != 1 {
		t.Fatalf("selected callback delivery=%d, want 1", calls)
	}
}
