package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
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
	terminalNotificationWorkers = 2
	inboundNotifQueueSize       = 128
	criticalNotifQueueSize      = 64
	terminalNotifQueueSize      = 64
	outboundWriteQueueSize      = 256
	readBufferSizeBytes         = 64 * 1024
	maxInboundMessageSizeBytes  = 10 * 1024 * 1024
	oversizeMetadataBytes       = 64 * 1024
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

type inboundFrame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
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
	requestQueue       chan Request
	criticalNotifQueue chan Notification
	terminalNotifQueue chan Notification
	notifQueue         chan Notification
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
	case json.Number:
		return normalizeJSONNumberString(v.String())
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

func normalizePendingRequestID(id interface{}) (string, error) {
	normalizedID, err := normalizeID(id)
	if err != nil {
		return "", err
	}
	switch id.(type) {
	case float64, json.Number, int64, int, uint64:
		return requestIDKeyPrefixNumber + normalizedID, nil
	case string:
		return requestIDKeyPrefixString + normalizedID, nil
	default:
		return "", fmt.Errorf("%w: %T", errUnexpectedIDType, id)
	}
}

// NewStdioTransport creates a new stdio transport using the provided reader and writer.
// reader is required to be an io.ReadCloser so Close can always unblock the read loop.
// Typically, reader is os.Stdin and writer is os.Stdout.
// The transport starts background goroutines for read/write and inbound dispatch.
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
	go t.readLoop()
	return t
}

// Send transmits a JSON-RPC request and waits for the response.
// The response is matched to this request by ID.
func (t *StdioTransport) Send(ctx context.Context, req Request) (Response, error) {
	if ctx == nil {
		return Response{}, NewTransportError("send failed", ErrNilContext)
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return Response{}, NewTransportError("send failed", errors.New("transport closed"))
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
		case req := <-t.requestQueue:
			t.handleRequest(req)
		}
	}
}

func (t *StdioTransport) notificationWorker() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case notif := <-t.notifQueue:
			t.handleNotification(notif)
		}
	}
}

func (t *StdioTransport) criticalNotificationWorker() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case notif := <-t.criticalNotifQueue:
			t.handleNotification(notif)
		}
	}
}

func (t *StdioTransport) terminalNotificationWorker() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case notif := <-t.terminalNotifQueue:
			t.handleNotification(notif)
		}
	}
}

// readLoop continuously reads newline-delimited JSON messages from the reader
func (t *StdioTransport) readLoop() {
	defer t.once.Do(func() { close(t.readerStopped) })

	reader := bufio.NewReaderSize(t.reader, readBufferSizeBytes)
	for {
		line, overLimit, err := readLimitedLine(reader, maxInboundMessageSizeBytes)
		if overLimit {
			// Best-effort: fail matching pending responses so Send callers do not
			// block waiting on a frame we intentionally discarded.
			t.handleOversizedFrame(line)
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				t.mu.Lock()
				t.scanErr = err
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
			t.scanErr = err
			t.mu.Unlock()
			return
		}

		frame, err := decodeInboundFrame(line)
		if err != nil {
			t.malformedCount.Add(1)
			continue
		}

		hasID := frameHasID(frame.ID)

		if frame.JSONRPC != jsonrpcVersion {
			t.handleInvalidJSONRPCVersion(frame, hasID)
			continue
		}

		// Response: has ID but no method
		if hasID && frame.Method == "" {
			resp, err := frame.toResponse()
			if err != nil {
				t.handleMalformedResponse(line)
				continue
			}
			t.handleResponse(resp)
			continue
		}

		// Request: has both ID and method
		if hasID && frame.Method != "" {
			req, err := frame.toRequest()
			if err != nil {
				t.handleMalformedRequest(line)
				continue
			}
			t.enqueueRequest(req)
			continue
		}

		// Notification: has method but no ID
		if frame.Method != "" {
			t.enqueueNotification(frame.toNotification())
			continue
		}

		// Unknown message type - skip it
	}
}

func frameHasID(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null"
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

func (t *StdioTransport) rejectInvalidProtocolVersion(rawID json.RawMessage) {
	id := RequestID{Value: nil}
	if parsed, err := parseRequestID(rawID); err == nil {
		id = parsed
	}
	_ = t.writeMessage(Response{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error: &Error{
			Code:    ErrCodeInvalidRequest,
			Message: errInvalidJSONRPCVersion,
		},
	})
}

func (t *StdioTransport) failPendingWithInvalidProtocolVersion(rawID json.RawMessage) {
	id, err := parseRequestID(rawID)
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
func readLimitedLine(r *bufio.Reader, limit int) ([]byte, bool, error) {
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

func handleOversizedLine(r *bufio.Reader, readErr error, line []byte) ([]byte, bool, error) {
	prefix := capSlice(line, oversizeMetadataBytes)
	switch {
	case readErr == nil:
		return prefix, true, nil
	case errors.Is(readErr, io.EOF):
		return prefix, true, io.EOF
	case !errors.Is(readErr, bufio.ErrBufferFull):
		return nil, false, readErr
	}

	if err := discardUntilNewline(r); err != nil {
		if errors.Is(err, io.EOF) {
			return prefix, true, io.EOF
		}
		return nil, false, err
	}
	return prefix, true, nil
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
	_ = t.writeMessage(Response{
		JSONRPC: jsonrpcVersion,
		ID:      req.ID,
		Error: &Error{
			Code:    ErrCodeInternalError,
			Message: "too many pending inbound requests",
		},
	})
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

func (f inboundFrame) toResponse() (Response, error) {
	id, err := parseRequestID(f.ID)
	if err != nil {
		return Response{}, err
	}
	var rpcErr *Error
	if len(f.Error) > 0 && string(f.Error) != "null" {
		var parsed Error
		if err := json.Unmarshal(f.Error, &parsed); err != nil {
			return Response{}, err
		}
		rpcErr = &parsed
	}
	return Response{
		JSONRPC: f.JSONRPC,
		ID:      id,
		Result:  f.Result,
		Error:   rpcErr,
	}, nil
}

func (f inboundFrame) toRequest() (Request, error) {
	id, err := parseRequestID(f.ID)
	if err != nil {
		return Request{}, err
	}
	return Request{
		JSONRPC: f.JSONRPC,
		ID:      id,
		Method:  f.Method,
		Params:  f.Params,
	}, nil
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

func normalizeJSONNumberString(raw string) (string, error) {
	if strings.ContainsAny(raw, "eE") {
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return "", fmt.Errorf("%w: %q", errUnexpectedIDType, raw)
		}
		return normalizeID(f)
	}

	if !strings.ContainsRune(raw, '.') {
		if isNegativeZeroString(raw) {
			return "0", nil
		}
		return raw, nil
	}

	sign := ""
	rest := raw
	if strings.HasPrefix(rest, "-") {
		sign = "-"
		rest = rest[1:]
	}

	parts := strings.SplitN(rest, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("%w: %q", errUnexpectedIDType, raw)
	}
	intPart := parts[0]
	fracPart := strings.TrimRight(parts[1], "0")
	if fracPart == "" {
		if sign == "-" && allDigitsAreZero(intPart) {
			return "0", nil
		}
		return sign + intPart, nil
	}
	return sign + intPart + "." + fracPart, nil
}

func isNegativeZeroString(raw string) bool {
	if !strings.HasPrefix(raw, "-") {
		return false
	}
	return allDigitsAreZero(raw[1:])
}

func allDigitsAreZero(raw string) bool {
	if raw == "" {
		return false
	}
	for _, ch := range raw {
		if ch != '0' {
			return false
		}
	}
	return true
}

func (t *StdioTransport) handleOversizedFrame(data []byte) {
	id, hasID, hasMethod := extractTopLevelIDAndMethod(data)
	if hasMethod || !hasID {
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
				Message: "oversized server response frame",
			},
		}
	}
}

func extractTopLevelIDAndMethod(data []byte) (RequestID, bool, bool) {
	var id RequestID
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	start, err := decoder.Token()
	if err != nil {
		return id, false, false
	}
	delim, ok := start.(json.Delim)
	if !ok || delim != '{' {
		return id, false, false
	}

	var hasID bool
	var hasMethod bool
	for decoder.More() {
		keyTok, err := decoder.Token()
		if err != nil {
			return id, hasID, hasMethod
		}
		key, ok := keyTok.(string)
		if !ok {
			return id, hasID, hasMethod
		}

		valueTok, err := decoder.Token()
		if err != nil {
			return id, hasID, hasMethod
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
				return id, hasID, hasMethod
			}
		}
	}

	return id, hasID, hasMethod
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

func capSlice(data []byte, limit int) []byte {
	if len(data) <= limit {
		return data
	}
	return data[:limit]
}

// handleRequest dispatches an incoming server→client request to the handler
func (t *StdioTransport) handleRequest(req Request) {
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
func (t *StdioTransport) handleNotification(notif Notification) {
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
