package codex

import "context"

// RequestHandler processes incoming JSON-RPC requests from the server and returns a response.
// This is used for approval flows where the server sends requests to the client.
type RequestHandler func(ctx context.Context, req Request) (Response, error)

// NotificationHandler processes incoming JSON-RPC notifications from the server.
// Notifications are fire-and-forget messages that don't expect a response.
type NotificationHandler func(ctx context.Context, notif Notification)

// Transport abstracts the underlying communication channel (stdio, WebSocket, etc.).
// It handles bidirectional JSON-RPC 2.0 communication between client and server.
//
// The transport must support:
// - Sending requests from client to server (Send)
// - Sending notifications from client to server (Notify)
// - Receiving requests from server to client (OnRequest)
// - Receiving notifications from server to client (OnNotify)
type Transport interface {
	// Send transmits a JSON-RPC request to the server and waits for the response.
	// The transport must match the response to this request by ID.
	// Returns an error if the request cannot be sent or if the response cannot be received.
	Send(ctx context.Context, req Request) (Response, error)

	// Notify transmits a JSON-RPC notification to the server.
	// Notifications are fire-and-forget and don't expect a response.
	// Returns an error if the notification cannot be sent.
	Notify(ctx context.Context, notif Notification) error

	// OnRequest registers a handler for incoming JSON-RPC requests from the server.
	// The server may send requests to the client for approval flows.
	// Only one handler can be registered; subsequent calls replace the previous handler.
	OnRequest(handler RequestHandler)

	// OnNotify registers a handler for incoming JSON-RPC notifications from the server.
	// The server sends notifications for events like thread updates, turn completion, etc.
	// Only one handler can be registered; subsequent calls replace the previous handler.
	OnNotify(handler NotificationHandler)

	// Close shuts down the transport, releasing any resources.
	// After Close is called, Send and Notify will return errors.
	// Close must be safe to call multiple times.
	Close() error
}
