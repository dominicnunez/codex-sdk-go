package codex_test

import (
	"context"
	"encoding/json"
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

			_ = mock.SetResponseData("mcp/listServerStatus", tt.responseData)

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

			_ = mock.SetResponseData("mcp/server/oauthLogin", tt.responseData)

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
	_ = mock.SetResponseData("mcp/server/refresh", map[string]interface{}{})

	_, err := client.Mcp.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Verify correct method name was sent
	req := mock.GetSentRequest(0)
	if req == nil {
		t.Fatal("no request sent")
	}
	if req.Method != "mcp/server/refresh" {
		t.Errorf("got method %q, want %q", req.Method, "mcp/server/refresh")
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
				Method:  "mcp/server/oauthLoginCompleted",
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
		Method:  "mcp/toolCallProgress",
		Params:  json.RawMessage(paramsJSON),
	})

	if !called {
		t.Fatal("notification handler not called")
	}

	if receivedNotif.ItemId != "item123" {
		t.Errorf("got itemId=%q, want %q", receivedNotif.ItemId, "item123")
	}
	if receivedNotif.ThreadId != "thread456" {
		t.Errorf("got threadId=%q, want %q", receivedNotif.ThreadId, "thread456")
	}
	if receivedNotif.TurnId != "turn789" {
		t.Errorf("got turnId=%q, want %q", receivedNotif.TurnId, "turn789")
	}
	if receivedNotif.Message != "Processing tool call..." {
		t.Errorf("got message=%q, want %q", receivedNotif.Message, "Processing tool call...")
	}
}

func TestMcpServiceMethodSignatures(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Compile-time verification that all methods exist with correct signatures
	var _ func(context.Context, codex.ListMcpServerStatusParams) (codex.ListMcpServerStatusResponse, error) = client.Mcp.ListServerStatus
	var _ func(context.Context, codex.McpServerOauthLoginParams) (codex.McpServerOauthLoginResponse, error) = client.Mcp.OauthLogin
	var _ func(context.Context) (codex.McpServerRefreshResponse, error) = client.Mcp.Refresh
}
