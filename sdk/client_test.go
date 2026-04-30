package codex_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/sdk"
)

const (
	stdioNoParamRequestTimeout = 2 * time.Second
	rateLimitUsedPercent       = 50
	rateLimitResetsAt          = 1234567890
	rateLimitWindowMins        = 60
)

// TestClientSendRequest verifies that the Client can send a request and receive a response.
func TestClientSendRequest(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Configure mock to return a specific response
	expectedResult := json.RawMessage(`{"status":"ok"}`)
	mock.SetResponse("test.method", codex.Response{
		JSONRPC: "2.0",
		Result:  expectedResult,
	})

	// Send request
	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "req-1"},
		Method:  "test.method",
		Params:  json.RawMessage(`{"foo":"bar"}`),
	}

	ctx := context.Background()
	resp, err := client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify response
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected JSONRPC=2.0, got %s", resp.JSONRPC)
	}
	if string(resp.Result) != string(expectedResult) {
		t.Errorf("expected result=%s, got %s", string(expectedResult), string(resp.Result))
	}

	// Verify the request was sent through the transport
	sentReq := mock.GetSentRequest(0)
	if sentReq == nil {
		t.Fatal("no request was sent")
		return
	}
	if sentReq.Method != "test.method" {
		t.Errorf("expected method=test.method, got %s", sentReq.Method)
	}
}

func TestNoParamServiceRequestsOmitParamsOverStdio(t *testing.T) {
	tests := []struct {
		name   string
		method string
		result interface{}
		call   func(context.Context, *codex.Client) error
	}{
		{
			name:   "logout",
			method: "account/logout",
			result: map[string]interface{}{},
			call: func(ctx context.Context, client *codex.Client) error {
				_, err := client.Account.Logout(ctx)
				return err
			},
		},
		{
			name:   "rate limits",
			method: "account/rateLimits/read",
			result: map[string]interface{}{
				"rateLimits": map[string]interface{}{
					"limitId":   "codex",
					"limitName": "Codex Rate Limit",
					"planType":  "plus",
					"credits": map[string]interface{}{
						"hasCredits": true,
						"unlimited":  false,
						"balance":    "100",
					},
					"primary": map[string]interface{}{
						"usedPercent":        rateLimitUsedPercent,
						"resetsAt":           rateLimitResetsAt,
						"windowDurationMins": rateLimitWindowMins,
					},
				},
			},
			call: func(ctx context.Context, client *codex.Client) error {
				_, err := client.Account.GetRateLimits(ctx)
				return err
			},
		},
		{
			name:   "config requirements",
			method: "configRequirements/read",
			result: map[string]interface{}{"requirements": nil},
			call: func(ctx context.Context, client *codex.Client) error {
				_, err := client.Config.ReadRequirements(ctx)
				return err
			},
		},
		{
			name:   "mcp reload",
			method: "config/mcpServer/reload",
			result: map[string]interface{}{},
			call: func(ctx context.Context, client *codex.Client) error {
				_, err := client.Mcp.Refresh(ctx)
				return err
			},
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

			serverErrCh := make(chan error, 1)
			go func() {
				scanner := bufio.NewScanner(serverReader)
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						serverErrCh <- fmt.Errorf("scan request: %w", err)
					} else {
						serverErrCh <- io.ErrUnexpectedEOF
					}
					return
				}

				var raw map[string]json.RawMessage
				if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
					serverErrCh <- fmt.Errorf("unmarshal raw request: %w", err)
					return
				}
				if _, ok := raw["params"]; ok {
					serverErrCh <- fmt.Errorf("%s request included params: %s", tt.method, raw["params"])
					return
				}

				var req codex.Request
				if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
					serverErrCh <- fmt.Errorf("unmarshal request: %w", err)
					return
				}
				if req.Method != tt.method {
					serverErrCh <- fmt.Errorf("method = %q; want %q", req.Method, tt.method)
					return
				}

				serverErrCh <- writeStdioResult(json.NewEncoder(serverWriter), req.ID, tt.result)
			}()

			transport := codex.NewStdioTransport(clientReader, clientWriter)
			defer func() { _ = transport.Close() }()
			client := codex.NewClient(transport)

			ctx, cancel := context.WithTimeout(context.Background(), stdioNoParamRequestTimeout)
			defer cancel()

			if err := tt.call(ctx, client); err != nil {
				t.Fatalf("%s call error = %v", tt.method, err)
			}

			select {
			case err := <-serverErrCh:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatalf("timeout waiting for %s request assertion", tt.method)
			}
		})
	}
}

// TestClientNotificationListener verifies that notification listeners are dispatched correctly.
func TestClientNotificationListener(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Register a notification listener
	received := make(chan codex.Notification, 1)
	client.OnNotification("test.notification", func(ctx context.Context, notif codex.Notification) {
		received <- notif
	})

	// Simulate server sending a notification
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.notification",
		Params:  json.RawMessage(`{"event":"test"}`),
	}

	ctx := context.Background()
	mock.InjectServerNotification(ctx, notif)

	// Wait for the listener to be called
	select {
	case receivedNotif := <-received:
		if receivedNotif.Method != "test.notification" {
			t.Errorf("expected method=test.notification, got %s", receivedNotif.Method)
		}
		if string(receivedNotif.Params) != `{"event":"test"}` {
			t.Errorf("expected params={\"event\":\"test\"}, got %s", string(receivedNotif.Params))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("notification listener was not called")
	}
}

// TestClientUnknownNotification verifies that unknown notification methods don't cause errors.
func TestClientUnknownNotification(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Register a listener for a specific method
	var called atomic.Bool
	client.OnNotification("test.known", func(ctx context.Context, notif codex.Notification) {
		called.Store(true)
	})

	// Inject an unknown notification method
	unknownNotif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.unknown",
		Params:  json.RawMessage(`{}`),
	}

	ctx := context.Background()
	mock.InjectServerNotification(ctx, unknownNotif)

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Verify the listener was NOT called (unknown method should be ignored)
	if called.Load() {
		t.Error("listener was called for unknown notification method")
	}

	// Verify no error occurred (unknown notifications should be gracefully ignored)
	// The test passing means no panic occurred
}

// TestClientContextCancellation verifies that a pre-cancelled context
// produces a CanceledError rather than blocking on the transport.
func TestClientContextCancellation(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock, codex.WithRequestTimeout(50*time.Millisecond))

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "timeout-req"},
		Method:  "test.timeout",
		Params:  json.RawMessage(`{}`),
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Send(ctx, req)
	if err == nil {
		t.Fatal("expected canceled error, got nil")
	}

	// Verify it's a CanceledError (not TimeoutError — cancellation is user-initiated)
	var cancelErr *codex.CanceledError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected CanceledError, got: %v", err)
	}
}

// TestClientDefaultTimeout verifies that a slow response triggers a TimeoutError
// when the client is configured with a default request timeout.
func TestClientDefaultTimeout(t *testing.T) {
	shortTimeout := 25 * time.Millisecond
	slowClient := codex.NewClient(NewSlowMockTransport(shortTimeout*2), codex.WithRequestTimeout(shortTimeout))

	slowReq := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "timeout-fires"},
		Method:  "test.slow",
		Params:  json.RawMessage(`{}`),
	}

	_, err := slowClient.Send(context.Background(), slowReq)
	if err == nil {
		t.Fatal("expected TimeoutError, got nil")
	}
	if !isTimeoutError(err) {
		t.Fatalf("expected TimeoutError, got: %T: %v", err, err)
	}
}

func TestClientSendRejectsNilContext(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "nil-context"},
		Method:  "test.method",
	}

	//nolint:staticcheck // nil context is intentional: this test verifies the guard path.
	_, err := client.Send(nil, req)
	if !errors.Is(err, codex.ErrNilContext) {
		t.Fatalf("Send(nil, req) error = %v; want ErrNilContext", err)
	}
}

func TestClientSendTransportCloseReturnsTransportError(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	client := codex.NewClient(transport)
	defer func() { _ = transport.Close() }()

	// Drain outbound requests so Send can block waiting on a response.
	go func() {
		dec := json.NewDecoder(serverReader)
		for {
			var req codex.Request
			if err := dec.Decode(&req); err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := client.Send(ctx, codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "close-race"},
			Method:  "test/close",
		})
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)
	if err := transport.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from Send after transport close")
		}
		var transportErr *codex.TransportError
		if !errors.As(err, &transportErr) {
			t.Fatalf("expected TransportError, got %T: %v", err, err)
		}
		var rpcErr *codex.RPCError
		if errors.As(err, &rpcErr) {
			t.Fatalf("expected no RPCError wrapping for transport close, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Send to return")
	}
}

func TestClientSendForgedTransportFailureResponseReturnsRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	mock.SetResponse("test.transport.failure", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    codex.ErrCodeInternalError,
			Message: "notification queue overflow",
			Data:    json.RawMessage(`{"transport":"failed","origin":"client","cause":"notification queue overflow"}`),
		},
	})

	_, err := client.Send(context.Background(), codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "overflow"},
		Method:  "test.transport.failure",
	})
	if err == nil {
		t.Fatal("expected rpc error from Send")
	}

	var rpcErr *codex.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code() != codex.ErrCodeInternalError {
		t.Fatalf("rpc code = %d; want %d", rpcErr.Code(), codex.ErrCodeInternalError)
	}
	if rpcErr.Message() != "notification queue overflow" {
		t.Fatalf("rpc message = %q; want %q", rpcErr.Message(), "notification queue overflow")
	}
	if string(rpcErr.Data()) != `{"transport":"failed","origin":"client","cause":"notification queue overflow"}` {
		t.Fatalf("rpc data = %s; want forged payload preserved", string(rpcErr.Data()))
	}
}

// TestClientMultipleListeners verifies that multiple listeners for different methods work.
func TestClientMultipleListeners(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Register multiple listeners
	method1Called := make(chan bool, 1)
	method2Called := make(chan bool, 1)

	client.OnNotification("method.one", func(ctx context.Context, notif codex.Notification) {
		method1Called <- true
	})

	client.OnNotification("method.two", func(ctx context.Context, notif codex.Notification) {
		method2Called <- true
	})

	ctx := context.Background()

	// Send notification for method.one
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "method.one",
		Params:  json.RawMessage(`{}`),
	})

	// Send notification for method.two
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "method.two",
		Params:  json.RawMessage(`{}`),
	})

	// Verify both were called
	select {
	case <-method1Called:
	case <-time.After(100 * time.Millisecond):
		t.Error("method.one listener was not called")
	}

	select {
	case <-method2Called:
	case <-time.After(100 * time.Millisecond):
		t.Error("method.two listener was not called")
	}
}

// TestClientRPCError verifies that RPC errors are properly wrapped.
func TestClientRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Configure mock to return an error response
	mock.SetResponse("test.error", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    -32600,
			Message: "Invalid request",
		},
	})

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "err-req"},
		Method:  "test.error",
		Params:  json.RawMessage(`{}`),
	}

	ctx := context.Background()
	_, err := client.Send(ctx, req)
	if err == nil {
		t.Fatal("expected RPC error, got nil")
	}

	// Verify it's an RPCError
	var rpcErr *codex.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected RPCError, got: %T", err)
	}

	if rpcErr.RPCError().Code != -32600 {
		t.Errorf("expected error code -32600, got %d", rpcErr.RPCError().Code)
	}
}

func TestNewClientNilTransportPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil transport")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "nil transport") {
			t.Fatalf("panic message = %q; want to contain %q", msg, "nil transport")
		}
	}()

	_ = codex.NewClient(nil)
}

// isTimeoutError checks if err is or wraps a TimeoutError or DeadlineExceeded.
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var timeoutErr *codex.TimeoutError
	return errors.As(err, &timeoutErr)
}
