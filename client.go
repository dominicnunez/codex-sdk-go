package codex

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// Client is the main entry point for interacting with the Codex JSON-RPC server.
// It uses a Transport for bidirectional communication and provides typed methods
// for all protocol operations.
type Client struct {
	transport Transport

	// Request timeout (optional, can be overridden per-request via context)
	requestTimeout time.Duration

	// Notification listeners: method → handler function
	notificationListeners map[string]NotificationHandler
	listenersMu           sync.RWMutex

	// Approval handlers for server→client requests
	approvalHandlers ApprovalHandlers

	// Request ID counter for generating unique request IDs
	requestIDCounter uint64

	// Service accessors
	Thread        *ThreadService
	Turn          *TurnService
	Account       *AccountService
	Config        *ConfigService
	Model         *ModelService
	Skills        *SkillsService
	Apps          *AppsService
	Mcp           *McpService
	Command       *CommandService
	Review        *ReviewService
	Feedback      *FeedbackService
	ExternalAgent *ExternalAgentService
	Experimental  *ExperimentalService
	System        *SystemService
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
		// Check if it's a context timeout or cancellation
		if ctx.Err() == context.DeadlineExceeded {
			return Response{}, NewTimeoutError("request timeout exceeded")
		}
		if ctx.Err() == context.Canceled {
			return Response{}, NewTimeoutError("request cancelled")
		}
		// Wrap other errors as transport errors
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
func (c *Client) OnNotification(method string, handler NotificationHandler) {
	c.listenersMu.Lock()
	defer c.listenersMu.Unlock()
	c.notificationListeners[method] = handler
}

// handleNotification is the internal handler registered with the transport.
// It routes incoming notifications to the appropriate registered listener.
func (c *Client) handleNotification(ctx context.Context, notif Notification) {
	c.listenersMu.RLock()
	handler, ok := c.notificationListeners[notif.Method]
	c.listenersMu.RUnlock()

	if !ok {
		// Unknown notification method - ignore silently
		return
	}

	// Call the handler (in the same goroutine for now - transport already dispatches in goroutines)
	handler(ctx, notif)
}

// handleRequest is the internal handler for server→client requests (approval flows).
// It routes incoming requests to the appropriate approval handler.
func (c *Client) handleRequest(ctx context.Context, req Request) (Response, error) {
	// Route based on method to the appropriate approval handler
	switch req.Method {
	case "applyPatchApproval":
		if c.approvalHandlers.OnApplyPatchApproval == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return c.handleApplyPatchApproval(ctx, req)

	case "item/commandExecution/requestApproval":
		if c.approvalHandlers.OnCommandExecutionRequestApproval == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return c.handleCommandExecutionRequestApproval(ctx, req)

	case "execCommandApproval":
		if c.approvalHandlers.OnExecCommandApproval == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return c.handleExecCommandApproval(ctx, req)

	case "item/fileChange/requestApproval":
		if c.approvalHandlers.OnFileChangeRequestApproval == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return c.handleFileChangeRequestApproval(ctx, req)

	case "skill/requestApproval":
		if c.approvalHandlers.OnSkillRequestApproval == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return c.handleSkillRequestApproval(ctx, req)

	case "item/tool/call":
		if c.approvalHandlers.OnDynamicToolCall == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return c.handleDynamicToolCall(ctx, req)

	case "item/tool/requestUserInput":
		if c.approvalHandlers.OnToolRequestUserInput == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return c.handleToolRequestUserInput(ctx, req)

	case "fuzzyFileSearch":
		if c.approvalHandlers.OnFuzzyFileSearch == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return c.handleFuzzyFileSearch(ctx, req)

	case "account/chatgptAuthTokens/refresh":
		if c.approvalHandlers.OnChatgptAuthTokensRefresh == nil {
			return methodNotFoundResponse(req.ID), nil
		}
		return c.handleChatgptAuthTokensRefresh(ctx, req)

	default:
		// Unknown method - return method not found error
		return methodNotFoundResponse(req.ID), nil
	}
}

// methodNotFoundResponse creates a JSON-RPC method-not-found error response.
func methodNotFoundResponse(id RequestID) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    ErrCodeMethodNotFound,
			Message: "Method not found",
		},
	}
}

// Individual approval handler methods

func (c *Client) handleApplyPatchApproval(ctx context.Context, req Request) (Response, error) {
	var params ApplyPatchApprovalParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}

	result, err := c.approvalHandlers.OnApplyPatchApproval(ctx, params)
	if err != nil {
		return Response{}, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{}, err
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

func (c *Client) handleCommandExecutionRequestApproval(ctx context.Context, req Request) (Response, error) {
	var params CommandExecutionRequestApprovalParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}

	result, err := c.approvalHandlers.OnCommandExecutionRequestApproval(ctx, params)
	if err != nil {
		return Response{}, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{}, err
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

func (c *Client) handleExecCommandApproval(ctx context.Context, req Request) (Response, error) {
	var params ExecCommandApprovalParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}

	result, err := c.approvalHandlers.OnExecCommandApproval(ctx, params)
	if err != nil {
		return Response{}, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{}, err
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

func (c *Client) handleFileChangeRequestApproval(ctx context.Context, req Request) (Response, error) {
	var params FileChangeRequestApprovalParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}

	result, err := c.approvalHandlers.OnFileChangeRequestApproval(ctx, params)
	if err != nil {
		return Response{}, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{}, err
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

func (c *Client) handleSkillRequestApproval(ctx context.Context, req Request) (Response, error) {
	var params SkillRequestApprovalParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}

	result, err := c.approvalHandlers.OnSkillRequestApproval(ctx, params)
	if err != nil {
		return Response{}, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{}, err
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

func (c *Client) handleDynamicToolCall(ctx context.Context, req Request) (Response, error) {
	var params DynamicToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}

	result, err := c.approvalHandlers.OnDynamicToolCall(ctx, params)
	if err != nil {
		return Response{}, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{}, err
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

func (c *Client) handleToolRequestUserInput(ctx context.Context, req Request) (Response, error) {
	var params ToolRequestUserInputParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}

	result, err := c.approvalHandlers.OnToolRequestUserInput(ctx, params)
	if err != nil {
		return Response{}, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{}, err
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

func (c *Client) handleFuzzyFileSearch(ctx context.Context, req Request) (Response, error) {
	var params FuzzyFileSearchParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}

	result, err := c.approvalHandlers.OnFuzzyFileSearch(ctx, params)
	if err != nil {
		return Response{}, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{}, err
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

func (c *Client) handleChatgptAuthTokensRefresh(ctx context.Context, req Request) (Response, error) {
	var params ChatgptAuthTokensRefreshParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}

	result, err := c.approvalHandlers.OnChatgptAuthTokensRefresh(ctx, params)
	if err != nil {
		return Response{}, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{}, err
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

// Close closes the underlying transport and releases resources.
func (c *Client) Close() error {
	return c.transport.Close()
}

// nextRequestID generates a unique request ID for outgoing requests.
func (c *Client) nextRequestID() interface{} {
	id := atomic.AddUint64(&c.requestIDCounter, 1)
	return id
}

// sendRequest is a helper that sends a typed request and unmarshals the response.
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}, result interface{}) error {
	// Marshal params to JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return err
	}

	// Create request
	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
		ID:      RequestID{Value: c.nextRequestID()},
	}

	// Send request
	resp, err := c.Send(ctx, req)
	if err != nil {
		return err
	}

	// Unmarshal result if we have one
	if result != nil && resp.Result != nil {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return err
		}
	}

	return nil
}

// sendRequestRaw is a helper that sends a typed request and returns the raw response result.
// This is useful for union types where the result needs custom unmarshaling.
func (c *Client) sendRequestRaw(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// Marshal params to JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	// Create request
	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
		ID:      RequestID{Value: c.nextRequestID()},
	}

	// Send request
	resp, err := c.Send(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}
