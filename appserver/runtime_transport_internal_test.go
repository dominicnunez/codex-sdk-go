package appserver

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type endedStartupTransport struct {
	*startupOrderTransport
	done       chan struct{}
	completion bool
}

func (m *endedStartupTransport) Done() <-chan struct{} { return m.done }
func (*endedStartupTransport) Err() error              { return io.EOF }
func (m *endedStartupTransport) Send(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	response, err := m.startupOrderTransport.Send(ctx, req)
	if req.Method == "turn/start" && err == nil {
		if m.completion {
			m.notify("turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)
		}
		// All accepted callbacks have returned. The runtime still owns their
		// startup replay, which must not be discarded because Done is closed.
		close(m.done)
	}
	return response, err
}

func TestTransportEndRetainsRuntimeStartupReplay(t *testing.T) {
	for _, completed := range []bool{false, true} {
		name := "without-completion"
		if completed {
			name = "with-completion"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			base := &startupOrderTransport{entered: make(chan struct{}), release: make(chan struct{}), pending: 100}
			tr := &endedStartupTransport{startupOrderTransport: base, done: make(chan struct{}), completion: completed}
			client := NewClient(tr)
			defer client.Close()
			stream := NewProcessFromClient(client).RunStreamed(ctx, RunOptions{Prompt: "replay"})
			defer func() { cancel(); waitStartupSignal(t, stream.done) }()
			seq := stream.Events()
			waitStartupSignal(t, base.entered)
			waiting := make(chan struct{})
			stream.queue.notFull = sync.NewCond(&publicationWaitLock{Mutex: &stream.queue.mu, waiting: waiting})
			close(base.release)
			waitStartupSignal(t, tr.Done())
			waitStartupSignal(t, waiting)
			select {
			case <-stream.done:
				t.Fatal("transport end bypassed runtime-owned replay")
			default:
			}
			var deltas strings.Builder
			var terminal error
			for event, err := range seq {
				if delta, ok := event.(*PlanDelta); ok {
					deltas.WriteString(delta.Delta)
				}
				if err != nil {
					terminal = err
				}
			}
			result := stream.Result()
			if deltas.String() != strings.Repeat("x", 100) {
				t.Errorf("replay lost accepted events: got%d", deltas.Len())
			}
			if completed {
				if terminal != nil || result == nil {
					t.Fatalf("accepted completion lost: result=%v error=%v", result, terminal)
				}
			} else if result != nil || !errors.Is(terminal, io.EOF) {
				t.Fatalf("missing completion did not retain EOF: result=%v error=%v", result, terminal)
			}
		})
	}
}
