package codex

import (
	"context"
	"encoding/json"
	"fmt"
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
	ExperimentalAPI bool `json:"experimentalApi,omitempty"`

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
	// Marshal params to JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return InitializeResponse{}, err
	}

	// Create request
	req := Request{
		JSONRPC: "2.0",
		ID:      RequestID{Value: c.nextRequestID()},
		Method:  "initialize",
		Params:  paramsJSON,
	}

	// Send request
	resp, err := c.Send(ctx, req)
	if err != nil {
		return InitializeResponse{}, err
	}

	// Parse response
	if resp.Result == nil {
		return InitializeResponse{}, fmt.Errorf("initialize: server returned empty result")
	}
	var result InitializeResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return InitializeResponse{}, err
	}

	return result, nil
}
