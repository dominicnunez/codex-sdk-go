package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

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
	pendingReqs   map[interface{}]chan Response
	reqHandler    RequestHandler
	notifHandler  NotificationHandler
	readerStopped chan struct{}
	once          sync.Once
}

// normalizeID normalizes request IDs for map key matching.
// JSON unmarshals all numbers as float64, so we need to normalize
// int64 and float64 values to the same type for matching.
func normalizeID(id interface{}) interface{} {
	switch v := id.(type) {
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case float64:
		return v
	case string:
		return v
	default:
		return id
	}
}

// NewStdioTransport creates a new stdio transport using the provided reader and writer.
// Typically, reader is os.Stdin and writer is os.Stdout.
// The transport starts a background goroutine to read incoming messages.
func NewStdioTransport(reader io.Reader, writer io.Writer) *StdioTransport {
	t := &StdioTransport{
		reader:        reader,
		writer:        writer,
		pendingReqs:   make(map[interface{}]chan Response),
		readerStopped: make(chan struct{}),
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
	normalizedID := normalizeID(req.ID.Value)
	respChan := make(chan Response, 1)
	t.pendingReqs[normalizedID] = respChan
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

	return t.writeMessage(notif)
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

// Close shuts down the transport. Safe to call multiple times.
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	// Close all pending request channels
	for _, ch := range t.pendingReqs {
		close(ch)
	}
	t.pendingReqs = make(map[interface{}]chan Response)

	return nil
}

// writeMessage writes a JSON-RPC message as newline-delimited JSON
func (t *StdioTransport) writeMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return NewTransportError("marshal message", err)
	}

	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	// Write message with newline delimiter
	data = append(data, '\n')
	if _, err := t.writer.Write(data); err != nil {
		return NewTransportError("write message", err)
	}

	return nil
}

// readLoop continuously reads newline-delimited JSON messages from the reader
func (t *StdioTransport) readLoop() {
	defer t.once.Do(func() { close(t.readerStopped) })

	scanner := bufio.NewScanner(t.reader)
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

		// Response: has ID but no method
		if len(msg.ID) > 0 && msg.Method == "" {
			t.handleResponse(line)
			continue
		}

		// Request: has both ID and method
		if len(msg.ID) > 0 && msg.Method != "" {
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
}

// handleResponse routes an incoming response to the pending request channel
func (t *StdioTransport) handleResponse(data []byte) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}

	// Normalize ID for matching
	normalizedID := normalizeID(resp.ID.Value)

	t.mu.Lock()
	respChan, ok := t.pendingReqs[normalizedID]
	t.mu.Unlock()

	if ok {
		select {
		case respChan <- resp:
		default:
			// Channel is full or closed - drop the response
		}
	}
}

// handleRequest dispatches an incoming server→client request to the handler
func (t *StdioTransport) handleRequest(data []byte) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}

	t.mu.Lock()
	handler := t.reqHandler
	t.mu.Unlock()

	if handler == nil {
		// No handler registered - send method not found error
		errorResp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    ErrCodeMethodNotFound,
				Message: "method not found",
			},
		}
		_ = t.writeMessage(errorResp) // Error writing error response - nothing more we can do
		return
	}

	// Dispatch to handler in goroutine
	go func() {
		ctx := context.Background()
		resp, err := handler(ctx, req)
		if err != nil {
			// Handler returned error - convert to JSON-RPC error response
			errorResp := Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &Error{
					Code:    ErrCodeInternalError,
					Message: err.Error(),
				},
			}
			_ = t.writeMessage(errorResp) // Error writing error response - nothing more we can do
			return
		}

		// Ensure response has correct ID and version
		resp.JSONRPC = "2.0"
		resp.ID = req.ID
		_ = t.writeMessage(resp) // Error writing response - nothing more we can do (already in goroutine)
	}()
}

// handleNotification dispatches an incoming server→client notification to the handler
func (t *StdioTransport) handleNotification(data []byte) {
	var notif Notification
	if err := json.Unmarshal(data, &notif); err != nil {
		return
	}

	t.mu.Lock()
	handler := t.notifHandler
	t.mu.Unlock()

	if handler == nil {
		// No handler registered - silently ignore notification
		return
	}

	// Dispatch to handler in goroutine
	go func() {
		ctx := context.Background()
		handler(ctx, notif)
	}()
}
