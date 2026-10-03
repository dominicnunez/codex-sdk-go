package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestObservedThreadStartFailureDoesNotPin(t *testing.T) {
	for _, failure := range []string{"transport", "provider", "malformed", "empty-id", "params", "canceled"} {
		t.Run(failure, func(t *testing.T) {
			mock := NewMockTransport()
			if err := mock.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread"))); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(mock)
			defer client.Close()
			ctx := context.Background()
			params := codex.ThreadStartParams{}
			switch failure {
			case "transport":
				mock.SetSendError(errors.New("synthetic transport failure"))
			case "provider":
				mock.SetResponse("thread/start", codex.Response{Error: &codex.Error{Code: -32603, Message: "synthetic provider failure"}})
			case "malformed":
				if err := mock.SetResponseData("thread/start", map[string]any{"thread": map[string]any{"id": "thread"}}); err != nil {
					t.Fatal(err)
				}
			case "empty-id":
				if err := mock.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload(""))); err != nil {
					t.Fatal(err)
				}
			case "params":
				params.Cwd = codex.Ptr("relative")
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			calls := 0
			response, generation, remove, err := client.Thread.StartWithStateListener(ctx, params, func(codex.Thread) { calls++ }, func() { calls++ })
			if err == nil || response.Thread.ID != "" || generation != 0 || remove == nil || calls != 0 {
				t.Fatalf("failed start admitted state: err=%v response=%+v generation=%d calls=%d", err, response, generation, calls)
			}
			if _, present := client.ThreadStateSnapshot("thread"); present {
				t.Fatal("failed response published snapshot")
			}
			client.CacheThreadState(codex.Thread{ID: "thread"})
			for i := range 65 {
				client.CacheThreadState(codex.Thread{ID: fmt.Sprintf("pressure-%d", i)})
			}
			if _, present := client.ThreadStateSnapshot("thread"); present || calls != 0 {
				t.Fatal("failed start leaked callback or pin")
			}
			remove()
			mock.SetSendError(nil)
			if err := mock.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread"))); err != nil {
				t.Fatal(err)
			}
			_, recovered, unsubscribe, recoveryErr := client.Thread.StartWithStateListener(context.Background(), codex.ThreadStartParams{}, func(codex.Thread) { calls++ }, nil)
			defer unsubscribe()
			if recoveryErr != nil || recovered == 0 || calls != 1 {
				t.Fatalf("recovery failed: err=%v generation=%d calls=%d", recoveryErr, recovered, calls)
			}
		})
	}
}

func TestObservedThreadStartKeepsOriginalIncarnation(t *testing.T) {
	mock := NewMockTransport()
	if err := mock.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread"))); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(mock)
	defer client.Close()
	var removeFirst func()
	removeFirst = client.AddThreadStateListener("thread", func(thread codex.Thread) {
		removeFirst()
		mock.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread"}`)})
		thread.Preview = "replacement"
		client.CacheThreadState(thread)
	}, nil)
	defer removeFirst()
	response, generation, remove, err := client.Thread.StartWithStateListener(context.Background(), codex.ThreadStartParams{}, func(codex.Thread) {}, func() {})
	defer remove()
	if err != nil || response.Thread.Preview != "" || generation == 0 || generation == client.ThreadStateGeneration("thread") {
		t.Fatalf("startup bound replacement: generation=%d current=%d err=%v", generation, client.ThreadStateGeneration("thread"), err)
	}
	if _, ok := client.CompleteThreadTurn("thread", generation, codex.Turn{ID: "old turn"}); ok {
		t.Fatal("old start generation published into replacement")
	}
	cached, _ := client.ThreadStateSnapshot("thread")
	if cached.Preview != "replacement" || len(cached.Turns) != 0 {
		t.Fatal("old completion altered replacement")
	}
	remove()
	remove()
}

func TestObservedThreadStartPanicReturnsCleanup(t *testing.T) {
	mock := NewMockTransport()
	if err := mock.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread"))); err != nil {
		t.Fatal(err)
	}
	var contexts []string
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, _ error) { contexts = append(contexts, method); panic("synthetic reporter failure") }))
	defer client.Close()
	response, generation, remove, err := client.Thread.StartWithStateListener(context.Background(), codex.ThreadStartParams{}, func(codex.Thread) { panic("synthetic observer failure") }, nil)
	if err != nil || response.Thread.ID != "thread" || generation == 0 || remove == nil || !reflect.DeepEqual(contexts, []string{"thread/start"}) {
		t.Fatalf("panic lost success/cleanup: response=%+v generation=%d err=%v contexts=%v", response, generation, err, contexts)
	}
	remove()
	remove()
	for i := range 65 {
		client.CacheThreadState(codex.Thread{ID: fmt.Sprintf("pressure-%d", i)})
	}
	if _, present := client.ThreadStateSnapshot("thread"); present {
		t.Fatal("returned unsubscribe retained pin")
	}
}

func TestObservedThreadStartOwnershipAndNilCallbacks(t *testing.T) {
	for _, observed := range []bool{false, true} {
		t.Run(fmt.Sprintf("observed=%v", observed), func(t *testing.T) {
			mock := NewMockTransport()
			thread := validProcessThreadPayload("thread")
			thread["name"] = "original"
			fixture := validProcessThreadStartResponse(thread)
			fixture["disabledPluginIds"] = []string{"plugin"}
			if err := mock.SetResponseData("thread/start", fixture); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(mock)
			defer client.Close()
			var update func(codex.Thread)
			calls := 0
			if observed {
				update = func(snapshot codex.Thread) {
					calls++
					*snapshot.Name = "callback"
					snapshot.DisabledPluginIDs[0] = "callback"
				}
			}
			response, generation, remove, err := client.Thread.StartWithStateListener(context.Background(), codex.ThreadStartParams{}, update, nil)
			defer remove()
			if err != nil || generation == 0 || *response.Thread.Name != "original" || response.Thread.DisabledPluginIDs[0] != "plugin" {
				t.Fatalf("initial callback changed response: response=%+v err=%v", response, err)
			}
			if (observed && calls != 1) || (!observed && calls != 0) {
				t.Fatalf("initial publication calls=%d", calls)
			}
			*response.Thread.Name = "result"
			response.Thread.DisabledPluginIDs[0] = "result"
			cached, _ := client.ThreadStateSnapshot("thread")
			if *cached.Name != "original" || cached.DisabledPluginIDs[0] != "plugin" {
				t.Fatal("observer/result aliases cache")
			}
			for i := range 65 {
				client.CacheThreadState(codex.Thread{ID: fmt.Sprintf("pressure-%d", i)})
			}
			_, present := client.ThreadStateSnapshot("thread")
			if present != observed {
				t.Fatalf("unexpected subscription pin: observed=%v present=%v", observed, present)
			}
		})
	}
}
