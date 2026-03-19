package codex

import (
	"context"
	"errors"
	"slices"
)

// ClientInfo represents information about the client application.
type ClientInfo struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Title   *string `json:"title,omitempty"`
}

// InitializeCapabilities represents client-declared capabilities negotiated during initialize.
type InitializeCapabilities struct {
	// ExperimentalAPI opts into receiving experimental API methods and fields.
	ExperimentalAPI bool `json:"experimentalApi"`

	// OptOutNotificationMethods are exact notification method names that should be suppressed
	// for this connection (for example "codex/event/session_configured").
	OptOutNotificationMethods []string `json:"optOutNotificationMethods,omitempty"`
}

// InitializeParams are the parameters for the initialize request.
type InitializeParams struct {
	ClientInfo   ClientInfo              `json:"clientInfo"`
	Capabilities *InitializeCapabilities `json:"capabilities,omitempty"`
}

// InitializeResponse is the response from the initialize request.
type InitializeResponse struct {
	PlatformFamily string `json:"platformFamily"`
	PlatformOS     string `json:"platformOs"`
	UserAgent      string `json:"userAgent"`
}

// InitializeParamsMismatchError reports that a later initialize call attempted
// to reuse an already initialized session with different handshake params.
type InitializeParamsMismatchError struct {
	Existing  InitializeParams
	Requested InitializeParams
}

func (e *InitializeParamsMismatchError) Error() string {
	return "initialize params do not match the active session"
}

func (r InitializeResponse) validate() error {
	switch {
	case r.PlatformFamily == "":
		return errors.New("missing platformFamily")
	case r.PlatformOS == "":
		return errors.New("missing platformOs")
	case r.UserAgent == "":
		return errors.New("missing userAgent")
	default:
		return nil
	}
}

func cloneClientInfo(info ClientInfo) ClientInfo {
	cp := info
	cp.Title = cloneStringPtr(info.Title)
	return cp
}

func cloneInitializeCapabilities(capabilities *InitializeCapabilities) *InitializeCapabilities {
	if capabilities == nil {
		return nil
	}
	cp := *capabilities
	cp.OptOutNotificationMethods = append([]string(nil), capabilities.OptOutNotificationMethods...)
	return &cp
}

func cloneInitializeParams(params InitializeParams) InitializeParams {
	cp := params
	cp.ClientInfo = cloneClientInfo(params.ClientInfo)
	cp.Capabilities = cloneInitializeCapabilities(params.Capabilities)
	return cp
}

func normalizeInitializeParams(params InitializeParams) InitializeParams {
	cp := cloneInitializeParams(params)
	if cp.Capabilities != nil && !cp.Capabilities.ExperimentalAPI && len(cp.Capabilities.OptOutNotificationMethods) == 0 {
		cp.Capabilities = nil
	}
	return cp
}

func initializeParamsEqual(a, b InitializeParams) bool {
	a = normalizeInitializeParams(a)
	b = normalizeInitializeParams(b)

	if a.ClientInfo.Name != b.ClientInfo.Name || a.ClientInfo.Version != b.ClientInfo.Version {
		return false
	}
	if !equalStringPtr(a.ClientInfo.Title, b.ClientInfo.Title) {
		return false
	}
	switch {
	case a.Capabilities == nil || b.Capabilities == nil:
		return a.Capabilities == nil && b.Capabilities == nil
	default:
		return a.Capabilities.ExperimentalAPI == b.Capabilities.ExperimentalAPI &&
			slices.Equal(a.Capabilities.OptOutNotificationMethods, b.Capabilities.OptOutNotificationMethods)
	}
}

func equalStringPtr(a, b *string) bool {
	switch {
	case a == nil || b == nil:
		return a == nil && b == nil
	default:
		return *a == *b
	}
}

func (c *Client) initializedParams() (InitializeParams, bool) {
	c.initializeMu.Lock()
	defer c.initializeMu.Unlock()

	if !c.initializeDone {
		return InitializeParams{}, false
	}
	return cloneInitializeParams(c.initializeParams), true
}

// Initialize sends an initialize request to the server.
// This is the one-time handshake that must be performed before using v2
// protocol methods. Successful calls are cached so repeated callers share the
// same initialized session, while failures are not latched and can be retried.
func (c *Client) Initialize(ctx context.Context, params InitializeParams) (InitializeResponse, error) {
	if err := validateContext(ctx); err != nil {
		return InitializeResponse{}, err
	}

	requested := normalizeInitializeParams(params)

	for {
		c.initializeMu.Lock()
		if c.initializeDone {
			existing := c.initializeParams
			resp := c.initializeResp
			c.initializeMu.Unlock()
			if !initializeParamsEqual(existing, requested) {
				return InitializeResponse{}, &InitializeParamsMismatchError{
					Existing:  cloneInitializeParams(existing),
					Requested: cloneInitializeParams(params),
				}
			}
			return resp, nil
		}
		if wait := c.initializeWait; wait != nil {
			c.initializeMu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return InitializeResponse{}, ctx.Err()
			}
		}

		wait := make(chan struct{})
		c.initializeWait = wait
		c.initializeMu.Unlock()

		var result InitializeResponse
		err := c.sendRequest(ctx, methodInitialize, params, &result)

		c.initializeMu.Lock()
		if err == nil {
			c.initializeDone = true
			c.initializeParams = requested
			c.initializeResp = result
		}
		c.initializeWait = nil
		close(wait)
		c.initializeMu.Unlock()

		if err != nil {
			return InitializeResponse{}, err
		}
		return result, nil
	}
}
