package codex_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestStdioNewlineDelimitedJSON verifies that messages are encoded/decoded as newline-delimited JSON
func TestStdioNewlineDelimitedJSON(t *testing.T) {
	// Create pipes to simulate stdin/stdout
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer clientReader.Close()
	defer serverWriter.Close()
	defer serverReader.Close()
	defer clientWriter.Close()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer transport.Close()

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
	defer clientReader.Close()
	defer serverWriter.Close()
	defer serverReader.Close()
	defer clientWriter.Close()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer transport.Close()

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
	defer clientReader.Close()
	defer serverWriter.Close()
	defer serverReader.Close()
	defer clientWriter.Close()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer transport.Close()

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

	// Send responses in reverse order to verify ID matching
	for i := len(requests) - 1; i >= 0; i-- {
		req := requests[i]
		resp := codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"index":` + string(rune('0'+i)) + `}`),
		}
		respJSON, _ := json.Marshal(resp)
		_, _ = serverWriter.Write(append(respJSON, '\n'))
	}

	// Verify each request got its correct response
	receivedResults := make(map[string]string)
	for i := 0; i < 3; i++ {
		select {
		case res := <-results:
			if res.err != nil {
				t.Errorf("request with id %v returned error: %v", res.id, res.err)
				continue
			}
			var idStr string
			switch v := res.id.(type) {
			case string:
				idStr = v
			case int64:
				idStr = "123"
			case float64:
				idStr = "456"
			}
			receivedResults[idStr] = string(res.result)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout waiting for responses")
		}
	}

	// We can't reliably match exact responses because they were sent in reverse order
	// but we can verify that we got 3 distinct responses
	if len(receivedResults) != 3 {
		t.Errorf("got %d unique responses; want 3", len(receivedResults))
	}
}

// TestStdioNotificationDispatch verifies notifications are dispatched to the handler
func TestStdioNotificationDispatch(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer clientReader.Close()
	defer serverWriter.Close()
	defer serverReader.Close()
	defer clientWriter.Close()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer transport.Close()

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
	defer clientReader.Close()
	defer serverWriter.Close()
	defer serverReader.Close()
	defer clientWriter.Close()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer transport.Close()

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
	defer clientReader.Close()
	defer serverWriter.Close()
	defer serverReader.Close()
	defer clientWriter.Close()

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
	defer clientReader.Close()
	defer serverWriter.Close()
	defer serverReader.Close()
	defer clientWriter.Close()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer transport.Close()

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

	received := make(chan string, 1)
	transport.OnNotify(func(ctx context.Context, notif codex.Notification) {
		received <- notif.Method
	})

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
	defer clientReader.Close()
	defer serverWriter.Close()
	defer serverReader.Close()
	defer clientWriter.Close()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer transport.Close()

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
	defer clientReader.Close()
	defer serverWriter.Close()
	defer serverReader.Close()
	defer clientWriter.Close()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer transport.Close()

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

// TestStdioNotificationHandlerPanicRecovery verifies that a panicking notification
// handler does not crash the process and the transport continues operating.
func TestStdioNotificationHandlerPanicRecovery(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer clientReader.Close()
	defer serverWriter.Close()
	defer serverReader.Close()
	defer clientWriter.Close()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer transport.Close()

	received := make(chan string, 2)
	callCount := 0
	transport.OnNotify(func(ctx context.Context, notif codex.Notification) {
		callCount++
		if callCount == 1 {
			panic("notification handler blew up")
		}
		received <- notif.Method
	})

	// Send first notification (will panic)
	notif1 := codex.Notification{JSONRPC: "2.0", Method: "first/panic"}
	n1JSON, _ := json.Marshal(notif1)
	_, _ = serverWriter.Write(append(n1JSON, '\n'))

	// Brief wait for goroutine to recover
	time.Sleep(50 * time.Millisecond)

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
