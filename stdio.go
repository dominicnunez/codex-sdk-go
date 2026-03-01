package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// pendingReq holds a pending request's response channel and original ID.
type pendingReq struct {
	ch chan Response
	id RequestID
}

// StdioTransport implements the Transport interface using stdin/stdout with newline-delimited JSON.
// It supports bidirectional JSON-RPC 2.0 communication:
// - Client→Server: Send requests and notifications
// - Server→Client: Receive requests (for approval flows) and notifications (for events)
type StdioTransport struct {
	reader io.Reader
	writer io.Writer

	mu            sync.Mutex
	closed        bool
	writeMu       sync.Mutex // separate mutex for write operations
	pendingReqs   map[string]pendingReq
	reqHandler    RequestHandler
	notifHandler  NotificationHandler
	readerStopped chan struct{}
	once          sync.Once
	scanErr       error // set by readLoop when scanner fails
	panicHandler  func(v any)
	ctx           context.Context
	cancelCtx     context.CancelFunc
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
	t := &StdioTransport{
		reader:        reader,
		writer:        writer,
		pendingReqs:   make(map[string]pendingReq),
		readerStopped: make(chan struct{}),
		ctx:           ctx,
		cancelCtx:     cancel,
	}
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

	// Send request in a goroutine so we can respect context cancellation
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- t.writeMessage(req)
	}()

	// Wait for write to complete or context cancellation
	select {
	case err := <-writeDone:
		if err != nil {
			return Response{}, err
		}
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case <-t.readerStopped:
		return Response{}, NewTransportError("send failed", errors.New("transport reader stopped"))
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

	writeDone := make(chan error, 1)
	go func() {
		writeDone <- t.writeMessage(notif)
	}()

	select {
	case err := <-writeDone:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-t.readerStopped:
		return NewTransportError("notify failed", errors.New("transport reader stopped"))
	}
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

// ScanErr returns the error (if any) from the reader goroutine's scanner.
// Returns nil if the reader stopped due to EOF or hasn't stopped yet.
func (t *StdioTransport) ScanErr() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.scanErr
}

// writeMessage writes a JSON-RPC message as newline-delimited JSON.
// It checks the transport context before acquiring the write lock so
// that goroutines dispatched before Close() do not write to a
// potentially invalid writer after the transport has shut down.
func (t *StdioTransport) writeMessage(msg interface{}) error {
	select {
	case <-t.ctx.Done():
		return NewTransportError("write message", errors.New("transport closed"))
	default:
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return NewTransportError("marshal message", err)
	}

	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	// Re-check after acquiring the lock: Close() may have been called
	// while we were waiting on writeMu.
	select {
	case <-t.ctx.Done():
		return NewTransportError("write message", errors.New("transport closed"))
	default:
	}

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

// readLoop continuously reads newline-delimited JSON messages from the reader
func (t *StdioTransport) readLoop() {
	defer t.once.Do(func() { close(t.readerStopped) })

	const initialBufferSize = 64 * 1024     // 64KB
	const maxMessageSize = 10 * 1024 * 1024 // 10MB — file diffs and base64 payloads exceed the default
	scanner := bufio.NewScanner(t.reader)
	scanner.Buffer(make([]byte, 0, initialBufferSize), maxMessageSize)
	for scanner.Scan() {
		line := scanner.Bytes()

		// Parse the message to determine its type
		var msg struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
		}

		if err := json.Unmarshal(line, &msg); err != nil {
			// Invalid JSON - skip it (transport stays alive)
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
			t.handleRequest(line)
			continue
		}

		// Notification: has method but no ID
		if msg.Method != "" {
			t.handleNotification(line)
			continue
		}

		// Unknown message type - skip it
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			err = fmt.Errorf("message exceeded %d byte limit: %w", maxMessageSize, err)
		}
		t.mu.Lock()
		t.scanErr = err
		t.mu.Unlock()
	}
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

	// Dispatch to handler in goroutine with transport-scoped context
	go func() {
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
		_ = t.writeMessage(resp) // Error writing response - nothing more we can do (already in goroutine)
	}()
}

// handleMalformedRequest attempts to extract the ID from a request that
// failed full unmarshal, and sends back a parse error response so the
// server knows the request failed instead of hanging indefinitely.
func (t *StdioTransport) handleMalformedRequest(data []byte) {
	var idOnly struct {
		ID RequestID `json:"id"`
	}
	if json.Unmarshal(data, &idOnly) != nil {
		return
	}

	errorResp := Response{
		JSONRPC: jsonrpcVersion,
		ID:      idOnly.ID,
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

	// Dispatch to handler in goroutine with transport-scoped context
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if panicFn != nil {
					panicFn(r)
				}
			}
		}()
		handler(t.ctx, notif)
	}()
}
