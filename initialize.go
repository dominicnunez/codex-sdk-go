package codex

import (
	"context"
	"errors"
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

// Initialize sends an initialize request to the server.
// This is the one-time handshake that must be performed before using v2
// protocol methods. Successful calls are cached so repeated callers share the
// same initialized session, while failures are not latched and can be retried.
func (c *Client) Initialize(ctx context.Context, params InitializeParams) (InitializeResponse, error) {
	if err := validateContext(ctx); err != nil {
		return InitializeResponse{}, err
	}

	for {
		c.initializeMu.Lock()
		if c.initializeDone {
			resp := c.initializeResp
			c.initializeMu.Unlock()
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
