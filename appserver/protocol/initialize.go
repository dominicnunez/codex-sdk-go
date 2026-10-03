package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonvalue"
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
	// ExplicitGatewayOAuth uses explicit gateway OAuth login instead of automatic browser authorization.
	ExplicitGatewayOAuth bool `json:"explicitGatewayOauth,omitempty"`
	// Extensions declares MCP extension settings. Empty maps declare no extensions.
	// Each value is arbitrary JSON. Initialize snapshots its canonical JSON representation.
	// Raw values preserve exact numbers without changing ordinary struct decoding. Handshake
	// identity ignores object key order but retains array order and number spelling.
	// Absent, null and empty root maps declare no extensions; named null settings
	// and empty settings objects still declare their extension names.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
	// McpServerOpenaiFormElicitation is the legacy openai/form opt-in.
	McpServerOpenaiFormElicitation bool `json:"mcpServerOpenaiFormElicitation,omitempty"`
	// RequestAttestation opts into attestation/generate requests.
	RequestAttestation bool `json:"requestAttestation,omitempty"`

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
	CodexHome      string `json:"codexHome"`
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
	case r.CodexHome == "":
		return errors.New("missing codexHome")
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
	return cloneArbitraryValue(capabilities)
}

func cloneInitializeParams(params InitializeParams) InitializeParams {
	cp := params
	cp.ClientInfo = cloneClientInfo(params.ClientInfo)
	cp.Capabilities = cloneInitializeCapabilities(params.Capabilities)
	return cp
}

func normalizeInitializeParams(params InitializeParams) InitializeParams {
	return normalizeOwnedInitializeParams(cloneInitializeParams(params))
}

// The input owns all references. Only normalization's capability fields need
// separate containers to preserve the admitted parameters for diagnostics.
func normalizeOwnedInitializeParams(params InitializeParams) InitializeParams {
	cp := params
	if cp.Capabilities != nil {
		capabilities := *cp.Capabilities
		cp.Capabilities = &capabilities
		cp.Capabilities.OptOutNotificationMethods = normalizeNotificationMethodSet(cp.Capabilities.OptOutNotificationMethods)
	}
	if cp.Capabilities != nil && !cp.Capabilities.ExperimentalAPI && !cp.Capabilities.ExplicitGatewayOAuth && !cp.Capabilities.McpServerOpenaiFormElicitation && !cp.Capabilities.RequestAttestation && len(cp.Capabilities.Extensions) == 0 && len(cp.Capabilities.OptOutNotificationMethods) == 0 {
		cp.Capabilities = nil
	}
	return cp
}

func normalizeNotificationMethodSet(methods []string) []string {
	if len(methods) == 0 {
		return nil
	}

	normalized := append([]string(nil), methods...)
	slices.Sort(normalized)
	return slices.Compact(normalized)
}

// Both inputs have been admitted as owned JSON settings and normalized.
func normalizedInitializeParamsEqual(a, b InitializeParams) bool {
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
			a.Capabilities.ExplicitGatewayOAuth == b.Capabilities.ExplicitGatewayOAuth &&
			a.Capabilities.McpServerOpenaiFormElicitation == b.Capabilities.McpServerOpenaiFormElicitation &&
			a.Capabilities.RequestAttestation == b.Capabilities.RequestAttestation &&
			extensionsEqual(a.Capabilities.Extensions, b.Capabilities.Extensions) &&
			slices.Equal(a.Capabilities.OptOutNotificationMethods, b.Capabilities.OptOutNotificationMethods)
	}
}

// Admission already canonicalized each raw setting, preserving number spelling.
func extensionsEqual(a, b map[string]json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == 0 && len(b) == 0
	}
	return reflect.DeepEqual(a, b)
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

// InitializedParams reports the latched initialize params after a successful initialize call.
func (c *Client) InitializedParams() (InitializeParams, bool) {
	return c.initializedParams()
}

// Initialize sends an initialize request to the server.
// This is the one-time handshake that must be performed before using v2
// protocol methods. Successful calls are cached so repeated callers share the
// same initialized session, while failures are not latched and can be retried.
func (c *Client) Initialize(ctx context.Context, params InitializeParams) (InitializeResponse, error) {
	if err := validateContext(ctx); err != nil {
		return InitializeResponse{}, err
	}

	admitted := params
	var extensions map[string]json.RawMessage
	if params.Capabilities != nil {
		var err error
		extensions, err = jsonvalue.CloneObject(params.Capabilities.Extensions)
		if err != nil {
			return InitializeResponse{}, fmt.Errorf("snapshot initialize extensions: %w", err)
		}
		capabilities := *params.Capabilities
		capabilities.Extensions = nil
		admitted.Capabilities = &capabilities
	}
	admitted = cloneInitializeParams(admitted)
	if admitted.Capabilities != nil {
		admitted.Capabilities.Extensions = extensions
	}
	requested := normalizeOwnedInitializeParams(admitted)

	for {
		c.initializeMu.Lock()
		if c.initializeDone {
			existing := c.initializeParams
			resp := c.initializeResp
			c.initializeMu.Unlock()
			if !normalizedInitializeParamsEqual(existing, requested) {
				return InitializeResponse{}, &InitializeParamsMismatchError{
					Existing:  cloneInitializeParams(existing),
					Requested: cloneInitializeParams(admitted),
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
		err := c.sendRequest(ctx, methodInitialize, requested, &result)

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
