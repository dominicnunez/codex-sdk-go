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
	inboundNotifQueueSize       = 128
	criticalNotifQueueSize      = 64
	outboundWriteQueueSize      = 256
)

type writeEnvelope struct {
	payload []byte
	done    chan error
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
	requestQueue       chan []byte
	criticalNotifQueue chan []byte
	notifQueue         chan []byte
	writeQueue         chan writeEnvelope
	readerStopped      chan struct{}
	once               sync.Once
	scanErr            error // set by readLoop when an unrecoverable read error occurs
	malformedCount     atomic.Uint64
	panicHandler       func(v any)
	ctx                context.Context
	cancelCtx          context.CancelFunc
}

// errUnexpectedIDType is returned when normalizeID encounters an ID value
// that is not a supported JSON-RPC ID type (string, number).
var errUnexpectedIDType = errors.New("unexpected ID type")

// errNullID is returned when normalizeID encounters a nil (JSON null) ID.
// JSON-RPC 2.0 responses with "id": null indicate the server could not
// parse the request ID.
var errNullID = errors.New("null request ID")

// normalizeID normalizes request IDs to a string key for map matching.
// JSON unmarshals all numbers as float64, so we format integer-valued
// floats without decimals for consistent lookups.
func normalizeID(id interface{}) (string, error) {
	switch v := id.(type) {
	case nil:
		return "", errNullID
	case float64:
		if v >= 0 {
			u := uint64(v)
			if v == float64(u) {
				return fmt.Sprintf("%d", u), nil
			}
		} else {
			i := int64(v)
			if v == float64(i) {
				return fmt.Sprintf("%d", i), nil
			}
		}
		return fmt.Sprintf("%v", v), nil
	case int64:
		return fmt.Sprintf("%d", v), nil
	case int:
		return fmt.Sprintf("%d", v), nil
	case uint64:
		return fmt.Sprintf("%d", v), nil
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("%w: %T", errUnexpectedIDType, id)
	}
}

// NewStdioTransport creates a new stdio transport using the provided reader and writer.
// Typically, reader is os.Stdin and writer is os.Stdout.
// The transport starts a background goroutine to read incoming messages.
func NewStdioTransport(reader io.Reader, writer io.Writer) *StdioTransport {
	ctx, cancel := context.WithCancel(context.Background())
	var readerCloser io.Closer
	if c, ok := reader.(io.Closer); ok {
		readerCloser = c
	}
	t := &StdioTransport{
		reader:             reader,
		readerCloser:       readerCloser,
		writer:             writer,
		pendingReqs:        make(map[string]pendingReq),
		requestQueue:       make(chan []byte, inboundRequestQueueSize),
		criticalNotifQueue: make(chan []byte, criticalNotifQueueSize),
		notifQueue:         make(chan []byte, inboundNotifQueueSize),
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
	go t.writeLoop()
	go t.readLoop()
	return t
}

// Send transmits a JSON-RPC request and waits for the response.
// The response is matched to this request by ID.
func (t *StdioTransport) Send(ctx context.Context, req Request) (Response, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return Response{}, NewTransportError("send failed", errors.New("transport closed"))
	}

	// Create response channel and store with normalized ID for matching
	normalizedID, err := normalizeID(req.ID.Value)
	if err != nil {
		t.mu.Unlock()
		return Response{}, NewTransportError("send failed", err)
	}
	if _, exists := t.pendingReqs[normalizedID]; exists {
		t.mu.Unlock()
		return Response{}, NewTransportError("send failed", fmt.Errorf("duplicate request ID: %v", req.ID.Value))
	}
	respChan := make(chan Response, 1)
	t.pendingReqs[normalizedID] = pendingReq{ch: respChan, id: req.ID}
	t.mu.Unlock()

	// Cleanup on exit
	defer func() {
		t.mu.Lock()
		delete(t.pendingReqs, normalizedID)
		t.mu.Unlock()
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
		return Response{}, NewTransportError("send failed", errors.New("transport reader stopped"))
	}
}

// Notify transmits a JSON-RPC notification (fire-and-forget).
func (t *StdioTransport) Notify(ctx context.Context, notif Notification) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return NewTransportError("notify failed", errors.New("transport closed"))
	}
	t.mu.Unlock()

	return t.enqueueWrite(ctx, notif, "notify failed", true)
}

// OnRequest registers a handler for incoming JSON-RPC requests from the server.
func (t *StdioTransport) OnRequest(handler RequestHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reqHandler = handler
}

// OnNotify registers a handler for incoming JSON-RPC notifications from the server.
func (t *StdioTransport) OnNotify(handler NotificationHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notifHandler = handler
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
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	t.cancelCtx()
	if t.readerCloser != nil {
		_ = t.readerCloser.Close()
	}

	// Unblock all pending request waiters with an error response indicating
	// the transport was closed. We send rather than close because
	// handleResponse may concurrently hold a reference to the channel.
	for key, pending := range t.pendingReqs {
		resp := Response{
			JSONRPC: jsonrpcVersion,
			ID:      pending.id,
			Error: &Error{
				Code:    ErrCodeInternalError,
				Message: "transport closed",
			},
		}
		// Defensive: default branch guards against a handleResponse
		// send racing between the closed check and this loop iteration.
		select {
		case pending.ch <- resp:
		default:
		}
		delete(t.pendingReqs, key)
	}

	return nil
}

// ScanErr returns the error (if any) from the reader goroutine's read loop.
// Returns nil if the reader stopped due to EOF or hasn't stopped yet.
func (t *StdioTransport) ScanErr() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.scanErr
}

// MalformedMessageCount reports how many inbound lines were invalid JSON.
func (t *StdioTransport) MalformedMessageCount() uint64 {
	return t.malformedCount.Load()
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

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.ctx.Done():
		return NewTransportError(op, errors.New("transport closed"))
	case <-t.readerStopped:
		if watchReaderStop {
			return NewTransportError(op, errors.New("transport reader stopped"))
		}
	case t.writeQueue <- env:
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
		return NewTransportError(op, errors.New("transport closed"))
	case <-t.readerStopped:
		if watchReaderStop {
			return NewTransportError(op, errors.New("transport reader stopped"))
		}
		return nil
	}
}

// writeMessage enqueues a JSON-RPC message for serialized writer-loop delivery.
func (t *StdioTransport) writeMessage(msg interface{}) error {
	return t.enqueueWrite(t.ctx, msg, "write message", false)
}

func (t *StdioTransport) writeLoop() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case env := <-t.writeQueue:
			err := t.writeRawMessage(env.payload)
			env.done <- err
		}
	}
}

func (t *StdioTransport) requestWorker() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case data := <-t.requestQueue:
			t.handleRequest(data)
		}
	}
}

func (t *StdioTransport) notificationWorker() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case data := <-t.notifQueue:
			t.handleNotification(data)
		}
	}
}

func (t *StdioTransport) criticalNotificationWorker() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case data := <-t.criticalNotifQueue:
			t.handleNotification(data)
		}
	}
}

// readLoop continuously reads newline-delimited JSON messages from the reader
func (t *StdioTransport) readLoop() {
	defer t.once.Do(func() { close(t.readerStopped) })

	const readBufferSize = 64 * 1024        // 64KB
	const maxMessageSize = 10 * 1024 * 1024 // 10MB — file diffs and base64 payloads exceed the default
	reader := bufio.NewReaderSize(t.reader, readBufferSize)
	for {
		line, overLimit, err := readLimitedLine(reader, maxMessageSize)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			t.mu.Lock()
			t.scanErr = err
			t.mu.Unlock()
			return
		}
		if overLimit {
			// Drop oversized frames and keep processing subsequent messages.
			continue
		}

		// Parse the message to determine its type
		var msg struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
		}

		if err := json.Unmarshal(line, &msg); err != nil {
			t.malformedCount.Add(1)
			continue
		}

		hasID := len(msg.ID) > 0 && string(msg.ID) != "null"

		// Response: has ID but no method
		if hasID && msg.Method == "" {
			t.handleResponse(line)
			continue
		}

		// Request: has both ID and method
		if hasID && msg.Method != "" {
			t.enqueueRequest(line)
			continue
		}

		// Notification: has method but no ID
		if msg.Method != "" {
			t.enqueueNotification(line, msg.Method)
			continue
		}

		// Unknown message type - skip it
	}
}

// readLimitedLine reads one newline-delimited frame and enforces an upper size
// bound. If a frame exceeds max bytes, it is fully discarded and overLimit=true
// is returned so callers can skip just that message and continue.
func readLimitedLine(r *bufio.Reader, limit int) ([]byte, bool, error) {
	var line []byte
	total := 0

	for {
		frag, err := r.ReadSlice('\n')
		total += len(frag)
		if total > limit {
			return handleOversizedLine(r, err)
		}

		line = append(line, frag...)
		switch {
		case err == nil:
			return bytes.TrimSuffix(line, []byte{'\n'}), false, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if len(line) == 0 {
				return nil, false, io.EOF
			}
			return line, false, nil
		default:
			return nil, false, err
		}
	}
}

func handleOversizedLine(r *bufio.Reader, readErr error) ([]byte, bool, error) {
	switch {
	case readErr == nil:
		return nil, true, nil
	case errors.Is(readErr, io.EOF):
		return nil, true, io.EOF
	case !errors.Is(readErr, bufio.ErrBufferFull):
		return nil, false, readErr
	}

	if err := discardUntilNewline(r); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, true, io.EOF
		}
		return nil, false, err
	}
	return nil, true, nil
}

func discardUntilNewline(r *bufio.Reader) error {
	for {
		_, err := r.ReadSlice('\n')
		switch {
		case err == nil:
			return nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		default:
			return err
		}
	}
}

func (t *StdioTransport) enqueueRequest(data []byte) {
	select {
	case <-t.ctx.Done():
		return
	case t.requestQueue <- data:
		return
	default:
		t.rejectRequestForOverload(data)
	}
}

func (t *StdioTransport) enqueueNotification(data []byte, method string) {
	if isCriticalNotificationMethod(method) {
		select {
		case <-t.ctx.Done():
			return
		case t.criticalNotifQueue <- data:
			return
		}
	}

	select {
	case <-t.ctx.Done():
		return
	case t.notifQueue <- data:
	default:
		// Notifications are fire-and-forget. If the queue is full we drop to
		// preserve process liveness and bound memory use under abuse.
	}
}

func isCriticalNotificationMethod(method string) bool {
	return method == notifyTurnCompleted
}

func (t *StdioTransport) rejectRequestForOverload(data []byte) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		t.handleMalformedRequest(data)
		return
	}
	_ = t.writeMessage(Response{
		JSONRPC: jsonrpcVersion,
		ID:      req.ID,
		Error: &Error{
			Code:    ErrCodeInternalError,
			Message: "too many pending inbound requests",
		},
	})
}

// handleResponse routes an incoming response to the pending request channel.
// It claims the channel under the lock via delete, then sends outside the
// lock. The delete-then-unlock-then-send pattern ensures exclusive access
// to the channel without holding the mutex during the send.
func (t *StdioTransport) handleResponse(data []byte) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		t.handleMalformedResponse(data)
		return
	}

	// Normalize ID for matching
	normalizedID, err := normalizeID(resp.ID.Value)
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

// handleMalformedResponse attempts to extract the ID from a response that
// failed full unmarshal, and sends a parse error to the pending caller.
func (t *StdioTransport) handleMalformedResponse(data []byte) {
	var idOnly struct {
		ID RequestID `json:"id"`
	}
	if json.Unmarshal(data, &idOnly) != nil {
		return
	}
	normalizedID, err := normalizeID(idOnly.ID.Value)
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

// handleRequest dispatches an incoming server→client request to the handler
func (t *StdioTransport) handleRequest(data []byte) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		t.handleMalformedRequest(data)
		return
	}

	t.mu.Lock()
	handler := t.reqHandler
	panicFn := t.panicHandler
	t.mu.Unlock()

	if handler == nil {
		// No handler registered - send method not found error
		errorResp := Response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error: &Error{
				Code:    ErrCodeMethodNotFound,
				Message: "method not found",
			},
		}
		_ = t.writeMessage(errorResp) // Error writing error response - nothing more we can do
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
			_ = t.writeMessage(errorResp)
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
		_ = t.writeMessage(errorResp) // Error writing error response - nothing more we can do
		return
	}

	// Ensure response has correct ID and version
	resp.JSONRPC = jsonrpcVersion
	resp.ID = req.ID
	_ = t.writeMessage(resp) // Error writing response - nothing more we can do
}

// handleMalformedRequest attempts to extract the ID from a request that
// failed full unmarshal, and sends back a parse error response so the
// server knows the request failed instead of hanging indefinitely.
func (t *StdioTransport) handleMalformedRequest(data []byte) {
	id := RequestID{Value: nil}
	var idOnly struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal(data, &idOnly) == nil && len(idOnly.ID) > 0 {
		var candidate RequestID
		if json.Unmarshal(idOnly.ID, &candidate) == nil {
			if _, err := normalizeID(candidate.Value); err == nil {
				id = candidate
			}
		}
	}

	errorResp := Response{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error: &Error{
			Code:    ErrCodeParseError,
			Message: "failed to parse server request",
		},
	}
	_ = t.writeMessage(errorResp)
}

// handleNotification dispatches an incoming server→client notification to the handler
func (t *StdioTransport) handleNotification(data []byte) {
	var notif Notification
	if err := json.Unmarshal(data, &notif); err != nil {
		return
	}

	t.mu.Lock()
	handler := t.notifHandler
	panicFn := t.panicHandler
	t.mu.Unlock()

	if handler == nil {
		return
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
