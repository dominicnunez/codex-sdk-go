package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

func TestMcpListServerStatus(t *testing.T) {
	tests := []struct {
		name         string
		params       codex.ListMcpServerStatusParams
		responseData map[string]interface{}
		wantData     int // number of servers expected
	}{
		{
			name:   "minimal list",
			params: codex.ListMcpServerStatusParams{},
			responseData: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"authStatus":        "notLoggedIn",
						"name":              "github",
						"resourceTemplates": []interface{}{},
						"resources":         []interface{}{},
						"tools":             map[string]interface{}{},
					},
				},
			},
			wantData: 1,
		},
		{
			name: "paginated with cursor and limit",
			params: codex.ListMcpServerStatusParams{
				Cursor: ptr("cursor123"),
				Limit:  ptr(uint32(10)),
			},
			responseData: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"authStatus": "oAuth",
						"name":       "slack",
						"resourceTemplates": []interface{}{
							map[string]interface{}{
								"name":        "channel",
								"uriTemplate": "slack://channel/{id}",
							},
						},
						"resources": []interface{}{
							map[string]interface{}{
								"name": "general",
								"uri":  "slack://channel/C123",
							},
						},
						"tools": map[string]interface{}{
							"send_message": map[string]interface{}{
								"name":        "send_message",
								"inputSchema": map[string]interface{}{"type": "object"},
							},
						},
					},
				},
				"nextCursor": "cursor456",
			},
			wantData: 1,
		},
		{
			name:   "empty list",
			params: codex.ListMcpServerStatusParams{},
			responseData: map[string]interface{}{
				"data": []interface{}{},
			},
			wantData: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			_ = mock.SetResponseData("mcpServerStatus/list", tt.responseData)

			resp, err := client.Mcp.ListServerStatus(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("ListServerStatus failed: %v", err)
			}

			if len(resp.Data) != tt.wantData {
				t.Errorf("got %d servers, want %d", len(resp.Data), tt.wantData)
			}

			if tt.wantData > 0 {
				// Verify first server has required fields
				server := resp.Data[0]
				if server.Name == "" {
					t.Error("server.Name is empty")
				}
				if server.AuthStatus == "" {
					t.Error("server.AuthStatus is empty")
				}
			}
		})
	}
}

func TestMcpListServerStatusRejectsInvalidAuthStatus(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	_ = mock.SetResponseData("mcpServerStatus/list", map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{
				"authStatus":        "sessionCookie",
				"name":              "github",
				"resourceTemplates": []interface{}{},
				"resources":         []interface{}{},
				"tools":             map[string]interface{}{},
			},
		},
	})

	_, err := client.Mcp.ListServerStatus(context.Background(), codex.ListMcpServerStatusParams{})
	if err == nil || !strings.Contains(err.Error(), `invalid mcpServerStatus.authStatus "sessionCookie"`) {
		t.Fatalf("ListServerStatus error = %v; want invalid auth status failure", err)
	}
}

func TestMcpOauthLogin(t *testing.T) {
	tests := []struct {
		name         string
		params       codex.McpServerOauthLoginParams
		responseData map[string]interface{}
		wantURL      string
	}{
		{
			name: "minimal login",
			params: codex.McpServerOauthLoginParams{
				Name: "github",
			},
			responseData: map[string]interface{}{
				"authorizationUrl": "https://github.com/login/oauth/authorize?client_id=abc",
			},
			wantURL: "https://github.com/login/oauth/authorize?client_id=abc",
		},
		{
			name: "with scopes and timeout",
			params: codex.McpServerOauthLoginParams{
				Name:        "slack",
				Scopes:      &[]string{"chat:write", "channels:read"},
				TimeoutSecs: ptr(int64(300)),
			},
			responseData: map[string]interface{}{
				"authorizationUrl": "https://slack.com/oauth/v2/authorize?scopes=chat:write,channels:read",
			},
			wantURL: "https://slack.com/oauth/v2/authorize?scopes=chat:write,channels:read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			_ = mock.SetResponseData("mcpServer/oauth/login", tt.responseData)

			resp, err := client.Mcp.OauthLogin(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("OauthLogin failed: %v", err)
			}

			if resp.AuthorizationUrl != tt.wantURL {
				t.Errorf("got URL %q, want %q", resp.AuthorizationUrl, tt.wantURL)
			}
		})
	}
}

func TestMcpRefresh(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Empty response per spec
	_ = mock.SetResponseData("config/mcpServer/reload", map[string]interface{}{})

	_, err := client.Mcp.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Verify correct method name was sent
	req := mock.GetSentRequest(0)
	if req == nil {
		t.Fatal("no request sent")
		return
	}
	if req.Method != "config/mcpServer/reload" {
		t.Errorf("got method %q, want %q", req.Method, "config/mcpServer/reload")
	}
}

func TestMcpOauthLoginCompletedNotification(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    map[string]interface{}
		wantSuccess bool
		wantError   *string
	}{
		{
			name: "success",
			jsonData: map[string]interface{}{
				"name":    "github",
				"success": true,
			},
			wantSuccess: true,
			wantError:   nil,
		},
		{
			name: "failure with error",
			jsonData: map[string]interface{}{
				"name":    "slack",
				"success": false,
				"error":   "user_denied_access",
			},
			wantSuccess: false,
			wantError:   ptr("user_denied_access"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			var called bool
			var receivedNotif codex.McpServerOauthLoginCompletedNotification

			client.OnMcpServerOauthLoginCompleted(func(notif codex.McpServerOauthLoginCompletedNotification) {
				called = true
				receivedNotif = notif
			})

			paramsJSON, _ := json.Marshal(tt.jsonData)
			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "mcpServer/oauthLogin/completed",
				Params:  json.RawMessage(paramsJSON),
			})

			if !called {
				t.Fatal("notification handler not called")
			}

			if receivedNotif.Success != tt.wantSuccess {
				t.Errorf("got success=%v, want %v", receivedNotif.Success, tt.wantSuccess)
			}

			if tt.wantError == nil {
				if receivedNotif.Error != nil {
					t.Errorf("got error=%q, want nil", *receivedNotif.Error)
				}
			} else {
				if receivedNotif.Error == nil {
					t.Error("got nil error, want non-nil")
				} else if *receivedNotif.Error != *tt.wantError {
					t.Errorf("got error=%q, want %q", *receivedNotif.Error, *tt.wantError)
				}
			}
		})
	}
}

func TestMcpOauthLoginCompletedMissingRequiredFieldReportsHandlerError(t *testing.T) {
	mock := NewMockTransport()

	var (
		gotMethod string
		gotErr    error
	)
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
		gotMethod = method
		gotErr = err
	}))

	var called bool
	client.OnMcpServerOauthLoginCompleted(func(codex.McpServerOauthLoginCompletedNotification) {
		called = true
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "mcpServer/oauthLogin/completed",
		Params:  json.RawMessage(`{"success":true}`),
	})

	if called {
		t.Fatal("handler should not be called for malformed payload")
	}
	if gotMethod != "mcpServer/oauthLogin/completed" {
		t.Fatalf("handler error method = %q; want %q", gotMethod, "mcpServer/oauthLogin/completed")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "missing required field") {
		t.Fatalf("handler error = %v; want missing required field failure", gotErr)
	}
}

func TestMcpToolCallProgressNotification(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	var called bool
	var receivedNotif codex.McpToolCallProgressNotification

	client.OnMcpToolCallProgress(func(notif codex.McpToolCallProgressNotification) {
		called = true
		receivedNotif = notif
	})

	jsonData := map[string]interface{}{
		"itemId":   "item123",
		"threadId": "thread456",
		"turnId":   "turn789",
		"message":  "Processing tool call...",
	}

	paramsJSON, _ := json.Marshal(jsonData)
	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/mcpToolCall/progress",
		Params:  json.RawMessage(paramsJSON),
	})

	if !called {
		t.Fatal("notification handler not called")
	}

	if receivedNotif.ItemID != "item123" {
		t.Errorf("got itemId=%q, want %q", receivedNotif.ItemID, "item123")
	}
	if receivedNotif.ThreadID != "thread456" {
		t.Errorf("got threadId=%q, want %q", receivedNotif.ThreadID, "thread456")
	}
	if receivedNotif.TurnID != "turn789" {
		t.Errorf("got turnId=%q, want %q", receivedNotif.TurnID, "turn789")
	}
	if receivedNotif.Message != "Processing tool call..." {
		t.Errorf("got message=%q, want %q", receivedNotif.Message, "Processing tool call...")
	}
}

func TestMcpToolCallProgressMissingRequiredFieldReportsHandlerError(t *testing.T) {
	mock := NewMockTransport()

	var (
		gotMethod string
		gotErr    error
	)
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
		gotMethod = method
		gotErr = err
	}))

	var called bool
	client.OnMcpToolCallProgress(func(codex.McpToolCallProgressNotification) {
		called = true
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/mcpToolCall/progress",
		Params:  json.RawMessage(`{"itemId":"item123","threadId":"thread456","turnId":"turn789"}`),
	})

	if called {
		t.Fatal("handler should not be called for malformed payload")
	}
	if gotMethod != "item/mcpToolCall/progress" {
		t.Fatalf("handler error method = %q; want %q", gotMethod, "item/mcpToolCall/progress")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "missing required field") {
		t.Fatalf("handler error = %v; want missing required field failure", gotErr)
	}
}

func TestMcpListServerStatus_RPCError_ReturnsRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	mock.SetResponse("mcpServerStatus/list", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    codex.ErrCodeInternalError,
			Message: "mcp registry unavailable",
		},
	})

	_, err := client.Mcp.ListServerStatus(context.Background(), codex.ListMcpServerStatusParams{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rpcErr *codex.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected error to unwrap to *RPCError, got %T", err)
	}
	if rpcErr.RPCError().Code != codex.ErrCodeInternalError {
		t.Errorf("expected error code %d, got %d", codex.ErrCodeInternalError, rpcErr.RPCError().Code)
	}
}
