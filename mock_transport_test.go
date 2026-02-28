package codex_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dominicnunez/codex-sdk-go"
)

// MockTransport is a test implementation of the Transport interface that records
// sent messages and allows injecting responses and notifications for testing.
type MockTransport struct {
	mu sync.Mutex

	// Sent messages
	SentRequests      []codex.Request
	SentNotifications []codex.Notification
	sentResponses     []codex.Response // Responses sent by request handler

	// Handlers
	requestHandler      codex.RequestHandler
	notificationHandler codex.NotificationHandler

	// Response injection: map request method → response
	responses map[string]codex.Response

	// Expected request→response pairs for verification
	expectedCalls map[string]int // method → expected count
	actualCalls   map[string]int // method → actual count

	// Injected errors for Send/Notify
	sendErr   error
	notifyErr error

	closed bool
}

// NewMockTransport creates a new MockTransport with empty state.
func NewMockTransport() *MockTransport {
	return &MockTransport{
		responses:     make(map[string]codex.Response),
		expectedCalls: make(map[string]int),
		actualCalls:   make(map[string]int),
	}
}

// Send implements Transport.Send by recording the request and returning an injected response.
func (m *MockTransport) Send(ctx context.Context, req codex.Request) (codex.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check context cancellation first
	select {
	case <-ctx.Done():
		return codex.Response{}, ctx.Err()
	default:
	}

	if m.closed {
		return codex.Response{}, fmt.Errorf("transport closed")
	}

	if m.sendErr != nil {
		return codex.Response{}, m.sendErr
	}

	m.SentRequests = append(m.SentRequests, req)
	m.actualCalls[req.Method]++

	resp, ok := m.responses[req.Method]
	if !ok {
		// Return a generic success response if no specific response is set
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}, nil
	}

	// Copy the request ID to the response
	resp.ID = req.ID
	return resp, nil
}

// Notify implements Transport.Notify by recording the notification.
func (m *MockTransport) Notify(ctx context.Context, notif codex.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("transport closed")
	}

	if m.notifyErr != nil {
		return m.notifyErr
	}

	m.SentNotifications = append(m.SentNotifications, notif)
	m.actualCalls[notif.Method]++

	return nil
}

// OnRequest implements Transport.OnRequest by storing the handler.
func (m *MockTransport) OnRequest(handler codex.RequestHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestHandler = handler
}

// OnNotify implements Transport.OnNotify by storing the handler.
func (m *MockTransport) OnNotify(handler codex.NotificationHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notificationHandler = handler
}

// Close implements Transport.Close by marking the transport as closed.
func (m *MockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// SetResponse configures the mock to return a specific response for a given method.
func (m *MockTransport) SetResponse(method string, resp codex.Response) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[method] = resp
}

// SetResponseData is a convenience helper that marshals data to JSON and sets it as the response result.
func (m *MockTransport) SetResponseData(method string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal response data: %w", err)
	}

	m.SetResponse(method, codex.Response{
		JSONRPC: "2.0",
		Result:  jsonData,
	})
	return nil
}

// SetSendError configures the mock to return an error on Send calls.
func (m *MockTransport) SetSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

// SetNotifyError configures the mock to return an error on Notify calls.
func (m *MockTransport) SetNotifyError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyErr = err
}

// ExpectCall configures the mock to expect a certain number of calls to a method.
func (m *MockTransport) ExpectCall(method string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expectedCalls[method] = count
}

// VerifyCalls checks that all expected calls were made. Returns an error if mismatch.
func (m *MockTransport) VerifyCalls() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for method, expected := range m.expectedCalls {
		actual := m.actualCalls[method]
		if actual != expected {
			return fmt.Errorf("method %s: expected %d calls, got %d", method, expected, actual)
		}
	}

	return nil
}

// GetSentRequest returns the nth sent request (0-indexed), or nil if not found.
func (m *MockTransport) GetSentRequest(index int) *codex.Request {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.SentRequests) {
		return nil
	}
	return &m.SentRequests[index]
}

// GetSentNotification returns the nth sent notification (0-indexed), or nil if not found.
func (m *MockTransport) GetSentNotification(index int) *codex.Notification {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.SentNotifications) {
		return nil
	}
	return &m.SentNotifications[index]
}

// GetSentResponses returns a copy of all responses sent by request handlers.
func (m *MockTransport) GetSentResponses() []codex.Response {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]codex.Response, len(m.sentResponses))
	copy(result, m.sentResponses)
	return result
}

// InjectServerRequest simulates the server sending a request to the client.
// Calls the registered request handler if one exists.
func (m *MockTransport) InjectServerRequest(ctx context.Context, req codex.Request) (codex.Response, error) {
	m.mu.Lock()
	handler := m.requestHandler
	m.mu.Unlock()

	if handler == nil {
		return codex.Response{}, fmt.Errorf("no request handler registered")
	}

	resp, err := handler(ctx, req)

	// Track sent responses for test verification
	m.mu.Lock()
	m.sentResponses = append(m.sentResponses, resp)
	m.mu.Unlock()

	return resp, err
}

// InjectServerNotification simulates the server sending a notification to the client.
// Calls the registered notification handler if one exists.
func (m *MockTransport) InjectServerNotification(ctx context.Context, notif codex.Notification) {
	m.mu.Lock()
	handler := m.notificationHandler
	m.mu.Unlock()

	if handler != nil {
		handler(ctx, notif)
	}
}

// Reset clears all recorded messages and state (useful for running multiple tests).
func (m *MockTransport) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SentRequests = nil
	m.SentNotifications = nil
	m.sentResponses = nil
	m.responses = make(map[string]codex.Response)
	m.expectedCalls = make(map[string]int)
	m.actualCalls = make(map[string]int)
	m.sendErr = nil
	m.notifyErr = nil
	m.closed = false
}

// SlowMockTransport is a mock transport that delays responses by a fixed duration.
// Used to test timeout behavior — Send blocks until the delay elapses or the
// context is cancelled, whichever comes first.
type SlowMockTransport struct {
	delay time.Duration
}

// NewSlowMockTransport creates a SlowMockTransport with the given response delay.
func NewSlowMockTransport(delay time.Duration) *SlowMockTransport {
	return &SlowMockTransport{delay: delay}
}

func (s *SlowMockTransport) Send(ctx context.Context, req codex.Request) (codex.Response, error) {
	select {
	case <-time.After(s.delay):
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}, nil
	case <-ctx.Done():
		return codex.Response{}, ctx.Err()
	}
}

func (s *SlowMockTransport) Notify(_ context.Context, _ codex.Notification) error {
	return nil
}

func (s *SlowMockTransport) OnRequest(_ codex.RequestHandler)     {}
func (s *SlowMockTransport) OnNotify(_ codex.NotificationHandler) {}
func (s *SlowMockTransport) Close() error                         { return nil }
