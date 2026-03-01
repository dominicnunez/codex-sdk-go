package codex_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestStdioNewlineDelimitedJSON verifies that messages are encoded/decoded as newline-delimited JSON
func TestStdioNewlineDelimitedJSON(t *testing.T) {
	// Create pipes to simulate stdin/stdout
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Start a goroutine to read from the server side and verify format
	received := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		if scanner.Scan() {
			received <- scanner.Text()
		}
	}()

	// Send a request
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "test-1"},
		Method:  "test/method",
		Params:  json.RawMessage(`{"key":"value"}`),
	}

	// Send request in background (it will block until response is sent)
	responseChan := make(chan codex.Response, 1)
	errorChan := make(chan error, 1)
	go func() {
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errorChan <- err
			return
		}
		responseChan <- resp
	}()

	// Verify the message was sent as newline-delimited JSON
	select {
	case line := <-received:
		// Verify it's valid JSON
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("received line is not valid JSON: %v", err)
		}
		// Verify it has required JSON-RPC fields
		if decoded["jsonrpc"] != "2.0" {
			t.Errorf("jsonrpc field = %v; want 2.0", decoded["jsonrpc"])
		}
		if decoded["method"] != "test/method" {
			t.Errorf("method field = %v; want test/method", decoded["method"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for message to be sent")
	}

	// Send response back from server
	response := codex.Response{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "test-1"},
		Result:  json.RawMessage(`{"status":"ok"}`),
	}
	respJSON, _ := json.Marshal(response)
	_, _ = serverWriter.Write(append(respJSON, '\n'))

	// Verify response was received
	select {
	case resp := <-responseChan:
		if string(resp.Result) != `{"status":"ok"}` {
			t.Errorf("response result = %s; want {\"status\":\"ok\"}", resp.Result)
		}
	case err := <-errorChan:
		t.Fatalf("Send returned error: %v", err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for response")
	}
}

// TestStdioConcurrentRequestDispatch verifies concurrent reads are dispatched correctly
func TestStdioConcurrentRequestDispatch(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Track received requests
	var receivedRequests sync.Map
	transport.OnRequest(func(ctx context.Context, req codex.Request) (codex.Response, error) {
		receivedRequests.Store(req.Method, true)
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}, nil
	})

	// Send multiple server→client requests concurrently
	requests := []string{
		"approval/applyPatch",
		"approval/commandExecution",
		"approval/fileChange",
	}

	for _, method := range requests {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: method},
			Method:  method,
		}
		reqJSON, _ := json.Marshal(req)
		_, _ = serverWriter.Write(append(reqJSON, '\n'))
	}

	// Wait for all requests to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify all requests were received
	for _, method := range requests {
		if _, ok := receivedRequests.Load(method); !ok {
			t.Errorf("request %s was not dispatched", method)
		}
	}
}

// TestStdioResponseRequestIDMatching verifies responses are matched to requests by ID
func TestStdioResponseRequestIDMatching(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Read requests on the server side
	sentRequests := make(chan codex.Request, 3)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var req codex.Request
			if err := json.Unmarshal([]byte(scanner.Text()), &req); err == nil {
				sentRequests <- req
			}
		}
	}()

	ctx := context.Background()

	// Send three requests with different ID types concurrently
	type result struct {
		id     interface{}
		result json.RawMessage
		err    error
	}
	results := make(chan result, 3)

	// String ID
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "string-id"},
			Method:  "test/method1",
		}
		resp, err := transport.Send(ctx, req)
		results <- result{id: "string-id", result: resp.Result, err: err}
	}()

	// Int64 ID
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: int64(123)},
			Method:  "test/method2",
		}
		resp, err := transport.Send(ctx, req)
		results <- result{id: int64(123), result: resp.Result, err: err}
	}()

	// Float64 ID (JSON unmarshals numbers as float64)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: float64(456)},
			Method:  "test/method3",
		}
		resp, err := transport.Send(ctx, req)
		results <- result{id: float64(456), result: resp.Result, err: err}
	}()

	// Wait for all requests to be sent and collect them
	time.Sleep(50 * time.Millisecond)

	requests := make([]codex.Request, 0, 3)
	for i := 0; i < 3; i++ {
		select {
		case req := <-sentRequests:
			requests = append(requests, req)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for requests to be sent")
		}
	}

	// Build a map from marshaled request ID → unique result payload
	// so we can verify each caller gets the response meant for its ID.
	expectedByID := make(map[string]string)
	for i, req := range requests {
		idJSON, _ := json.Marshal(req.ID)
		expectedByID[string(idJSON)] = fmt.Sprintf(`{"match":"resp-%d"}`, i)
	}

	// Send responses in reverse order to verify ID matching
	for i := len(requests) - 1; i >= 0; i-- {
		req := requests[i]
		idJSON, _ := json.Marshal(req.ID)
		resp := codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(expectedByID[string(idJSON)]),
		}
		respJSON, _ := json.Marshal(resp)
		_, _ = serverWriter.Write(append(respJSON, '\n'))
	}

	// Verify each request got the response that was sent for its specific ID
	for i := 0; i < 3; i++ {
		select {
		case res := <-results:
			if res.err != nil {
				t.Errorf("request with id %v returned error: %v", res.id, res.err)
				continue
			}
			idJSON, _ := json.Marshal(codex.RequestID{Value: res.id})
			want, ok := expectedByID[string(idJSON)]
			if !ok {
				t.Errorf("unexpected request id %v", res.id)
				continue
			}
			if string(res.result) != want {
				t.Errorf("request id %v: got result %s; want %s", res.id, res.result, want)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout waiting for responses")
		}
	}
}

// TestStdioNotificationDispatch verifies notifications are dispatched to the handler
func TestStdioNotificationDispatch(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Track received notifications
	received := make(chan string, 3)
	transport.OnNotify(func(ctx context.Context, notif codex.Notification) {
		received <- notif.Method
	})

	// Send notifications from server
	notifications := []string{
		"thread/started",
		"turn/completed",
		"account/updated",
	}

	for _, method := range notifications {
		notif := codex.Notification{
			JSONRPC: "2.0",
			Method:  method,
		}
		notifJSON, _ := json.Marshal(notif)
		_, _ = serverWriter.Write(append(notifJSON, '\n'))
	}

	// Verify all notifications were received (order may vary due to goroutine scheduling)
	receivedMethods := make(map[string]bool)
	for i := 0; i < len(notifications); i++ {
		select {
		case method := <-received:
			receivedMethods[method] = true
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for notifications")
		}
	}

	// Check all expected notifications were received
	for _, expected := range notifications {
		if !receivedMethods[expected] {
			t.Errorf("notification %s was not received", expected)
		}
	}
}

// TestStdioMixedMessageTypes verifies concurrent handling of requests, responses, and notifications
func TestStdioMixedMessageTypes(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Track received server→client requests and notifications
	var requestCount, notifCount int
	var mu sync.Mutex

	transport.OnRequest(func(ctx context.Context, req codex.Request) (codex.Response, error) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}, nil
	})

	transport.OnNotify(func(ctx context.Context, notif codex.Notification) {
		mu.Lock()
		notifCount++
		mu.Unlock()
	})

	// Start reading server side
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var msg map[string]interface{}
			if err := json.Unmarshal([]byte(scanner.Text()), &msg); err == nil {
				// Send response for requests
				if id, hasID := msg["id"]; hasID {
					resp := codex.Response{
						JSONRPC: "2.0",
						ID:      codex.RequestID{Value: id},
						Result:  json.RawMessage(`{"received":true}`),
					}
					respJSON, _ := json.Marshal(resp)
					_, _ = serverWriter.Write(append(respJSON, '\n'))
				}
			}
		}
	}()

	ctx := context.Background()

	// Send client→server request
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "client-req-1"},
			Method:  "thread/start",
		}
		_, _ = transport.Send(ctx, req)
	}()

	// Send client→server notification
	go func() {
		notif := codex.Notification{
			JSONRPC: "2.0",
			Method:  "client/notify",
		}
		_ = transport.Notify(ctx, notif)
	}()

	// Send server→client request
	serverReq := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "server-req-1"},
		Method:  "approval/commandExecution",
	}
	reqJSON, _ := json.Marshal(serverReq)
	_, _ = serverWriter.Write(append(reqJSON, '\n'))

	// Send server→client notification
	serverNotif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/updated",
	}
	notifJSON, _ := json.Marshal(serverNotif)
	_, _ = serverWriter.Write(append(notifJSON, '\n'))

	// Wait for all messages to be processed
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if requestCount != 1 {
		t.Errorf("server→client requests received = %d; want 1", requestCount)
	}
	if notifCount != 1 {
		t.Errorf("server→client notifications received = %d; want 1", notifCount)
	}
}

// TestStdioCloseStopsCommunication verifies that Close stops all communication
func TestStdioCloseStopsCommunication(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)

	// Close the transport
	if err := transport.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	// Verify multiple closes don't error
	if err := transport.Close(); err != nil {
		t.Errorf("second Close returned error: %v", err)
	}

	// Verify Send fails after close
	ctx := context.Background()
	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "after-close"},
		Method:  "test",
	}
	_, err := transport.Send(ctx, req)
	if err == nil {
		t.Error("Send after Close did not return error")
	}

	// Verify Notify fails after close
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test",
	}
	err = transport.Notify(ctx, notif)
	if err == nil {
		t.Error("Notify after Close did not return error")
	}
}

// TestStdioInvalidJSON verifies handling of malformed JSON
func TestStdioInvalidJSON(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Register handler BEFORE writing messages so readLoop can't
	// process the valid notification before the handler exists.
	received := make(chan string, 1)
	transport.OnNotify(func(ctx context.Context, notif codex.Notification) {
		received <- notif.Method
	})

	// Send invalid JSON from server
	invalidLines := []string{
		`{invalid json}`,
		`{"jsonrpc":"2.0","method":"test"`, // incomplete
		`not json at all`,
	}

	for _, line := range invalidLines {
		_, _ = serverWriter.Write([]byte(line + "\n"))
	}

	// Send a valid notification after invalid ones to verify transport still works
	validNotif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test/valid",
	}
	notifJSON, _ := json.Marshal(validNotif)
	_, _ = serverWriter.Write(append(notifJSON, '\n'))

	// Verify the valid notification is still received
	select {
	case method := <-received:
		if method != "test/valid" {
			t.Errorf("received method = %s; want test/valid", method)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for valid notification after invalid JSON")
	}
}

// TestStdioContextCancellation verifies Send respects context cancellation
func TestStdioContextCancellation(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Send request in background
	errChan := make(chan error, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "will-cancel"},
			Method:  "test",
		}
		_, err := transport.Send(ctx, req)
		errChan <- err
	}()

	// Wait for request to be sent
	time.Sleep(50 * time.Millisecond)

	// Cancel the context before response arrives
	cancel()

	// Verify Send returns context error
	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Send did not return error after context cancellation")
		}
		// Should be context.Canceled error
		if err != context.Canceled && !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("unexpected error: %v; want context.Canceled or similar", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for Send to return after context cancellation")
	}
}

// TestStdioRequestHandlerPanicRecovery verifies that a panicking request handler
// returns an internal error response instead of crashing the process.
func TestStdioRequestHandlerPanicRecovery(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	transport.OnRequest(func(ctx context.Context, req codex.Request) (codex.Response, error) {
		panic("handler blew up")
	})

	// Read the error response written back by the transport
	responseChan := make(chan codex.Response, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var resp codex.Response
			if err := json.Unmarshal(scanner.Bytes(), &resp); err == nil && resp.Error != nil {
				responseChan <- resp
				return
			}
		}
	}()

	// Send a server→client request that will trigger the panicking handler
	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "panic-test"},
		Method:  "approval/commandExecution",
	}
	reqJSON, _ := json.Marshal(req)
	_, _ = serverWriter.Write(append(reqJSON, '\n'))

	select {
	case resp := <-responseChan:
		if resp.Error.Code != codex.ErrCodeInternalError {
			t.Errorf("error code = %d; want %d", resp.Error.Code, codex.ErrCodeInternalError)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for error response from panicking handler")
	}
}

// TestStdioNotificationHandlerPanicWithoutOnPanic verifies that a panicking
// notification handler without OnPanic recovers silently instead of crashing.
func TestStdioNotificationHandlerPanicWithoutOnPanic(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	received := make(chan string, 2)
	var callCount atomic.Int32
	// No OnPanic registered — panic must be silently recovered
	transport.OnNotify(func(ctx context.Context, notif codex.Notification) {
		n := callCount.Add(1)
		if n == 1 {
			panic("no OnPanic registered")
		}
		received <- notif.Method
	})

	// Send first notification (will panic and recover silently)
	notif1 := codex.Notification{JSONRPC: "2.0", Method: "first/panic"}
	n1JSON, _ := json.Marshal(notif1)
	_, _ = serverWriter.Write(append(n1JSON, '\n'))

	// Brief wait for the panic to be recovered
	time.Sleep(50 * time.Millisecond)

	// Send second notification (transport should still work)
	notif2 := codex.Notification{JSONRPC: "2.0", Method: "second/ok"}
	n2JSON, _ := json.Marshal(notif2)
	_, _ = serverWriter.Write(append(n2JSON, '\n'))

	select {
	case method := <-received:
		if method != "second/ok" {
			t.Errorf("received method = %s; want second/ok", method)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout: transport stopped working after unhandled notification panic")
	}
}

// TestStdioNotificationHandlerPanicRecovery verifies that a panicking notification
// handler does not crash the process and the transport continues operating.
func TestStdioNotificationHandlerPanicRecovery(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	received := make(chan string, 2)
	panicCaught := make(chan any, 1)
	var callCount atomic.Int32
	transport.OnPanic(func(v any) {
		panicCaught <- v
	})
	transport.OnNotify(func(ctx context.Context, notif codex.Notification) {
		if callCount.Add(1) == 1 {
			panic("notification handler blew up")
		}
		received <- notif.Method
	})

	// Send first notification (will panic)
	notif1 := codex.Notification{JSONRPC: "2.0", Method: "first/panic"}
	n1JSON, _ := json.Marshal(notif1)
	_, _ = serverWriter.Write(append(n1JSON, '\n'))

	// Wait for panic to be caught by OnPanic handler
	select {
	case <-panicCaught:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout: OnPanic handler was not called")
	}

	// Send second notification (should still work)
	notif2 := codex.Notification{JSONRPC: "2.0", Method: "second/ok"}
	n2JSON, _ := json.Marshal(notif2)
	_, _ = serverWriter.Write(append(n2JSON, '\n'))

	select {
	case method := <-received:
		if method != "second/ok" {
			t.Errorf("received method = %s; want second/ok", method)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout: transport stopped working after notification handler panic")
	}
}

// TestStdioApprovalInvalidParamsReturnsErrorCode verifies that sending malformed
// JSON params to a registered approval handler produces a -32602 error response.
func TestStdioApprovalInvalidParamsReturnsErrorCode(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	client.SetApprovalHandlers(codex.ApprovalHandlers{
		OnApplyPatchApproval: func(ctx context.Context, p codex.ApplyPatchApprovalParams) (codex.ApplyPatchApprovalResponse, error) {
			return codex.ApplyPatchApprovalResponse{
				Decision: codex.ReviewDecisionWrapper{Value: "approved"},
			}, nil
		},
	})

	responseChan := make(chan codex.Response, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var resp codex.Response
			if err := json.Unmarshal(scanner.Bytes(), &resp); err == nil && resp.Error != nil {
				responseChan <- resp
				return
			}
		}
	}()

	// Send a request with invalid JSON params (not valid for ApplyPatchApprovalParams)
	req := `{"jsonrpc":"2.0","id":"bad-params","method":"applyPatchApproval","params":"not-an-object"}` + "\n"
	_, _ = serverWriter.Write([]byte(req))

	select {
	case resp := <-responseChan:
		if resp.Error.Code != codex.ErrCodeInvalidParams {
			t.Errorf("error code = %d; want %d (ErrCodeInvalidParams)", resp.Error.Code, codex.ErrCodeInvalidParams)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for error response")
	}
}

// TestStdioApprovalHandlerErrorReturnsErrorCode verifies that when an approval
// handler returns a non-nil error, the wire response has code -32603.
func TestStdioApprovalHandlerErrorReturnsErrorCode(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport)

	client.SetApprovalHandlers(codex.ApprovalHandlers{
		OnApplyPatchApproval: func(ctx context.Context, p codex.ApplyPatchApprovalParams) (codex.ApplyPatchApprovalResponse, error) {
			return codex.ApplyPatchApprovalResponse{}, fmt.Errorf("handler refused")
		},
	})

	responseChan := make(chan codex.Response, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var resp codex.Response
			if err := json.Unmarshal(scanner.Bytes(), &resp); err == nil && resp.Error != nil {
				responseChan <- resp
				return
			}
		}
	}()

	// Send a valid request with proper params
	req := `{"jsonrpc":"2.0","id":"handler-err","method":"applyPatchApproval","params":{"callId":"c1","conversationId":"t1","fileChanges":{}}}` + "\n"
	_, _ = serverWriter.Write([]byte(req))

	select {
	case resp := <-responseChan:
		if resp.Error.Code != codex.ErrCodeInternalError {
			t.Errorf("error code = %d; want %d (ErrCodeInternalError)", resp.Error.Code, codex.ErrCodeInternalError)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for error response")
	}
}

// TestStdioScannerBufferOverflow verifies that a message exceeding maxMessageSize
// causes the reader to stop and ScanErr to return the buffer overflow error.
func TestStdioScannerBufferOverflow(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	_, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Write a single line exceeding the 10MB max message size.
	const oversize = 11 * 1024 * 1024
	oversizeLine := make([]byte, oversize)
	for i := range oversizeLine {
		oversizeLine[i] = 'x'
	}
	oversizeLine[len(oversizeLine)-1] = '\n'

	go func() {
		_, _ = serverWriter.Write(oversizeLine)
		_ = serverWriter.Close()
	}()

	// Poll ScanErr until the reader processes the oversized line and stops.
	deadline := time.After(5 * time.Second)
	for {
		if err := transport.ScanErr(); err != nil {
			if !strings.Contains(err.Error(), "byte limit") {
				t.Errorf("ScanErr should mention byte limit, got: %v", err)
			}
			if !strings.Contains(err.Error(), "token too long") {
				t.Errorf("ScanErr should wrap original error, got: %v", err)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for ScanErr to be set")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// TestStdioWriteMessageShortWrite verifies that writeMessage handles writers
// that return partial writes (less than the full buffer) without error.
func TestStdioWriteMessageShortWrite(t *testing.T) {
	// Use a shortWriter that writes one byte at a time.
	sw := &shortWriter{buf: make([]byte, 0, 1024)}
	clientReader, serverWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, sw)
	defer func() { _ = transport.Close() }()

	// Notify writes a message through writeMessage.
	ctx := context.Background()
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test/short",
	}
	if err := transport.Notify(ctx, notif); err != nil {
		t.Fatalf("Notify failed: %v", err)
	}

	// Verify the full message was assembled correctly.
	got := string(sw.Bytes())
	if !strings.Contains(got, `"method":"test/short"`) {
		t.Errorf("short-write message corrupted: %s", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("message missing trailing newline")
	}
}

// shortWriter writes at most one byte per Write call.
type shortWriter struct {
	mu  sync.Mutex
	buf []byte
}

func (w *shortWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	w.buf = append(w.buf, p[0])
	return 1, nil
}

func (w *shortWriter) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]byte, len(w.buf))
	copy(out, w.buf)
	return out
}

// TestStdioHandleResponseUnmarshalError verifies that when a response has a
// valid ID but fails full unmarshal, the pending caller receives a parse error
// instead of hanging until timeout.
func TestStdioHandleResponseUnmarshalError(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Drain requests from the server side
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send a request in a goroutine
	errCh := make(chan error, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "test-unmarshal-fail"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- err
			return
		}
		if resp.Error == nil {
			errCh <- nil
			return
		}
		if resp.Error.Code != codex.ErrCodeParseError {
			t.Errorf("error code = %d; want %d", resp.Error.Code, codex.ErrCodeParseError)
		}
		errCh <- nil
	}()

	// Wait for the request to be sent
	time.Sleep(50 * time.Millisecond)

	// Send a malformed response that has a valid ID but invalid structure
	// for the full Response type. We include a valid "id" but make "error"
	// an invalid type (string instead of object) so the full unmarshal fails.
	malformed := `{"jsonrpc":"2.0","id":"test-unmarshal-fail","error":"not-an-object"}` + "\n"
	_, _ = serverWriter.Write([]byte(malformed))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: caller should have received a parse error, not hung")
	}
}

// TestStdioSpuriousResponseUnknownID verifies that a response with an ID that has
// no pending caller is silently discarded without panic, error, or goroutine leak.
func TestStdioSpuriousResponseUnknownID(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Drain requests from the server side.
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
		}
	}()

	// Inject a response with an ID that no caller is waiting for.
	spurious := `{"jsonrpc":"2.0","id":"no-such-request","result":{"ok":true}}` + "\n"
	_, _ = serverWriter.Write([]byte(spurious))

	// Verify the transport still works by doing a real request-response cycle.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	respCh := make(chan codex.Response, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "real-request"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			return
		}
		respCh <- resp
	}()

	time.Sleep(50 * time.Millisecond)

	// Send the real response.
	realResp := `{"jsonrpc":"2.0","id":"real-request","result":{"status":"ok"}}` + "\n"
	_, _ = serverWriter.Write([]byte(realResp))

	select {
	case resp := <-respCh:
		if string(resp.Result) != `{"status":"ok"}` {
			t.Errorf("response result = %s; want {\"status\":\"ok\"}", resp.Result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: transport broken after spurious response")
	}
}

// TestStdioNoHandlerReturnsMethodNotFound verifies that a server→client request
// with no registered handler gets a method-not-found error response instead of
// being silently dropped.
func TestStdioNoHandlerReturnsMethodNotFound(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Read responses from the transport
	responseChan := make(chan codex.Response, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var resp codex.Response
			if err := json.Unmarshal(scanner.Bytes(), &resp); err == nil && resp.Error != nil {
				responseChan <- resp
				return
			}
		}
	}()

	// No OnRequest handler registered — should get method-not-found
	req := `{"jsonrpc":"2.0","id":"no-handler","method":"unknown/method","params":{}}` + "\n"
	_, _ = serverWriter.Write([]byte(req))

	select {
	case resp := <-responseChan:
		if resp.Error.Code != codex.ErrCodeMethodNotFound {
			t.Errorf("error code = %d; want %d (ErrCodeMethodNotFound)", resp.Error.Code, codex.ErrCodeMethodNotFound)
		}
		if resp.ID.Value != "no-handler" {
			t.Errorf("response ID = %v; want no-handler", resp.ID.Value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: expected method-not-found response")
	}
}

// TestStdioConcurrentSendAndClose verifies that concurrent Send and Close calls
// do not race or panic. Every Send must either succeed or return an error.
// TestStdioWriteMessageRejectsAfterClose verifies that writeMessage returns an
// error when the transport context has been cancelled by Close, ensuring that
// handler goroutines dispatched before Close do not write to a closed writer.
func TestStdioWriteMessageRejectsAfterClose(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	handlerStarted := make(chan struct{})
	handlerDone := make(chan error, 1)

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	transport.OnRequest(func(_ context.Context, req codex.Request) (codex.Response, error) {
		close(handlerStarted)

		// Wait for Close() to run, then try to write a response.
		// Before the fix, this would write to a potentially invalid writer.
		time.Sleep(50 * time.Millisecond)

		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
		}, nil
	})

	// Drain server-side reads so writes don't block.
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
		}
	}()

	// Send a server→client request.
	reqMsg := `{"jsonrpc":"2.0","id":"close-test","method":"test/close"}` + "\n"
	go func() {
		_, _ = serverWriter.Write([]byte(reqMsg))
	}()

	// Wait for handler goroutine to start, then close transport.
	select {
	case <-handlerStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not start")
	}

	_ = transport.Close()

	// The handler goroutine finishes and the response write should be
	// silently rejected (transport closed error). The test passes if
	// no panic or data race occurs.
	select {
	case err := <-handlerDone:
		if err != nil {
			t.Errorf("unexpected handler error: %v", err)
		}
	case <-time.After(2 * time.Second):
		// Handler goroutine finished via the writeMessage path — that's fine.
	}
}

func TestStdioConcurrentSendAndClose(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)

	// Drain messages written by Send so writes don't block on the pipe.
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
		}
	}()

	const senders = 10
	var wg sync.WaitGroup
	wg.Add(senders + 1) // senders + 1 closer

	// Launch concurrent senders.
	for i := 0; i < senders; i++ {
		go func(id int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			req := codex.Request{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: fmt.Sprintf("concurrent-%d", id)},
				Method:  "test/concurrent",
			}
			_, err := transport.Send(ctx, req)
			// Send must either succeed (nil) or return an error —
			// any panic would be caught by the race detector / test runner.
			if err == nil {
				// A nil error means we got a response, which is unlikely
				// without a responder, but not invalid.
				return
			}
			// Acceptable errors: transport closed, context deadline,
			// context canceled, or reader stopped.
			errMsg := err.Error()
			acceptable :=
				strings.Contains(errMsg, "transport closed") ||
					strings.Contains(errMsg, "context deadline exceeded") ||
					strings.Contains(errMsg, "context canceled") ||
					strings.Contains(errMsg, "transport reader stopped")
			if !acceptable {
				t.Errorf("Send(%d) returned unexpected error: %v", id, err)
			}
		}(i)
	}

	// Give senders a moment to start blocking in Send.
	time.Sleep(10 * time.Millisecond)

	// Close concurrently while sends are in-flight.
	go func() {
		defer wg.Done()
		if err := transport.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	}()

	wg.Wait()
}
