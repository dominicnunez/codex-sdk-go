package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestThreadStateReentrantDeliveryDoesNotRegress(t *testing.T) {
	client := codex.NewClient(NewMockTransport())
	defer client.Close()
	var first, last []string
	removeFirst := client.AddThreadStateListener("thread", func(thread codex.Thread) {
		first = append(first, thread.Preview)
		if thread.Preview == "outer" {
			client.CacheThreadState(codex.Thread{ID: "thread", Preview: "inner"})
		}
	}, nil)
	defer removeFirst()
	removeLast := client.AddThreadStateListener("thread", func(thread codex.Thread) {
		last = append(last, thread.Preview)
	}, nil)
	defer removeLast()
	client.CacheThreadState(codex.Thread{ID: "thread", Preview: "outer"})
	if !reflect.DeepEqual(first, []string{"outer", "inner"}) {
		t.Fatalf("reentrant writer received %v", first)
	}
	if len(last) == 0 || last[len(last)-1] != "inner" {
		t.Fatalf("later listener regressed: %v", last)
	}
	latest, ok := client.ThreadStateSnapshot("thread")
	if !ok || latest.Preview != "inner" {
		t.Fatalf("cache=%+v present=%v", latest, ok)
	}
}

func TestThreadStateConcurrentDeliveryIsSerial(t *testing.T) {
	client := codex.NewClient(NewMockTransport())
	defer client.Close()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var active, overlapping atomic.Int32
	var mu sync.Mutex
	var received []string
	remove := client.AddThreadStateListener("thread", func(thread codex.Thread) {
		if active.Add(1) != 1 {
			overlapping.Add(1)
		}
		defer active.Add(-1)
		if thread.Preview == "outer" {
			close(entered)
			<-release
		}
		mu.Lock()
		received = append(received, thread.Preview)
		mu.Unlock()
	}, nil)
	defer remove()
	done := make(chan struct{})
	go func() {
		client.CacheThreadState(codex.Thread{ID: "thread", Preview: "outer"})
		close(done)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first callback did not start")
	}
	client.CacheThreadState(codex.Thread{ID: "thread", Preview: "inner"})
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("delivery did not finish")
	}
	mu.Lock()
	defer mu.Unlock()
	if overlapping.Load() != 0 || !reflect.DeepEqual(received, []string{"outer", "inner"}) {
		t.Fatalf("overlapping=%d delivery=%v", overlapping.Load(), received)
	}
}

func TestThreadStateReplayReentrantDeliveryIsSerial(t *testing.T) {
	client := codex.NewClient(NewMockTransport())
	defer client.Close()
	client.CacheThreadState(codex.Thread{ID: "thread", Preview: "replay"})
	var received []string
	remove := client.AddThreadStateListener("thread", func(thread codex.Thread) {
		if thread.Preview == "replay" {
			client.CacheThreadState(codex.Thread{ID: "thread", Preview: "latest"})
		}
		received = append(received, thread.Preview)
	}, nil)
	defer remove()
	if !reflect.DeepEqual(received, []string{"replay", "latest"}) {
		t.Fatalf("replay regressed: %v", received)
	}
}

func TestThreadStateReentrantClosePrecedesReopen(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	defer client.Close()
	removeFirst := client.AddThreadStateListener("thread", func(thread codex.Thread) {
		if thread.Preview == "outer" {
			mock.InjectServerNotification(context.Background(), codex.Notification{
				Method: "thread/closed", Params: json.RawMessage(`{"threadId":"thread"}`),
			})
			client.CacheThreadState(codex.Thread{ID: "thread", Preview: "reopened"})
		}
	}, nil)
	defer removeFirst()
	var received []string
	removeLast := client.AddThreadStateListener("thread", func(thread codex.Thread) {
		received = append(received, thread.Preview)
	}, func() { received = append(received, "closed") })
	defer removeLast()
	client.CacheThreadState(codex.Thread{ID: "thread", Preview: "outer"})
	if !reflect.DeepEqual(received, []string{"closed", "reopened"}) {
		t.Fatalf("closure/reopen delivery=%v", received)
	}
}

func TestThreadStateCompletionPreservesMetadataAndOwnership(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
		t.Errorf("unexpected handler failure %s: %v", method, err)
	}))
	defer client.Close()
	client.CacheThreadState(codex.Thread{ID: "thread", Name: codex.Ptr("old"), DisabledPluginIDs: []string{"old"}})
	generation := client.ThreadStateGeneration("thread")
	mock.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/name/updated", Params: json.RawMessage(`{"threadId":"thread","threadName":"latest"}`)})
	mock.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/settings/updated", Params: json.RawMessage(`{"threadId":"thread","threadSettings":{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/tmp","model":"model","modelProvider":"openai","sandboxPolicy":{"type":"readOnly"},"collaborationMode":{"mode":"default","settings":{"model":"model"}},"disabledPluginIds":["latest"]}}`)})
	turn := codex.Turn{ID: "turn", Items: []codex.ThreadItemWrapper{{Value: &codex.AgentMessageThreadItem{ID: "item", Text: "original"}}}}
	completed, ok := client.CompleteThreadTurn("thread", generation, turn)
	if !ok || completed.Name == nil || *completed.Name != "latest" || !reflect.DeepEqual(completed.DisabledPluginIDs, []string{"latest"}) || len(completed.Turns) != 1 {
		t.Fatalf("completion did not merge latest metadata: %+v present=%v", completed, ok)
	}
	*completed.Name = "changed"
	completed.DisabledPluginIDs[0] = "changed"
	completed.Turns[0].Items[0].Value.(*codex.AgentMessageThreadItem).Text = "changed result"
	turn.Items[0].Value.(*codex.AgentMessageThreadItem).Text = "changed input"
	cached, _ := client.ThreadStateSnapshot("thread")
	if *cached.Name != "latest" || cached.DisabledPluginIDs[0] != "latest" || cached.Turns[0].Items[0].Value.(*codex.AgentMessageThreadItem).Text != "original" {
		t.Fatal("completion input/result aliases committed state")
	}
}

func TestThreadStateOldCompletionCannotSurviveEviction(t *testing.T) {
	client := codex.NewClient(NewMockTransport())
	defer client.Close()
	client.CacheThreadState(codex.Thread{ID: "thread"})
	generation := client.ThreadStateGeneration("thread")
	for i := range 65 {
		client.CacheThreadState(codex.Thread{ID: string(rune('A' + i))})
	}
	if client.ThreadStateGeneration("thread") != 0 {
		t.Fatal("expected original entry eviction")
	}
	client.CacheThreadState(codex.Thread{ID: "thread", Preview: "recreated"})
	if _, ok := client.CompleteThreadTurn("thread", generation, codex.Turn{ID: "old turn"}); ok {
		t.Fatal("old completion matched recreated entry")
	}
	cached, _ := client.ThreadStateSnapshot("thread")
	if len(cached.Turns) != 0 || cached.Preview != "recreated" {
		t.Fatal("old completion changed recreated entry")
	}
}

func TestThreadStateReporterReentrancyDoesNotRegress(t *testing.T) {
	var client *codex.Client
	var contexts []string
	client = codex.NewClient(NewMockTransport(), codex.WithHandlerErrorCallback(func(method string, _ error) {
		contexts = append(contexts, method)
		client.CacheThreadState(codex.Thread{ID: "thread", Preview: "inner"})
	}))
	defer client.Close()
	removeFirst := client.AddThreadStateListener("thread", func(thread codex.Thread) {
		if thread.Preview == "outer" {
			panic("synthetic listener failure")
		}
	}, nil)
	defer removeFirst()
	var received []string
	removeLast := client.AddThreadStateListener("thread", func(thread codex.Thread) { received = append(received, thread.Preview) }, nil)
	defer removeLast()
	client.CacheThreadState(codex.Thread{ID: "thread", Preview: "outer"})
	if len(received) == 0 || received[len(received)-1] != "inner" || !reflect.DeepEqual(contexts, []string{"CacheThreadState"}) {
		t.Fatalf("reporter delivery=%v contexts=%v", received, contexts)
	}
}

func TestThreadStateConcurrentReplayDoesNotRegress(t *testing.T) {
	client := codex.NewClient(NewMockTransport())
	defer client.Close()
	client.CacheThreadState(codex.Thread{ID: "thread", Preview: "replay"})
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan func(), 1)
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var mu sync.Mutex
	var received []string
	go func() {
		done <- client.AddThreadStateListener("thread", func(thread codex.Thread) {
			if thread.Preview == "replay" {
				close(entered)
				<-release
			}
			mu.Lock()
			received = append(received, thread.Preview)
			mu.Unlock()
		}, nil)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("replay did not start")
	}
	client.CacheThreadState(codex.Thread{ID: "thread", Preview: "latest"})
	unblock()
	select {
	case remove := <-done:
		defer remove()
	case <-time.After(5 * time.Second):
		t.Fatal("replay did not drain")
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(received, []string{"replay", "latest"}) {
		t.Fatalf("concurrent replay regressed: %v", received)
	}
}
