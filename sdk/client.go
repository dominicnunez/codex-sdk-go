package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

var errInvalidParams = errors.New("invalid params")

// ErrEmptyResult indicates the server returned a successful response with a
// missing or null result where the caller expected a value.
var ErrEmptyResult = errors.New("server returned empty result")

// ErrResultNotObject indicates the server returned a successful response whose
// result was not a JSON object where the protocol requires one.
var ErrResultNotObject = errors.New("server returned non-object result")

// ErrMissingResultField indicates the server returned a successful object
// result that omitted a required JSON field for the target response type.
var ErrMissingResultField = errors.New("server returned result missing required field")

// ErrNullResultField indicates the server returned null for a required
// non-nullable JSON field in the target response type.
var ErrNullResultField = errors.New("server returned null for required result field")

// wireMarshaler is implemented by types whose MarshalJSON is redacted for safety.
// marshalForWire uses this to get the unredacted representation for protocol serialization.
type wireMarshaler interface {
	marshalWire() ([]byte, error)
}

var errNilWireMarshaler = errors.New("nil wire marshaler")
var errNilResponseTarget = errors.New("response target must not be nil")

type responseValidator interface {
	validate() error
}

// marshalForWire marshals v for wire-protocol use. If v implements wireMarshaler
// (because its MarshalJSON redacts sensitive fields), the unredacted wire
// representation is returned instead.
func marshalForWire(v interface{}) ([]byte, error) {
	if wm, ok := v.(wireMarshaler); ok {
		if isNilWireMarshaler(wm) {
			return nil, errNilWireMarshaler
		}
		return wm.marshalWire()
	}
	return json.Marshal(v)
}

func isNilWireMarshaler(wm wireMarshaler) bool {
	rv := reflect.ValueOf(wm)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func isEmptyResponseResult(result json.RawMessage) bool {
	return len(result) == 0 || bytes.Equal(bytes.TrimSpace(result), []byte("null"))
}

func validateObjectResponseResult(result json.RawMessage) error {
	if isEmptyResponseResult(result) {
		return ErrEmptyResult
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(result, &payload); err != nil {
		return fmt.Errorf("%w: %w", ErrResultNotObject, err)
	}

	return nil
}

func isNullJSONValue(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func validateObjectFields(data []byte, requiredFields []string, nonNullFields []string) error {
	return decodeObjectWithValidation(data, nil, requiredFields, nonNullFields, responseObjectValidationErrors())
}

func validateRequiredObjectKeys(data []byte, requiredFields ...string) error {
	return validateObjectFields(data, requiredFields, nil)
}

func validateRequiredObjectFields(data []byte, requiredFields ...string) error {
	return validateObjectFields(data, requiredFields, requiredFields)
}

func validateTaggedObjectFields(
	data []byte,
	requiredFields []string,
	nonNullFields []string,
) error {
	required := make([]string, 0, len(requiredFields)+1)
	required = append(required, "type")
	required = append(required, requiredFields...)

	nonNull := make([]string, 0, len(nonNullFields)+1)
	nonNull = append(nonNull, "type")
	nonNull = append(nonNull, nonNullFields...)

	return validateObjectFields(data, required, nonNull)
}

func validateRequiredTaggedObjectFields(data []byte, requiredFields ...string) error {
	return validateTaggedObjectFields(data, requiredFields, requiredFields)
}

func decodeRequiredObjectTypeField(data []byte, context string) (string, error) {
	if err := validateRequiredObjectFields(data, "type"); err != nil {
		return "", err
	}

	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", err
	}
	if raw.Type == "" {
		return "", fmt.Errorf("%s: missing or empty type field", context)
	}

	return raw.Type, nil
}

func validateDecodedResponse(result interface{}) error {
	validator, ok := result.(responseValidator)
	if !ok {
		return nil
	}
	return validator.validate()
}

// internalListener is a notification handler registered via addNotificationListener.
// Each listener has a unique ID for unsubscription.
type internalListener struct {
	id      uint64
	handler NotificationHandler
}

type threadStateListener struct {
	id       uint64
	onUpdate func(Thread)
	onClose  func()
}

type threadStateEntry struct {
	thread      Thread
	hasSnapshot bool
	closed      bool
}

// Client is the main entry point for interacting with the Codex JSON-RPC server.
// It uses a Transport for bidirectional communication and provides typed methods
// for all protocol operations.
type Client struct {
	transport Transport

	// Request timeout (optional, can be overridden per-request via context)
	requestTimeout time.Duration

	// Initialize handshake state. Successful initialize responses are cached so
	// direct Client.Initialize calls and Process helper methods share the same
	// one-time protocol handshake.
	initializeMu     sync.Mutex
	initializeDone   bool
	initializeWait   chan struct{}
	initializeParams InitializeParams
	initializeResp   InitializeResponse

	// Notification listeners: method → handler function (public, replacement semantics)
	notificationListeners map[string]NotificationHandler
	// Internal notification listeners: method → list of listeners (append semantics)
	internalListeners   map[string][]internalListener
	internalListenerSeq uint64
	listenersMu         sync.RWMutex

	// Best-effort latest thread snapshots keyed by thread ID. This is updated
	// from thread-bearing responses and thread metadata notifications so
	// conversations and direct thread APIs can share recent snapshots. The
	// cache is bounded to avoid retaining snapshots for every thread a
	// long-lived client has ever touched. Active conversations subscribe to
	// updates for their own thread so cache eviction cannot regress their local
	// state between turns.
	threadStates           map[string]threadStateEntry
	threadStateOrder       []string
	threadStateListeners   map[string][]threadStateListener
	threadStateListenerSeq uint64
	threadStateMu          sync.RWMutex

	// Approval handlers for server→client requests
	approvalHandlers ApprovalHandlers
	approvalMu       sync.RWMutex

	// Request ID counter for generating unique request IDs
	requestIDCounter atomic.Uint64

	// Handler error callback (optional, set once during construction)
	handlerErrorCallback func(method string, err error)

	// Service accessors
	Thread          *ThreadService
	Turn            *TurnService
	Account         *AccountService
	Config          *ConfigService
	DeviceKey       *DeviceKeyService
	Model           *ModelService
	ModelProvider   *ModelProviderService
	Skills          *SkillsService
	Apps            *AppsService
	Marketplace     *MarketplaceService
	Mcp             *McpService
	Command         *CommandService
	Review          *ReviewService
	Feedback        *FeedbackService
	ExternalAgent   *ExternalAgentService
	Experimental    *ExperimentalService
	System          *SystemService
	Fs              *FsService
	Plugin          *PluginService
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

// WithHandlerErrorCallback sets a callback that is invoked when a notification
// handler or approval handler panics or returns an error. The callback receives
// the JSON-RPC method name and the error. If the callback itself panics, the
// panic is silently recovered.
func WithHandlerErrorCallback(cb func(method string, err error)) ClientOption {
	return func(c *Client) {
		c.handlerErrorCallback = cb
	}
}

// NewClient creates a new Client using the given transport and options.
func NewClient(transport Transport, opts ...ClientOption) *Client {
	if transport == nil {
		panic("nil transport")
	}

	c := &Client{
		transport:             transport,
		notificationListeners: make(map[string]NotificationHandler),
		internalListeners:     make(map[string][]internalListener),
		threadStates:          make(map[string]threadStateEntry),
		threadStateListeners:  make(map[string][]threadStateListener),
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
	c.DeviceKey = newDeviceKeyService(c)
	c.Model = newModelService(c)
	c.ModelProvider = newModelProviderService(c)
	c.Skills = newSkillsService(c)
	c.Apps = newAppsService(c)
	c.Marketplace = newMarketplaceService(c)
	c.Mcp = newMcpService(c)
	c.Command = newCommandService(c)
	c.Review = newReviewService(c)
	c.Feedback = newFeedbackService(c)
	c.ExternalAgent = newExternalAgentService(c)
	c.Experimental = newExperimentalService(c)
	c.System = newSystemService(c)
	c.Fs = newFsService(c)
	c.Plugin = newPluginService(c)
	c.FuzzyFileSearch = newFuzzyFileSearchService(c)
	c.installThreadStateCache()

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
	if ctx == nil {
		return Response{}, ErrNilContext
	}

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

// panicToError converts a recovered panic value to an error.
func panicToError(v any) error {
	switch e := v.(type) {
	case error:
		return e
	case string:
		return errors.New(e)
	default:
		return fmt.Errorf("panic: %v", e)
	}
}

// reportHandlerError invokes the handler error callback if set.
// Recovers from callback panics to prevent double-fault crashes.
func (c *Client) reportHandlerError(method string, err error) {
	cb := c.handlerErrorCallback
	if cb == nil {
		return
	}
	defer func() { recover() }() //nolint:errcheck // callback panic is intentionally swallowed
	cb(method, err)
}

// safeCallNotificationHandler calls fn, recovering any panic and reporting it
// via reportHandlerError.
func (c *Client) safeCallNotificationHandler(method string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			c.reportHandlerError(method, panicToError(r))
		}
	}()
	fn()
}

// handleNotification is the internal handler registered with the transport.
// It dispatches internal listeners before the public listener so lifecycle
// bookkeeping cannot be stalled behind user callbacks for the same
// notification.
// Each handler is called in isolation so a panic in one does not prevent others
// from executing.
func (c *Client) handleNotification(ctx context.Context, notif Notification) {
	c.listenersMu.RLock()
	handler := c.notificationListeners[notif.Method]
	// Deep-copy internal listeners so concurrent unsubscribe can't mutate the
	// backing array while we iterate outside the lock.
	src := c.internalListeners[notif.Method]
	internals := make([]internalListener, len(src))
	copy(internals, src)
	c.listenersMu.RUnlock()

	for _, il := range internals {
		c.safeCallNotificationHandler(notif.Method, func() {
			il.handler(ctx, notif)
		})
	}

	if handler != nil {
		c.safeCallNotificationHandler(notif.Method, func() {
			handler(ctx, notif)
		})
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
// Panics in approval handlers are recovered and reported via the handler error
// callback. Errors returned by approval handlers are also reported.
func (c *Client) handleRequest(ctx context.Context, req Request) (resp Response, err error) {
	defer func() {
		if r := recover(); r != nil {
			pErr := panicToError(r)
			c.reportHandlerError(req.Method, pErr)
			resp = Response{}
			err = pErr
		}
	}()

	resp, err = c.dispatchApproval(ctx, req)
	if err != nil {
		c.reportHandlerError(req.Method, err)
	}
	return resp, err
}

// dispatchApproval routes an incoming server→client request to the appropriate
// approval handler based on method name.
func (c *Client) dispatchApproval(ctx context.Context, req Request) (Response, error) {
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

	case methodPermissionsRequestApproval:
		if handlers.OnPermissionsRequestApproval == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return handleApproval(ctx, req, handlers.OnPermissionsRequestApproval)

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

	case methodMcpServerElicitationRequest:
		if handlers.OnMcpServerElicitationRequest == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return handleApproval(ctx, req, handlers.OnMcpServerElicitationRequest)

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
		return Response{}, fmt.Errorf("unmarshal %s params: %w", req.Method, errors.Join(errInvalidParams, err))
	}

	result, err := handler(ctx, params)
	if err != nil {
		return Response{}, fmt.Errorf("approval handler %s failed: %w", req.Method, err)
	}
	if err := validateDecodedResponse(result); err != nil {
		return Response{}, fmt.Errorf("validate %s result: %w", req.Method, err)
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
	return c.requestIDCounter.Add(1)
}

// sendResponse is a helper that sends a typed request and returns the raw response.
func (c *Client) sendResponse(ctx context.Context, method string, params interface{}) (Response, error) {
	preparedParams := params
	if params != nil {
		var err error
		preparedParams, err = prepareRequestParams(params)
		if err != nil {
			return Response{}, fmt.Errorf("%s: %w", method, err)
		}
	}

	// Marshal params to JSON
	paramsJSON, err := marshalForWire(preparedParams)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request params for %s: %w", method, err)
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
		return Response{}, fmt.Errorf("%s: %w", method, err)
	}

	return resp, nil
}

// sendRequest is a helper that sends a typed request and unmarshals the response.
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}, result interface{}) error {
	if result == nil {
		return fmt.Errorf("%s: %w", method, errNilResponseTarget)
	}

	resp, err := c.sendResponse(ctx, method, params)
	if err != nil {
		return err
	}

	if isEmptyResponseResult(resp.Result) {
		return fmt.Errorf("%s: %w", method, ErrEmptyResult)
	}
	if err := json.Unmarshal(resp.Result, result); err != nil {
		return fmt.Errorf("unmarshal response result for %s: %w", method, err)
	}
	if err := validateDecodedResponse(result); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}

	return nil
}

// sendEmptyObjectRequest is a helper for methods whose successful result schema
// is an object with no required fields. It rejects missing, null, and non-object
// results so protocol violations are surfaced instead of being treated as success.
func (c *Client) sendEmptyObjectRequest(ctx context.Context, method string, params interface{}) error {
	resp, err := c.sendResponse(ctx, method, params)
	if err != nil {
		return err
	}

	if err := validateObjectResponseResult(resp.Result); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}

	return nil
}

// sendRequestRaw is a helper that sends a typed request and returns the raw response result.
// This is useful for union types where the result needs custom unmarshaling.
func (c *Client) sendRequestRaw(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	resp, err := c.sendResponse(ctx, method, params)
	if err != nil {
		return nil, err
	}

	if isEmptyResponseResult(resp.Result) {
		return nil, fmt.Errorf("%s: %w", method, ErrEmptyResult)
	}

	return resp.Result, nil
}
