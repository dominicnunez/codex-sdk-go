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
	ch chan Response
	id RequestID
}

// inbound/outbound queue sizing. These are intentionally conservative defaults:
// large enough for normal bursts, bounded to prevent untrusted-peer DoS via
// unbounded goroutine or memory growth.
const (
	inboundRequestWorkers       = 8
	inboundRequestQueueSize     = 64
	inboundNotificationWorkers  = 8
	criticalNotificationWorkers = 2
	terminalNotificationWorkers = 2
	inboundNotifQueueSize       = 128
	criticalNotifQueueSize      = 64
	terminalNotifQueueSize      = 64
	outboundWriteQueueSize      = 256
	readBufferSizeBytes         = 64 * 1024
	maxInboundMessageSizeBytes  = 10 * 1024 * 1024
	defaultSendTimeout          = 5 * time.Minute
)

const (
	errNilTransportReader     = "stdio transport reader must not be nil"
	errNilTransportWriter     = "stdio transport writer must not be nil"
	errInvalidJSONRPCVersion  = `invalid request: jsonrpc must be "2.0"`
	errInvalidResponseJSONRPC = `invalid response: jsonrpc must be "2.0"`
	requestIDKeyPrefixNumber  = "n:"
	requestIDKeyPrefixString  = "s:"
	transportClosedErrorData  = `{"transport":"closed","origin":"client"}`
)

type writeEnvelope struct {
	payload []byte
	done    chan error
}

type inboundFrame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      inboundID       `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   inboundError    `json:"error,omitempty"`
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
	id        RequestID
	hasID     bool
	hasMethod bool
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
		return nil //nolint:nilerr // Preserve frame routing; invalid error payload is handled as malformed response.
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

	mu                 sync.Mutex
	closed             bool
	pendingReqs        map[string]pendingReq
	reqHandler         RequestHandler
	notifHandler       NotificationHandler
	pendingReqHandler  []Request
	pendingNotifHandle []Notification
	requestQueue       chan Request
	criticalNotifQueue chan Notification
	terminalNotifQueue chan Notification
	notifQueue         chan Notification
	writeQueue         chan writeEnvelope
	readerStopped      chan struct{}
	once               sync.Once
	startReadLoopOnce  sync.Once
	scanErr            error // set by readLoop when an unrecoverable read error occurs
	malformedCount     atomic.Uint64
	panicHandler       func(v any)
	ctx                context.Context
	cancelCtx          context.CancelFunc
}

// errUnexpectedIDType is returned when normalizeID encounters an ID value
// that is not a supported JSON-RPC ID type (string, number).
var errUnexpectedIDType = errors.New("unexpected ID type")
var errTransportClosed = errors.New("transport closed")

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
		reader:             reader,
		readerCloser:       reader,
		writer:             writer,
		pendingReqs:        make(map[string]pendingReq),
		requestQueue:       make(chan Request, inboundRequestQueueSize),
		criticalNotifQueue: make(chan Notification, criticalNotifQueueSize),
		terminalNotifQueue: make(chan Notification, terminalNotifQueueSize),
		notifQueue:         make(chan Notification, inboundNotifQueueSize),
		writeQueue:         make(chan writeEnvelope, outboundWriteQueueSize),
		readerStopped:      make(chan struct{}),
		ctx:                ctx,
		cancelCtx:          cancel,
	}
	for range inboundRequestWorkers {
		go t.requestWorker()
	}
	for range inboundNotificationWorkers {
		go t.notificationWorker()
	}
	for range criticalNotificationWorkers {
		go t.criticalNotificationWorker()
	}
	for range terminalNotificationWorkers {
		go t.terminalNotificationWorker()
	}
	go t.writeLoop()
	t.ensureReadLoopStarted()
	return t
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
		return Response{}, NewTransportError("send failed", errTransportClosed)
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
	respChan := make(chan Response, 1)
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
	case resp := <-respChan:
		return resp, nil
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case <-t.readerStopped:
		// Prefer a response already delivered to this request over the generic
		// reader-stopped error; both can become ready at nearly the same time.
		select {
		case resp := <-respChan:
			return resp, nil
		default:
		}
		return Response{}, NewTransportError("send failed", errors.New("transport reader stopped"))
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
		return NewTransportError("notify failed", errTransportClosed)
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

// OnPanic registers a handler called when a notification handler panics.
// The transport recovers from the panic and continues operating; this
// callback provides observability into the recovered value.
func (t *StdioTransport) OnPanic(handler func(v any)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.panicHandler = handler
}

// Close shuts down the transport. Safe to call multiple times.
func (t *StdioTransport) Close() error {
	t.closeWithFailure(nil, errTransportClosed.Error(), json.RawMessage(transportClosedErrorData))
	return nil
}

// ScanErr returns the terminal transport I/O error, if any.
// Returns nil if the reader stopped due to EOF or hasn't stopped yet.
func (t *StdioTransport) ScanErr() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.scanErr
}

// MalformedMessageCount reports how many inbound response frames could not be
// routed because they were invalid JSON or carried an invalid/unusable ID.
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

func (t *StdioTransport) closeWithFailure(scanErr error, message string, data json.RawMessage) {
	t.mu.Lock()
	if scanErr != nil && t.scanErr == nil {
		t.scanErr = scanErr
	}
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	pending := t.pendingReqs
	t.pendingReqs = make(map[string]pendingReq)
	cancel := t.cancelCtx
	readerCloser := t.readerCloser
	t.mu.Unlock()

	cancel()
	t.once.Do(func() { close(t.readerStopped) })
	if readerCloser != nil {
		_ = readerCloser.Close()
	}

	for _, pendingReq := range pending {
		resp := Response{
			JSONRPC: jsonrpcVersion,
			ID:      pendingReq.id,
			Error: &Error{
				Code:    ErrCodeInternalError,
				Message: message,
				Data:    data,
			},
		}
		select {
		case pendingReq.ch <- resp:
		default:
		}
	}
}

func (t *StdioTransport) handleWriteFailure(err error) {
	if err == nil {
		return
	}
	t.closeWithFailure(err, "transport writer failed", nil)
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
			return NewTransportError(op, errTransportClosed)
		case <-t.readerStopped:
			return NewTransportError(op, errors.New("transport reader stopped"))
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
			if err != nil {
				return err
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-t.ctx.Done():
			return NewTransportError(op, errTransportClosed)
		case <-t.readerStopped:
			return NewTransportError(op, errors.New("transport reader stopped"))
		}
	}

	select {
	case err := <-env.done:
		if err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.ctx.Done():
		return NewTransportError(op, errTransportClosed)
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
		env.done <- err
		if err != nil {
			t.handleWriteFailure(err)
			return
		}
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

func (t *StdioTransport) criticalNotificationWorker() {
	for {
		notif, ok := recvWhileRunning(t.ctx, t.criticalNotifQueue)
		if !ok {
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) terminalNotificationWorker() {
	for {
		notif, ok := recvWhileRunning(t.ctx, t.terminalNotifQueue)
		if !ok {
			return
		}
		t.handleNotification(notif)
	}
}

// readLoop continuously reads newline-delimited JSON messages from the reader
func (t *StdioTransport) readLoop() {
	defer t.once.Do(func() { close(t.readerStopped) })

	reader := bufio.NewReaderSize(t.reader, readBufferSizeBytes)
	for {
		line, oversize, err := readLimitedLine(reader, maxInboundMessageSizeBytes)
		if oversize != nil {
			// Best-effort: fail matching pending responses so Send callers do not
			// block waiting on a frame we intentionally discarded.
			t.handleOversizedFrame(*oversize)
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				t.mu.Lock()
				if t.scanErr == nil {
					t.scanErr = err
				}
				t.mu.Unlock()
				return
			}
			continue
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			t.mu.Lock()
			if t.scanErr == nil {
				t.scanErr = err
			}
			t.mu.Unlock()
			return
		}

		t.processInboundLine(line)
	}
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

	// Response: has ID but no method.
	if hasID && frame.Method == "" {
		id, ok := frame.ID.requestID()
		if !ok || frame.Error.invalid {
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
	}
}

func (t *StdioTransport) handleInvalidJSONRPCVersion(frame inboundFrame, hasID bool) {
	// Request with invalid protocol version: reject with JSON-RPC invalid request.
	if hasID && frame.Method != "" {
		t.rejectInvalidProtocolVersion(frame.ID)
		return
	}

	// Response with invalid protocol version: fail matching pending request so
	// callers do not wait for context timeout.
	if hasID && frame.Method == "" {
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
		pending.ch <- Response{
			JSONRPC: jsonrpcVersion,
			ID:      pending.id,
			Error: &Error{
				Code:    ErrCodeInvalidRequest,
				Message: errInvalidResponseJSONRPC,
			},
		}
	}
}

// readLimitedLine reads one newline-delimited frame and enforces an upper size
// bound. If a frame exceeds max bytes, it is fully discarded and overLimit=true
// is returned so callers can skip just that message and continue.
func readLimitedLine(r *bufio.Reader, limit int) ([]byte, *oversizedFrameInfo, error) {
	var line []byte
	total := 0

	for {
		frag, err := r.ReadSlice('\n')
		total += len(frag)
		line = append(line, frag...)
		if total > limit {
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

func handleOversizedLine(r *bufio.Reader, readErr error, line []byte) ([]byte, *oversizedFrameInfo, error) {
	info := extractOversizedFrameInfo(line, r, readErr)
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
	if isTerminalNotificationMethod(notif.Method) {
		t.enqueueTerminalNotification(notif)
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
		// Notifications are fire-and-forget. If the queue is full we drop to
		// preserve process liveness and bound memory use under abuse.
	}
}

func (t *StdioTransport) enqueueTerminalNotification(notif Notification) {
	t.enqueueBoundedNotification(t.terminalNotifQueue, notif)
}

func (t *StdioTransport) enqueueBoundedNotification(queue chan Notification, notif Notification) {
	select {
	case <-t.ctx.Done():
		return
	case queue <- notif:
		return
	default:
	}

	// Queue is full: drop oldest and enqueue newest to preserve read-loop
	// liveness under sustained notification pressure.
	select {
	case <-queue:
	default:
	}

	select {
	case <-t.ctx.Done():
	case queue <- notif:
	default:
	}
}

func (t *StdioTransport) enqueueCriticalNotification(notif Notification) {
	t.enqueueBoundedNotification(t.criticalNotifQueue, notif)
}

func isCriticalNotificationMethod(method string) bool {
	switch method {
	case notifyItemCompleted, notifyError, notifyRealtimeError:
		return true
	default:
		return false
	}
}

func isTerminalNotificationMethod(method string) bool {
	return method == notifyTurnCompleted
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
		pending.ch <- resp // safe: buffer 1, only one sender claims via delete
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
		pending.ch <- Response{
			JSONRPC: jsonrpcVersion,
			ID:      pending.id,
			Error: &Error{
				Code:    ErrCodeParseError,
				Message: "failed to parse server response",
			},
		}
	}
}

// handleMalformedResponse attempts to extract the ID from a response that
// failed full unmarshal, and sends a parse error to the pending caller.
func (t *StdioTransport) handleMalformedResponse(data []byte) {
	var partial struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal(data, &partial) != nil || len(partial.ID) == 0 {
		return
	}

	id, err := parseRequestID(partial.ID)
	if err != nil {
		t.malformedCount.Add(1)
		return
	}
	normalizedID, err := normalizePendingRequestID(id.Value)
	if err != nil {
		t.malformedCount.Add(1)
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
		pending.ch <- Response{
			JSONRPC: jsonrpcVersion,
			ID:      pending.id,
			Error: &Error{
				Code:    ErrCodeParseError,
				Message: "failed to parse server response",
			},
		}
	}
}

func (t *StdioTransport) handleOversizedFrame(info oversizedFrameInfo) {
	if info.hasMethod || !info.hasID {
		return
	}

	normalizedID, err := normalizePendingRequestID(info.id.Value)
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
		pending.ch <- Response{
			JSONRPC: jsonrpcVersion,
			ID:      pending.id,
			Error: &Error{
				Code:    ErrCodeParseError,
				Message: "oversized server response frame",
			},
		}
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

func extractOversizedFrameInfo(line []byte, r *bufio.Reader, readErr error) oversizedFrameInfo {
	stream := newOversizedFrameReader(line, r, readErr)
	id, hasID, hasMethod, _ := extractTopLevelIDAndMethodFromReader(stream)
	_, _ = io.Copy(io.Discard, stream)
	return oversizedFrameInfo{
		id:        id,
		hasID:     hasID,
		hasMethod: hasMethod,
	}
}

func newOversizedFrameReader(prefix []byte, r *bufio.Reader, readErr error) io.Reader {
	if !errors.Is(readErr, bufio.ErrBufferFull) {
		return bytes.NewReader(prefix)
	}
	return io.MultiReader(
		bytes.NewReader(prefix),
		&oversizedFrameReader{reader: r, readErr: readErr},
	)
}

type oversizedFrameReader struct {
	reader  *bufio.Reader
	pending []byte
	readErr error
	done    bool
}

func (r *oversizedFrameReader) Read(p []byte) (int, error) {
	for {
		if len(r.pending) > 0 {
			n := copy(p, r.pending)
			r.pending = r.pending[n:]
			return n, nil
		}
		if r.done {
			return 0, io.EOF
		}

		switch {
		case r.readErr == nil || errors.Is(r.readErr, io.EOF):
			r.done = true
			return 0, io.EOF
		case !errors.Is(r.readErr, bufio.ErrBufferFull):
			err := r.readErr
			r.done = true
			return 0, err
		}

		frag, err := r.reader.ReadSlice('\n')
		r.pending = frag
		r.readErr = err
		if len(r.pending) == 0 {
			if err == nil {
				continue
			}
			if errors.Is(err, io.EOF) {
				r.done = true
				return 0, io.EOF
			}
			if errors.Is(err, bufio.ErrBufferFull) {
				continue
			}
			r.done = true
			return 0, err
		}
	}
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
