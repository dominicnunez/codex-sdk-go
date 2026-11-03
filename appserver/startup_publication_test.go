package appserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type startupOrderTransport struct {
	capabilityProcessTransport
	handler protocol.NotificationHandler
	entered chan struct{}
	release chan struct{}
	pending int
}

func (m *startupOrderTransport) OnNotify(h protocol.NotificationHandler) { m.handler = h }
func (m *startupOrderTransport) Send(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	var result string
	switch req.Method {
	case "initialize":
		result = `{"codexHome":"/tmp","platformFamily":"unix","platformOs":"test","userAgent":"test"}`
	case "thread/start":
		result = `{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/tmp","model":"test","modelProvider":"openai","sandbox":{"type":"readOnly"},"thread":{"id":"thread-1","projectId":null,"sessionId":"session-1","cliVersion":"test","createdAt":1,"cwd":"/tmp","modelProvider":"openai","preview":"","source":"exec","status":{"type":"idle"},"turns":[],"updatedAt":1,"ephemeral":true}}`
	case "turn/start":
		close(m.entered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return protocol.Response{}, ctx.Err()
		}
		for range m.pending {
			m.notify("item/plan/delta", `{"threadId":"thread-1","turnId":"turn-1","itemId":"plan","delta":"x"}`)
		}
		result = `{"turn":{"id":"turn-1","status":"inProgress","items":[]}}`
	default:
		return protocol.Response{}, fmt.Errorf("unexpected request %s", req.Method)
	}
	return protocol.Response{ID: req.ID, Result: json.RawMessage(result)}, nil
}

func (m *startupOrderTransport) notify(method, params string) {
	m.handler(context.Background(), Notification{Method: method, Params: json.RawMessage(params)})
}

func startOrderStream(t *testing.T, route string, pending int) (*Stream, *startupOrderTransport, *StreamCollector, *Client, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tr := &startupOrderTransport{entered: make(chan struct{}), release: make(chan struct{}), pending: pending}
	client := NewClient(tr)
	t.Cleanup(func() { _ = client.Close() })
	process := NewProcessFromClient(client)
	collector := NewStreamCollector()
	var stream *Stream
	switch route {
	case "stream":
		stream = process.RunStreamed(ctx, RunOptions{Prompt: "order"})
	case "collector":
		stream = process.RunStreamedWithCollector(ctx, RunOptions{Prompt: "order"}, collector)
	default:
		conv, err := process.StartConversation(ctx, ConversationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conv.Close() })
		stream = conv.TurnStreamed(ctx, TurnOptions{Prompt: "order"})
	}
	waitStartupSignal(t, tr.entered)
	return stream, tr, collector, client, cancel
}

func waitStartupSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("startup fixture did not progress")
	}
}

type startupWaitLock struct {
	*sync.Mutex
	waits int
	first chan struct{}
	later chan struct{}
}

func (l *startupWaitLock) Unlock() {
	l.waits++ // Cond.Wait calls this while holding the queue's mutex.
	switch l.waits {
	case 1:
		close(l.first)
	case 2:
		close(l.later)
	}
	l.Mutex.Unlock()
}

func TestStartupReplayPrecedesLivePublication(t *testing.T) {
	for _, route := range []string{"stream", "collector", "conversation"} {
		t.Run(route, func(t *testing.T) {
			stream, tr, collector, _, cancel := startOrderStream(t, route, 100)
			defer func() { cancel(); waitStartupSignal(t, stream.done) }()
			seq := stream.Events()
			waits := &startupWaitLock{Mutex: &stream.queue.mu, first: make(chan struct{}), later: make(chan struct{})}
			stream.queue.notFull = sync.NewCond(waits)
			close(tr.release)
			waitStartupSignal(t, waits.first) // Replay is blocked on its 65th event.
			liveDone := make(chan struct{})
			go func() {
				defer close(liveDone)
				tr.notify("item/plan/delta", `{"threadId":"thread-1","turnId":"turn-1","itemId":"plan","delta":"L"}`)
				tr.notify("turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)
			}()
			// An independent live publisher must not enter the full event queue
			// while replay owns publication. The old handoff allows both to wait.
			select {
			case <-waits.later:
				t.Error("live publication overtook startup replay")
			case <-time.After(100 * time.Millisecond):
			}
			var deltas strings.Builder
			consumerDone := make(chan struct{})
			go func() {
				defer close(consumerDone)
				for event, err := range seq {
					if err != nil {
						t.Errorf("stream error: %v", err)
						return
					}
					if delta, ok := event.(*PlanDelta); ok {
						deltas.WriteString(delta.Delta)
					}
				}
			}()
			defer func() {
				cancel()
				waitStartupSignal(t, liveDone)
				waitStartupSignal(t, consumerDone)
			}()
			waitStartupSignal(t, consumerDone)
			if stream.Result() == nil {
				t.Fatal("missing successful result")
			}
			want := strings.Repeat("x", 100) + "L"
			if deltas.String() != want {
				t.Errorf("event order changed: L position=%d, want100", strings.Index(deltas.String(), "L"))
			}
			if route == "collector" {
				summary := collector.Summary()
				if summary.LatestPlanText == nil || *summary.LatestPlanText != want {
					t.Errorf("collector order differs from received order: %+v", summary.LatestPlanText)
				}
			}
		})
	}
}

func TestStartupCancellationReleasesLivePublisher(t *testing.T) {
	for _, route := range []string{"stream", "collector", "conversation"} {
		t.Run(route, func(t *testing.T) {
			stream, tr, _, _, cancel := startOrderStream(t, route, 100)
			defer func() { cancel(); waitStartupSignal(t, stream.done) }()
			seq := stream.Events()
			waiting := make(chan struct{})
			stream.queue.notFull = sync.NewCond(&publicationWaitLock{Mutex: &stream.queue.mu, waiting: waiting})
			close(tr.release)
			waitStartupSignal(t, waiting)
			liveEntered, liveDone := make(chan struct{}), make(chan struct{})
			go func() {
				defer close(liveDone)
				close(liveEntered)
				tr.notify("item/plan/delta", `{"threadId":"thread-1","turnId":"turn-1","itemId":"plan","delta":"L"}`)
			}()
			defer func() { cancel(); waitStartupSignal(t, liveDone) }()
			waitStartupSignal(t, liveEntered)
			select {
			case <-liveDone:
				t.Error("live publisher finished before startup replay was released")
			case <-time.After(100 * time.Millisecond):
			}
			cancel() // No consumer drains the queue to release either publisher.
			waitStartupSignal(t, stream.done)
			waitStartupSignal(t, liveDone)
			if stream.Result() != nil {
				t.Fatal("canceled turn returned a result without completion")
			}
			var terminal error
			for _, err := range seq {
				if err != nil {
					terminal = err
				}
			}
			if terminal != context.Canceled {
				t.Fatalf("terminal error = %v, want cancellation", terminal)
			}
		})
	}
}

func TestBlockingStartupReplayPrecedesLivePublication(t *testing.T) {
	for _, kind := range []string{"item", "completion"} {
		t.Run(kind, func(t *testing.T) { checkBlockingStartupPublication(t, kind) })
	}
}

func checkBlockingStartupPublication(t *testing.T, kind string) {
	t.Helper()
	var state blockingTurnState
	first := ItemCompletedNotification{TurnID: "turn-1", Item: ThreadItemWrapper{Value: &AgentMessageThreadItem{ID: "first"}}}
	if _, ready, err := state.queueItem(first); ready || err != nil {
		t.Fatalf("startup item not buffered: ready=%v err=%v", ready, err)
	}
	entered, release, replayDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	var order []string
	go func() {
		defer close(replayDone)
		if err := state.start("turn-1", func(items []ItemCompletedNotification, _ []turnCompletionCandidate) {
			close(entered)
			<-release
			for _, item := range items {
				order = append(order, item.Item.Value.(*AgentMessageThreadItem).ID)
			}
		}); err != nil {
			t.Errorf("start replay: %v", err)
		}
	}()
	waitStartupSignal(t, entered)
	liveDone := make(chan struct{})
	go func() {
		defer close(liveDone)
		if kind == "item" {
			item, ready, err := state.queueItem(ItemCompletedNotification{TurnID: "turn-1", Item: ThreadItemWrapper{Value: &AgentMessageThreadItem{ID: "live"}}})
			if !ready || err != nil {
				t.Errorf("live item not admitted: ready=%v err=%v", ready, err)
				return
			}
			state.publish(func() { order = append(order, item.Value.(*AgentMessageThreadItem).ID) })
			return
		}
		completion, ready, err := state.queueCompletion(turnCompletionCandidate{turnID: "turn-1", notification: TurnCompletedNotification{Turn: Turn{ID: "turn-1", Status: TurnStatusCompleted}}})
		if !ready || err != nil {
			t.Errorf("live completion not admitted: ready=%v err=%v", ready, err)
			return
		}
		state.publish(func() { order = append(order, string(completion.Turn.Status)) })
	}()
	defer func() { unblock(); waitStartupSignal(t, replayDone); waitStartupSignal(t, liveDone) }()
	select {
	case <-liveDone:
		t.Error("live publication finished while startup replay was paused")
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	waitStartupSignal(t, replayDone)
	waitStartupSignal(t, liveDone)
	want := "first,live"
	if kind == "completion" {
		want = "first,completed"
	}
	if got := strings.Join(order, ","); got != want {
		t.Errorf("publication order = %q", got)
	}
}

func TestStreamStartedEventsBelongToActiveTurn(t *testing.T) {
	for _, route := range []string{"stream", "collector", "conversation"} {
		t.Run(route, func(t *testing.T) {
			stream, tr, _, client, cancel := startOrderStream(t, route, 0)
			defer cancel()
			globalStarts := 0
			client.OnTurnStarted(func(TurnStartedNotification) { globalStarts++ })
			seq := stream.Events()
			ready, consumed := make(chan struct{}), make(chan struct{})
			var starts []string
			go func() {
				defer close(consumed)
				for event, err := range seq {
					if err != nil {
						t.Errorf("stream error: %v", err)
						return
					}
					switch event := event.(type) {
					case *TurnStarted:
						starts = append(starts, event.Turn.ID)
					case *PlanDelta:
						close(ready)
					}
				}
			}()
			defer func() { cancel(); waitStartupSignal(t, consumed) }()
			notifyStarts := func() {
				for _, owner := range [][2]string{{"thread-1", "turn-1"}, {"thread-1", "stale"}, {"other", "turn-1"}} {
					tr.notify("turn/started", fmt.Sprintf(`{"threadId":%q,"turn":{"id":%q,"status":"inProgress","items":[]}}`, owner[0], owner[1]))
				}
			}
			notifyStarts() // Runtime has not received the active turn ID yet.
			close(tr.release)
			tr.notify("item/plan/delta", `{"threadId":"thread-1","turnId":"turn-1","itemId":"plan","delta":"ready"}`)
			waitStartupSignal(t, ready) // A published matching delta proves readiness.
			notifyStarts()
			tr.notify("turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)
			waitStartupSignal(t, consumed)
			if stream.Result() == nil {
				t.Fatal("missing successful result")
			}
			if len(starts) != 2 || starts[0] != "turn-1" || starts[1] != "turn-1" {
				t.Errorf("single-operation start events = %v, want two active starts", starts)
			}
			if globalStarts != 6 {
				t.Errorf("client-wide start count = %d, want all6", globalStarts)
			}
		})
	}
}
