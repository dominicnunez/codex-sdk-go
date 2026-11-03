package appserver

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type completionPublicationTransport struct {
	capabilityProcessTransport
	handler protocol.NotificationHandler
}

func (m *completionPublicationTransport) OnNotify(handler protocol.NotificationHandler) {
	m.handler = handler
}

func (*completionPublicationTransport) Send(_ context.Context, req protocol.Request) (protocol.Response, error) {
	return protocol.Response{ID: req.ID, Result: json.RawMessage(`{"turn":{"id":"turn-1","status":"inProgress","items":[]}}`)}, nil
}

// Cond.Wait unlocks this same queue mutex only after deciding to wait for
// capacity. Observing that unlock proves completion publication is blocked,
// without relying on sleeps or adding a production test hook.
type publicationWaitLock struct {
	*sync.Mutex
	waiting chan struct{}
	once    sync.Once
}

func (l *publicationWaitLock) Unlock() {
	l.once.Do(func() { close(l.waiting) })
	l.Mutex.Unlock()
}

func TestSuccessfulCompletionSurvivesCancellationDuringPublication(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr := &completionPublicationTransport{}
	client := NewClient(tr)
	defer client.Close()
	g, stream := newActiveStream(1)
	seq := stream.Events()
	streamSendEvent(g, &TextDelta{Delta: "queued"})
	waiting := make(chan struct{})
	g.notFull = sync.NewCond(&publicationWaitLock{Mutex: &g.mu, waiting: waiting})
	started := make(chan struct{})
	go func() {
		defer g.closeOnce()
		defer close(stream.done)
		executeStreamedTurn(ctx, turnLifecycleParams{
			client: client, threadID: "thread-1", thread: Thread{ID: "thread-1"},
			turnParams: TurnStartParams{ThreadID: "thread-1", Input: []UserInput{}},
			onStart:    func() { close(started) },
		}, g, stream)
	}()
	defer func() {
		g.detachConsumer()
		cancel()
		select {
		case <-stream.done:
		case <-time.After(time.Second):
			t.Error("lifecycle did not exit during cleanup")
		}
	}()
	select {
	case <-started:
	case <-stream.done:
		seq(func(_ Event, err error) bool { t.Errorf("turn ended before start: %v", err); return true })
		t.Fatal("turn ended before start")
	case <-time.After(time.Second):
		t.Fatal("turn did not start")
	}
	tr.handler(context.Background(), Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
	select {
	case <-waiting:
	case <-time.After(time.Second):
		t.Fatal("selected completion did not wait for event capacity")
	}
	cancel()
	select {
	case <-stream.done:
	case <-time.After(time.Second):
		t.Fatal("selected completion stayed blocked after cancellation")
	}
	result := stream.Result()
	if result == nil || result.Turn.ID != "turn-1" || result.Turn.Status != TurnStatusCompleted {
		t.Fatalf("selected successful result lost: %+v", result)
	}
	for _, err := range seq {
		if err != nil {
			t.Fatalf("successful completion gained terminal error: %v", err)
		}
	}
}
