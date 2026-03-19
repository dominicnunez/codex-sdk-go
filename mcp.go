package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// McpAuthStatus represents the authentication status of an MCP server.
type McpAuthStatus string

const (
	McpAuthStatusUnsupported McpAuthStatus = "unsupported"
	McpAuthStatusNotLoggedIn McpAuthStatus = "notLoggedIn"
	McpAuthStatusBearerToken McpAuthStatus = "bearerToken"
	McpAuthStatusOAuth       McpAuthStatus = "oAuth"
)

// Resource represents a resource exposed by an MCP server.
type Resource struct {
	Name        string      `json:"name"`
	URI         string      `json:"uri"`
	Description *string     `json:"description,omitempty"`
	Title       *string     `json:"title,omitempty"`
	MimeType    *string     `json:"mimeType,omitempty"`
	Size        *int64      `json:"size,omitempty"`
	Icons       interface{} `json:"icons,omitempty"`
	Meta        interface{} `json:"_meta,omitempty"`
	Annotations interface{} `json:"annotations,omitempty"`
}

func (r *Resource) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "name", "uri"); err != nil {
		return err
	}
	type wire Resource
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = Resource(decoded)
	return nil
}

// ResourceTemplate represents a URI template for dynamic resources.
type ResourceTemplate struct {
	Name        string      `json:"name"`
	UriTemplate string      `json:"uriTemplate"`
	Description *string     `json:"description,omitempty"`
	Title       *string     `json:"title,omitempty"`
	MimeType    *string     `json:"mimeType,omitempty"`
	Annotations interface{} `json:"annotations,omitempty"`
}

func (r *ResourceTemplate) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "name", "uriTemplate"); err != nil {
		return err
	}
	type wire ResourceTemplate
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ResourceTemplate(decoded)
	return nil
}

// Tool represents a tool (function) exposed by an MCP server.
type Tool struct {
	Name         string      `json:"name"`
	InputSchema  interface{} `json:"inputSchema"`
	Description  *string     `json:"description,omitempty"`
	Title        *string     `json:"title,omitempty"`
	OutputSchema interface{} `json:"outputSchema,omitempty"`
	Icons        interface{} `json:"icons,omitempty"`
	Meta         interface{} `json:"_meta,omitempty"`
	Annotations  interface{} `json:"annotations,omitempty"`
}

func (t *Tool) UnmarshalJSON(data []byte) error {
	if err := validateObjectFields(data, []string{"inputSchema", "name"}, []string{"name"}); err != nil {
		return err
	}
	type wire Tool
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*t = Tool(decoded)
	return nil
}

// McpServerStatus represents the status of a single MCP server.
type McpServerStatus struct {
	AuthStatus        McpAuthStatus      `json:"authStatus"`
	Name              string             `json:"name"`
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	Resources         []Resource         `json:"resources"`
	Tools             map[string]Tool    `json:"tools"`
}

func (s *McpServerStatus) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "authStatus", "name", "resourceTemplates", "resources", "tools"); err != nil {
		return err
	}
	type wire McpServerStatus
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = McpServerStatus(decoded)
	return nil
}

// ListMcpServerStatusParams are parameters for the mcpServerStatus/list request.
type ListMcpServerStatusParams struct {
	Cursor *string `json:"cursor,omitempty"`
	Limit  *uint32 `json:"limit,omitempty"`
}

// ListMcpServerStatusResponse is the response from mcpServerStatus/list.
type ListMcpServerStatusResponse struct {
	Data       []McpServerStatus `json:"data"`
	NextCursor *string           `json:"nextCursor,omitempty"`
}

func (r *ListMcpServerStatusResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "data"); err != nil {
		return err
	}
	type wire ListMcpServerStatusResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ListMcpServerStatusResponse(decoded)
	return nil
}

// McpServerOauthLoginParams are parameters for the mcpServer/oauth/login request.
type McpServerOauthLoginParams struct {
	Name        string    `json:"name"`
	Scopes      *[]string `json:"scopes,omitempty"`
	TimeoutSecs *int64    `json:"timeoutSecs,omitempty"`
}

// McpServerOauthLoginResponse is the response from mcpServer/oauth/login.
type McpServerOauthLoginResponse struct {
	AuthorizationUrl string `json:"authorizationUrl"`
}

func (r *McpServerOauthLoginResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "authorizationUrl"); err != nil {
		return err
	}
	type wire McpServerOauthLoginResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = McpServerOauthLoginResponse(decoded)
	return nil
}

// McpServerRefreshResponse is the response from config/mcpServer/reload.
type McpServerRefreshResponse struct{}

// McpServerOauthLoginCompletedNotification is sent when OAuth login completes.
type McpServerOauthLoginCompletedNotification struct {
	Name    string  `json:"name"`
	Success bool    `json:"success"`
	Error   *string `json:"error,omitempty"`
}

func (n *McpServerOauthLoginCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire McpServerOauthLoginCompletedNotification
	var decoded wire
	required := []string{"name", "success"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = McpServerOauthLoginCompletedNotification(decoded)
	return nil
}

// McpToolCallProgressNotification is sent to report progress of MCP tool calls.
type McpToolCallProgressNotification struct {
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	Message  string `json:"message"`
}

func (n *McpToolCallProgressNotification) UnmarshalJSON(data []byte) error {
	type wire McpToolCallProgressNotification
	var decoded wire
	required := []string{"itemId", "message", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = McpToolCallProgressNotification(decoded)
	return nil
}

// McpService handles MCP server operations.
type McpService struct {
	client *Client
}

func newMcpService(client *Client) *McpService {
	return &McpService{client: client}
}

// ListServerStatus retrieves the status of all configured MCP servers.
func (s *McpService) ListServerStatus(ctx context.Context, params ListMcpServerStatusParams) (ListMcpServerStatusResponse, error) {
	var resp ListMcpServerStatusResponse
	if err := s.client.sendRequest(ctx, methodMcpServerStatusList, params, &resp); err != nil {
		return ListMcpServerStatusResponse{}, err
	}
	return resp, nil
}

// OauthLogin initiates OAuth login flow for an MCP server.
func (s *McpService) OauthLogin(ctx context.Context, params McpServerOauthLoginParams) (McpServerOauthLoginResponse, error) {
	var resp McpServerOauthLoginResponse
	if err := s.client.sendRequest(ctx, methodMcpServerOauthLogin, params, &resp); err != nil {
		return McpServerOauthLoginResponse{}, err
	}
	return resp, nil
}

// Refresh refreshes MCP server connections.
func (s *McpService) Refresh(ctx context.Context) (McpServerRefreshResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodConfigMcpServerReload, nil); err != nil {
		return McpServerRefreshResponse{}, err
	}
	return McpServerRefreshResponse{}, nil
}

// OnMcpServerOauthLoginCompleted registers a listener for OAuth login completion notifications.
func (c *Client) OnMcpServerOauthLoginCompleted(handler func(McpServerOauthLoginCompletedNotification)) {
	if handler == nil {
		c.OnNotification(notifyMcpServerOauthLoginCompleted, nil)
		return
	}
	c.OnNotification(notifyMcpServerOauthLoginCompleted, func(ctx context.Context, notif Notification) {
		var params McpServerOauthLoginCompletedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyMcpServerOauthLoginCompleted, fmt.Errorf("unmarshal %s: %w", notifyMcpServerOauthLoginCompleted, err))
			return
		}
		handler(params)
	})
}

// OnMcpToolCallProgress registers a listener for MCP tool call progress notifications.
func (c *Client) OnMcpToolCallProgress(handler func(McpToolCallProgressNotification)) {
	if handler == nil {
		c.OnNotification(notifyMcpToolCallProgress, nil)
		return
	}
	c.OnNotification(notifyMcpToolCallProgress, func(ctx context.Context, notif Notification) {
		var params McpToolCallProgressNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyMcpToolCallProgress, fmt.Errorf("unmarshal %s: %w", notifyMcpToolCallProgress, err))
			return
		}
		handler(params)
	})
}
