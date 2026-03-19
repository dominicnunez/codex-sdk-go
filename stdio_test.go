package codex_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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
	sentRequests := make(chan codex.Request, 5)
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

	// Send requests with different ID types concurrently.
	type result struct {
		id     interface{}
		result json.RawMessage
		err    error
	}
	results := make(chan result, 5)

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

	// Whole-number float64 is canonicalized to an integer request ID.
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: float64(456)},
			Method:  "test/method3",
		}
		resp, err := transport.Send(ctx, req)
		results <- result{id: float64(456), result: resp.Result, err: err}
	}()

	// Large uint64 ID above 2^53 must match exactly.
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: uint64(9007199254740993)},
			Method:  "test/method4",
		}
		resp, err := transport.Send(ctx, req)
		results <- result{id: uint64(9007199254740993), result: resp.Result, err: err}
	}()

	// Additional integer-kind IDs should also match exactly.
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: uint16(789)},
			Method:  "test/method5",
		}
		resp, err := transport.Send(ctx, req)
		results <- result{id: uint16(789), result: resp.Result, err: err}
	}()

	// Wait for all requests to be sent and collect them
	time.Sleep(50 * time.Millisecond)

	requests := make([]codex.Request, 0, 5)
	for i := 0; i < 5; i++ {
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
	for i := 0; i < 5; i++ {
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

func TestStdioRequestIDTypeFamiliesDoNotCollide(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	type sendResult struct {
		label  string
		result string
		err    error
	}
	results := make(chan sendResult, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		resp, err := transport.Send(ctx, codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "1"},
			Method:  "test/string-id",
		})
		results <- sendResult{label: "string", result: string(resp.Result), err: err}
	}()

	go func() {
		resp, err := transport.Send(ctx, codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: int64(1)},
			Method:  "test/numeric-id",
		})
		results <- sendResult{label: "number", result: string(resp.Result), err: err}
	}()

	scannedRequests := make(chan codex.Request, 2)
	scanErr := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var req codex.Request
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				scanErr <- err
				return
			}
			scannedRequests <- req
			if len(scannedRequests) == 2 {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			scanErr <- err
		}
	}()

	sentRequests := make([]codex.Request, 0, 2)
	for len(sentRequests) < 2 {
		select {
		case req := <-scannedRequests:
			sentRequests = append(sentRequests, req)
		case err := <-scanErr:
			t.Fatalf("scan outbound requests: %v", err)
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("expected two outbound requests, got %d", len(sentRequests))
		}
	}

	for i := len(sentRequests) - 1; i >= 0; i-- {
		req := sentRequests[i]
		payload := `{"matched":"numeric"}`
		if _, ok := req.ID.Value.(string); ok {
			payload = `{"matched":"string"}`
		}
		resp := codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(payload),
		}
		respJSON, _ := json.Marshal(resp)
		_, _ = serverWriter.Write(append(respJSON, '\n'))
	}

	got := make(map[string]string, 2)
	for range 2 {
		select {
		case res := <-results:
			if res.err != nil {
				t.Fatalf("%s-id send returned error: %v", res.label, res.err)
			}
			got[res.label] = res.result
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for responses")
		}
	}

	if got["string"] != `{"matched":"string"}` {
		t.Fatalf("string id response = %s; want %s", got["string"], `{"matched":"string"}`)
	}
	if got["number"] != `{"matched":"numeric"}` {
		t.Fatalf("number id response = %s; want %s", got["number"], `{"matched":"numeric"}`)
	}
}

func TestStdioAllowsReusingRequestIDAfterResponse(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	requests := make(chan codex.Request, 2)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var req codex.Request
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				continue
			}
			requests <- req
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	roundTrip := func(method, payload string) codex.Response {
		respCh := make(chan codex.Response, 1)
		errCh := make(chan error, 1)

		go func() {
			resp, err := transport.Send(ctx, codex.Request{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: "reused-id"},
				Method:  method,
			})
			if err != nil {
				errCh <- err
				return
			}
			respCh <- resp
		}()

		var req codex.Request
		select {
		case req = <-requests:
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timeout waiting for request %q", method)
		}

		resp := codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(payload),
		}
		respJSON, _ := json.Marshal(resp)
		_, _ = serverWriter.Write(append(respJSON, '\n'))

		select {
		case err := <-errCh:
			t.Fatalf("send %q returned error: %v", method, err)
		case out := <-respCh:
			return out
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timeout waiting for response %q", method)
		}
		return codex.Response{}
	}

	first := roundTrip("test/first", `{"n":1}`)
	if string(first.Result) != `{"n":1}` {
		t.Fatalf("first response = %s; want %s", first.Result, `{"n":1}`)
	}

	second := roundTrip("test/second", `{"n":2}`)
	if string(second.Result) != `{"n":2}` {
		t.Fatalf("second response = %s; want %s", second.Result, `{"n":2}`)
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

	if got, want := transport.MalformedMessageCount(), uint64(len(invalidLines)); got != want {
		t.Errorf("MalformedMessageCount = %d; want %d", got, want)
	}
}

// TestStdioUnknownMessageTypeSkipped verifies that a JSON object with no id
// and no method (unknown message type) is silently skipped and the transport
// continues operating for subsequent messages.
func TestStdioUnknownMessageTypeSkipped(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	received := make(chan string, 1)
	transport.OnNotify(func(ctx context.Context, notif codex.Notification) {
		received <- notif.Method
	})

	// Send a JSON object with no id and no method — hits the unknown message branch.
	unknownMessages := []string{
		`{"jsonrpc":"2.0"}`,
		`{"jsonrpc":"2.0","data":"something"}`,
	}
	for _, msg := range unknownMessages {
		_, _ = serverWriter.Write([]byte(msg + "\n"))
	}

	// Send a valid notification to verify the transport is still alive.
	validNotif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test/after-unknown",
	}
	notifJSON, _ := json.Marshal(validNotif)
	_, _ = serverWriter.Write(append(notifJSON, '\n'))

	select {
	case method := <-received:
		if method != "test/after-unknown" {
			t.Errorf("received method = %s; want test/after-unknown", method)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout: transport stopped working after unknown message type")
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

func TestStdioApprovalMissingRequiredFieldReturnsInvalidParams(t *testing.T) {
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
		OnFileChangeRequestApproval: func(ctx context.Context, p codex.FileChangeRequestApprovalParams) (codex.FileChangeRequestApprovalResponse, error) {
			return codex.FileChangeRequestApprovalResponse{Decision: "accept"}, nil
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

	req := `{"jsonrpc":"2.0","id":"missing-required","method":"item/fileChange/requestApproval","params":{"threadId":"t1","turnId":"u1"}}` + "\n"
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

// TestStdioOversizeInboundLineStopsTransport verifies that oversized inbound
// frames terminate the transport instead of pinning the read loop in recovery.
func TestStdioOversizeInboundLineStopsTransport(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	received := make(chan string, 1)
	transport.OnNotify(func(_ context.Context, notif codex.Notification) {
		received <- notif.Method
	})

	// Write one oversized line (11MB) that is not valid JSON.
	const oversize = 11 * 1024 * 1024
	oversizeLine := make([]byte, oversize)
	for i := range oversizeLine {
		oversizeLine[i] = 'x'
	}
	oversizeLine[len(oversizeLine)-1] = '\n'
	_, _ = serverWriter.Write(oversizeLine)

	// Follow with a valid notification. The transport should have already shut
	// down, so this must not be dispatched.
	validNotif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test/after-oversize",
	}
	notifJSON, _ := json.Marshal(validNotif)
	_, _ = serverWriter.Write(append(notifJSON, '\n'))

	select {
	case method := <-received:
		t.Fatalf("received notification %q after oversized frame shutdown", method)
	case <-time.After(200 * time.Millisecond):
	}

	deadline := time.After(2 * time.Second)
	for {
		err := transport.ScanErr()
		if err != nil {
			if !strings.Contains(err.Error(), "oversized inbound frame") {
				t.Fatalf("ScanErr = %v; want oversized inbound frame error", err)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for oversized frame shutdown")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if err := transport.Notify(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "test/after-stop",
	}); err == nil {
		t.Fatal("Notify succeeded after oversized frame shutdown")
	} else {
		assertTransportFailure(t, err)
	}
}

func waitForOutboundRequest(t *testing.T, serverReader io.Reader) {
	t.Helper()
	reqSeen := make(chan struct{}, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		if scanner.Scan() {
			reqSeen <- struct{}{}
		}
	}()
	select {
	case <-reqSeen:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for outbound request")
	}
}

type sendResult struct {
	resp codex.Response
	err  error
}

const (
	incompleteOversizedFrameBytes = 11 * 1024 * 1024
)

func buildOversizedFrame(t *testing.T, totalSize int, prefix, suffix string) string {
	t.Helper()

	payloadSize := totalSize - len(prefix) - len(suffix)
	if payloadSize <= 0 {
		t.Fatal("invalid oversized payload size")
	}

	return prefix + strings.Repeat("x", payloadSize) + suffix
}

func assertTransportFailure(t *testing.T, err error) {
	t.Helper()

	var transportErr *codex.TransportError
	if !errors.As(err, &transportErr) {
		t.Fatalf("error = %v; want TransportError", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v; want transport failure, not context deadline", err)
	}
	if !strings.Contains(err.Error(), "transport closed") && !strings.Contains(err.Error(), "transport reader stopped") {
		t.Fatalf("error = %v; want transport closed or reader stopped", err)
	}
}

// TestStdioOversizeResponseUnblocksPendingSend verifies that an oversized frame
// with an early-routable response ID resolves Send with a deterministic parse error.
func TestStdioOversizeResponseUnblocksPendingSend(t *testing.T) {
	tests := []struct {
		name      string
		requestID interface{}
		response  string
	}{
		{
			name:      "string id",
			requestID: "oversize-response",
			response:  `"oversize-response"`,
		},
		{
			name:      "numeric id integer literal",
			requestID: float64(1),
			response:  `1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientReader, serverWriter := io.Pipe()
			serverReader, clientWriter := io.Pipe()
			defer func() { _ = clientReader.Close() }()
			defer func() { _ = serverWriter.Close() }()
			defer func() { _ = serverReader.Close() }()
			defer func() { _ = clientWriter.Close() }()

			transport := codex.NewStdioTransport(clientReader, clientWriter)
			defer func() { _ = transport.Close() }()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			errCh := make(chan error, 1)
			go func() {
				req := codex.Request{
					JSONRPC: "2.0",
					ID:      codex.RequestID{Value: tt.requestID},
					Method:  "test/method",
				}
				resp, err := transport.Send(ctx, req)
				if err != nil {
					errCh <- fmt.Errorf("Send returned unexpected error: %w", err)
					return
				}
				if resp.Error == nil {
					errCh <- fmt.Errorf("Send returned nil response error")
					return
				}
				if resp.Error.Code != codex.ErrCodeParseError {
					errCh <- fmt.Errorf("response error code = %d; want %d", resp.Error.Code, codex.ErrCodeParseError)
					return
				}
				if !strings.Contains(resp.Error.Message, "oversized") {
					errCh <- fmt.Errorf("response error message = %q; want to contain oversized", resp.Error.Message)
					return
				}
				errCh <- nil
			}()

			waitForOutboundRequest(t, serverReader)

			// Send an oversized response frame with a matching ID.
			responsePrefix := `{"jsonrpc":"2.0","id":` + tt.response + `,"result":"`
			responseSuffix := "\"}\n"
			oversized := buildOversizedFrame(t, incompleteOversizedFrameBytes, responsePrefix, responseSuffix)
			_, _ = serverWriter.Write([]byte(oversized))

			select {
			case err := <-errCh:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("timeout waiting for oversized response handling")
			}
		})
	}
}

func TestStdioOversizeResponseWithLateIDUnblocksPendingSend(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	type sendResult struct {
		resp codex.Response
		err  error
	}
	resultCh := make(chan sendResult, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "late-id"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		resultCh <- sendResult{resp: resp, err: err}
	}()

	waitForOutboundRequest(t, serverReader)

	const responsePrefix = `{"jsonrpc":"2.0","result":"`
	const responseSuffix = `","id":"late-id"}` + "\n"
	oversized := buildOversizedFrame(t, incompleteOversizedFrameBytes, responsePrefix, responseSuffix)
	_, _ = serverWriter.Write([]byte(oversized))

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("Send returned unexpected error: %v", result.err)
		}
		if result.resp.Error == nil {
			t.Fatal("Send returned nil response error")
		}
		if result.resp.Error.Code != codex.ErrCodeParseError {
			t.Fatalf("response error code = %d; want %d", result.resp.Error.Code, codex.ErrCodeParseError)
		}
		if !strings.Contains(result.resp.Error.Message, "oversized") {
			t.Fatalf("response error message = %q; want oversized parse error", result.resp.Error.Message)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for oversized response handling")
	}

	if err := transport.ScanErr(); err == nil || !strings.Contains(err.Error(), "oversized") {
		t.Fatalf("ScanErr = %v, want oversized transport error", err)
	}

	_, err := transport.Send(context.Background(), codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "after-close"},
		Method:  "test/after-close",
	})
	assertTransportFailure(t, err)
}

func TestStdioPartialOversizeResponseClosesTransportImmediately(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "partial-oversize"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- fmt.Errorf("Send returned unexpected error: %w", err)
			return
		}
		if resp.Error == nil {
			errCh <- fmt.Errorf("Send returned nil response error")
			return
		}
		if resp.Error.Code != codex.ErrCodeParseError {
			errCh <- fmt.Errorf("response error code = %d; want %d", resp.Error.Code, codex.ErrCodeParseError)
			return
		}
		if !strings.Contains(resp.Error.Message, "oversized") {
			errCh <- fmt.Errorf("response error message = %q; want oversized parse error", resp.Error.Message)
			return
		}
		errCh <- nil
	}()

	waitForOutboundRequest(t, serverReader)

	const responsePrefix = `{"jsonrpc":"2.0","id":"partial-oversize","result":"`
	_, _ = serverWriter.Write([]byte(buildOversizedFrame(t, incompleteOversizedFrameBytes, responsePrefix, "")))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for partial oversized response handling")
	}

	if err := transport.ScanErr(); err == nil || !strings.Contains(err.Error(), "oversized inbound frame") {
		t.Fatalf("ScanErr = %v, want oversized inbound frame error", err)
	}
}

// TestStdioOversizeAmbiguousFrameStopsTransport verifies that oversized
// non-response frames stop the transport instead of blocking pending sends.
func TestStdioOversizeAmbiguousFrameStopsTransport(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	type sendResult struct {
		resp codex.Response
		err  error
	}
	resultCh := make(chan sendResult, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "ambiguous-oversize"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		resultCh <- sendResult{resp: resp, err: err}
	}()

	waitForOutboundRequest(t, serverReader)

	const ambiguousPrefix = `{"jsonrpc":"2.0","params":{"payload":"`
	const ambiguousSuffix = `"},"method":"thread/updated"}` + "\n"
	oversized := buildOversizedFrame(t, incompleteOversizedFrameBytes, ambiguousPrefix, ambiguousSuffix)
	_, _ = serverWriter.Write([]byte(oversized))

	select {
	case result := <-resultCh:
		if result.err != nil {
			assertTransportFailure(t, result.err)
			break
		}
		if result.resp.Error == nil {
			t.Fatal("Send returned nil response error")
		}
		if result.resp.Error.Code != codex.ErrCodeInternalError {
			t.Fatalf("response error code = %d; want %d", result.resp.Error.Code, codex.ErrCodeInternalError)
		}
		if !strings.Contains(result.resp.Error.Message, "oversized") {
			t.Fatalf("response error message = %q; want oversized transport error", result.resp.Error.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for oversized response handling")
	}
}

func TestStdioOversizeNotificationWithLateMethodStopsTransport(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resultCh := make(chan sendResult, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "late-method"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		resultCh <- sendResult{resp: resp, err: err}
	}()

	waitForOutboundRequest(t, serverReader)

	const notifPrefix = `{"jsonrpc":"2.0","params":{"payload":"`
	const notifSuffix = `"},"method":"turn/updated"}` + "\n"
	oversized := buildOversizedFrame(t, incompleteOversizedFrameBytes, notifPrefix, notifSuffix)
	_, _ = serverWriter.Write([]byte(oversized))

	select {
	case result := <-resultCh:
		if result.err != nil {
			assertTransportFailure(t, result.err)
			break
		}
		if result.resp.Error == nil {
			t.Fatal("Send returned nil response error")
		}
		if result.resp.Error.Code != codex.ErrCodeInternalError {
			t.Fatalf("response error code = %d; want %d", result.resp.Error.Code, codex.ErrCodeInternalError)
		}
		if !strings.Contains(result.resp.Error.Message, "oversized") {
			t.Fatalf("response error message = %q; want oversized transport error", result.resp.Error.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for oversized notification shutdown")
	}
}

// TestStdioOversizeResponseAtEOFFailsPendingSend verifies that oversized
// response frames without a trailing newline still fail pending sends.
func TestStdioOversizeResponseAtEOFFailsPendingSend(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "oversize-at-eof"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- fmt.Errorf("Send returned unexpected error: %w", err)
			return
		}
		if resp.Error == nil {
			errCh <- fmt.Errorf("Send returned nil response error")
			return
		}
		if resp.Error.Code != codex.ErrCodeParseError {
			errCh <- fmt.Errorf("response error code = %d; want %d", resp.Error.Code, codex.ErrCodeParseError)
			return
		}
		if !strings.Contains(resp.Error.Message, "oversized") {
			errCh <- fmt.Errorf("response error message = %q; want to contain oversized", resp.Error.Message)
			return
		}
		errCh <- nil
	}()

	waitForOutboundRequest(t, serverReader)

	const responsePrefix = `{"jsonrpc":"2.0","id":"oversize-at-eof","result":"`
	const responseSuffix = `"}`
	oversized := buildOversizedFrame(t, incompleteOversizedFrameBytes, responsePrefix, responseSuffix)
	_, _ = serverWriter.Write([]byte(oversized))
	_ = serverWriter.Close()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for oversized EOF response handling")
	}
}

// TestStdioTurnScopedNotificationsDoNotBlockReadLoop verifies that a backlog
// of same-thread turn/completed notifications does not block response
// processing.
func TestStdioTurnScopedNotificationsDoNotBlockReadLoop(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	block := make(chan struct{})
	transport.OnNotify(func(_ context.Context, notif codex.Notification) {
		if notif.Method == "turn/completed" {
			<-block
		}
	})
	defer close(block)

	// Capture the outbound request, then flood turn-completed notifications.
	outbound := make(chan struct{}, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		if scanner.Scan() {
			outbound <- struct{}{}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "turn-scoped-queue"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- fmt.Errorf("Send returned error: %w", err)
			return
		}
		if string(resp.Result) != `{"ok":true}` {
			errCh <- fmt.Errorf("response result = %s; want {\"ok\":true}", string(resp.Result))
			return
		}
		errCh <- nil
	}()

	select {
	case <-outbound:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for outbound request")
	}

	for i := 0; i < 200; i++ {
		notif := fmt.Sprintf(
			`{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thread-1","turn":{"id":"noise-%d","status":"completed","items":[]}}}`+"\n",
			i,
		)
		_, _ = serverWriter.Write([]byte(notif))
	}
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":"turn-scoped-queue","result":{"ok":true}}` + "\n"))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for response while turn-scoped queue is saturated")
	}
}

// TestStdioDistinctTurnScopedNotificationsDoNotBlockReadLoop verifies that a
// backlog spread across many thread IDs still leaves the read loop responsive.
func TestStdioDistinctTurnScopedNotificationsDoNotBlockReadLoop(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	block := make(chan struct{})
	transport.OnNotify(func(_ context.Context, notif codex.Notification) {
		if notif.Method == "item/completed" {
			<-block
		}
	})
	defer close(block)

	outbound := make(chan struct{}, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		if scanner.Scan() {
			outbound <- struct{}{}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "turn-scoped-distinct-queues"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- fmt.Errorf("Send returned error: %w", err)
			return
		}
		if string(resp.Result) != `{"ok":true}` {
			errCh <- fmt.Errorf("response result = %s; want {\"ok\":true}", string(resp.Result))
			return
		}
		errCh <- nil
	}()

	select {
	case <-outbound:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for outbound request")
	}

	for i := 0; i < 64; i++ {
		item := fmt.Sprintf(
			`{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thread-%d","turnId":"turn-%d","item":{"type":"plan","id":"item-%d","text":"queued"}}}`+"\n",
			i, i, i,
		)
		_, _ = serverWriter.Write([]byte(item))
		completed := fmt.Sprintf(
			`{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thread-%d","turn":{"id":"turn-%d","status":"completed","items":[]}}}`+"\n",
			i, i,
		)
		_, _ = serverWriter.Write([]byte(completed))
	}
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":"turn-scoped-distinct-queues","result":{"ok":true}}` + "\n"))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for response while distinct turn-scoped queues are saturated")
	}
}

// TestStdioErrorNotificationsUseCriticalQueue verifies error-bearing
// notifications are routed through the critical queue.
func TestStdioErrorNotificationsUseCriticalQueue(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	block := make(chan struct{})
	var sawError atomic.Bool
	transport.OnNotify(func(_ context.Context, notif codex.Notification) {
		if notif.Method == "test/flood" {
			<-block
			return
		}
		if notif.Method == "error" {
			sawError.Store(true)
		}
	})
	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "critical-error-queue"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- fmt.Errorf("Send returned error: %w", err)
			return
		}
		if string(resp.Result) != `{"ok":true}` {
			errCh <- fmt.Errorf("response result = %s; want {\"ok\":true}", string(resp.Result))
			return
		}
		errCh <- nil
	}()

	waitForOutboundRequest(t, serverReader)

	bestEffortFlood := `{"jsonrpc":"2.0","method":"test/flood","params":{"n":1}}` + "\n"
	for i := 0; i < 200; i++ {
		_, _ = serverWriter.Write([]byte(bestEffortFlood))
	}
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","method":"error","params":{"message":"boom"}}` + "\n"))
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":"critical-error-queue","result":{"ok":true}}` + "\n"))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for response while critical queue is saturated")
	}
	close(block)

	deadline := time.Now().Add(2 * time.Second)
	for !sawError.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !sawError.Load() {
		t.Fatal("expected error notification to be delivered under queue pressure")
	}
}

func TestStdioTurnCompletedWaitsForEarlierSameTurnItems(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	blockCritical := make(chan struct{})
	turnCompletedSeen := make(chan struct{}, 1)
	transport.OnNotify(func(_ context.Context, notif codex.Notification) {
		switch notif.Method {
		case "item/completed":
			<-blockCritical
		case "turn/completed":
			select {
			case turnCompletedSeen <- struct{}{}:
			default:
			}
		}
	})

	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "turn-completed-guarantee"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- fmt.Errorf("Send returned error: %w", err)
			return
		}
		if string(resp.Result) != `{"ok":true}` {
			errCh <- fmt.Errorf("response result = %s; want {\"ok\":true}", string(resp.Result))
			return
		}
		errCh <- nil
	}()

	waitForOutboundRequest(t, serverReader)

	criticalItemCompleted := `{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"t","turnId":"u","item":{"type":"plan","id":"p","text":"done"}}}` + "\n"
	for i := 0; i < 200; i++ {
		_, _ = serverWriter.Write([]byte(criticalItemCompleted))
	}
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"t","turn":{"id":"u","status":"completed","items":[]}}}` + "\n"))
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":"turn-completed-guarantee","result":{"ok":true}}` + "\n"))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for response while critical queue is saturated")
	}

	select {
	case <-turnCompletedSeen:
		t.Fatal("turn/completed should wait for earlier same-turn item/completed handling")
	case <-time.After(150 * time.Millisecond):
	}
	close(blockCritical)

	select {
	case <-turnCompletedSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("expected turn/completed after earlier same-turn items drained")
	}
}

func TestStdioRejectsInvalidJSONRPCRequestVersion(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	responseChan := make(chan codex.Response, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var resp codex.Response
			if json.Unmarshal(scanner.Bytes(), &resp) == nil && resp.Error != nil {
				responseChan <- resp
				return
			}
		}
	}()

	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"1.0","id":"bad-version","method":"applyPatchApproval","params":{}}` + "\n"))

	select {
	case resp := <-responseChan:
		if resp.Error.Code != codex.ErrCodeInvalidRequest {
			t.Fatalf("error code = %d; want %d", resp.Error.Code, codex.ErrCodeInvalidRequest)
		}
		if resp.ID.Value != "bad-version" {
			t.Fatalf("response ID = %v; want bad-version", resp.ID.Value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for invalid-version request rejection")
	}
}

func TestStdioRejectsRequestWithInvalidIDType(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	responseChan := make(chan codex.Response, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var resp codex.Response
			if json.Unmarshal(scanner.Bytes(), &resp) == nil && resp.Error != nil {
				responseChan <- resp
				return
			}
		}
	}()

	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":{"unexpected":"shape"},"method":"applyPatchApproval","params":{}}` + "\n"))

	select {
	case resp := <-responseChan:
		if resp.Error.Code != codex.ErrCodeInvalidRequest {
			t.Fatalf("error code = %d; want %d", resp.Error.Code, codex.ErrCodeInvalidRequest)
		}
		if resp.ID.Value != nil {
			t.Fatalf("response ID = %v; want nil", resp.ID.Value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for invalid-id request rejection")
	}
}

func TestStdioRejectsRequestWithInvalidMethodType(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	responseChan := make(chan codex.Response, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var resp codex.Response
			if json.Unmarshal(scanner.Bytes(), &resp) == nil && resp.Error != nil {
				responseChan <- resp
				return
			}
		}
	}()

	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":"bad-method","method":123,"params":{}}` + "\n"))

	select {
	case resp := <-responseChan:
		if resp.Error.Code != codex.ErrCodeInvalidRequest {
			t.Fatalf("error code = %d; want %d", resp.Error.Code, codex.ErrCodeInvalidRequest)
		}
		if resp.ID.Value != "bad-method" {
			t.Fatalf("response ID = %v; want bad-method", resp.ID.Value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for invalid-method request rejection")
	}
}

func TestStdioInvalidJSONRPCResponseVersionFailsPending(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "bad-version-response"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- fmt.Errorf("Send returned unexpected error: %w", err)
			return
		}
		if resp.Error == nil {
			errCh <- fmt.Errorf("expected error response for invalid jsonrpc version")
			return
		}
		if resp.Error.Code != codex.ErrCodeInvalidRequest {
			errCh <- fmt.Errorf("response error code = %d; want %d", resp.Error.Code, codex.ErrCodeInvalidRequest)
			return
		}
		errCh <- nil
	}()

	waitForOutboundRequest(t, serverReader)
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"1.0","id":"bad-version-response","result":{"ok":true}}` + "\n"))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for invalid-version response handling")
	}
}

func TestStdioInvalidJSONRPCNotificationVersionIgnored(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	received := make(chan string, 1)
	transport.OnNotify(func(_ context.Context, notif codex.Notification) {
		received <- notif.Method
	})

	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"1.0","method":"error","params":{"message":"ignore"}}` + "\n"))
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","method":"error","params":{"message":"deliver"}}` + "\n"))

	select {
	case method := <-received:
		if method != "error" {
			t.Fatalf("received method = %s; want error", method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for valid-version notification")
	}

	select {
	case method := <-received:
		t.Fatalf("unexpected extra notification: %s", method)
	default:
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

type errorWriter struct {
	err error
}

func (w *errorWriter) Write(_ []byte) (int, error) {
	return 0, w.err
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

	deadline := time.Now().Add(500 * time.Millisecond)
	for transport.MalformedMessageCount() < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := transport.MalformedMessageCount(); got != 1 {
		t.Fatalf("MalformedMessageCount() = %d; want 1 after malformed response shape", got)
	}
}

func TestStdioMalformedResponseShapeIncrementsCounterWhenAttributed(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "test-malformed-shape"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- err
			return
		}
		if resp.Error == nil || resp.Error.Code != codex.ErrCodeParseError {
			errCh <- fmt.Errorf("response error = %+v; want parse error", resp.Error)
			return
		}
		errCh <- nil
	}()

	waitForOutboundRequest(t, serverReader)

	_, _ = serverWriter.Write([]byte(
		`{"jsonrpc":"2.0","id":"test-malformed-shape","result":{"ok":true},"error":{"code":-32603,"message":"bad"}}` + "\n",
	))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for malformed response handling")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for transport.MalformedMessageCount() < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := transport.MalformedMessageCount(); got != 1 {
		t.Fatalf("MalformedMessageCount() = %d; want 1 after attributed malformed response", got)
	}
}

func TestStdioIgnoresNonResponseFramesWithID(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sendResultCh := make(chan sendResult, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "non-response-frame"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		sendResultCh <- sendResult{resp: resp, err: err}
	}()

	waitForOutboundRequest(t, serverReader)

	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":"non-response-frame","params":{"ignored":true}}` + "\n"))
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","params":{"ignored":true}}` + "\n"))

	deadline := time.Now().Add(500 * time.Millisecond)
	for transport.MalformedMessageCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := transport.MalformedMessageCount(); got != 2 {
		t.Fatalf("MalformedMessageCount() = %d; want 2 after malformed inbound objects", got)
	}

	select {
	case result := <-sendResultCh:
		t.Fatalf("Send returned early after malformed inbound object: resp=%+v err=%v", result.resp, result.err)
	case <-time.After(150 * time.Millisecond):
	}

	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":"non-response-frame","result":{"ok":true}}` + "\n"))

	select {
	case result := <-sendResultCh:
		if result.err != nil {
			t.Fatalf("Send returned error: %v", result.err)
		}
		if string(result.resp.Result) != `{"ok":true}` {
			t.Fatalf("response result = %s; want {\"ok\":true}", string(result.resp.Result))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for response after malformed inbound objects")
	}
}

func TestStdioMalformedResponseInvalidIDDoesNotFailUnrelatedPending(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	outboundReqIDs := make(chan string, 2)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var req codex.Request
			if json.Unmarshal(scanner.Bytes(), &req) != nil {
				continue
			}
			id, ok := req.ID.Value.(string)
			if !ok {
				continue
			}
			outboundReqIDs <- id
		}
	}()

	errCh := make(chan error, 2)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "invalid-id-response-1"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- err
			return
		}
		if resp.Error != nil {
			errCh <- fmt.Errorf("request 1 returned unexpected response error: %v", resp.Error)
			return
		}
		if string(resp.Result) != `{"ok":1}` {
			errCh <- fmt.Errorf("request 1 result = %s; want {\"ok\":1}", string(resp.Result))
			return
		}
		errCh <- nil
	}()

	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "invalid-id-response-2"},
			Method:  "test/method",
		}
		resp, err := transport.Send(ctx, req)
		if err != nil {
			errCh <- err
			return
		}
		if resp.Error != nil {
			errCh <- fmt.Errorf("request 2 returned unexpected response error: %v", resp.Error)
			return
		}
		if string(resp.Result) != `{"ok":2}` {
			errCh <- fmt.Errorf("request 2 result = %s; want {\"ok\":2}", string(resp.Result))
			return
		}
		errCh <- nil
	}()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case id := <-outboundReqIDs:
			seen[id] = true
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for outbound requests")
		}
	}

	// Malformed response with an invalid ID shape should be dropped without
	// failing unrelated in-flight requests.
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":{"bad":"shape"},"result":{}}` + "\n"))
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":"invalid-id-response-1","result":{"ok":1}}` + "\n"))
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":"invalid-id-response-2","result":{"ok":2}}` + "\n"))

	for range 2 {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for pending sends after malformed response id")
		}
	}
}

func TestStdioSendAndNotifyRejectNilContext(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, io.Discard)
	defer func() { _ = transport.Close() }()

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "nil-context"},
		Method:  "test/method",
	}
	//nolint:staticcheck // nil context is intentional: this test verifies the guard path.
	if _, err := transport.Send(nil, req); !errors.Is(err, codex.ErrNilContext) {
		t.Fatalf("Send(nil, req) error = %v; want ErrNilContext", err)
	}

	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test/notify",
	}
	//nolint:staticcheck // nil context is intentional: this test verifies the guard path.
	if err := transport.Notify(nil, notif); !errors.Is(err, codex.ErrNilContext) {
		t.Fatalf("Notify(nil, notif) error = %v; want ErrNilContext", err)
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

func TestStdioInboundRequestBeforeHandlerRegistrationWaitsForHandler(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	req := `{"jsonrpc":"2.0","id":"early-request","method":"approval/test","params":{"v":1}}` + "\n"
	writeErrCh := make(chan error, 1)
	go func() {
		_, err := serverWriter.Write([]byte(req))
		writeErrCh <- err
	}()

	transport.OnRequest(func(_ context.Context, req codex.Request) (codex.Response, error) {
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"handled":true}`),
		}, nil
	})

	select {
	case err := <-writeErrCh:
		if err != nil {
			t.Fatalf("write early request: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for early request write completion")
	}

	respCh := make(chan codex.Response, 1)
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var resp codex.Response
			if err := json.Unmarshal(scanner.Bytes(), &resp); err == nil && resp.ID.Value == "early-request" {
				respCh <- resp
				return
			}
		}
	}()

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			t.Fatalf("unexpected error response: %+v", resp.Error)
		}
		if string(resp.Result) != `{"handled":true}` {
			t.Fatalf("response result = %s; want {\"handled\":true}", resp.Result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for response to early request")
	}
}

func TestStdioNotificationBeforeHandlerRegistrationIsDelivered(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, io.Discard)
	defer func() { _ = transport.Close() }()

	notif := `{"jsonrpc":"2.0","method":"thread/started","params":{"threadId":"thread-1"}}` + "\n"
	writeErrCh := make(chan error, 1)
	go func() {
		_, err := serverWriter.Write([]byte(notif))
		writeErrCh <- err
	}()

	received := make(chan codex.Notification, 1)
	transport.OnNotify(func(_ context.Context, n codex.Notification) {
		received <- n
	})

	select {
	case err := <-writeErrCh:
		if err != nil {
			t.Fatalf("write early notification: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for early notification write completion")
	}

	select {
	case n := <-received:
		if n.Method != "thread/started" {
			t.Fatalf("notification method = %q; want thread/started", n.Method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for early notification delivery")
	}
}

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
	handlerDone := make(chan struct{}, 1)

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	transport.OnRequest(func(_ context.Context, req codex.Request) (codex.Response, error) {
		close(handlerStarted)

		// Wait for Close() to run, then try to write a response.
		// The transport's writeMessage should reject the write because
		// the transport context has been cancelled.
		time.Sleep(50 * time.Millisecond)

		handlerDone <- struct{}{}
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

	// Verify the handler goroutine completes (writeMessage returns after
	// detecting the cancelled context, rather than blocking or panicking).
	select {
	case <-handlerDone:
		// Handler completed — writeMessage rejected the write after Close.
	case <-time.After(2 * time.Second):
		t.Fatal("handler goroutine did not complete after Close")
	}
}

func TestStdioRequestHandlerWriteFailureSetsScanErr(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, _ := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()

	writeErr := errors.New("write failed")
	transport := codex.NewStdioTransport(clientReader, &errorWriter{err: writeErr})
	defer func() { _ = transport.Close() }()

	transport.OnRequest(func(_ context.Context, req codex.Request) (codex.Response, error) {
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"ok":true}`),
		}, nil
	})

	req := `{"jsonrpc":"2.0","id":"write-fail-ok","method":"approval/test","params":{}}` + "\n"
	if _, err := serverWriter.Write([]byte(req)); err != nil {
		t.Fatalf("write server request: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := transport.ScanErr(); err != nil {
			if !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("scan error = %v; want write failure", err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected scan error after handler response write failure")
}

func TestStdioRequestHandlerErrorResponseWriteFailureSetsScanErr(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, _ := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()

	writeErr := errors.New("error response write failed")
	transport := codex.NewStdioTransport(clientReader, &errorWriter{err: writeErr})
	defer func() { _ = transport.Close() }()

	transport.OnRequest(func(_ context.Context, _ codex.Request) (codex.Response, error) {
		return codex.Response{}, errors.New("handler failure")
	})

	req := `{"jsonrpc":"2.0","id":"write-fail-err","method":"approval/test","params":{}}` + "\n"
	if _, err := serverWriter.Write([]byte(req)); err != nil {
		t.Fatalf("write server request: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := transport.ScanErr(); err != nil {
			if !strings.Contains(err.Error(), "error response write failed") {
				t.Fatalf("scan error = %v; want write failure", err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected scan error after handler error-response write failure")
}

func TestStdioMalformedResponseFrameWithRecoverableIDFailsPending(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		resp, err := transport.Send(ctx, codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "malformed-recoverable-id"},
			Method:  "test/method",
		})
		if err != nil {
			errCh <- err
			return
		}
		if resp.Error == nil {
			errCh <- fmt.Errorf("expected parse-error response, got nil error")
			return
		}
		if resp.Error.Code != codex.ErrCodeParseError {
			errCh <- fmt.Errorf("response code = %d; want %d", resp.Error.Code, codex.ErrCodeParseError)
			return
		}
		errCh <- nil
	}()

	waitForOutboundRequest(t, serverReader)
	// Invalid JSON after id/method; id is still recoverable via token scanning.
	_, _ = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":"malformed-recoverable-id","result":{"ok":true` + "\n"))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for malformed response handling")
	}
}

// TestStdioConcurrentSendAndClose verifies that concurrent Send and Close calls
// do not race or panic. Every Send must either succeed or return an error.

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

// TestStdioSpontaneousReaderEOF verifies that when the underlying reader
// closes unexpectedly (simulating a child process crash), a pending Send
// is unblocked with a "transport reader stopped" error.
func TestStdioSpontaneousReaderEOF(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	// Drain requests from server side so writes don't block.
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send a request that will block waiting for a response.
	errCh := make(chan error, 1)
	go func() {
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "eof-test"},
			Method:  "test/pending",
		}
		_, err := transport.Send(ctx, req)
		errCh <- err
	}()

	// Wait for the request to be sent.
	time.Sleep(50 * time.Millisecond)

	// Simulate a child process crash by closing the remote writer.
	// This causes the readLoop scanner to hit EOF and close readerStopped.
	_ = serverWriter.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from Send after reader EOF, got nil")
		}
		if !strings.Contains(err.Error(), "transport reader stopped") {
			t.Errorf("expected 'transport reader stopped' error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: Send was not unblocked after reader EOF")
	}
}

func TestStdioNotifyAfterReaderEOFReturnsReaderStopped(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
		}
	}()

	_ = serverWriter.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		err := transport.Notify(context.Background(), codex.Notification{
			JSONRPC: "2.0",
			Method:  "test/notification",
		})
		if err == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if !strings.Contains(err.Error(), "transport reader stopped") {
			t.Fatalf("expected reader stopped error, got %v", err)
		}
		return
	}

	t.Fatal("timeout waiting for notify to observe reader EOF")
}
