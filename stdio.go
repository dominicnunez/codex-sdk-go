package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// pendingReq holds a pending request's response channel and original ID.
type pendingReq struct {
	ch chan pendingReqResult
	id RequestID
}

type pendingReqResult struct {
	resp Response
	err  error
}

// inbound/outbound queue sizing. These are intentionally conservative defaults:
// large enough for normal bursts, bounded to prevent untrusted-peer DoS via
// unbounded goroutine or memory growth.
const (
	inboundRequestWorkers              = 8
	inboundRequestQueueSize            = 64
	inboundNotificationWorkers         = 8
	streamingNotificationWorkers       = 8
	protectedNotificationWorkers       = 8
	criticalNotificationWorkers        = 2
	turnScopedNotificationWorkers      = 8
	maxTurnScopedNotificationQueueSize = 256
	maxTurnScopedNotificationQueues    = 128
	maxStreamingNotificationBacklog    = 1024
	inboundNotifQueueSize              = 128
	streamingNotifQueueSize            = 256
	protectedNotifQueueSize            = 128
	criticalNotifQueueSize             = 64
	outboundWriteQueueSize             = 256
	readBufferSizeBytes                = 64 * 1024
	maxInboundMessageSizeBytes         = 10 * 1024 * 1024
	defaultSendTimeout                 = 5 * time.Minute
)

const (
	errNilTransportReader     = "stdio transport reader must not be nil"
	errNilTransportWriter     = "stdio transport writer must not be nil"
	errInvalidJSONRPCVersion  = `invalid request: jsonrpc must be "2.0"`
	errInvalidResponseJSONRPC = `invalid response: jsonrpc must be "2.0"`
	requestIDKeyPrefixNumber  = "n:"
	requestIDKeyPrefixString  = "s:"
)

type writeEnvelope struct {
	payload []byte
	done    chan error
}

type turnScopedNotificationQueue struct {
	mu        sync.Mutex
	threadKey string
	queue     []Notification
	scheduled bool
}

type streamingNotificationBacklog struct {
	mu       sync.Mutex
	queue    []Notification
	draining bool
}

type inboundFrame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      inboundID       `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   inboundError    `json:"error,omitempty"`
}

func (f inboundFrame) hasResultField() bool {
	return len(f.Result) > 0
}

func (f inboundFrame) hasResponseFields() bool {
	return f.hasResultField() || f.Error.present
}

func (f inboundFrame) hasMalformedResponseShape() bool {
	hasResult := f.hasResultField()
	hasError := f.Error.present
	if !hasResult && !hasError {
		return false
	}
	if hasResult && hasError {
		return true
	}
	if hasError {
		return f.Error.invalid || f.Error.isNull || f.Error.value == nil
	}
	return false
}

type inboundID struct {
	present bool
	isNull  bool
	value   RequestID
	invalid bool
}

func (i *inboundID) UnmarshalJSON(data []byte) error {
	i.present = true
	i.isNull = bytes.Equal(data, []byte("null"))
	if i.isNull {
		i.value = RequestID{}
		i.invalid = false
		return nil
	}

	var parsed RequestID
	if json.Unmarshal(data, &parsed) != nil {
		i.invalid = true
		return nil //nolint:nilerr // Preserve frame routing; invalid ID is handled after frame classification.
	}
	i.value = parsed
	i.invalid = false
	return nil
}

func (i inboundID) hasValue() bool {
	return i.present && !i.isNull
}

func (i inboundID) requestID() (RequestID, bool) {
	if !i.hasValue() || i.invalid {
		return RequestID{}, false
	}
	return i.value, true
}

type inboundError struct {
	present bool
	isNull  bool
	value   *Error
	invalid bool
}

type oversizedFrameInfo struct {
	id                RequestID
	hasID             bool
	hasMethod         bool
	hasResponseFields bool
}

func (e *inboundError) UnmarshalJSON(data []byte) error {
	e.present = true
	e.isNull = bytes.Equal(data, []byte("null"))
	if e.isNull {
		e.value = nil
		e.invalid = false
		return nil
	}

	var parsed Error
	if json.Unmarshal(data, &parsed) != nil {
		e.invalid = true
		//nolint:nilerr // Preserve frame routing; invalid error payload is handled as malformed response.
		return nil
	}
	e.value = &parsed
	e.invalid = false
	return nil
}

// StdioTransport implements the Transport interface using stdin/stdout with newline-delimited JSON.
// It supports bidirectional JSON-RPC 2.0 communication:
// - Client→Server: Send requests and notifications
// - Server→Client: Receive requests (for approval flows) and notifications (for events)
type StdioTransport struct {
	reader       io.Reader
	readerCloser io.Closer
	writer       io.Writer

	mu                  sync.Mutex
	closed              bool
	readerEOF           bool
	pendingReqs         map[string]pendingReq
	reqHandler          RequestHandler
	notifHandler        NotificationHandler
	pendingReqHandler   []Request
	pendingNotifHandle  []Notification
	requestQueue        chan Request
	turnNotifQueuesMu   sync.Mutex
	turnNotifQueues     map[string]*turnScopedNotificationQueue
	turnNotifReadyMu    sync.Mutex
	turnNotifReady      []*turnScopedNotificationQueue
	turnNotifReadyCond  *sync.Cond
	turnNotifReadyOnce  sync.Once
	streamingNotifQueue chan Notification
	streamingBacklog    streamingNotificationBacklog
	protectedNotifQueue chan Notification
	criticalNotifQueue  chan Notification
	notifQueue          chan Notification
	writeQueue          chan writeEnvelope
	readerStopped       chan struct{}
	once                sync.Once
	startReadLoopOnce   sync.Once
	scanErr             error // terminal transport I/O error, if any
	malformedCount      atomic.Uint64
	panicHandler        func(v any)
	ctx                 context.Context
	cancelCtx           context.CancelFunc
}

// errUnexpectedIDType is returned when normalizeID encounters an ID value
// that is not a supported JSON-RPC ID type (string, number).
var errUnexpectedIDType = errors.New("unexpected ID type")
var errTransportClosed = errors.New("transport closed")
var errTransportReaderStopped = errors.New("transport reader stopped")
var errOversizedInboundFrame = errors.New("oversized inbound frame exceeded maximum size")
var errNotificationQueueOverflow = errors.New("notification queue overflow")
var errStreamingNotificationBacklogOverflow = errors.New("streaming notification backlog overflow")
var errTurnScopedNotificationQueueOverflow = errors.New("turn-scoped notification queue overflow")
var errTurnScopedNotificationQueueLimit = errors.New("turn-scoped notification queue limit exceeded")

// errNullID is returned when normalizeID encounters a nil (JSON null) ID.
// JSON-RPC 2.0 responses with "id": null indicate the server could not
// parse the request ID.
var errNullID = errors.New("null request ID")

// normalizeID normalizes request IDs to a string key for map matching.
// JSON unmarshals all numbers as float64, so we format integer-valued
// floats without decimals for consistent lookups.
func normalizeID(id interface{}) (string, error) {
	normalizedID, _, err := normalizeRequestID(id)
	return normalizedID, err
}

func normalizePendingRequestID(id interface{}) (string, error) {
	normalizedID, familyPrefix, err := normalizeRequestID(id)
	if err != nil {
		return "", err
	}
	return familyPrefix + normalizedID, nil
}

func normalizeRequestID(id interface{}) (string, string, error) {
	switch v := id.(type) {
	case nil:
		return "", "", errNullID
	case string:
		return v, requestIDKeyPrefixString, nil
	}

	normalizedID, isNumeric, err := normalizeNumericID(id)
	if err != nil {
		return "", "", err
	}
	if !isNumeric {
		return "", "", fmt.Errorf("%w: %T", errUnexpectedIDType, id)
	}
	return normalizedID, requestIDKeyPrefixNumber, nil
}

func normalizeNumericID(id interface{}) (string, bool, error) {
	return canonicalNumericRequestIDString(id)
}

// NewStdioTransport creates a new stdio transport using the provided reader and writer.
// reader is required to be an io.ReadCloser so Close can always unblock the read loop.
// Typically, reader is os.Stdin and writer is os.Stdout.
// The transport starts background goroutines for write and inbound dispatch.
// The read loop starts eagerly before this constructor returns.
// It panics if reader/writer are invalid.
func NewStdioTransport(reader io.ReadCloser, writer io.Writer) *StdioTransport {
	if reader == nil {
		panic(errNilTransportReader)
	}
	if writer == nil {
		panic(errNilTransportWriter)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t := &StdioTransport{
		reader:              reader,
		readerCloser:        reader,
		writer:              writer,
		pendingReqs:         make(map[string]pendingReq),
		requestQueue:        make(chan Request, inboundRequestQueueSize),
		turnNotifQueues:     make(map[string]*turnScopedNotificationQueue),
		streamingNotifQueue: make(chan Notification, streamingNotifQueueSize),
		protectedNotifQueue: make(chan Notification, protectedNotifQueueSize),
		criticalNotifQueue:  make(chan Notification, criticalNotifQueueSize),
		notifQueue:          make(chan Notification, inboundNotifQueueSize),
		writeQueue:          make(chan writeEnvelope, outboundWriteQueueSize),
		readerStopped:       make(chan struct{}),
		ctx:                 ctx,
		cancelCtx:           cancel,
	}
	t.initTurnScopedScheduler()
	for range inboundRequestWorkers {
		go t.requestWorker()
	}
	for range inboundNotificationWorkers {
		go t.notificationWorker()
	}
	for range streamingNotificationWorkers {
		go t.streamingNotificationWorker()
	}
	for range protectedNotificationWorkers {
		go t.protectedNotificationWorker()
	}
	for range criticalNotificationWorkers {
		go t.criticalNotificationWorker()
	}
	for range turnScopedNotificationWorkers {
		go t.turnScopedNotificationWorker()
	}
	go t.writeLoop()
	t.ensureReadLoopStarted()
	return t
}

func (t *StdioTransport) initTurnScopedScheduler() {
	t.turnNotifReadyOnce.Do(func() {
		if t.turnNotifQueues == nil {
			t.turnNotifQueues = make(map[string]*turnScopedNotificationQueue)
		}
		t.turnNotifReadyCond = sync.NewCond(&t.turnNotifReadyMu)
	})
}

// Send transmits a JSON-RPC request and waits for the response.
// The response is matched to this request by ID.
func (t *StdioTransport) Send(ctx context.Context, req Request) (Response, error) {
	if ctx == nil {
		return Response{}, NewTransportError("send failed", ErrNilContext)
	}
	ctx, cancel := applyDefaultSendTimeout(ctx)
	defer cancel()

	t.ensureReadLoopStarted()

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return Response{}, t.transportStopError("send failed")
	}

	// Create response channel and store with normalized ID for matching
	normalizedID, err := normalizePendingRequestID(req.ID.Value)
	if err != nil {
		t.mu.Unlock()
		return Response{}, NewTransportError("send failed", err)
	}
	if _, exists := t.pendingReqs[normalizedID]; exists {
		t.mu.Unlock()
		return Response{}, NewTransportError("send failed", fmt.Errorf("duplicate request ID: %v", req.ID.Value))
	}
	respChan := make(chan pendingReqResult, 1)
	pending := pendingReq{ch: respChan, id: req.ID}
	t.pendingReqs[normalizedID] = pending
	t.mu.Unlock()

	// Cleanup on exit
	defer func() {
		t.cleanupPendingReq(normalizedID, pending)
	}()

	if err := t.enqueueWrite(ctx, req, "send failed", true); err != nil {
		return Response{}, err
	}

	// Wait for response or context cancellation
	select {
	case result := <-respChan:
		if result.err != nil {
			return Response{}, result.err
		}
		return result.resp, nil
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case <-t.readerStopped:
		// Prefer a response already delivered to this request over the generic
		// reader-stopped error; both can become ready at nearly the same time.
		select {
		case result := <-respChan:
			if result.err != nil {
				return Response{}, result.err
			}
			return result.resp, nil
		default:
		}
		return Response{}, t.transportStopError("send failed")
	}
}

// Notify transmits a JSON-RPC notification (fire-and-forget).
func (t *StdioTransport) Notify(ctx context.Context, notif Notification) error {
	if ctx == nil {
		return NewTransportError("notify failed", ErrNilContext)
	}
	t.ensureReadLoopStarted()

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return t.transportStopError("notify failed")
	}
	t.mu.Unlock()

	return t.enqueueWrite(ctx, notif, "notify failed", true)
}

// OnRequest registers a handler for incoming JSON-RPC requests from the server.
func (t *StdioTransport) OnRequest(handler RequestHandler) {
	t.mu.Lock()
	t.reqHandler = handler
	pending := t.pendingReqHandler
	t.pendingReqHandler = nil
	t.mu.Unlock()

	for _, req := range pending {
		t.enqueueRequest(req)
	}
}

// OnNotify registers a handler for incoming JSON-RPC notifications from the server.
func (t *StdioTransport) OnNotify(handler NotificationHandler) {
	t.mu.Lock()
	t.notifHandler = handler
	pending := t.pendingNotifHandle
	t.pendingNotifHandle = nil
	t.mu.Unlock()

	for _, notif := range pending {
		t.enqueueNotification(notif)
	}
}

// OnPanic registers a handler called when a request handler or notification
// handler panics. The transport recovers from the panic and continues
// operating; this callback provides observability into the recovered value.
func (t *StdioTransport) OnPanic(handler func(v any)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.panicHandler = handler
}

// Close shuts down the transport. Safe to call multiple times.
func (t *StdioTransport) Close() error {
	t.closeWithFailure(nil, errTransportClosed)
	return nil
}

// ScanErr returns the terminal transport I/O error, if any.
// Returns nil if the reader stopped due to EOF or hasn't stopped yet.
func (t *StdioTransport) ScanErr() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.scanErr
}

// MalformedMessageCount reports how many inbound messages were malformed,
// unclassifiable, or carried an unusable response ID.
func (t *StdioTransport) MalformedMessageCount() uint64 {
	return t.malformedCount.Load()
}

func applyDefaultSendTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultSendTimeout)
}

func (t *StdioTransport) ensureReadLoopStarted() {
	t.startReadLoopOnce.Do(func() {
		go t.readLoop()
	})
}

func (t *StdioTransport) wakeTurnScopedNotificationWorkers() {
	if t.turnNotifReadyCond == nil {
		return
	}
	t.turnNotifReadyMu.Lock()
	t.turnNotifReadyCond.Broadcast()
	t.turnNotifReadyMu.Unlock()
}

func (t *StdioTransport) transportStopError(op string) error {
	t.mu.Lock()
	cause := t.transportStopCauseLocked()
	t.mu.Unlock()
	return NewTransportError(op, cause)
}

func (t *StdioTransport) transportStopCauseLocked() error {
	if t.scanErr != nil {
		if errors.Is(t.scanErr, errOversizedInboundFrame) {
			return errTransportReaderStopped
		}
		return t.scanErr
	}
	if t.readerEOF {
		return errTransportReaderStopped
	}
	return errTransportClosed
}

func (t *StdioTransport) closeWithFailure(scanErr error, cause error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	if scanErr != nil && t.scanErr == nil {
		t.scanErr = scanErr
	}
	t.closed = true
	t.readerEOF = false
	pending := t.pendingReqs
	t.pendingReqs = make(map[string]pendingReq)
	cancel := t.cancelCtx
	readerCloser := t.readerCloser
	t.mu.Unlock()

	cancel()
	t.wakeTurnScopedNotificationWorkers()
	if readerCloser != nil {
		_ = readerCloser.Close()
	}

	pendingErr := pendingRequestTransportError("send failed", cause)
	for _, pendingReq := range pending {
		select {
		case pendingReq.ch <- pendingReqResult{err: pendingErr}:
		default:
		}
	}

	t.once.Do(func() { close(t.readerStopped) })
}

func (t *StdioTransport) handleWriteFailure(err error) {
	if err == nil {
		return
	}
	t.closeWithFailure(err, err)
}

// writeRawMessage writes a pre-marshaled JSON-RPC message and trailing newline.
func (t *StdioTransport) writeRawMessage(data []byte) error {
	// Write message then newline delimiter, handling short writes.
	// The newline is written separately to avoid copying the entire
	// payload just to append one byte.
	for len(data) > 0 {
		n, err := t.writer.Write(data)
		if err != nil {
			return NewTransportError("write message", err)
		}
		if n == 0 {
			return NewTransportError("write message", errors.New("writer returned zero bytes written without error"))
		}
		data = data[n:]
	}

	delim := []byte{'\n'}
	for len(delim) > 0 {
		n, err := t.writer.Write(delim)
		if err != nil {
			return NewTransportError("write message", err)
		}
		if n == 0 {
			return NewTransportError("write message", errors.New("writer returned zero bytes written without error"))
		}
		delim = delim[n:]
	}

	return nil
}

func (t *StdioTransport) enqueueWrite(ctx context.Context, msg interface{}, op string, watchReaderStop bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return NewTransportError("marshal message", err)
	}
	env := writeEnvelope{
		payload: data,
		done:    make(chan error, 1),
	}

	if watchReaderStop {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.ctx.Done():
			return t.transportStopError(op)
		case <-t.readerStopped:
			return t.transportStopError(op)
		case t.writeQueue <- env:
		}
	} else {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.ctx.Done():
			return NewTransportError(op, errTransportClosed)
		case t.writeQueue <- env:
		}
	}

	if watchReaderStop {
		select {
		case err := <-env.done:
			return t.normalizeWriteCompletionError(op, err)
		case <-ctx.Done():
			return ctx.Err()
		case <-t.ctx.Done():
			return t.transportStopError(op)
		case <-t.readerStopped:
			return t.transportStopError(op)
		}
	}

	select {
	case err := <-env.done:
		return t.normalizeWriteCompletionError(op, err)
	case <-ctx.Done():
		return ctx.Err()
	case <-t.ctx.Done():
		return NewTransportError(op, errTransportClosed)
	}
}

func (t *StdioTransport) normalizeWriteCompletionError(op string, err error) error {
	if err == nil {
		return nil
	}

	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return t.transportStopError(op)
	}

	select {
	case <-t.readerStopped:
		return t.transportStopError(op)
	case <-t.ctx.Done():
		return t.transportStopError(op)
	default:
		return err
	}
}

// writeMessage enqueues a JSON-RPC message for serialized writer-loop delivery.
func (t *StdioTransport) writeMessage(msg interface{}) error {
	return t.enqueueWrite(t.ctx, msg, "write message", false)
}

func recvWhileRunning[T any](ctx context.Context, queue <-chan T) (T, bool) {
	var zero T

	select {
	case <-ctx.Done():
		return zero, false
	case value, ok := <-queue:
		if !ok || ctx.Err() != nil {
			return zero, false
		}
		return value, true
	}
}

func (t *StdioTransport) writeLoop() {
	for {
		env, ok := recvWhileRunning(t.ctx, t.writeQueue)
		if !ok {
			return
		}

		err := t.writeRawMessage(env.payload)
		if err != nil {
			t.handleWriteFailure(err)
			env.done <- err
			return
		}
		env.done <- nil
	}
}

func (t *StdioTransport) requestWorker() {
	for {
		req, ok := recvWhileRunning(t.ctx, t.requestQueue)
		if !ok {
			return
		}
		t.handleRequest(req)
	}
}

func (t *StdioTransport) notificationWorker() {
	for {
		notif, ok := recvWhileRunning(t.ctx, t.notifQueue)
		if !ok {
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) streamingNotificationWorker() {
	for {
		notif, ok := recvWhileRunning(t.ctx, t.streamingNotifQueue)
		if !ok {
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) protectedNotificationWorker() {
	for {
		notif, ok := recvWhileRunning(t.ctx, t.protectedNotifQueue)
		if !ok {
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) criticalNotificationWorker() {
	for {
		notif, ok := recvWhileRunning(t.ctx, t.criticalNotifQueue)
		if !ok {
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) turnScopedNotificationWorker() {
	t.initTurnScopedScheduler()
	for {
		queue, ok := t.nextTurnScopedNotificationQueue()
		if !ok {
			return
		}
		t.handleTurnScopedNotificationQueue(queue)
	}
}

func (t *StdioTransport) nextTurnScopedNotificationQueue() (*turnScopedNotificationQueue, bool) {
	t.turnNotifReadyMu.Lock()
	defer t.turnNotifReadyMu.Unlock()

	for len(t.turnNotifReady) == 0 && t.ctx.Err() == nil {
		t.turnNotifReadyCond.Wait()
	}
	if len(t.turnNotifReady) == 0 {
		return nil, false
	}

	queue := t.turnNotifReady[0]
	t.turnNotifReady[0] = nil
	t.turnNotifReady = t.turnNotifReady[1:]
	return queue, true
}

func (t *StdioTransport) scheduleTurnScopedNotificationQueue(queue *turnScopedNotificationQueue) {
	t.turnNotifReadyMu.Lock()
	t.turnNotifReady = append(t.turnNotifReady, queue)
	t.turnNotifReadyMu.Unlock()
	t.turnNotifReadyCond.Signal()
}

// readLoop continuously reads newline-delimited JSON messages from the reader
func (t *StdioTransport) readLoop() {
	defer t.once.Do(func() { close(t.readerStopped) })

	reader := bufio.NewReaderSize(t.reader, readBufferSizeBytes)
	for {
		line, oversize, err := readLimitedLine(reader, maxInboundMessageSizeBytes)
		if oversize != nil {
			if t.handleOversizedRead(*oversize, err) {
				return
			}
			continue
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				t.stopAfterReaderEOF()
				return
			}
			t.stopAfterReadFailure(err)
			return
		}

		t.processInboundLine(line)
	}
}

func (t *StdioTransport) handleOversizedRead(info oversizedFrameInfo, err error) bool {
	// Best-effort: fail matching pending responses so Send callers do not
	// block waiting on a frame we intentionally rejected.
	t.handleOversizedFrame(info)
	_ = err
	t.stopAfterReadFailure(errOversizedInboundFrame)
	return true
}

func (t *StdioTransport) processInboundLine(line []byte) {
	frame, err := decodeInboundFrame(line)
	if err != nil {
		if json.Valid(line) && t.handleInvalidRequestObject(line) {
			return
		}
		t.handleMalformedFrame(line)
		return
	}

	hasID := frame.ID.hasValue()
	if frame.JSONRPC != jsonrpcVersion {
		t.handleInvalidJSONRPCVersion(frame, hasID)
		return
	}

	// Response: has ID, no method, and explicit response fields.
	if hasID && frame.hasResponseFields() {
		if frame.Method != "" {
			t.failPendingWithParseError(frame.ID, "failed to parse server response")
			t.handleMalformedInboundObject()
			return
		}
		id, ok := frame.ID.requestID()
		if !ok || frame.hasMalformedResponseShape() {
			t.handleMalformedResponse(line)
			return
		}
		resp := Response{
			JSONRPC: frame.JSONRPC,
			ID:      id,
			Result:  frame.Result,
			Error:   frame.Error.value,
		}
		t.handleResponse(resp)
		return
	}

	// Request: has both ID and method.
	if hasID && frame.Method != "" {
		id, ok := frame.ID.requestID()
		if !ok {
			t.handleInvalidRequestObject(line)
			return
		}
		req := Request{
			JSONRPC: frame.JSONRPC,
			ID:      id,
			Method:  frame.Method,
			Params:  frame.Params,
		}
		t.enqueueRequest(req)
		return
	}

	// Notification: has method but no ID.
	if frame.Method != "" {
		t.enqueueNotification(frame.toNotification())
		return
	}

	t.handleMalformedInboundObject()
}

func (t *StdioTransport) handleInvalidJSONRPCVersion(frame inboundFrame, hasID bool) {
	// Request with invalid protocol version: reject with JSON-RPC invalid request.
	if hasID && frame.Method != "" {
		t.rejectInvalidProtocolVersion(frame.ID)
		return
	}

	// Response with invalid protocol version: fail matching pending request so
	// callers do not wait for context timeout.
	if hasID && frame.Method == "" && frame.hasResponseFields() {
		t.failPendingWithInvalidProtocolVersion(frame.ID)
	}
}

func (t *StdioTransport) rejectInvalidProtocolVersion(idField inboundID) {
	id := RequestID{Value: nil}
	if parsed, ok := idField.requestID(); ok {
		id = parsed
	}
	if err := t.writeMessage(Response{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error: &Error{
			Code:    ErrCodeInvalidRequest,
			Message: errInvalidJSONRPCVersion,
		},
	}); err != nil {
		t.handleWriteFailure(err)
	}
}

func (t *StdioTransport) failPendingWithInvalidProtocolVersion(idField inboundID) {
	t.failPendingWithError(
		idField,
		func(id RequestID) pendingReqResult {
			return pendingReqResult{
				resp: Response{
					JSONRPC: jsonrpcVersion,
					ID:      id,
					Error: &Error{
						Code:    ErrCodeInvalidRequest,
						Message: errInvalidResponseJSONRPC,
					},
				},
			}
		},
	)
}

func (t *StdioTransport) failPendingWithParseError(idField inboundID, message string) {
	t.failPendingWithError(
		idField,
		func(id RequestID) pendingReqResult {
			return pendingReqResult{
				resp: Response{
					JSONRPC: jsonrpcVersion,
					ID:      id,
					Error: &Error{
						Code:    ErrCodeParseError,
						Message: message,
					},
				},
			}
		},
	)
}

func (t *StdioTransport) failPendingWithError(
	idField inboundID,
	build func(RequestID) pendingReqResult,
) {
	id, ok := idField.requestID()
	if !ok {
		return
	}
	normalizedID, err := normalizePendingRequestID(id.Value)
	if err != nil {
		return
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	pending, ok := t.pendingReqs[normalizedID]
	if ok {
		delete(t.pendingReqs, normalizedID)
	}
	t.mu.Unlock()

	if ok {
		pending.ch <- build(pending.id)
	}
}

// readLimitedLine reads one newline-delimited frame and enforces an upper size
// bound. If a frame exceeds max bytes, it returns the oversized frame prefix so
// callers can best-effort route a matching response before terminating the
// transport.
func readLimitedLine(r *bufio.Reader, limit int) ([]byte, *oversizedFrameInfo, error) {
	var line []byte
	for {
		frag, err := r.ReadSlice('\n')
		line = append(line, frag...)
		if lineExceedsLimit(line, limit) {
			return handleOversizedLine(r, err, line)
		}
		switch {
		case err == nil:
			return bytes.TrimSuffix(line, []byte{'\n'}), nil, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if len(line) == 0 {
				return nil, nil, io.EOF
			}
			return line, nil, nil
		default:
			return nil, nil, err
		}
	}
}

func lineExceedsLimit(line []byte, limit int) bool {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		return len(line)-1 > limit
	}
	return len(line) > limit
}

func handleOversizedLine(reader *bufio.Reader, readErr error, line []byte) ([]byte, *oversizedFrameInfo, error) {
	info := extractOversizedFrameInfo(line, reader)
	switch {
	case readErr == nil:
		return nil, &info, nil
	case errors.Is(readErr, io.EOF):
		return nil, &info, io.EOF
	case !errors.Is(readErr, bufio.ErrBufferFull):
		return nil, &info, readErr
	}
	return nil, &info, nil
}

func (t *StdioTransport) enqueueRequest(req Request) {
	select {
	case <-t.ctx.Done():
		return
	case t.requestQueue <- req:
		return
	default:
		t.rejectRequestForOverload(req)
	}
}

func (t *StdioTransport) enqueueNotification(notif Notification) {
	t.annotateTurnScopedNotification(&notif)
	if isTurnScopedNotification(notif) {
		t.enqueueTurnScopedNotification(notif)
		return
	}
	if isStreamingNotificationMethod(notif.Method) {
		t.enqueueStreamingNotification(notif)
		return
	}
	if isProtectedNotificationMethod(notif.Method) {
		t.enqueueProtectedNotification(notif)
		return
	}
	if isCriticalNotificationMethod(notif.Method) {
		t.enqueueCriticalNotification(notif)
		return
	}

	select {
	case <-t.ctx.Done():
		return
	case t.notifQueue <- notif:
	default:
		// Unknown notifications remain best-effort to preserve read-loop
		// liveness without changing known SDK-visible behavior.
	}
}

func (t *StdioTransport) enqueueTurnScopedNotification(notif Notification) {
	t.initTurnScopedScheduler()

	if notif.threadKey == "" {
		select {
		case <-t.ctx.Done():
			return
		case t.notifQueue <- notif:
		default:
			// Non-attributable notifications remain best-effort.
		}
		return
	}

	t.turnNotifQueuesMu.Lock()
	queue := t.turnNotifQueues[notif.threadKey]
	if queue == nil {
		if len(t.turnNotifQueues) >= maxTurnScopedNotificationQueues {
			t.turnNotifQueuesMu.Unlock()
			t.closeWithFailure(
				errTurnScopedNotificationQueueLimit,
				errTurnScopedNotificationQueueLimit,
			)
			return
		}
		queue = &turnScopedNotificationQueue{threadKey: notif.threadKey}
		t.turnNotifQueues[notif.threadKey] = queue
	}
	t.turnNotifQueuesMu.Unlock()

	queue.mu.Lock()
	if len(queue.queue) >= maxTurnScopedNotificationQueueSize {
		queue.mu.Unlock()
		t.closeWithFailure(
			errTurnScopedNotificationQueueOverflow,
			errTurnScopedNotificationQueueOverflow,
		)
		return
	}
	queue.queue = append(queue.queue, notif)
	if queue.scheduled {
		queue.mu.Unlock()
		return
	}
	queue.scheduled = true
	queue.mu.Unlock()

	if t.ctx.Err() != nil {
		queue.mu.Lock()
		queue.queue = nil
		queue.scheduled = false
		queue.mu.Unlock()
		t.removeTurnScopedNotificationQueue(queue.threadKey, queue)
		return
	}
	t.scheduleTurnScopedNotificationQueue(queue)
}

func (t *StdioTransport) handleTurnScopedNotificationQueue(queue *turnScopedNotificationQueue) {
	for {
		queue.mu.Lock()
		if len(queue.queue) == 0 {
			queue.scheduled = false
			queue.mu.Unlock()
			t.removeTurnScopedNotificationQueue(queue.threadKey, queue)
			return
		}
		notif := queue.queue[0]
		queue.queue[0] = Notification{}
		queue.queue = queue.queue[1:]
		queue.mu.Unlock()

		if t.ctx.Err() != nil {
			queue.mu.Lock()
			queue.queue = nil
			queue.scheduled = false
			queue.mu.Unlock()
			t.removeTurnScopedNotificationQueue(queue.threadKey, queue)
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) removeTurnScopedNotificationQueue(threadKey string, queue *turnScopedNotificationQueue) {
	t.turnNotifQueuesMu.Lock()
	defer t.turnNotifQueuesMu.Unlock()

	current, ok := t.turnNotifQueues[threadKey]
	if !ok || current != queue {
		return
	}

	queue.mu.Lock()
	empty := len(queue.queue) == 0 && !queue.scheduled
	queue.mu.Unlock()
	if empty {
		delete(t.turnNotifQueues, threadKey)
	}
}

func (t *StdioTransport) enqueueLosslessNotification(
	queue chan Notification,
	notif Notification,
) {
	select {
	case <-t.ctx.Done():
		return
	case queue <- notif:
		return
	default:
	}
	t.closeWithFailure(errNotificationQueueOverflow, errNotificationQueueOverflow)
}

func (t *StdioTransport) enqueueStreamingNotification(notif Notification) {
	var startDrainer bool

	t.streamingBacklog.mu.Lock()
	if len(t.streamingBacklog.queue) == 0 && !t.streamingBacklog.draining {
		select {
		case <-t.ctx.Done():
			t.streamingBacklog.mu.Unlock()
			return
		case t.streamingNotifQueue <- notif:
			t.streamingBacklog.mu.Unlock()
			return
		default:
		}
	}
	if len(t.streamingBacklog.queue) >= maxStreamingNotificationBacklog {
		t.streamingBacklog.mu.Unlock()
		t.closeWithFailure(
			errStreamingNotificationBacklogOverflow,
			errStreamingNotificationBacklogOverflow,
		)
		return
	}
	t.streamingBacklog.queue = append(t.streamingBacklog.queue, notif)
	if !t.streamingBacklog.draining {
		t.streamingBacklog.draining = true
		startDrainer = true
	}
	t.streamingBacklog.mu.Unlock()

	if startDrainer {
		go t.flushStreamingNotificationBacklog()
	}
}

func (t *StdioTransport) flushStreamingNotificationBacklog() {
	for {
		notif, ok := t.nextStreamingBacklogNotification()
		if !ok {
			return
		}

		select {
		case <-t.ctx.Done():
			return
		case t.streamingNotifQueue <- notif:
		}
	}
}

func (t *StdioTransport) nextStreamingBacklogNotification() (Notification, bool) {
	t.streamingBacklog.mu.Lock()
	defer t.streamingBacklog.mu.Unlock()

	if len(t.streamingBacklog.queue) == 0 {
		t.streamingBacklog.draining = false
		return Notification{}, false
	}

	notif := t.streamingBacklog.queue[0]
	t.streamingBacklog.queue[0] = Notification{}
	t.streamingBacklog.queue = t.streamingBacklog.queue[1:]
	return notif, true
}

func (t *StdioTransport) enqueueProtectedNotification(notif Notification) {
	t.enqueueLosslessNotification(t.protectedNotifQueue, notif)
}

func (t *StdioTransport) enqueueCriticalNotification(notif Notification) {
	t.enqueueLosslessNotification(t.criticalNotifQueue, notif)
}

func isCriticalNotificationMethod(method string) bool {
	switch method {
	case notifyError, notifyRealtimeError:
		return true
	default:
		return false
	}
}

func isStreamingNotificationMethod(method string) bool {
	switch method {
	case notifyAgentMessageDelta,
		notifyFileChangeOutputDelta,
		notifyPlanDelta,
		notifyReasoningTextDelta,
		notifyReasoningSummaryTextDelta,
		notifyReasoningSummaryPartAdded,
		notifyRealtimeOutputAudioDelta,
		notifyCommandExecutionOutputDelta,
		notifyCommandExecOutputDelta:
		return true
	default:
		return false
	}
}

func isProtectedNotificationMethod(method string) bool {
	switch method {
	case notifyItemStarted,
		notifyThreadStarted,
		notifyThreadClosed,
		notifyThreadArchived,
		notifyThreadUnarchived,
		notifyThreadNameUpdated,
		notifyThreadStatusChanged,
		notifyThreadTokenUsageUpdated,
		notifyTurnStarted,
		notifyTurnPlanUpdated,
		notifyTurnDiffUpdated,
		notifyAccountUpdated,
		notifyAccountLoginCompleted,
		notifyAccountRateLimitsUpdated,
		notifyRealtimeStarted,
		notifyRealtimeClosed,
		notifyRealtimeItemAdded,
		notifyWindowsSandboxSetupCompleted,
		notifyWindowsWorldWritableWarning,
		notifyThreadCompacted,
		notifyDeprecationNotice,
		notifyTerminalInteraction,
		notifyMcpServerOauthLoginCompleted,
		notifyMcpToolCallProgress,
		notifyServerRequestResolved,
		notifyModelRerouted,
		notifyFuzzyFileSearchSessionCompleted,
		notifyFuzzyFileSearchSessionUpdated,
		notifyAppListUpdated,
		notifyConfigWarning,
		notifySkillsChanged,
		notifyHookStarted,
		notifyHookCompleted,
		notifyItemGuardianApprovalReviewStarted,
		notifyItemGuardianApprovalReviewCompleted:
		return true
	default:
		return false
	}
}

func pendingRequestTransportError(op string, cause error) error {
	if cause == nil {
		return NewTransportError(op, errTransportClosed)
	}

	var transportErr *TransportError
	if errors.As(cause, &transportErr) {
		if unwrapped := transportErr.Unwrap(); unwrapped != nil {
			return NewTransportError(op, unwrapped)
		}
	}

	return NewTransportError(op, cause)
}

func isTurnScopedNotification(notif Notification) bool {
	switch notif.Method {
	case notifyItemCompleted, notifyTurnCompleted:
		return notif.threadKey != ""
	default:
		return false
	}
}

func (t *StdioTransport) annotateTurnScopedNotification(notif *Notification) {
	switch notif.Method {
	case notifyItemCompleted:
		notif.threadKey = itemCompletedThreadKey(notif.Params)
	case notifyTurnCompleted:
		notif.threadKey = turnCompletedThreadKey(notif.Params)
	}
}

func (t *StdioTransport) rejectRequestForOverload(req Request) {
	if err := t.writeMessage(Response{
		JSONRPC: jsonrpcVersion,
		ID:      req.ID,
		Error: &Error{
			Code:    ErrCodeInternalError,
			Message: "too many pending inbound requests",
		},
	}); err != nil {
		t.handleWriteFailure(err)
	}
}

func decodeInboundFrame(data []byte) (inboundFrame, error) {
	var frame inboundFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		return inboundFrame{}, err
	}
	return frame, nil
}

func parseRequestID(data json.RawMessage) (RequestID, error) {
	if len(data) == 0 {
		return RequestID{}, errors.New("missing id")
	}
	var id RequestID
	if err := json.Unmarshal(data, &id); err != nil {
		return RequestID{}, err
	}
	return id, nil
}

func (f inboundFrame) toNotification() Notification {
	return Notification{
		JSONRPC: f.JSONRPC,
		Method:  f.Method,
		Params:  f.Params,
	}
}

// handleResponse routes an incoming response to the pending request channel.
// It claims the channel under the lock via delete, then sends outside the
// lock. The delete-then-unlock-then-send pattern ensures exclusive access
// to the channel without holding the mutex during the send.
func (t *StdioTransport) handleResponse(resp Response) {
	// Normalize ID for matching
	normalizedID, err := normalizePendingRequestID(resp.ID.Value)
	if err != nil {
		return
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	pending, ok := t.pendingReqs[normalizedID]
	if ok {
		delete(t.pendingReqs, normalizedID)
	}
	t.mu.Unlock()

	if ok {
		pending.ch <- pendingReqResult{resp: resp} // safe: buffer 1, only one sender claims via delete
	}
}

func (t *StdioTransport) cleanupPendingReq(normalizedID string, pending pendingReq) {
	t.mu.Lock()
	defer t.mu.Unlock()

	current, ok := t.pendingReqs[normalizedID]
	if !ok {
		return
	}
	if current.ch != pending.ch {
		return
	}
	delete(t.pendingReqs, normalizedID)
}

func (t *StdioTransport) handleMalformedFrame(data []byte) {
	t.malformedCount.Add(1)

	id, hasID, hasMethod := extractTopLevelIDAndMethod(data)
	if !hasID || hasMethod {
		return
	}
	normalizedID, err := normalizePendingRequestID(id.Value)
	if err != nil {
		return
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	pending, ok := t.pendingReqs[normalizedID]
	if ok {
		delete(t.pendingReqs, normalizedID)
	}
	t.mu.Unlock()

	if ok {
		pending.ch <- pendingReqResult{
			resp: Response{
				JSONRPC: jsonrpcVersion,
				ID:      pending.id,
				Error: &Error{
					Code:    ErrCodeParseError,
					Message: "failed to parse server response",
				},
			},
		}
	}
}

func (t *StdioTransport) handleMalformedInboundObject() {
	t.malformedCount.Add(1)
}

// handleMalformedResponse attempts to extract the ID from a response that
// failed full unmarshal, and sends a parse error to the pending caller.
func (t *StdioTransport) handleMalformedResponse(data []byte) {
	t.malformedCount.Add(1)

	var partial struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal(data, &partial) != nil || len(partial.ID) == 0 {
		return
	}

	id, err := parseRequestID(partial.ID)
	if err != nil {
		return
	}
	normalizedID, err := normalizePendingRequestID(id.Value)
	if err != nil {
		return
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	pending, ok := t.pendingReqs[normalizedID]
	if ok {
		delete(t.pendingReqs, normalizedID)
	}
	t.mu.Unlock()

	if ok {
		pending.ch <- pendingReqResult{
			resp: Response{
				JSONRPC: jsonrpcVersion,
				ID:      pending.id,
				Error: &Error{
					Code:    ErrCodeParseError,
					Message: "failed to parse server response",
				},
			},
		}
	}
}

func (t *StdioTransport) handleOversizedFrame(info oversizedFrameInfo) {
	if !info.hasResponseFields || info.hasMethod || !info.hasID {
		return
	}

	normalizedID, err := normalizePendingRequestID(info.id.Value)
	if err != nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	pending, ok := t.pendingReqs[normalizedID]
	if !ok {
		return
	}
	delete(t.pendingReqs, normalizedID)
	pending.ch <- pendingReqResult{
		resp: Response{
			JSONRPC: jsonrpcVersion,
			ID:      pending.id,
			Error: &Error{
				Code:    ErrCodeParseError,
				Message: "oversized server response frame",
			},
		},
	}
}

func extractTopLevelIDAndMethod(data []byte) (RequestID, bool, bool) {
	id, hasID, hasMethod, _ := extractTopLevelIDAndMethodFromReader(bytes.NewReader(data))
	return id, hasID, hasMethod
}

func extractTopLevelIDAndMethodFromReader(reader io.Reader) (RequestID, bool, bool, error) {
	var id RequestID
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()

	start, err := decoder.Token()
	if err != nil {
		return id, false, false, err
	}
	delim, ok := start.(json.Delim)
	if !ok || delim != '{' {
		return id, false, false, nil
	}

	var hasID bool
	var hasMethod bool
	for decoder.More() {
		keyTok, err := decoder.Token()
		if err != nil {
			return id, hasID, hasMethod, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return id, hasID, hasMethod, nil
		}

		valueTok, err := decoder.Token()
		if err != nil {
			return id, hasID, hasMethod, err
		}

		switch key {
		case "id":
			switch v := valueTok.(type) {
			case string:
				id = RequestID{Value: v}
				hasID = true
			case json.Number:
				id = RequestID{Value: v}
				hasID = true
			case float64:
				id = RequestID{Value: v}
				hasID = true
			}
		case "method":
			if _, ok := valueTok.(string); ok {
				hasMethod = true
			}
		}

		if valueDelim, ok := valueTok.(json.Delim); ok && (valueDelim == '{' || valueDelim == '[') {
			if err := consumeNestedJSONValue(decoder); err != nil {
				return id, hasID, hasMethod, err
			}
		}
	}

	return id, hasID, hasMethod, nil
}

func extractOversizedFrameInfo(prefix []byte, reader *bufio.Reader) oversizedFrameInfo {
	info := inspectOversizedFramePrefix(prefix)
	if info.hasMethod || (info.hasResponseFields && info.hasID) {
		return info
	}
	if reader == nil || bytes.HasSuffix(prefix, []byte{'\n'}) {
		return info
	}

	scanner := newOversizedFrameScanner(prefix, reader)
	return scanner.scan()
}

func inspectOversizedFramePrefix(data []byte) oversizedFrameInfo {
	var info oversizedFrameInfo

	i := skipJSONWhitespace(data, 0)
	if i >= len(data) || data[i] != '{' {
		return info
	}
	i++

	for i < len(data) {
		i = skipJSONWhitespace(data, i)
		if i >= len(data) {
			return info
		}
		switch data[i] {
		case ',':
			i++
			continue
		case '}':
			return info
		default:
			if data[i] != '"' {
				return info
			}
		}

		key, next, ok := consumeJSONString(data, i)
		if !ok {
			return info
		}
		i = skipJSONWhitespace(data, next)
		if i >= len(data) || data[i] != ':' {
			return info
		}
		i = skipJSONWhitespace(data, i+1)
		if i >= len(data) {
			return info
		}

		switch key {
		case "id":
			id, valueEnd, ok := consumeRequestIDValue(data, i)
			if !ok {
				return info
			}
			info.id = id
			info.hasID = id.Value != nil
			i = valueEnd
		case "method":
			_, _, ok := consumeJSONString(data, i)
			if !ok {
				return info
			}
			info.hasMethod = true
			return info
		case "result", "error":
			info.hasResponseFields = true
			return info
		default:
			valueEnd, ok := consumeJSONValue(data, i)
			if !ok {
				return info
			}
			i = valueEnd
		}
	}

	return info
}

type oversizedFrameScanner struct {
	reader *bufio.Reader
}

func newOversizedFrameScanner(prefix []byte, reader *bufio.Reader) oversizedFrameScanner {
	source := io.Reader(bytes.NewReader(prefix))
	if reader != nil && !bytes.HasSuffix(prefix, []byte{'\n'}) {
		source = io.MultiReader(bytes.NewReader(prefix), &newlineTerminatedReader{reader: reader})
	}
	return oversizedFrameScanner{reader: bufio.NewReader(source)}
}

func (s *oversizedFrameScanner) scan() oversizedFrameInfo {
	var info oversizedFrameInfo

	start, ok := s.nextNonWhitespaceByte()
	if !ok || start != '{' {
		return info
	}

	for {
		next, ok := s.nextNonWhitespaceByte()
		if !ok {
			return info
		}
		switch next {
		case ',':
			continue
		case '}':
			return info
		default:
			if next != '"' {
				return info
			}
		}

		key, ok := s.readJSONString()
		if !ok || !s.consumeColon() {
			return info
		}

		switch key {
		case "id":
			id, hasID, ok := s.readRequestID()
			if !ok {
				return info
			}
			info.id = id
			info.hasID = hasID
		case "method":
			if !s.skipJSONValue() {
				return info
			}
			info.hasMethod = true
		case "result", "error":
			info.hasResponseFields = true
			if !s.skipJSONValue() {
				return info
			}
		default:
			if !s.skipJSONValue() {
				return info
			}
		}
	}
}

type newlineTerminatedReader struct {
	reader *bufio.Reader
	done   bool
}

func (r *newlineTerminatedReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}

	n := 0
	for n < len(p) {
		b, err := r.reader.ReadByte()
		if err != nil {
			r.done = true
			if n > 0 {
				return n, nil
			}
			return 0, err
		}
		p[n] = b
		n++
		if b == '\n' {
			r.done = true
			return n, nil
		}
	}
	return n, nil
}

func (s *oversizedFrameScanner) nextNonWhitespaceByte() (byte, bool) {
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			return 0, false
		}
		switch b {
		case ' ', '\n', '\r', '\t':
			continue
		default:
			return b, true
		}
	}
}

func (s *oversizedFrameScanner) consumeColon() bool {
	b, ok := s.nextNonWhitespaceByte()
	return ok && b == ':'
}

func (s *oversizedFrameScanner) readJSONString() (string, bool) {
	raw, ok := s.readRawJSONString('"')
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func (s *oversizedFrameScanner) readRawJSONString(opening byte) ([]byte, bool) {
	if opening != '"' {
		return nil, false
	}

	var raw bytes.Buffer
	raw.WriteByte('"')
	escaped := false
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			return nil, false
		}
		raw.WriteByte(b)
		if escaped {
			escaped = false
			continue
		}
		switch b {
		case '\\':
			escaped = true
		case '"':
			return raw.Bytes(), true
		}
	}
}

func (s *oversizedFrameScanner) skipJSONValue() bool {
	first, ok := s.nextNonWhitespaceByte()
	if !ok {
		return false
	}

	switch first {
	case '"':
		return s.skipJSONStringBody()
	case '{':
		return s.skipJSONObjectBody()
	case '[':
		return s.skipJSONArrayBody()
	default:
		return s.skipJSONScalar(first)
	}
}

func (s *oversizedFrameScanner) skipJSONStringBody() bool {
	for {
		segment, err := s.reader.ReadSlice('"')
		switch {
		case err == nil:
			if stringTerminatorUnescaped(segment) {
				return true
			}
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		default:
			return false
		}
	}
}

func stringTerminatorUnescaped(segment []byte) bool {
	backslashes := 0
	for i := len(segment) - 2; i >= 0 && segment[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 0
}

func (s *oversizedFrameScanner) skipJSONObjectBody() bool {
	return s.skipCompositeJSONValue('}')
}

func (s *oversizedFrameScanner) skipJSONArrayBody() bool {
	return s.skipCompositeJSONValue(']')
}

func (s *oversizedFrameScanner) skipCompositeJSONValue(closing byte) bool {
	stack := []byte{closing}
	for len(stack) > 0 {
		b, err := s.reader.ReadByte()
		if err != nil {
			return false
		}
		switch b {
		case '"':
			if !s.skipJSONStringBody() {
				return false
			}
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if b != stack[len(stack)-1] {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}
	return true
}

func (s *oversizedFrameScanner) skipJSONScalar(first byte) bool {
	if isJSONScalarTerminator(first) {
		return false
	}
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			return errors.Is(err, io.EOF)
		}
		if isJSONScalarTerminator(b) {
			if err := s.reader.UnreadByte(); err != nil {
				return false
			}
			return true
		}
	}
}

func (s *oversizedFrameScanner) readRequestID() (RequestID, bool, bool) {
	first, ok := s.nextNonWhitespaceByte()
	if !ok {
		return RequestID{}, false, false
	}

	var raw []byte
	switch first {
	case '"':
		var ok bool
		raw, ok = s.readRawJSONString(first)
		if !ok {
			return RequestID{}, false, false
		}
	case '{', '[':
		return RequestID{}, false, false
	default:
		var ok bool
		raw, ok = s.readJSONScalar(first)
		if !ok {
			return RequestID{}, false, false
		}
	}

	var id RequestID
	if err := json.Unmarshal(raw, &id); err != nil {
		return RequestID{}, false, false
	}
	return id, id.Value != nil, true
}

func (s *oversizedFrameScanner) readJSONScalar(first byte) ([]byte, bool) {
	if isJSONScalarTerminator(first) {
		return nil, false
	}

	var raw bytes.Buffer
	raw.WriteByte(first)
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return raw.Bytes(), true
			}
			return nil, false
		}
		if isJSONScalarTerminator(b) {
			if err := s.reader.UnreadByte(); err != nil {
				return nil, false
			}
			return raw.Bytes(), true
		}
		raw.WriteByte(b)
	}
}

func isJSONScalarTerminator(b byte) bool {
	switch b {
	case ',', '}', ']', ' ', '\n', '\r', '\t':
		return true
	default:
		return false
	}
}

func skipJSONWhitespace(data []byte, start int) int {
	for start < len(data) {
		switch data[start] {
		case ' ', '\n', '\r', '\t':
			start++
		default:
			return start
		}
	}
	return start
}

func consumeJSONString(data []byte, start int) (string, int, bool) {
	if start >= len(data) || data[start] != '"' {
		return "", start, false
	}

	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			raw := data[start : i+1]
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				return "", start, false
			}
			return value, i + 1, true
		}
	}

	return "", start, false
}

func consumeRequestIDValue(data []byte, start int) (RequestID, int, bool) {
	if start >= len(data) {
		return RequestID{}, start, false
	}

	switch data[start] {
	case '"':
		_, end, ok := consumeJSONString(data, start)
		if !ok {
			return RequestID{}, start, false
		}
		var id RequestID
		if err := json.Unmarshal(data[start:end], &id); err != nil {
			return RequestID{}, start, false
		}
		return id, end, true
	case '{', '[':
		return RequestID{}, start, false
	default:
		end, ok := consumeJSONScalar(data, start)
		if !ok {
			return RequestID{}, start, false
		}
		var id RequestID
		if err := json.Unmarshal(data[start:end], &id); err != nil {
			return RequestID{}, start, false
		}
		return id, end, true
	}
}

func consumeJSONValue(data []byte, start int) (int, bool) {
	if start >= len(data) {
		return start, false
	}

	switch data[start] {
	case '"':
		_, end, ok := consumeJSONString(data, start)
		return end, ok
	case '{', '[':
		return consumeCompositeJSONValue(data, start)
	default:
		return consumeJSONScalar(data, start)
	}
}

func consumeCompositeJSONValue(data []byte, start int) (int, bool) {
	var stack []byte
	i := start

	for i < len(data) {
		switch data[i] {
		case '"':
			_, next, ok := consumeJSONString(data, i)
			if !ok {
				return start, false
			}
			i = next
			continue
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) == 0 || data[i] != stack[len(stack)-1] {
				return start, false
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1, true
			}
		}
		i++
	}

	return start, false
}

func consumeJSONScalar(data []byte, start int) (int, bool) {
	i := start
	for i < len(data) {
		switch data[i] {
		case ',', '}', ']', ' ', '\n', '\r', '\t':
			end := i
			i = skipJSONWhitespace(data, i)
			if end > start && json.Valid(data[start:end]) {
				return i, true
			}
			return start, false
		default:
			i++
		}
	}

	if json.Valid(data[start:i]) {
		return i, true
	}
	return start, false
}

func (t *StdioTransport) stopAfterReadFailure(scanErr error) {
	t.stopAfterReaderTermination(scanErr)
}

func (t *StdioTransport) stopAfterReaderEOF() {
	t.stopAfterReaderTermination(nil)
}

func (t *StdioTransport) stopAfterReaderTermination(scanErr error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	if t.scanErr == nil {
		t.scanErr = scanErr
	}
	t.closed = true
	t.readerEOF = true
	t.pendingReqs = make(map[string]pendingReq)
	cancel := t.cancelCtx
	readerCloser := t.readerCloser
	t.mu.Unlock()

	cancel()
	t.wakeTurnScopedNotificationWorkers()
	if readerCloser != nil {
		_ = readerCloser.Close()
	}
	go t.drainPendingNotificationsAfterStop()
}

func (t *StdioTransport) drainPendingNotificationsAfterStop() {
	for {
		drained := false

		drained = t.drainNotificationQueue(t.criticalNotifQueue) || drained
		drained = t.drainNotificationQueue(t.protectedNotifQueue) || drained
		drained = t.drainNotificationQueue(t.streamingNotifQueue) || drained
		drained = t.drainStreamingNotificationBacklog() || drained
		drained = t.drainNotificationQueue(t.notifQueue) || drained
		drained = t.drainTurnScopedNotificationQueues() || drained

		if !drained {
			return
		}
	}
}

func (t *StdioTransport) drainNotificationQueue(queue chan Notification) bool {
	drained := false
	for {
		select {
		case notif := <-queue:
			t.handleNotification(notif)
			drained = true
		default:
			return drained
		}
	}
}

func (t *StdioTransport) drainStreamingNotificationBacklog() bool {
	t.streamingBacklog.mu.Lock()
	if len(t.streamingBacklog.queue) == 0 {
		t.streamingBacklog.draining = false
		t.streamingBacklog.mu.Unlock()
		return false
	}
	queue := append([]Notification(nil), t.streamingBacklog.queue...)
	t.streamingBacklog.queue = nil
	t.streamingBacklog.draining = false
	t.streamingBacklog.mu.Unlock()

	for _, notif := range queue {
		t.handleNotification(notif)
	}
	return true
}

func (t *StdioTransport) drainTurnScopedNotificationQueues() bool {
	t.turnNotifQueuesMu.Lock()
	queues := make([]*turnScopedNotificationQueue, 0, len(t.turnNotifQueues))
	for _, queue := range t.turnNotifQueues {
		queues = append(queues, queue)
	}
	t.turnNotifQueuesMu.Unlock()

	drained := false
	for _, queue := range queues {
		for {
			queue.mu.Lock()
			if len(queue.queue) == 0 {
				queue.scheduled = false
				queue.mu.Unlock()
				t.removeTurnScopedNotificationQueue(queue.threadKey, queue)
				break
			}
			notif := queue.queue[0]
			queue.queue[0] = Notification{}
			queue.queue = queue.queue[1:]
			queue.mu.Unlock()

			t.handleNotification(notif)
			drained = true
		}
	}

	return drained
}

func consumeNestedJSONValue(decoder *json.Decoder) error {
	depth := 1
	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return err
		}
		d, ok := tok.(json.Delim)
		if !ok {
			continue
		}
		switch d {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
	}
	return nil
}

// handleRequest dispatches an incoming server→client request to the handler
func (t *StdioTransport) handleRequest(req Request) {
	handler, panicFn, queued := t.resolveRequestHandler(req)
	if handler == nil {
		if queued {
			return
		}
		errorResp := Response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error: &Error{
				Code:    ErrCodeMethodNotFound,
				Message: "method not found",
			},
		}
		if err := t.writeMessage(errorResp); err != nil {
			t.handleWriteFailure(err)
		}
		return
	}

	defer func() {
		if r := recover(); r != nil {
			errorResp := Response{
				JSONRPC: jsonrpcVersion,
				ID:      req.ID,
				Error: &Error{
					Code:    ErrCodeInternalError,
					Message: "internal handler error",
				},
			}
			if err := t.writeMessage(errorResp); err != nil {
				t.handleWriteFailure(err)
			}
			if panicFn != nil {
				panicFn(r)
			}
		}
	}()

	resp, err := handler(t.ctx, req)
	if err != nil {
		// Handler returned error - use generic message to avoid leaking
		// internal details across the trust boundary
		code := ErrCodeInternalError
		msg := "internal handler error"
		if errors.Is(err, errInvalidParams) {
			code = ErrCodeInvalidParams
			msg = "invalid params"
		}
		errorResp := Response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error: &Error{
				Code:    code,
				Message: msg,
			},
		}
		if err := t.writeMessage(errorResp); err != nil {
			t.handleWriteFailure(err)
		}
		return
	}

	// Ensure response has correct ID and version
	resp.JSONRPC = jsonrpcVersion
	resp.ID = req.ID
	if err := t.writeMessage(resp); err != nil {
		t.handleWriteFailure(err)
	}
}

func (t *StdioTransport) resolveRequestHandler(req Request) (RequestHandler, func(any), bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.reqHandler != nil {
		return t.reqHandler, t.panicHandler, false
	}
	if len(t.pendingReqHandler) >= inboundRequestQueueSize {
		return nil, nil, false
	}
	t.pendingReqHandler = append(t.pendingReqHandler, req)
	return nil, nil, true
}

// handleInvalidRequestObject sends an invalid-request response for a structurally
// invalid inbound request object. It returns true when the frame looked like a
// request (had a top-level method field), in which case the caller should stop
// further processing.
func (t *StdioTransport) handleInvalidRequestObject(data []byte) bool {
	id, hasValidID, isRequest := extractInboundRequestObjectID(data)
	if !isRequest {
		return false
	}

	if !hasValidID {
		id = RequestID{Value: nil}
	}

	errorResp := Response{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error: &Error{
			Code:    ErrCodeInvalidRequest,
			Message: "invalid server request",
		},
	}
	if err := t.writeMessage(errorResp); err != nil {
		t.handleWriteFailure(err)
	}
	return true
}

func extractInboundRequestObjectID(data []byte) (RequestID, bool, bool) {
	var topLevel map[string]json.RawMessage
	if json.Unmarshal(data, &topLevel) != nil {
		return RequestID{}, false, false
	}

	if _, hasMethod := topLevel["method"]; !hasMethod {
		return RequestID{}, false, false
	}

	rawID, hasID := topLevel["id"]
	if !hasID {
		return RequestID{}, false, true
	}

	id, err := parseRequestID(rawID)
	if err != nil {
		return RequestID{}, false, true
	}
	if _, err := normalizeID(id.Value); err != nil {
		return RequestID{}, false, true
	}
	return id, true, true
}

// handleNotification dispatches an incoming server→client notification to the handler
func (t *StdioTransport) handleNotification(notif Notification) {
	t.mu.Lock()
	handler := t.notifHandler
	panicFn := t.panicHandler
	t.mu.Unlock()

	if handler == nil {
		t.mu.Lock()
		if t.notifHandler == nil {
			if len(t.pendingNotifHandle) >= inboundNotifQueueSize {
				t.pendingNotifHandle = append(t.pendingNotifHandle[1:], notif)
			} else {
				t.pendingNotifHandle = append(t.pendingNotifHandle, notif)
			}
			t.mu.Unlock()
			return
		}
		handler = t.notifHandler
		panicFn = t.panicHandler
		t.mu.Unlock()
	}

	defer func() {
		if r := recover(); r != nil {
			if panicFn != nil {
				panicFn(r)
			}
		}
	}()
	handler(t.ctx, notif)
}
