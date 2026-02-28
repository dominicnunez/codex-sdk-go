package codex

import (
	"context"
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
	UserAgent string `json:"userAgent"`
}

// Initialize sends an initialize request to the server.
// This is the v1 handshake that must be performed before using v2 protocol methods.
func (c *Client) Initialize(ctx context.Context, params InitializeParams) (InitializeResponse, error) {
	var result InitializeResponse
	if err := c.sendRequest(ctx, methodInitialize, params, &result); err != nil {
		return InitializeResponse{}, err
	}
	return result, nil
}
