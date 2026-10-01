package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestStdioOrdersNotificationsWithinScope(t *testing.T) {
	for _, tc := range []struct {
		name  string
		first Notification
		rest  []Notification
	}{
		{"thread", Notification{Method: protocol.NotifyAgentMessageDelta, Params: json.RawMessage(`{"threadId":"a","turnId":"turn","itemId":"item","delta":"A"}`)}, []Notification{
			{Method: protocol.NotifyThreadNameUpdated, Params: json.RawMessage(`{"threadId":"a","threadName":"new"}`)},
			{Method: protocol.NotifyItemCompleted, Params: json.RawMessage(`{"threadId":"a","turnId":"turn","item":{"type":"agentMessage","id":"item","text":"AB"}}`)},
			{Method: protocol.NotifyTurnCompleted, Params: json.RawMessage(`{"threadId":"a","turn":{"id":"turn","status":"completed","items":[]}}`)},
			{Method: protocol.NotifyThreadClosed, Params: json.RawMessage(`{"threadId":"a"}`)},
		}},
		{"process", Notification{Method: protocol.NotifyProcessOutputDelta, Params: json.RawMessage(`{"processHandle":"a","deltaBase64":"QQ==","stream":"stdout","capReached":false}`)}, []Notification{
			{Method: protocol.NotifyProcessOutputDelta, Params: json.RawMessage(`{"processHandle":"a","deltaBase64":"Qg==","stream":"stdout","capReached":false}`)},
			{Method: protocol.NotifyProcessExited, Params: json.RawMessage(`{"processHandle":"a","exitCode":0,"stdout":"","stderr":"","stdoutCapReached":false,"stderrCapReached":false}`)},
		}},
		{"process ignores extra thread identity", Notification{Method: protocol.NotifyProcessOutputDelta, Params: json.RawMessage(`{"processHandle":"a","threadId":99,"deltaBase64":"QQ==","stream":"stdout","capReached":false}`)}, []Notification{
			{Method: protocol.NotifyProcessExited, Params: json.RawMessage(`{"processHandle":"a","threadId":"other","exitCode":0,"stdout":"","stderr":"","stdoutCapReached":false,"stderrCapReached":false}`)},
		}},
		{"command", Notification{Method: protocol.NotifyCommandExecOutputDelta, Params: json.RawMessage(`{"processId":"a","threadId":99,"deltaBase64":"QQ==","stream":"stdout","capReached":false}`)}, []Notification{
			{Method: protocol.NotifyCommandExecOutputDelta, Params: json.RawMessage(`{"processId":"a","threadId":"other","deltaBase64":"Qg==","stream":"stderr","capReached":false}`)},
		}},
		{"thread start owns nested identity", Notification{Method: protocol.NotifyThreadStarted, Params: json.RawMessage(`{"thread":{"id":"a"},"threadId":"other"}`)}, []Notification{
			{Method: protocol.NotifyError, Params: json.RawMessage(`{"threadId":"a","error":{"message":"failed"}}`)},
			{Method: protocol.NotifyThreadClosed, Params: json.RawMessage(`{"threadId":"a"}`)},
		}},
		{"search session", Notification{Method: protocol.NotifyFuzzyFileSearchSessionUpdated, Params: json.RawMessage(`{"sessionId":"a","threadId":99}`)}, []Notification{
			{Method: protocol.NotifyFuzzyFileSearchSessionCompleted, Params: json.RawMessage(`{"sessionId":"a"}`)},
		}},
		{"account", Notification{Method: protocol.NotifyAccountUpdated, Params: json.RawMessage(`{"threadId":"other"}`)}, []Notification{
			{Method: protocol.NotifyGatewayOAuthChanged, Params: json.RawMessage(`{}`)},
		}},
		{"case-sensitive thread identity", Notification{Method: protocol.NotifyThreadNameUpdated, Params: json.RawMessage(`{"threadId":"a","ThreadID":"other","threadName":"old"}`)}, []Notification{
			{Method: protocol.NotifyThreadNameUpdated, Params: json.RawMessage(`{"threadId":"a","threadName":"new"}`)},
		}},
		{"case-sensitive nested owner", Notification{Method: protocol.NotifyThreadStarted, Params: json.RawMessage(`{"thread":{"id":"a"},"Thread":{"id":"other"}}`)}, []Notification{
			{Method: protocol.NotifyThreadClosed, Params: json.RawMessage(`{"threadId":"a"}`)},
		}},
		{"case-sensitive process handle", Notification{Method: protocol.NotifyProcessOutputDelta, Params: json.RawMessage(`{"processHandle":"a","ProcessHandle":"other","processId":"other","deltaBase64":"QQ==","stream":"stdout","capReached":false}`)}, []Notification{
			{Method: protocol.NotifyProcessExited, Params: json.RawMessage(`{"processHandle":"a","exitCode":0,"stdout":"","stderr":"","stdoutCapReached":false,"stderrCapReached":false}`)},
		}},
		{"case-sensitive command identity", Notification{Method: protocol.NotifyCommandExecOutputDelta, Params: json.RawMessage(`{"processId":"a","ProcessID":99,"deltaBase64":"QQ==","stream":"stdout","capReached":false}`)}, []Notification{
			{Method: protocol.NotifyCommandExecOutputDelta, Params: json.RawMessage(`{"processId":"a","deltaBase64":"Qg==","stream":"stdout","capReached":false}`)},
		}},
		{"case-sensitive search identity", Notification{Method: protocol.NotifyFuzzyFileSearchSessionUpdated, Params: json.RawMessage(`{"sessionId":"a","SessionID":"other"}`)}, []Notification{
			{Method: protocol.NotifyFuzzyFileSearchSessionCompleted, Params: json.RawMessage(`{"sessionId":"a"}`)},
		}},
		{"optional MCP global owner", Notification{Method: protocol.NotifyMcpServerOauthLoginCompleted, Params: json.RawMessage(`{"name":"server","success":true}`)}, []Notification{
			{Method: protocol.NotifyMcpServerOauthLoginCompleted, Params: json.RawMessage(`{"name":"server","success":false,"threadId":null}`)},
		}},
		{"optional warning global owner", Notification{Method: "warning", Params: json.RawMessage(`{"message":"first","ThreadID":"other"}`)}, []Notification{
			{Method: "warning", Params: json.RawMessage(`{"message":"second","threadId":null}`)},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, w := io.Pipe()
			tr := NewStdioTransport(r, io.Discard)
			t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			seen := make(chan string, len(tc.rest)+1)
			unrelated := make(chan struct{})
			var firstEntered atomic.Bool
			tr.OnNotify(func(_ context.Context, n Notification) {
				if n.Method == protocol.NotifyConfigWarning {
					close(unrelated)
					return
				}
				if firstEntered.CompareAndSwap(false, true) {
					close(entered)
					<-release
				}
				seen <- n.Method + ":" + string(n.Params)
			})
			writeAuditNotification(t, w, tc.first)
			waitAuditSignal(t, entered)
			for _, n := range tc.rest {
				writeAuditNotification(t, w, n)
			}
			writeAuditNotification(t, w, Notification{Method: protocol.NotifyConfigWarning, Params: json.RawMessage(`{}`)})
			waitAuditSignal(t, unrelated)
			unblock()
			want := []string{tc.first.Method + ":" + string(tc.first.Params)}
			for _, n := range tc.rest {
				want = append(want, n.Method+":"+string(n.Params))
			}
			var got []string
			for range want {
				select {
				case s := <-seen:
					got = append(got, s)
				case <-time.After(2 * time.Second):
					t.Fatal("notifications did not finish")
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("delivery order = %v; want %v", got, want)
			}
		})
	}
}

func TestStdioOrderedNotificationsDrainAfterReaderStop(t *testing.T) {
	for _, cause := range []error{nil, errors.New("reader failed")} {
		t.Run(fmt.Sprint(cause), func(t *testing.T) {
			r, w := io.Pipe()
			tr := NewStdioTransport(r, io.Discard)
			t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			seen := make(chan string, 2)
			tr.OnNotify(func(_ context.Context, n Notification) {
				if n.Method == protocol.NotifyAgentMessageDelta {
					close(entered)
					<-release
				}
				seen <- n.Method
			})
			writeAuditNotification(t, w, Notification{Method: protocol.NotifyAgentMessageDelta, Params: json.RawMessage(`{"threadId":"a","delta":"A"}`)})
			waitAuditSignal(t, entered)
			writeAuditNotification(t, w, Notification{Method: protocol.NotifyTurnCompleted, Params: json.RawMessage(`{"threadId":"a"}`)})
			_ = w.CloseWithError(cause)
			waitAuditSignal(t, tr.ReaderStopped())
			unblock()
			for _, want := range []string{protocol.NotifyAgentMessageDelta, protocol.NotifyTurnCompleted} {
				select {
				case got := <-seen:
					if got != want {
						t.Fatalf("reader stop delivery = %q; want %q", got, want)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("accepted notification was lost at EOF")
				}
			}
		})
	}
}

func TestStdioOrderedNotificationsWaitForHandler(t *testing.T) {
	for _, stop := range []string{"live", "EOF", "read failure"} {
		t.Run(stop, func(t *testing.T) {
			r, w := io.Pipe()
			tr := NewStdioTransport(r, io.Discard)
			t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
			for _, delta := range []string{"A", "B"} {
				writeAuditNotification(t, w, Notification{Method: protocol.NotifyAgentMessageDelta, Params: json.RawMessage(`{"threadId":"a","delta":"` + delta + `"}`)})
			}
			if stop != "live" {
				var cause error
				if stop == "read failure" {
					cause = errors.New("reader failed")
				}
				_ = w.CloseWithError(cause)
				waitAuditSignal(t, tr.ReaderStopped())
			}
			seen := make(chan string, 3)
			tr.OnNotify(func(_ context.Context, n Notification) {
				var params struct {
					Delta string `json:"delta"`
				}
				if err := json.Unmarshal(n.Params, &params); err != nil {
					t.Error(err)
				}
				seen <- params.Delta
			})
			if stop == "live" {
				writeAuditNotification(t, w, Notification{Method: protocol.NotifyAgentMessageDelta, Params: json.RawMessage(`{"threadId":"a","delta":"C"}`)})
			}
			want := []string{"A", "B"}
			if stop == "live" {
				want = append(want, "C")
			}
			for _, delta := range want {
				select {
				case got := <-seen:
					if got != delta {
						t.Fatalf("registered handler got %q; want %q", got, delta)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("queued event was not delivered after registration")
				}
			}
		})
	}
}

func TestStdioOrderedHandlerReplacementAndPanic(t *testing.T) {
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	seen := make(chan string, 2)
	tr.OnPanic(func(v any) { seen <- fmt.Sprint(v) })
	tr.OnNotify(func(context.Context, Notification) {
		tr.OnNotify(nil)
		close(entered)
		<-release
		panic("first")
	})
	writeAuditNotification(t, w, Notification{Method: protocol.NotifyAgentMessageDelta, Params: json.RawMessage(`{"threadId":"a"}`)})
	waitAuditSignal(t, entered)
	writeAuditNotification(t, w, Notification{Method: protocol.NotifyTurnCompleted, Params: json.RawMessage(`{"threadId":"a"}`)})
	barrier := make(chan struct{})
	tr.OnNotify(func(_ context.Context, n Notification) {
		if n.Method == protocol.NotifyConfigWarning {
			close(barrier)
			return
		}
		seen <- "second"
	})
	writeAuditNotification(t, w, Notification{Method: protocol.NotifyConfigWarning, Params: json.RawMessage(`{}`)})
	waitAuditSignal(t, barrier)
	unblock()
	for _, want := range []string{"first", "second"} {
		select {
		case got := <-seen:
			if got != want {
				t.Fatalf("replacement/panic delivery = %q; want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("ordered worker did not recover")
		}
	}
}

func TestStdioOrderedQueueReuse(t *testing.T) {
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	seen := make(chan int, 1)
	var active atomic.Int32
	tr.OnNotify(func(_ context.Context, n Notification) {
		if active.Add(1) != 1 {
			t.Error("same owner ran concurrent callbacks")
		}
		defer active.Add(-1)
		var params struct {
			Sequence int `json:"sequence"`
		}
		if err := json.Unmarshal(n.Params, &params); err != nil {
			t.Error(err)
		}
		seen <- params.Sequence
		runtime.Gosched()
	})
	// The next admission races the previous callback's return and queue
	// retirement, repeatedly exercising both reuse and new queue creation.
	for i := range 500 {
		writeAuditNotification(t, w, Notification{Method: protocol.NotifyAgentMessageDelta, Params: json.RawMessage(fmt.Sprintf(`{"threadId":"a","sequence":%d}`, i))})
		select {
		case got := <-seen:
			if got != i {
				t.Fatalf("reused owner got %d; want %d", got, i)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("reused owner stopped progressing")
		}
	}
}

func TestStdioMalformedOrderedIdentityUsesFallback(t *testing.T) {
	for _, params := range []string{`{}`, `{"threadId":null}`, `{"threadId":99}`, `{"threadId":[]}`, `[]`} {
		t.Run(params, func(t *testing.T) {
			r, w := io.Pipe()
			tr := NewStdioTransport(r, io.Discard)
			t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
			seen := make(chan string, 2)
			tr.OnNotify(func(_ context.Context, n Notification) { seen <- string(n.Params) })
			writeAuditNotification(t, w, Notification{Method: protocol.NotifyThreadNameUpdated, Params: json.RawMessage(params)})
			writeAuditNotification(t, w, Notification{Method: protocol.NotifyAgentMessageDelta, Params: json.RawMessage(`{"threadId":"valid"}`)})
			got := map[string]bool{}
			for range 2 {
				select {
				case s := <-seen:
					got[s] = true
				case <-time.After(2 * time.Second):
					t.Fatal("fallback prevented unrelated delivery")
				}
			}
			if !got[params] || !got[`{"threadId":"valid"}`] {
				t.Fatalf("fallback delivery = %v", got)
			}
		})
	}
}

func TestOrderedAggregateBacklogIsBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tr := &StdioTransport{ctx: ctx, cancelCtx: cancel, readerStopped: make(chan struct{})}
	for i := range maxOrderedNotificationBacklog {
		tr.enqueueTurnScopedNotification(Notification{Method: protocol.NotifyTurnCompleted}, fmt.Sprintf("thread:%d", i/maxTurnScopedNotificationQueueSize))
	}
	if tr.ScanErr() != nil {
		t.Fatalf("valid aggregate backlog rejected: %v", tr.ScanErr())
	}
	queue := tr.turnNotifQueues["thread:0"]
	if _, ok := tr.dequeueTurnScopedNotification(queue); !ok {
		t.Fatal("full backlog could not drain")
	}
	tr.enqueueTurnScopedNotification(Notification{Method: protocol.NotifyAgentMessageDelta}, "thread:0")
	if tr.ScanErr() != nil || tr.orderedNotifCount != maxOrderedNotificationBacklog {
		t.Fatal("dequeue did not restore aggregate capacity")
	}
	tr.clearTurnScopedNotificationQueue(queue)
	if tr.orderedNotifCount != maxOrderedNotificationBacklog-maxTurnScopedNotificationQueueSize {
		t.Fatal("clear did not release all queued capacity")
	}
	for range maxTurnScopedNotificationQueueSize {
		tr.enqueueTurnScopedNotification(Notification{Method: protocol.NotifyTurnCompleted}, "thread:0")
	}
	if tr.ScanErr() != nil || tr.orderedNotifCount != maxOrderedNotificationBacklog {
		t.Fatal("clear did not restore aggregate capacity")
	}
	tr.enqueueTurnScopedNotification(Notification{Method: protocol.NotifyAgentMessageDelta}, "thread:extra")
	if !errors.Is(tr.ScanErr(), errTurnScopedNotificationQueueOverflow) {
		t.Fatalf("aggregate overflow = %v", tr.ScanErr())
	}
	if tr.orderedNotifCount != 0 {
		t.Fatalf("overflow retained %d queued events", tr.orderedNotifCount)
	}
}

type orderedFailureWriter struct {
	entered, release chan struct{}
}

func (w orderedFailureWriter) Write([]byte) (int, error) {
	close(w.entered)
	<-w.release
	return 0, errors.New("writer failed")
}

func TestStdioWriteFailureAfterEOFPreservesOrderedDrain(t *testing.T) {
	r, w := io.Pipe()
	writeEntered, releaseWrite := make(chan struct{}), make(chan struct{})
	tr := NewStdioTransport(r, orderedFailureWriter{writeEntered, releaseWrite})
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	var releaseWriteOnce sync.Once
	unblockWrite := func() { releaseWriteOnce.Do(func() { close(releaseWrite) }) }
	t.Cleanup(unblockWrite)
	writeDone := make(chan error, 1)
	// Retain this envelope's completion signal so EOF cannot wake the test
	// before the write loop has actually handled the writer's error.
	tr.writeQueue <- writeEnvelope{payload: []byte(`{"method":"client/event"}`), done: writeDone}
	waitAuditSignal(t, writeEntered)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	seen := make(chan string, 2)
	tr.OnNotify(func(_ context.Context, n Notification) {
		if n.Method == protocol.NotifyAgentMessageDelta {
			close(entered)
			<-release
		}
		seen <- n.Method
	})
	writeAuditNotification(t, w, Notification{Method: protocol.NotifyAgentMessageDelta, Params: json.RawMessage(`{"threadId":"a"}`)})
	waitAuditSignal(t, entered)
	writeAuditNotification(t, w, Notification{Method: protocol.NotifyTurnCompleted, Params: json.RawMessage(`{"threadId":"a"}`)})
	_ = w.Close()
	waitAuditSignal(t, tr.ReaderStopped())
	unblockWrite()
	select {
	case err := <-writeDone:
		if err == nil {
			t.Fatal("in-flight write succeeded after reader stop")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight write did not stop")
	}
	// Reader termination already owns accepted-event drain. A late writer
	// error does not undo it; explicit Close remains the way to abort it.
	unblock()
	for _, want := range []string{protocol.NotifyAgentMessageDelta, protocol.NotifyTurnCompleted} {
		select {
		case got := <-seen:
			if got != want {
				t.Fatalf("late failure delivery = %q; want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("late failure lost accepted notification")
		}
	}
}

func TestCloseRejectsOrderedQueueSelectedBeforeEOF(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tr := &StdioTransport{ctx: ctx, cancelCtx: cancel, readerStopped: make(chan struct{})}
	var handled atomic.Int32
	tr.OnNotify(func(context.Context, Notification) { handled.Add(1) })
	tr.enqueueTurnScopedNotification(Notification{Method: protocol.NotifyTurnCompleted}, "thread:a")
	queue, ok := tr.nextTurnScopedNotificationQueue()
	if !ok {
		t.Fatal("queue was not selected")
	}
	tr.stopAfterReaderEOF()
	_ = tr.Close()
	tr.handleTurnScopedNotificationQueue(queue)
	if handled.Load() != 0 {
		t.Fatal("selected queued callback started after explicit Close")
	}
}

func TestStdioCloseReleasesOrderedPayloads(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(fmt.Sprint(active), func(t *testing.T) {
			r, w := io.Pipe()
			tr := NewStdioTransport(r, io.Discard)
			t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			if active {
				tr.OnNotify(func(context.Context, Notification) { close(entered); <-release })
			}
			writeAuditNotification(t, w, Notification{Method: protocol.NotifyAgentMessageDelta, Params: json.RawMessage(`{"threadId":"a","delta":"first"}`)})
			if active {
				waitAuditSignal(t, entered)
			}
			for range 10 {
				writeAuditNotification(t, w, Notification{Method: protocol.NotifyTurnCompleted, Params: json.RawMessage(`{"threadId":"a","payload":"retained"}`)})
			}
			_ = w.Close()
			waitAuditSignal(t, tr.ReaderStopped())
			tr.turnNotifQueuesMu.Lock()
			queue := tr.turnNotifQueues["thread:a"]
			tr.turnNotifQueuesMu.Unlock()
			_ = tr.Close()
			tr.turnNotifQueuesMu.Lock()
			if len(tr.turnNotifQueues) != 0 || tr.orderedNotifCount != 0 {
				t.Errorf("Close retained %d queues and %d events", len(tr.turnNotifQueues), tr.orderedNotifCount)
			}
			tr.turnNotifQueuesMu.Unlock()
			tr.turnNotifReadyMu.Lock()
			if len(tr.turnNotifReady) != 0 {
				t.Errorf("Close retained %d ready owners", len(tr.turnNotifReady))
			}
			tr.turnNotifReadyMu.Unlock()
			queue.mu.Lock()
			if queue.queue != nil || queue.scheduled {
				t.Error("Close retained queued payload ownership")
			}
			queue.mu.Unlock()
			unblock()
		})
	}
}

func TestStdioOrdersAllSchemaThreadNotifications(t *testing.T) {
	data, err := os.ReadFile("../protocol/schema/json/ServerNotification.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Definitions map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"definitions"`
		Events []struct {
			Properties struct {
				Method struct {
					Values []string `json:"enum"`
				} `json:"method"`
				Params struct {
					Ref string `json:"$ref"`
				} `json:"params"`
			} `json:"properties"`
		} `json:"oneOf"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if len(schema.Events) == 0 {
		t.Fatal("schema notification inventory is empty")
	}
	for _, event := range schema.Events {
		definition, exists := schema.Definitions[strings.TrimPrefix(event.Properties.Params.Ref, "#/definitions/")]
		if !exists || len(event.Properties.Method.Values) != 1 {
			t.Fatal("unsupported schema notification shape")
		}
		if _, owned := definition.Properties["threadId"]; !owned {
			continue
		}
		method := event.Properties.Method.Values[0]
		t.Run(method, func(t *testing.T) {
			r, w := io.Pipe()
			tr := NewStdioTransport(r, io.Discard)
			t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			seen := make(chan int, 2)
			var first atomic.Bool
			barrier := make(chan struct{})
			tr.OnNotify(func(_ context.Context, n Notification) {
				if n.Method == protocol.NotifyConfigWarning {
					close(barrier)
					return
				}
				var payload struct {
					Sequence int `json:"sequence"`
				}
				if err := json.Unmarshal(n.Params, &payload); err != nil {
					t.Error(err)
				}
				if first.CompareAndSwap(false, true) {
					close(entered)
					<-release
				}
				seen <- payload.Sequence
			})
			// The schema selects methods and their owned key independently of
			// the dispatch lists. Payload validation remains the typed client's job.
			writeAuditNotification(t, w, Notification{Method: method, Params: json.RawMessage(`{"threadId":"a","ThreadID":"other","sequence":1}`)})
			waitAuditSignal(t, entered)
			writeAuditNotification(t, w, Notification{Method: protocol.NotifyTurnCompleted, Params: json.RawMessage(`{"threadId":"a","sequence":2}`)})
			writeAuditNotification(t, w, Notification{Method: protocol.NotifyConfigWarning, Params: json.RawMessage(`{}`)})
			waitAuditSignal(t, barrier)
			unblock()
			for _, want := range []int{1, 2} {
				select {
				case got := <-seen:
					if got != want {
						t.Fatalf("schema-owned event = %d; want %d", got, want)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("schema-owned event lost")
				}
			}
		})
	}
}

func TestStdioCloseStopsOrderedEOFDrain(t *testing.T) {
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	seen := make(chan string, 2)
	tr.OnNotify(func(_ context.Context, n Notification) {
		if n.Method == protocol.NotifyAgentMessageDelta {
			close(entered)
			<-release
		}
		seen <- n.Method
	})
	writeAuditNotification(t, w, Notification{Method: protocol.NotifyAgentMessageDelta, Params: json.RawMessage(`{"threadId":"a"}`)})
	waitAuditSignal(t, entered)
	writeAuditNotification(t, w, Notification{Method: protocol.NotifyTurnCompleted, Params: json.RawMessage(`{"threadId":"a"}`)})
	_ = w.Close()
	waitAuditSignal(t, tr.ReaderStopped())
	_ = tr.Close()
	unblock()
	select {
	case got := <-seen:
		if got != protocol.NotifyAgentMessageDelta {
			t.Fatalf("Close admitted queued %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight handler did not return")
	}
	if _, ok := tr.nextTurnScopedNotificationQueue(); ok {
		t.Fatal("Close admitted more ordered work")
	}
}

func writeAuditNotification(t *testing.T, w io.Writer, n Notification) {
	t.Helper()
	n.JSONRPC = jsonrpcVersion
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		t.Fatal(err)
	}
}

func waitAuditSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("notification barrier timed out")
	}
}
