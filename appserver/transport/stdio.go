package transport

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

	"github.com/dominicnunez/codex-sdk-go/internal/diagnostic"
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

type requestHandlerResolution int

const (
	requestHandlerResolved requestHandlerResolution = iota
	requestHandlerQueued
	requestHandlerQueueFull
	requestHandlerStopped
)

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
	maxTurnScopedNotificationQueueSize = streamingNotifQueueSize + maxStreamingNotificationBacklog
	maxTurnScopedNotificationQueues    = 128
	// Preserve the previous aggregate completion-plus-streaming queue budget.
	maxOrderedNotificationBacklog   = maxTurnScopedNotificationQueues*256 + maxTurnScopedNotificationQueueSize
	maxStreamingNotificationBacklog = 1024
	inboundNotifQueueSize           = 128
	streamingNotifQueueSize         = 256
	protectedNotifQueueSize         = 128
	criticalNotifQueueSize          = 64
	outboundWriteQueueSize          = 256
	readBufferSizeBytes             = 64 * 1024
	maxInboundMessageSizeBytes      = 10 * 1024 * 1024
	defaultSendTimeout              = 5 * time.Minute
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
	pendingNotifHandle  []bufferedNotification
	notificationBudget  notificationBudget
	requestQueue        chan Request
	turnNotifQueuesMu   sync.Mutex
	turnNotifQueues     map[string]*turnScopedNotificationQueue
	orderedNotifCount   int // guarded by turnNotifQueuesMu
	turnNotifReadyMu    sync.Mutex
	turnNotifReady      []*turnScopedNotificationQueue
	turnNotifReadyCond  *sync.Cond
	turnNotifReadyOnce  sync.Once
	turnNotifStopped    atomic.Bool
	streamingNotifQueue chan bufferedNotification
	streamingBacklog    streamingNotificationBacklog
	protectedNotifQueue chan bufferedNotification
	criticalNotifQueue  chan bufferedNotification
	notifQueue          chan bufferedNotification
	writeQueue          chan writeEnvelope
	queueAdmissionMu    sync.RWMutex
	queuesStopped       bool // guarded by queueAdmissionMu
	readerStopped       chan struct{}
	once                sync.Once
	startReadLoopOnce   sync.Once
	scanErr             error // terminal transport I/O error, if any
	malformedCount      atomic.Uint64
	panicHandler        func(v any)
	ctx                 context.Context
	cancelCtx           context.CancelFunc
	sendTimeout         time.Duration
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
		streamingNotifQueue: make(chan bufferedNotification, streamingNotifQueueSize),
		protectedNotifQueue: make(chan bufferedNotification, protectedNotifQueueSize),
		criticalNotifQueue:  make(chan bufferedNotification, criticalNotifQueueSize),
		notifQueue:          make(chan bufferedNotification, inboundNotifQueueSize),
		writeQueue:          make(chan writeEnvelope, outboundWriteQueueSize),
		readerStopped:       make(chan struct{}),
		ctx:                 ctx,
		cancelCtx:           cancel,
		sendTimeout:         defaultSendTimeout,
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
	ctx, cancel, err := t.prepareOutbound(ctx, "send failed")
	if err != nil {
		return Response{}, err
	}
	defer cancel()
	t.mu.Lock()
	if err := t.closedTransportErrorLocked("send failed"); err != nil {
		t.mu.Unlock()
		return Response{}, err
	}

	// Create response channel and store with normalized ID for matching
	normalizedID, err := normalizePendingRequestID(req.ID.Value)
	if err != nil {
		t.mu.Unlock()
		return Response{}, NewTransportError("send failed", err)
	}
	if _, exists := t.pendingReqs[normalizedID]; exists {
		t.mu.Unlock()
		return Response{}, duplicateRequestIDError(req.ID)
	}
	t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	// Invalid outbound JSON cannot acquire a correlated request outcome.
	// Serialize outside the mutex so large payloads do not block inbound work.
	data, err := marshalStdioFrame(req)
	if err != nil {
		return Response{}, NewTransportError("marshal message", err)
	}
	t.mu.Lock()
	// Shutdown or another caller may have admitted this ID during serialization.
	if err := t.closedTransportErrorLocked("send failed"); err != nil {
		t.mu.Unlock()
		return Response{}, err
	}
	if _, exists := t.pendingReqs[normalizedID]; exists {
		t.mu.Unlock()
		return Response{}, duplicateRequestIDError(req.ID)
	}
	respChan := make(chan pendingReqResult, 1)
	pending := pendingReq{ch: respChan, id: req.ID}
	t.pendingReqs[normalizedID] = pending
	t.mu.Unlock()

	// Cleanup on exit
	defer func() {
		t.cleanupPendingReq(normalizedID, pending)
	}()

	env, err := t.queuePayload(ctx, data, "send failed", true)
	if err != nil {
		// No outbound envelope was admitted. A speculative inbound match must
		// not turn failed admission into a successful request.
		return Response{}, err
	}

	// A correlated response establishes the request outcome independently of
	// the writer's return. Notifications and internal responses still wait for
	// write completion because they have no correlated inbound outcome.
	writeDone := env.done
	for {
		select {
		case result := <-respChan:
			return responseFromPendingResult(result)
		case err := <-writeDone:
			if err != nil {
				return t.finishPendingRequest(normalizedID, pending, t.normalizeWriteCompletionError("send failed", err))
			}
			writeDone = nil
		case <-ctx.Done():
			return t.finishPendingRequest(normalizedID, pending, ctx.Err())
		case <-t.readerStopped:
			return t.finishPendingRequest(normalizedID, pending, t.transportStopError("send failed"))
		case <-t.ctx.Done():
			return t.finishPendingRequest(normalizedID, pending, t.transportStopError("send failed"))
		}
	}
}

func duplicateRequestIDError(id RequestID) error {
	if value, ok := id.Value.(string); ok {
		return NewTransportError("send failed", fmt.Errorf("duplicate request ID: %s", diagnostic.Display(value)))
	}
	// The admission owner already normalized non-string IDs to int64.
	return NewTransportError("send failed", fmt.Errorf("duplicate request ID: %v", id.Value))
}

// Acceptance and abandonment share the pending-map lock. An accepted result
// wins over later termination; abandoning a request prevents later publication.
func (t *StdioTransport) finishPendingRequest(id string, pending pendingReq, err error) (Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	select {
	case result := <-pending.ch:
		return responseFromPendingResult(result)
	default:
	}
	if current, ok := t.pendingReqs[id]; ok && current.ch == pending.ch {
		delete(t.pendingReqs, id)
	}
	return Response{}, err
}

// Notify transmits a JSON-RPC notification (fire-and-forget).
func (t *StdioTransport) Notify(ctx context.Context, notif Notification) error {
	ctx, cancel, err := t.prepareOutbound(ctx, "notify failed")
	if err != nil {
		return err
	}
	defer cancel()

	t.mu.Lock()
	if err := t.closedTransportErrorLocked("notify failed"); err != nil {
		t.mu.Unlock()
		return err
	}
	t.mu.Unlock()

	return t.enqueueWrite(ctx, notif, "notify failed", true)
}

// OnRequest registers a handler for incoming JSON-RPC requests from the server.
func (t *StdioTransport) OnRequest(handler RequestHandler) {
	pending := swapPendingHandler(t, func() { t.reqHandler = handler }, &t.pendingReqHandler)
	replayPending(pending, t.enqueueRequest)
}

// OnNotify registers a handler for incoming JSON-RPC notifications from the server.
func (t *StdioTransport) OnNotify(handler NotificationHandler) {
	pending := swapPendingHandler(t, func() { t.notifHandler = handler }, &t.pendingNotifHandle)
	for i := range pending {
		notif := pending[i]
		pending[i] = bufferedNotification{}
		t.enqueueBufferedNotification(notif)
	}
	t.wakeTurnScopedNotificationWorkers()
}

// OnPanic registers a handler called when a request handler or notification
// handler panics. The transport recovers from the panic and continues
// operating; this callback provides observability into the recovered value.
// A panic in this callback is suppressed so dispatch can continue.
func (t *StdioTransport) OnPanic(handler func(v any)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.panicHandler = handler
}

// Close shuts down the transport. Safe to call multiple times.
func (t *StdioTransport) Close() error {
	t.stopNotificationWorkers()
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

// ReaderStopped returns a channel that is closed when the read loop exits.
func (t *StdioTransport) ReaderStopped() <-chan struct{} {
	return t.readerStopped
}

func (t *StdioTransport) applyDefaultSendTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := t.sendTimeout
	if timeout <= 0 {
		timeout = defaultSendTimeout
	}
	return applyDefaultSendTimeout(ctx, timeout)
}

func applyDefaultSendTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (t *StdioTransport) prepareOutbound(ctx context.Context, op string) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, NewTransportError(op, ErrNilContext)
	}
	ctx, cancel := t.applyDefaultSendTimeout(ctx)
	t.ensureReadLoopStarted()
	return ctx, cancel, nil
}

func (t *StdioTransport) closedTransportErrorLocked(op string) error {
	if !t.closed {
		return nil
	}
	return NewTransportError(op, t.transportStopCauseLocked())
}

func responseFromPendingResult(result pendingReqResult) (Response, error) {
	if result.err != nil {
		return Response{}, result.err
	}
	return result.resp, nil
}

func swapPendingHandler[T any](t *StdioTransport, setHandler func(), pendingStore *[]T) []T {
	t.mu.Lock()
	setHandler()
	pending := *pendingStore
	*pendingStore = nil
	t.mu.Unlock()
	return pending
}

func replayPending[T any](pending []T, enqueue func(T)) {
	for i, item := range pending {
		var zero T
		pending[i] = zero
		enqueue(item)
	}
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
	pending, cancel, readerCloser := t.takeStopResourcesLocked()
	t.mu.Unlock()

	t.stopNotificationWorkers()
	t.finishTransportStop(cancel, readerCloser)

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

	hasID := frame.ID.isPresent()
	if frame.hasInvalidProtocolVersion() {
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
			JSONRPC: frame.protocolVersion(),
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
			JSONRPC: frame.protocolVersion(),
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
	t.writeErrorResponse(id, ErrCodeInvalidRequest, message)
}

func (t *StdioTransport) failPendingWithInvalidProtocolVersion(idField inboundID) {
	t.failPendingWithError(idField, ErrCodeInvalidRequest, errInvalidResponseJSONRPC)
}

func (t *StdioTransport) failPendingWithParseError(idField inboundID, message string) {
	t.failPendingWithError(idField, ErrCodeParseError, message)
}

func (t *StdioTransport) failPendingWithError(idField inboundID, code int, message string) {
	id, ok := idField.requestID()
	if !ok {
		return
	}
	t.failPendingIDWithError(id, code, message)
}

func (t *StdioTransport) failPendingIDWithError(id RequestID, code int, message string) {
	normalizedID, err := normalizePendingRequestID(id.Value)
	if err != nil {
		return
	}
	t.publishPendingResult(normalizedID, func(pending pendingReq) pendingReqResult {
		return pendingErrorResult(pending.id, code, message)
	})
}

func (t *StdioTransport) publishPendingResult(normalizedID string, result func(pendingReq) pendingReqResult) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	pending, ok := t.pendingReqs[normalizedID]
	if ok {
		// Each request has one buffered slot and only this owner may publish.
		// Publishing under the lock closes the claim-to-publication gap.
		pending.ch <- result(pending)
		delete(t.pendingReqs, normalizedID)
	}
}

func pendingErrorResult(id RequestID, code int, message string) pendingReqResult {
	return pendingReqResult{resp: errorResponse(id, code, message)}
}

func errorResponse(id RequestID, code int, message string) Response {
	return Response{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
}

func (t *StdioTransport) writeErrorResponse(id RequestID, code int, message string) {
	if err := t.writeMessage(errorResponse(id, code, message)); err != nil {
		t.handleWriteFailure(err)
	}
}

func (t *StdioTransport) enqueueRequest(req Request) {
	t.queueAdmissionMu.RLock()
	if t.queuesStopped || t.ctx.Err() != nil {
		t.queueAdmissionMu.RUnlock()
		return
	}
	select {
	case <-t.ctx.Done():
	case t.requestQueue <- req:
	default:
		t.queueAdmissionMu.RUnlock()
		t.rejectRequestForOverload(req)
		return
	}
	t.queueAdmissionMu.RUnlock()
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
	t.writeErrorResponse(req.ID, ErrCodeInternalError, "too many pending inbound requests")
}

// handleResponse routes an incoming response to the pending request channel.
// It atomically publishes and removes the request under the pending-map lock.
func (t *StdioTransport) handleResponse(resp Response) {
	// Normalize ID for matching
	normalizedID, err := normalizePendingRequestID(resp.ID.Value)
	if err != nil {
		return
	}
	t.publishPendingResult(normalizedID, func(pendingReq) pendingReqResult {
		return pendingReqResult{resp: resp}
	})
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
	// A longer JSON number cannot correlate to an int64 request ID. Avoid
	// passing peer-sized rejected spellings into numeric error formatting.
	if number, ok := id.Value.(json.Number); ok && len(number) > 20 {
		return
	}
	t.failPendingIDWithError(id, ErrCodeParseError, "failed to parse server response")
}

func (t *StdioTransport) handleMalformedInboundObject() {
	t.malformedCount.Add(1)
}

// handleMalformedResponse attempts to extract the ID from a response that
// failed full unmarshal, and sends a parse error to the pending caller.
func (t *StdioTransport) handleMalformedResponse(data []byte) {
	t.malformedCount.Add(1)

	if !json.Valid(data) {
		return
	}
	var rawID json.RawMessage
	if !walkJSONObjectFields(data, true, func(key, value []byte) {
		if jsonFieldMatchesFolded(key, "id") {
			rawID = value
		}
	}) || len(rawID) == 0 {
		return
	}

	id, err := parseRequestID(rawID)
	if err != nil {
		return
	}
	t.failPendingIDWithError(id, ErrCodeParseError, "failed to parse server response")
}

func (t *StdioTransport) handleOversizedFrame(info oversizedFrameInfo) {
	if !info.hasResponseFields || info.hasMethod || !info.hasID {
		return
	}

	t.failPendingIDWithError(info.id, ErrCodeParseError, "oversized server response frame")
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
	_, cancel, readerCloser := t.takeStopResourcesLocked()
	t.mu.Unlock()

	t.finishTransportStop(cancel, readerCloser)
	go t.drainPendingNotificationsAfterStop()
}

func (t *StdioTransport) takeStopResourcesLocked() (map[string]pendingReq, context.CancelFunc, io.Closer) {
	pending := t.pendingReqs
	t.pendingReqs = make(map[string]pendingReq)
	t.pendingReqHandler = nil
	return pending, t.cancelCtx, t.readerCloser
}

func (t *StdioTransport) finishTransportStop(cancel context.CancelFunc, readerCloser io.Closer) {
	cancel()
	t.discardQueuedWork()
	t.wakeTurnScopedNotificationWorkers()
	if readerCloser != nil {
		_ = readerCloser.Close()
	}
}

// handleRequest dispatches an incoming server→client request to the handler
func (t *StdioTransport) handleRequest(req Request) {
	handler, panicFn, resolution := t.resolveRequestHandler(req)
	if handler == nil {
		switch resolution {
		case requestHandlerQueued:
			return
		case requestHandlerStopped:
			return
		case requestHandlerQueueFull:
			t.rejectRequestForOverload(req)
			return
		}

		t.writeErrorResponse(req.ID, ErrCodeMethodNotFound, "method not found")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			t.writeErrorResponse(req.ID, ErrCodeInternalError, "internal handler error")
			reportRecoveredPanic(panicFn, r)
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
		t.writeErrorResponse(req.ID, code, msg)
		return
	}

	// Ensure response has correct ID and version
	resp.JSONRPC = jsonrpcVersion
	resp.ID = req.ID
	if err := t.writeMessage(resp); err != nil {
		t.handleWriteFailure(err)
	}
}

// Reporting must not turn a recovered handler panic into a worker crash.
func reportRecoveredPanic(handler func(any), value any) {
	if handler == nil {
		return
	}
	defer func() { _ = recover() }()
	handler(value)
}

func (t *StdioTransport) resolveRequestHandler(req Request) (RequestHandler, func(any), requestHandlerResolution) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.ctx.Err() != nil {
		return nil, nil, requestHandlerStopped
	}

	if t.reqHandler != nil {
		return t.reqHandler, t.panicHandler, requestHandlerResolved
	}
	if len(t.pendingReqHandler) >= inboundRequestQueueSize {
		return nil, nil, requestHandlerQueueFull
	}
	t.pendingReqHandler = append(t.pendingReqHandler, req)
	return nil, nil, requestHandlerQueued
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

	t.writeErrorResponse(id, ErrCodeInvalidRequest, errInvalidServerRequest)
	return true
}
