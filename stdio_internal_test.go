package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHandleInvalidRequestObjectInvalidIDUsesNullID(t *testing.T) {
	var buf safeBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transport := &StdioTransport{
		reader:        strings.NewReader(""),
		writer:        &buf,
		pendingReqs:   make(map[string]pendingReq),
		writeQueue:    make(chan writeEnvelope, outboundWriteQueueSize),
		readerStopped: make(chan struct{}),
		ctx:           ctx,
		cancelCtx:     cancel,
	}
	go transport.writeLoop()

	// JSON-RPC invalid request responses must use id:null when the id cannot be parsed.
	data := []byte(`{"id":{"unexpected":"shape"},"method":"test"}`)
	if handled := transport.handleInvalidRequestObject(data); !handled {
		t.Fatal("expected invalid request object to be handled")
	}

	output := strings.TrimSpace(buf.String())
	if output == "" {
		t.Fatal("expected invalid request response to be written")
	}

	var resp Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != ErrCodeInvalidRequest {
		t.Fatalf("expected invalid request response, got %+v", resp.Error)
	}
	if resp.ID.Value != nil {
		t.Fatalf("response ID = %v; want nil", resp.ID.Value)
	}
}

func TestHandleInvalidRequestObjectValidIDPreservesID(t *testing.T) {
	var buf safeBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transport := &StdioTransport{
		reader:        strings.NewReader(""),
		writer:        &buf,
		pendingReqs:   make(map[string]pendingReq),
		writeQueue:    make(chan writeEnvelope, outboundWriteQueueSize),
		readerStopped: make(chan struct{}),
		ctx:           ctx,
		cancelCtx:     cancel,
	}
	go transport.writeLoop()

	data := []byte(`{"jsonrpc":"2.0","id":"req-1","method":123}`)
	if handled := transport.handleInvalidRequestObject(data); !handled {
		t.Fatal("expected invalid request object to be handled")
	}

	output := strings.TrimSpace(buf.String())
	if output == "" {
		t.Fatal("expected invalid request response to be written")
	}

	var resp Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != ErrCodeInvalidRequest {
		t.Fatalf("expected invalid request response, got %+v", resp.Error)
	}
	if resp.ID.Value != "req-1" {
		t.Fatalf("response ID = %v; want req-1", resp.ID.Value)
	}
}

func TestStdioInboundRequestDispatchIsBounded(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	var current atomic.Int32
	var peak atomic.Int32
	release := make(chan struct{})
	transport.OnRequest(func(_ context.Context, req Request) (Response, error) {
		active := current.Add(1)
		for {
			prev := peak.Load()
			if active <= prev || peak.CompareAndSwap(prev, active) {
				break
			}
		}
		defer current.Add(-1)
		<-release
		return Response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}, nil
	})

	errCodes := make(chan int, inboundRequestWorkers+inboundRequestQueueSize+16)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var resp Response
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil || resp.Error == nil {
				continue
			}
			errCodes <- resp.Error.Code
		}
	}()

	total := inboundRequestWorkers + inboundRequestQueueSize + 16
	for i := range total {
		req := Request{
			JSONRPC: jsonrpcVersion,
			ID:      RequestID{Value: i},
			Method:  "approval/flood",
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal request %d: %v", i, err)
		}
		if _, err := serverWriter.Write(append(data, '\n')); err != nil {
			t.Fatalf("write request %d: %v", i, err)
		}
	}

	time.Sleep(200 * time.Millisecond)
	if got := peak.Load(); got > inboundRequestWorkers {
		t.Fatalf("peak concurrent handlers = %d; want <= %d", got, inboundRequestWorkers)
	}

	close(release)

	deadline := time.After(2 * time.Second)
	var sawOverload bool
	for !sawOverload {
		select {
		case code := <-errCodes:
			if code == ErrCodeInternalError {
				sawOverload = true
			}
		case <-deadline:
			t.Fatal("expected at least one overload response under request flood")
		}
	}
}

func TestNotifyCanceledContextDoesNotAttemptWrite(t *testing.T) {
	reader, _ := io.Pipe()
	bw := &writeCountWriter{}
	transport := NewStdioTransport(reader, bw)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := transport.Notify(ctx, Notification{
		JSONRPC: jsonrpcVersion,
		Method:  "test/canceled",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Notify error = %v; want context.Canceled", err)
	}

	time.Sleep(50 * time.Millisecond)
	if got := bw.calls.Load(); got != 0 {
		t.Fatalf("writer was called %d times for canceled notify; want 0", got)
	}
}

func TestRecvWhileRunningReturnsQueuedValue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue := make(chan int, 1)
	queue <- 7

	value, ok := recvWhileRunning(ctx, queue)
	if !ok {
		t.Fatal("recvWhileRunning() reported queue closed while context was active")
	}
	if value != 7 {
		t.Fatalf("recvWhileRunning() = %d; want 7", value)
	}
}

func TestRecvWhileRunningDropsBufferedValueAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	queue := make(chan int, 1)
	queue <- 7
	cancel()

	value, ok := recvWhileRunning(ctx, queue)
	if ok {
		t.Fatalf("recvWhileRunning() reported buffered value %d after cancellation", value)
	}
}

func TestWriteLoopSkipsBufferedWriteAfterCancellation(t *testing.T) {
	var buf safeBuffer

	ctx, cancel := context.WithCancel(context.Background())
	transport := &StdioTransport{
		writer:     &buf,
		writeQueue: make(chan writeEnvelope, 1),
		ctx:        ctx,
		cancelCtx:  cancel,
	}
	transport.writeQueue <- writeEnvelope{
		payload: []byte(`{"jsonrpc":"2.0","method":"test"}`),
		done:    make(chan error, 1),
	}

	cancel()
	transport.writeLoop()

	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Fatalf("writeLoop wrote %q after cancellation; want no writes", got)
	}
}

func TestEnqueueWriteReaderStoppedWithoutWatcherWaitsForActualWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	transport := &StdioTransport{
		writeQueue:    make(chan writeEnvelope),
		readerStopped: make(chan struct{}),
		ctx:           context.Background(),
	}
	close(transport.readerStopped)

	err := transport.enqueueWrite(ctx, Notification{Method: "test/notification"}, "write message", false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("enqueueWrite() error = %v; want context deadline exceeded", err)
	}
}

func TestEnqueueWriteReaderStoppedWithoutWatcherCanStillFlushQueuedWrite(t *testing.T) {
	var buf safeBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	transport := &StdioTransport{
		writer:        &buf,
		writeQueue:    make(chan writeEnvelope, 1),
		readerStopped: make(chan struct{}),
		ctx:           ctx,
		cancelCtx:     cancel,
	}
	go transport.writeLoop()
	close(transport.readerStopped)

	if err := transport.enqueueWrite(context.Background(), Notification{
		JSONRPC: jsonrpcVersion,
		Method:  "test/notification",
	}, "write message", false); err != nil {
		t.Fatalf("enqueueWrite() error = %v", err)
	}

	if got := strings.TrimSpace(buf.String()); got == "" {
		t.Fatal("enqueueWrite() did not flush queued notification")
	}
}

func TestRequestWorkerSkipsBufferedRequestAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	transport := &StdioTransport{
		requestQueue: make(chan Request, 1),
		ctx:          ctx,
		cancelCtx:    cancel,
	}

	var handled atomic.Int32
	transport.OnRequest(func(_ context.Context, _ Request) (Response, error) {
		handled.Add(1)
		return Response{}, nil
	})

	transport.requestQueue <- Request{Method: "approval/test"}
	cancel()
	transport.requestWorker()

	if got := handled.Load(); got != 0 {
		t.Fatalf("requestWorker invoked handler %d times after cancellation; want 0", got)
	}
}

func TestNotificationWorkerSkipsBufferedNotificationsAfterCancellation(t *testing.T) {
	tests := []struct {
		name   string
		queue  chan Notification
		worker func(*StdioTransport)
	}{
		{
			name: "standard",
			queue: func() chan Notification {
				return make(chan Notification, 1)
			}(),
			worker: (*StdioTransport).notificationWorker,
		},
		{
			name: "critical",
			queue: func() chan Notification {
				return make(chan Notification, 1)
			}(),
			worker: (*StdioTransport).criticalNotificationWorker,
		},
		{
			name: "terminal",
			queue: func() chan Notification {
				return make(chan Notification, 1)
			}(),
			worker: (*StdioTransport).terminalNotificationWorker,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			transport := &StdioTransport{
				notifQueue:         tt.queue,
				criticalNotifQueue: tt.queue,
				terminalNotifQueue: tt.queue,
				ctx:                ctx,
				cancelCtx:          cancel,
			}

			var handled atomic.Int32
			transport.OnNotify(func(_ context.Context, _ Notification) {
				handled.Add(1)
			})

			tt.queue <- Notification{Method: "test/notification"}
			cancel()
			tt.worker(transport)

			if got := handled.Load(); got != 0 {
				t.Fatalf("notification handler invoked %d times after cancellation; want 0", got)
			}
		})
	}
}

func TestCleanupPendingReqDeletesMatchingEntry(t *testing.T) {
	transport := &StdioTransport{
		pendingReqs: make(map[string]pendingReq),
	}
	normalizedID := "s:req-1"
	pending := pendingReq{
		ch: make(chan Response, 1),
		id: RequestID{Value: "req-1"},
	}
	transport.pendingReqs[normalizedID] = pending

	transport.cleanupPendingReq(normalizedID, pending)

	if _, ok := transport.pendingReqs[normalizedID]; ok {
		t.Fatal("pending request was not removed")
	}
}

func TestCleanupPendingReqSkipsReusedIDEntry(t *testing.T) {
	transport := &StdioTransport{
		pendingReqs: make(map[string]pendingReq),
	}
	normalizedID := "s:req-1"
	first := pendingReq{
		ch: make(chan Response, 1),
		id: RequestID{Value: "req-1"},
	}
	second := pendingReq{
		ch: make(chan Response, 1),
		id: RequestID{Value: "req-1"},
	}
	transport.pendingReqs[normalizedID] = second

	transport.cleanupPendingReq(normalizedID, first)

	current, ok := transport.pendingReqs[normalizedID]
	if !ok {
		t.Fatal("cleanup removed a newer pending request for the same ID")
	}
	if current.ch != second.ch {
		t.Fatal("cleanup replaced pending request unexpectedly")
	}
}

func TestStdioNotificationFloodStillDeliversTurnCompleted(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()

	transport := NewStdioTransport(clientReader, &safeBuffer{})
	defer func() { _ = transport.Close() }()

	release := make(chan struct{})
	criticalSeen := make(chan struct{}, 1)
	transport.OnNotify(func(_ context.Context, notif Notification) {
		if notif.Method == notifyTurnCompleted {
			select {
			case criticalSeen <- struct{}{}:
			default:
			}
			return
		}
		<-release
	})

	nonCritical := Notification{
		JSONRPC: jsonrpcVersion,
		Method:  notifyAgentMessageDelta,
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","delta":"x"}`),
	}
	nonCriticalBytes, err := json.Marshal(nonCritical)
	if err != nil {
		t.Fatalf("marshal non-critical notification: %v", err)
	}

	totalFlood := inboundNotificationWorkers + inboundNotifQueueSize + 32
	for range totalFlood {
		if _, err := serverWriter.Write(append(nonCriticalBytes, '\n')); err != nil {
			t.Fatalf("write flood notification: %v", err)
		}
	}

	critical := Notification{
		JSONRPC: jsonrpcVersion,
		Method:  notifyTurnCompleted,
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	}
	criticalBytes, err := json.Marshal(critical)
	if err != nil {
		t.Fatalf("marshal critical notification: %v", err)
	}
	if _, err := serverWriter.Write(append(criticalBytes, '\n')); err != nil {
		t.Fatalf("write critical notification: %v", err)
	}

	select {
	case <-criticalSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("turn/completed notification was not delivered under queue pressure")
	}

	close(release)
}

func TestStdioNotificationFloodStillDeliversItemCompleted(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()

	transport := NewStdioTransport(clientReader, &safeBuffer{})
	defer func() { _ = transport.Close() }()

	release := make(chan struct{})
	itemSeen := make(chan struct{}, 1)
	transport.OnNotify(func(_ context.Context, notif Notification) {
		if notif.Method == notifyItemCompleted {
			select {
			case itemSeen <- struct{}{}:
			default:
			}
			return
		}
		<-release
	})

	nonCritical := Notification{
		JSONRPC: jsonrpcVersion,
		Method:  notifyAgentMessageDelta,
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","delta":"x"}`),
	}
	nonCriticalBytes, err := json.Marshal(nonCritical)
	if err != nil {
		t.Fatalf("marshal non-critical notification: %v", err)
	}

	totalFlood := inboundNotificationWorkers + inboundNotifQueueSize + 32
	for range totalFlood {
		if _, err := serverWriter.Write(append(nonCriticalBytes, '\n')); err != nil {
			t.Fatalf("write flood notification: %v", err)
		}
	}

	itemCompleted := Notification{
		JSONRPC: jsonrpcVersion,
		Method:  notifyItemCompleted,
		Params: json.RawMessage(
			`{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"agent_message","text":"done"}}`,
		),
	}
	itemBytes, err := json.Marshal(itemCompleted)
	if err != nil {
		t.Fatalf("marshal item/completed notification: %v", err)
	}
	if _, err := serverWriter.Write(append(itemBytes, '\n')); err != nil {
		t.Fatalf("write item/completed notification: %v", err)
	}

	select {
	case <-itemSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("item/completed notification was not delivered under queue pressure")
	}

	close(release)
}

func TestStdioCloseStopsReaderForClosableReader(t *testing.T) {
	reader := newBlockingReadCloser()
	transport := NewStdioTransport(reader, &safeBuffer{})

	if err := transport.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	select {
	case <-transport.readerStopped:
	case <-time.After(2 * time.Second):
		t.Fatal("reader goroutine did not stop after Close")
	}
}

func TestNewStdioTransportPanicsOnNilReader(t *testing.T) {
	defer func() {
		if got := recover(); got != errNilTransportReader {
			t.Fatalf("panic = %v; want %q", got, errNilTransportReader)
		}
	}()

	NewStdioTransport(nil, &safeBuffer{})
}

func TestNewStdioTransportPanicsOnNilWriter(t *testing.T) {
	reader := newBlockingReadCloser()
	defer func() { _ = reader.Close() }()
	defer func() {
		if got := recover(); got != errNilTransportWriter {
			t.Fatalf("panic = %v; want %q", got, errNilTransportWriter)
		}
	}()

	NewStdioTransport(reader, nil)
}

// safeBuffer is a concurrency-safe bytes.Buffer for testing.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type writeCountWriter struct {
	calls atomic.Int32
}

func (w *writeCountWriter) Write(p []byte) (int, error) {
	w.calls.Add(1)
	return len(p), nil
}

type blockingReadCloser struct {
	once sync.Once
	done chan struct{}
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{done: make(chan struct{})}
}

func (r *blockingReadCloser) Read(_ []byte) (int, error) {
	<-r.done
	return 0, io.EOF
}

func (r *blockingReadCloser) Close() error {
	r.once.Do(func() {
		close(r.done)
	})
	return nil
}
