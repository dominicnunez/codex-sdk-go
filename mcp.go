package codex

import (
	"context"
	"encoding/json"
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

// ResourceTemplate represents a URI template for dynamic resources.
type ResourceTemplate struct {
	Name        string      `json:"name"`
	UriTemplate string      `json:"uriTemplate"`
	Description *string     `json:"description,omitempty"`
	Title       *string     `json:"title,omitempty"`
	MimeType    *string     `json:"mimeType,omitempty"`
	Annotations interface{} `json:"annotations,omitempty"`
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

// McpServerStatus represents the status of a single MCP server.
type McpServerStatus struct {
	AuthStatus        McpAuthStatus      `json:"authStatus"`
	Name              string             `json:"name"`
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	Resources         []Resource         `json:"resources"`
	Tools             map[string]Tool    `json:"tools"`
}

// ListMcpServerStatusParams are parameters for the mcp/listServerStatus request.
type ListMcpServerStatusParams struct {
	Cursor *string `json:"cursor,omitempty"`
	Limit  *uint32 `json:"limit,omitempty"`
}

// ListMcpServerStatusResponse is the response from mcp/listServerStatus.
type ListMcpServerStatusResponse struct {
	Data       []McpServerStatus `json:"data"`
	NextCursor *string           `json:"nextCursor,omitempty"`
}

// McpServerOauthLoginParams are parameters for the mcp/server/oauthLogin request.
type McpServerOauthLoginParams struct {
	Name        string    `json:"name"`
	Scopes      *[]string `json:"scopes,omitempty"`
	TimeoutSecs *int64    `json:"timeoutSecs,omitempty"`
}

// McpServerOauthLoginResponse is the response from mcp/server/oauthLogin.
type McpServerOauthLoginResponse struct {
	AuthorizationUrl string `json:"authorizationUrl"`
}

// McpServerRefreshResponse is the response from mcp/server/refresh.
type McpServerRefreshResponse struct{}

// McpServerOauthLoginCompletedNotification is sent when OAuth login completes.
type McpServerOauthLoginCompletedNotification struct {
	Name    string  `json:"name"`
	Success bool    `json:"success"`
	Error   *string `json:"error,omitempty"`
}

// McpToolCallProgressNotification is sent to report progress of MCP tool calls.
type McpToolCallProgressNotification struct {
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	Message  string `json:"message"`
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
	if err := s.client.sendRequest(ctx, "mcpServerStatus/list", params, &resp); err != nil {
		return ListMcpServerStatusResponse{}, err
	}
	return resp, nil
}

// OauthLogin initiates OAuth login flow for an MCP server.
func (s *McpService) OauthLogin(ctx context.Context, params McpServerOauthLoginParams) (McpServerOauthLoginResponse, error) {
	var resp McpServerOauthLoginResponse
	if err := s.client.sendRequest(ctx, "mcpServer/oauth/login", params, &resp); err != nil {
		return McpServerOauthLoginResponse{}, err
	}
	return resp, nil
}

// Refresh refreshes MCP server connections.
func (s *McpService) Refresh(ctx context.Context) (McpServerRefreshResponse, error) {
	if err := s.client.sendRequest(ctx, "config/mcpServer/reload", nil, nil); err != nil {
		return McpServerRefreshResponse{}, err
	}
	return McpServerRefreshResponse{}, nil
}

// OnMcpServerOauthLoginCompleted registers a listener for OAuth login completion notifications.
func (c *Client) OnMcpServerOauthLoginCompleted(handler func(McpServerOauthLoginCompletedNotification)) {
	c.OnNotification("mcpServer/oauthLogin/completed", func(ctx context.Context, notif Notification) {
		var params McpServerOauthLoginCompletedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			return
		}
		handler(params)
	})
}

// OnMcpToolCallProgress registers a listener for MCP tool call progress notifications.
func (c *Client) OnMcpToolCallProgress(handler func(McpToolCallProgressNotification)) {
	c.OnNotification("item/mcpToolCall/progress", func(ctx context.Context, notif Notification) {
		var params McpToolCallProgressNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			return
		}
		handler(params)
	})
}
