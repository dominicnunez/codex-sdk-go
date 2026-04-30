package codex

import (
	"bufio"
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
	errInvalidServerRequest   = "invalid server request"
)

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

var errTransportClosed = errors.New("transport closed")
var errTransportReaderStopped = errors.New("transport reader stopped")
var errOversizedInboundFrame = errors.New("oversized inbound frame exceeded maximum size")
var errNotificationQueueOverflow = errors.New("notification queue overflow")
var errStreamingNotificationBacklogOverflow = errors.New("streaming notification backlog overflow")
var errTurnScopedNotificationQueueOverflow = errors.New("turn-scoped notification queue overflow")
var errTurnScopedNotificationQueueLimit = errors.New("turn-scoped notification queue limit exceeded")

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

func (t *StdioTransport) requestWorker() {
	for {
		req, ok := recvWhileRunning(t.ctx, t.requestQueue)
		if !ok {
			return
		}
		t.handleRequest(req)
	}
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

	if hasID {
		t.rejectInvalidRequestID(frame.ID)
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
	t.rejectInvalidRequestIDWithMessage(idField, errInvalidJSONRPCVersion)
}

func (t *StdioTransport) rejectInvalidRequestID(idField inboundID) {
	t.rejectInvalidRequestIDWithMessage(idField, errInvalidServerRequest)
}

func (t *StdioTransport) rejectInvalidRequestIDWithMessage(idField inboundID, message string) {
	id := RequestID{Value: nil}
	if parsed, ok := idField.requestID(); ok {
		id = parsed
	}
	if err := t.writeMessage(Response{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error: &Error{
			Code:    ErrCodeInvalidRequest,
			Message: message,
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
			Message: errInvalidServerRequest,
		},
	}
	if err := t.writeMessage(errorResp); err != nil {
		t.handleWriteFailure(err)
	}
	return true
}
