package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

func TestMockTransportSendRequest(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "req-1"},
		Method:  "test.method",
		Params:  json.RawMessage(`{"key":"value"}`),
	}

	resp, err := mock.Send(ctx, req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify default response
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC=2.0, got %s", resp.JSONRPC)
	}
	if resp.ID.Value != "req-1" {
		t.Errorf("Expected ID=req-1, got %v", resp.ID.Value)
	}

	// Verify request was recorded
	if len(mock.SentRequests) != 1 {
		t.Fatalf("Expected 1 sent request, got %d", len(mock.SentRequests))
	}
	if mock.SentRequests[0].Method != "test.method" {
		t.Errorf("Expected method=test.method, got %s", mock.SentRequests[0].Method)
	}
}

func TestMockTransportSendWithInjectedResponse(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	// Set up injected response
	expectedResp := codex.Response{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{"status":"success"}`),
	}
	mock.SetResponse("test.method", expectedResp)

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: int64(42)},
		Method:  "test.method",
	}

	resp, err := mock.Send(ctx, req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify the injected response was returned (with request ID copied)
	if string(resp.Result) != `{"status":"success"}` {
		t.Errorf("Expected injected result, got %s", string(resp.Result))
	}
	if resp.ID.Value != int64(42) {
		t.Errorf("Expected ID=42, got %v", resp.ID.Value)
	}
}

func TestMockTransportSendError(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	expectedErr := errors.New("network error")
	mock.SetSendError(expectedErr)

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "test"},
		Method:  "test.method",
	}

	_, err := mock.Send(ctx, req)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestMockTransportNotify(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.notification",
		Params:  json.RawMessage(`{"event":"started"}`),
	}

	err := mock.Notify(ctx, notif)
	if err != nil {
		t.Fatalf("Notify failed: %v", err)
	}

	// Verify notification was recorded
	if len(mock.SentNotifications) != 1 {
		t.Fatalf("Expected 1 sent notification, got %d", len(mock.SentNotifications))
	}
	if mock.SentNotifications[0].Method != "test.notification" {
		t.Errorf("Expected method=test.notification, got %s", mock.SentNotifications[0].Method)
	}
}

func TestMockTransportNotifyError(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	expectedErr := errors.New("notify error")
	mock.SetNotifyError(expectedErr)

	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.notification",
	}

	err := mock.Notify(ctx, notif)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestMockTransportCallTracking(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	// Set up expectations
	mock.ExpectCall("method1", 2)
	mock.ExpectCall("method2", 1)

	// Make calls
	mock.Send(ctx, codex.Request{JSONRPC: "2.0", ID: codex.RequestID{Value: "1"}, Method: "method1"})
	mock.Send(ctx, codex.Request{JSONRPC: "2.0", ID: codex.RequestID{Value: "2"}, Method: "method1"})
	mock.Send(ctx, codex.Request{JSONRPC: "2.0", ID: codex.RequestID{Value: "3"}, Method: "method2"})

	// Verify expectations met
	if err := mock.VerifyCalls(); err != nil {
		t.Errorf("VerifyCalls failed: %v", err)
	}
}

func TestMockTransportCallTrackingMismatch(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	mock.ExpectCall("method1", 2)
	mock.Send(ctx, codex.Request{JSONRPC: "2.0", ID: codex.RequestID{Value: "1"}, Method: "method1"})

	// Only made 1 call but expected 2
	err := mock.VerifyCalls()
	if err == nil {
		t.Error("Expected VerifyCalls to fail, but it succeeded")
	}
}

func TestMockTransportGetSentRequest(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	req1 := codex.Request{JSONRPC: "2.0", ID: codex.RequestID{Value: "1"}, Method: "first"}
	req2 := codex.Request{JSONRPC: "2.0", ID: codex.RequestID{Value: "2"}, Method: "second"}

	mock.Send(ctx, req1)
	mock.Send(ctx, req2)

	// Test valid indices
	got := mock.GetSentRequest(0)
	if got == nil || got.Method != "first" {
		t.Errorf("Expected first request, got %v", got)
	}

	got = mock.GetSentRequest(1)
	if got == nil || got.Method != "second" {
		t.Errorf("Expected second request, got %v", got)
	}

	// Test invalid indices
	if mock.GetSentRequest(-1) != nil {
		t.Error("Expected nil for negative index")
	}
	if mock.GetSentRequest(2) != nil {
		t.Error("Expected nil for out-of-bounds index")
	}
}

func TestMockTransportGetSentNotification(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	notif1 := codex.Notification{JSONRPC: "2.0", Method: "first"}
	notif2 := codex.Notification{JSONRPC: "2.0", Method: "second"}

	mock.Notify(ctx, notif1)
	mock.Notify(ctx, notif2)

	// Test valid indices
	got := mock.GetSentNotification(0)
	if got == nil || got.Method != "first" {
		t.Errorf("Expected first notification, got %v", got)
	}

	got = mock.GetSentNotification(1)
	if got == nil || got.Method != "second" {
		t.Errorf("Expected second notification, got %v", got)
	}

	// Test invalid indices
	if mock.GetSentNotification(-1) != nil {
		t.Error("Expected nil for negative index")
	}
	if mock.GetSentNotification(2) != nil {
		t.Error("Expected nil for out-of-bounds index")
	}
}

func TestMockTransportRequestHandler(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	handlerCalled := false
	var receivedReq codex.Request

	mock.OnRequest(func(ctx context.Context, req codex.Request) (codex.Response, error) {
		handlerCalled = true
		receivedReq = req
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"approved":true}`),
		}, nil
	})

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "approval-1"},
		Method:  "server.request",
	}

	resp, err := mock.InjectServerRequest(ctx, req)
	if err != nil {
		t.Fatalf("InjectServerRequest failed: %v", err)
	}

	if !handlerCalled {
		t.Error("Handler was not called")
	}
	if receivedReq.Method != "server.request" {
		t.Errorf("Expected method=server.request, got %s", receivedReq.Method)
	}
	if string(resp.Result) != `{"approved":true}` {
		t.Errorf("Expected approved result, got %s", string(resp.Result))
	}
}

func TestMockTransportRequestHandlerNotSet(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "test"},
		Method:  "server.request",
	}

	_, err := mock.InjectServerRequest(ctx, req)
	if err == nil {
		t.Error("Expected error when no handler is set, got nil")
	}
}

func TestMockTransportNotificationHandler(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	handlerCalled := false
	var receivedNotif codex.Notification

	mock.OnNotify(func(ctx context.Context, notif codex.Notification) {
		handlerCalled = true
		receivedNotif = notif
	})

	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "server.notification",
		Params:  json.RawMessage(`{"status":"completed"}`),
	}

	mock.InjectServerNotification(ctx, notif)

	if !handlerCalled {
		t.Error("Handler was not called")
	}
	if receivedNotif.Method != "server.notification" {
		t.Errorf("Expected method=server.notification, got %s", receivedNotif.Method)
	}
}

func TestMockTransportNotificationHandlerNotSet(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	// Should not panic when no handler is set
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "server.notification",
	}

	mock.InjectServerNotification(ctx, notif)
	// If we reach here, the test passes (no panic)
}

func TestMockTransportClose(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	// Close should succeed
	if err := mock.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Operations after close should fail
	req := codex.Request{JSONRPC: "2.0", ID: codex.RequestID{Value: "test"}, Method: "test"}
	_, err := mock.Send(ctx, req)
	if err == nil {
		t.Error("Expected Send to fail after Close, got nil")
	}

	notif := codex.Notification{JSONRPC: "2.0", Method: "test"}
	err = mock.Notify(ctx, notif)
	if err == nil {
		t.Error("Expected Notify to fail after Close, got nil")
	}
}

func TestMockTransportReset(t *testing.T) {
	mock := NewMockTransport()
	ctx := context.Background()

	// Make some calls
	mock.Send(ctx, codex.Request{JSONRPC: "2.0", ID: codex.RequestID{Value: "1"}, Method: "method1"})
	mock.Notify(ctx, codex.Notification{JSONRPC: "2.0", Method: "notif1"})
	mock.SetSendError(errors.New("test error"))
	mock.Close()

	// Reset
	mock.Reset()

	// Verify state is cleared
	if len(mock.SentRequests) != 0 {
		t.Errorf("Expected 0 sent requests after reset, got %d", len(mock.SentRequests))
	}
	if len(mock.SentNotifications) != 0 {
		t.Errorf("Expected 0 sent notifications after reset, got %d", len(mock.SentNotifications))
	}

	// Should be able to send again after reset
	_, err := mock.Send(ctx, codex.Request{JSONRPC: "2.0", ID: codex.RequestID{Value: "2"}, Method: "method2"})
	if err != nil {
		t.Errorf("Expected Send to work after reset, got error: %v", err)
	}
}
