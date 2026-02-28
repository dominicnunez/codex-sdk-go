package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var errInvalidParams = errors.New("invalid params")
var errHandlerFailed = errors.New("handler failed")

// ErrEmptyResult indicates the server returned a successful response with a
// null result where the caller expected a value.
var ErrEmptyResult = errors.New("server returned empty result")

// wireMarshaler is implemented by types whose MarshalJSON is redacted for safety.
// marshalForWire uses this to get the unredacted representation for protocol serialization.
type wireMarshaler interface {
	marshalWire() ([]byte, error)
}

// marshalForWire marshals v for wire-protocol use. If v implements wireMarshaler
// (because its MarshalJSON redacts sensitive fields), the unredacted wire
// representation is returned instead.
func marshalForWire(v interface{}) ([]byte, error) {
	if wm, ok := v.(wireMarshaler); ok {
		return wm.marshalWire()
	}
	return json.Marshal(v)
}

// internalListener is a notification handler registered via addNotificationListener.
// Each listener has a unique ID for unsubscription.
type internalListener struct {
	id      uint64
	handler NotificationHandler
}

// Client is the main entry point for interacting with the Codex JSON-RPC server.
// It uses a Transport for bidirectional communication and provides typed methods
// for all protocol operations.
type Client struct {
	transport Transport

	// Request timeout (optional, can be overridden per-request via context)
	requestTimeout time.Duration

	// Notification listeners: method → handler function (public, replacement semantics)
	notificationListeners map[string]NotificationHandler
	// Internal notification listeners: method → list of listeners (append semantics)
	internalListeners   map[string][]internalListener
	internalListenerSeq uint64
	listenersMu         sync.RWMutex

	// Approval handlers for server→client requests
	approvalHandlers ApprovalHandlers
	approvalMu       sync.RWMutex

	// Request ID counter for generating unique request IDs
	requestIDCounter uint64

	// Service accessors
	Thread          *ThreadService
	Turn            *TurnService
	Account         *AccountService
	Config          *ConfigService
	Model           *ModelService
	Skills          *SkillsService
	Apps            *AppsService
	Mcp             *McpService
	Command         *CommandService
	Review          *ReviewService
	Feedback        *FeedbackService
	ExternalAgent   *ExternalAgentService
	Experimental    *ExperimentalService
	System          *SystemService
	FuzzyFileSearch *FuzzyFileSearchService
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithRequestTimeout sets the default timeout for requests.
// This timeout is applied if the context passed to Send doesn't have a deadline.
func WithRequestTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.requestTimeout = timeout
	}
}

// NewClient creates a new Client using the given transport and options.
func NewClient(transport Transport, opts ...ClientOption) *Client {
	c := &Client{
		transport:             transport,
		notificationListeners: make(map[string]NotificationHandler),
		internalListeners:     make(map[string][]internalListener),
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	// Initialize services
	c.Thread = newThreadService(c)
	c.Turn = newTurnService(c)
	c.Account = newAccountService(c)
	c.Config = newConfigService(c)
	c.Model = newModelService(c)
	c.Skills = newSkillsService(c)
	c.Apps = newAppsService(c)
	c.Mcp = newMcpService(c)
	c.Command = newCommandService(c)
	c.Review = newReviewService(c)
	c.Feedback = newFeedbackService(c)
	c.ExternalAgent = newExternalAgentService(c)
	c.Experimental = newExperimentalService(c)
	c.System = newSystemService(c)
	c.FuzzyFileSearch = newFuzzyFileSearchService(c)

	// Register the transport's notification handler to route to our listeners
	transport.OnNotify(c.handleNotification)

	// Register the transport's request handler for server→client approval requests
	transport.OnRequest(c.handleRequest)

	return c
}

// Send transmits a JSON-RPC request and waits for the response.
// Returns an RPCError if the response contains an error field.
// Returns a TimeoutError if the context deadline is exceeded.
// Returns a TransportError if the transport fails.
func (c *Client) Send(ctx context.Context, req Request) (Response, error) {
	// Apply default timeout if context has no deadline and we have a default timeout
	if c.requestTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
			defer cancel()
		}
	}

	// Send the request
	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		// Only translate to context errors when the transport error was
		// actually caused by context cancellation/deadline, not when the
		// context happens to be done concurrently for an unrelated reason.
		if errors.Is(err, context.DeadlineExceeded) {
			return Response{}, NewTimeoutError("request timeout exceeded", err)
		}
		if errors.Is(err, context.Canceled) {
			return Response{}, NewCanceledError("request cancelled", err)
		}
		// Wrap as transport error if not already one
		var te *TransportError
		if errors.As(err, &te) {
			return Response{}, err
		}
		return Response{}, NewTransportError("failed to send request", err)
	}

	// Check if the response contains an error
	if resp.Error != nil {
		return Response{}, NewRPCError(resp.Error)
	}

	return resp, nil
}

// OnNotification registers a listener for incoming notifications with the given method.
// When a notification with this method arrives from the server, the handler will be called.
// Only one handler can be registered per method; subsequent calls replace the previous handler.
// Passing nil removes the handler for the given method.
func (c *Client) OnNotification(method string, handler NotificationHandler) {
	c.listenersMu.Lock()
	defer c.listenersMu.Unlock()
	if handler == nil {
		delete(c.notificationListeners, method)
	} else {
		c.notificationListeners[method] = handler
	}
}

// handleNotification is the internal handler registered with the transport.
// It routes incoming notifications to the appropriate registered listener,
// then dispatches to all internal listeners for the same method.
func (c *Client) handleNotification(ctx context.Context, notif Notification) {
	c.listenersMu.RLock()
	handler := c.notificationListeners[notif.Method]
	// Snapshot internal listeners so we can release the lock before calling.
	internals := c.internalListeners[notif.Method]
	c.listenersMu.RUnlock()

	if handler != nil {
		handler(ctx, notif)
	}

	for _, il := range internals {
		il.handler(ctx, notif)
	}
}

// addNotificationListener appends an internal listener for the given method.
// Returns an unsubscribe function that removes this specific listener.
// Unlike OnNotification, multiple listeners can coexist for the same method.
func (c *Client) addNotificationListener(method string, handler NotificationHandler) func() {
	c.listenersMu.Lock()
	c.internalListenerSeq++
	id := c.internalListenerSeq
	c.internalListeners[method] = append(c.internalListeners[method], internalListener{
		id:      id,
		handler: handler,
	})
	c.listenersMu.Unlock()

	return func() {
		c.listenersMu.Lock()
		defer c.listenersMu.Unlock()
		listeners := c.internalListeners[method]
		for i, l := range listeners {
			if l.id == id {
				c.internalListeners[method] = append(listeners[:i], listeners[i+1:]...)
				break
			}
		}
	}
}

// handleRequest is the internal handler for server→client requests (approval flows).
// It routes incoming requests to the appropriate approval handler.
func (c *Client) handleRequest(ctx context.Context, req Request) (Response, error) {
	// Snapshot handlers under read lock, then release before calling
	c.approvalMu.RLock()
	handlers := c.approvalHandlers
	c.approvalMu.RUnlock()

	// Route based on method to the appropriate approval handler.
	// Each handler function is passed from the snapshot to avoid a
	// TOCTOU race — no second lock acquisition needed in the helpers.
	switch req.Method {
	case methodApplyPatchApproval:
		if handlers.OnApplyPatchApproval == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return handleApproval(ctx, req, handlers.OnApplyPatchApproval)

	case methodCommandExecutionRequestApproval:
		if handlers.OnCommandExecutionRequestApproval == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return handleApproval(ctx, req, handlers.OnCommandExecutionRequestApproval)

	case methodExecCommandApproval:
		if handlers.OnExecCommandApproval == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return handleApproval(ctx, req, handlers.OnExecCommandApproval)

	case methodFileChangeRequestApproval:
		if handlers.OnFileChangeRequestApproval == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return handleApproval(ctx, req, handlers.OnFileChangeRequestApproval)

	case methodDynamicToolCall:
		if handlers.OnDynamicToolCall == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return handleApproval(ctx, req, handlers.OnDynamicToolCall)

	case methodToolRequestUserInput:
		if handlers.OnToolRequestUserInput == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return handleApproval(ctx, req, handlers.OnToolRequestUserInput)

	case methodChatgptAuthTokensRefresh:
		if handlers.OnChatgptAuthTokensRefresh == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return handleApproval(ctx, req, handlers.OnChatgptAuthTokensRefresh)

	default:
		// Unknown method - return method not found error
		return methodNotFoundResponse(req.ID), nil
	}
}

// methodNotFoundResponse creates a JSON-RPC method-not-found error response.
func methodNotFoundResponse(id RequestID) Response {
	return Response{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error: &Error{
			Code:    ErrCodeMethodNotFound,
			Message: "Method not found",
		},
	}
}

// handleApproval is a generic helper that unmarshals params, calls the handler,
// and marshals the result into a JSON-RPC response. The handler function is passed
// from the snapshot taken in handleRequest, so no additional lock is needed.
func handleApproval[P any, R any](ctx context.Context, req Request, handler func(context.Context, P) (R, error)) (Response, error) {
	var params P
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, fmt.Errorf("unmarshal %s params: %w: %w", req.Method, errInvalidParams, err)
	}

	result, err := handler(ctx, params)
	if err != nil {
		return Response{}, fmt.Errorf("approval handler %s: %w: %w", req.Method, errHandlerFailed, err)
	}

	resultJSON, err := marshalForWire(&result)
	if err != nil {
		return Response{}, fmt.Errorf("marshal %s result: %w", req.Method, err)
	}

	return Response{
		JSONRPC: jsonrpcVersion,
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

// Close closes the underlying transport and releases resources.
func (c *Client) Close() error {
	return c.transport.Close()
}

// nextRequestID generates a unique request ID for outgoing requests.
func (c *Client) nextRequestID() uint64 {
	return atomic.AddUint64(&c.requestIDCounter, 1)
}

// sendRequest is a helper that sends a typed request and unmarshals the response.
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}, result interface{}) error {
	// Marshal params to JSON
	paramsJSON, err := marshalForWire(params)
	if err != nil {
		return fmt.Errorf("marshal request params for %s: %w", method, err)
	}

	// Create request
	req := Request{
		JSONRPC: jsonrpcVersion,
		Method:  method,
		Params:  paramsJSON,
		ID:      RequestID{Value: c.nextRequestID()},
	}

	// Send request
	resp, err := c.Send(ctx, req)
	if err != nil {
		return err
	}

	// Unmarshal result if caller expects one
	if result != nil {
		if resp.Result == nil {
			return fmt.Errorf("%s: %w", method, ErrEmptyResult)
		}
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("unmarshal response result for %s: %w", method, err)
		}
	}

	return nil
}

// sendRequestRaw is a helper that sends a typed request and returns the raw response result.
// This is useful for union types where the result needs custom unmarshaling.
func (c *Client) sendRequestRaw(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// Marshal params to JSON
	paramsJSON, err := marshalForWire(params)
	if err != nil {
		return nil, fmt.Errorf("marshal request params for %s: %w", method, err)
	}

	// Create request
	req := Request{
		JSONRPC: jsonrpcVersion,
		Method:  method,
		Params:  paramsJSON,
		ID:      RequestID{Value: c.nextRequestID()},
	}

	// Send request
	resp, err := c.Send(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Result == nil {
		return nil, fmt.Errorf("%s: %w", method, ErrEmptyResult)
	}

	return resp.Result, nil
}
