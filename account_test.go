package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// ptr is a helper to create a pointer to any value
func ptr[T any](v T) *T {
	return &v
}

func TestAccountGet(t *testing.T) {
	tests := []struct {
		name     string
		params   codex.GetAccountParams
		response map[string]interface{}
		wantErr  bool
	}{
		{
			name:   "minimal_params",
			params: codex.GetAccountParams{},
			response: map[string]interface{}{
				"requiresOpenaiAuth": false,
				"account":            nil,
			},
		},
		{
			name: "with_refresh_token",
			params: codex.GetAccountParams{
				RefreshToken: ptr(true),
			},
			response: map[string]interface{}{
				"requiresOpenaiAuth": true,
				"account": map[string]interface{}{
					"type":     "chatgpt",
					"email":    "test@example.com",
					"planType": "plus",
				},
			},
		},
		{
			name: "apikey_account",
			params: codex.GetAccountParams{
				RefreshToken: ptr(false),
			},
			response: map[string]interface{}{
				"requiresOpenaiAuth": false,
				"account": map[string]interface{}{
					"type": "apiKey",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)

			_ = transport.SetResponseData("account/read", tt.response)

			ctx := context.Background()
			resp, err := client.Account.Get(ctx, tt.params)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				req := transport.GetSentRequest(0)
				if req.Method != "account/read" {
					t.Errorf("expected method account/get, got %s", req.Method)
				}

				// Verify params serialization
				var params codex.GetAccountParams
				if err := json.Unmarshal(req.Params, &params); err != nil {
					t.Fatalf("failed to unmarshal params: %v", err)
				}

				if resp.RequiresOpenaiAuth != tt.response["requiresOpenaiAuth"].(bool) {
					t.Errorf("RequiresOpenaiAuth mismatch")
				}
			}
		})
	}
}

func TestAccountGetRateLimits(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)

	_ = transport.SetResponseData("account/rateLimits/read", map[string]interface{}{
		"rateLimits": map[string]interface{}{
			"limitId":   "codex",
			"limitName": "Codex Rate Limit",
			"planType":  "plus",
			"credits": map[string]interface{}{
				"hasCredits": true,
				"unlimited":  false,
				"balance":    "100",
			},
			"primary": map[string]interface{}{
				"usedPercent":        50,
				"resetsAt":           1234567890,
				"windowDurationMins": 60,
			},
			"secondary": nil,
		},
		"rateLimitsByLimitId": map[string]interface{}{
			"codex": map[string]interface{}{
				"limitId": "codex",
				"primary": map[string]interface{}{
					"usedPercent": 50,
				},
			},
		},
	})

	ctx := context.Background()
	resp, err := client.Account.GetRateLimits(ctx)

	if err != nil {
		t.Fatalf("GetRateLimits() error = %v", err)
	}

	req := transport.GetSentRequest(0)
	if req.Method != "account/rateLimits/read" {
		t.Errorf("expected method account/getRateLimits, got %s", req.Method)
	}

	if resp.RateLimits.Primary == nil {
		t.Fatal("expected non-nil primary rate limit window")
	}
	if resp.RateLimits.Primary.UsedPercent != 50 {
		t.Errorf("expected usedPercent = 50, got %d", resp.RateLimits.Primary.UsedPercent)
	}
}

func TestAccountLogin(t *testing.T) {
	tests := []struct {
		name     string
		params   codex.LoginAccountParams
		response map[string]interface{}
		wantErr  bool
	}{
		{
			name: "apikey_login",
			params: &codex.ApiKeyLoginAccountParams{
				Type:   "apiKey",
				ApiKey: "sk-test-key-123",
			},
			response: map[string]interface{}{
				"type": "apiKey",
			},
		},
		{
			name: "chatgpt_login",
			params: &codex.ChatgptLoginAccountParams{
				Type: "chatgpt",
			},
			response: map[string]interface{}{
				"type":    "chatgpt",
				"authUrl": "https://auth.example.com/oauth/authorize",
				"loginId": "login-123",
			},
		},
		{
			name: "chatgpt_auth_tokens_login",
			params: &codex.ChatgptAuthTokensLoginAccountParams{
				Type:              "chatgptAuthTokens",
				AccessToken:       "token-123",
				ChatgptAccountId:  "account-456",
				ChatgptPlanType:   ptr("plus"),
			},
			response: map[string]interface{}{
				"type": "chatgptAuthTokens",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)

			_ = transport.SetResponseData("account/login/start", tt.response)

			ctx := context.Background()
			resp, err := client.Account.Login(ctx, tt.params)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Login() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				req := transport.GetSentRequest(0)
				if req.Method != "account/login/start" {
					t.Errorf("expected method account/login, got %s", req.Method)
				}

				// Verify response type matches request type
				switch p := tt.params.(type) {
				case *codex.ApiKeyLoginAccountParams:
					if _, ok := resp.(*codex.ApiKeyLoginAccountResponse); !ok {
						t.Error("expected ApiKeyLoginAccountResponse")
					}
				case *codex.ChatgptLoginAccountParams:
					if chatgptResp, ok := resp.(*codex.ChatgptLoginAccountResponse); !ok {
						t.Error("expected ChatgptLoginAccountResponse")
					} else {
						if chatgptResp.AuthUrl != tt.response["authUrl"].(string) {
							t.Error("authUrl mismatch")
						}
						if chatgptResp.LoginId != tt.response["loginId"].(string) {
							t.Error("loginId mismatch")
						}
					}
				case *codex.ChatgptAuthTokensLoginAccountParams:
					if _, ok := resp.(*codex.ChatgptAuthTokensLoginAccountResponse); !ok {
						t.Error("expected ChatgptAuthTokensLoginAccountResponse")
					}
				default:
					_ = p
				}
			}
		})
	}
}

func TestAccountCancelLogin(t *testing.T) {
	tests := []struct {
		name     string
		params   codex.CancelLoginAccountParams
		response map[string]interface{}
		status   string
	}{
		{
			name: "canceled",
			params: codex.CancelLoginAccountParams{
				LoginId: "login-123",
			},
			response: map[string]interface{}{
				"status": "canceled",
			},
			status: "canceled",
		},
		{
			name: "not_found",
			params: codex.CancelLoginAccountParams{
				LoginId: "nonexistent-login",
			},
			response: map[string]interface{}{
				"status": "notFound",
			},
			status: "notFound",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)

			_ = transport.SetResponseData("account/login/cancel", tt.response)

			ctx := context.Background()
			resp, err := client.Account.CancelLogin(ctx, tt.params)

			if err != nil {
				t.Fatalf("CancelLogin() error = %v", err)
			}

			req := transport.GetSentRequest(0)
			if req.Method != "account/login/cancel" {
				t.Errorf("expected method account/login/cancel, got %s", req.Method)
			}

			if string(resp.Status) != tt.status {
				t.Errorf("expected status %s, got %s", tt.status, resp.Status)
			}
		})
	}
}

func TestAccountLogout(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)

	_ = transport.SetResponseData("account/logout", map[string]interface{}{})

	ctx := context.Background()
	resp, err := client.Account.Logout(ctx)

	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	req := transport.GetSentRequest(0)
	if req.Method != "account/logout" {
		t.Errorf("expected method account/logout, got %s", req.Method)
	}

	// Response is a struct, so it can't be nil - just verify no error
	_ = resp
}

func TestAccountServiceMethodSignatures(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)

	// Compile-time verification that all methods exist with correct signatures
	_ = func(ctx context.Context, params codex.GetAccountParams) (codex.GetAccountResponse, error) {
		return client.Account.Get(ctx, params)
	}
	_ = func(ctx context.Context) (codex.GetAccountRateLimitsResponse, error) {
		return client.Account.GetRateLimits(ctx)
	}
	_ = func(ctx context.Context, params codex.LoginAccountParams) (codex.LoginAccountResponse, error) {
		return client.Account.Login(ctx, params)
	}
	_ = func(ctx context.Context, params codex.CancelLoginAccountParams) (codex.CancelLoginAccountResponse, error) {
		return client.Account.CancelLogin(ctx, params)
	}
	_ = func(ctx context.Context) (codex.LogoutAccountResponse, error) {
		return client.Account.Logout(ctx)
	}
}

func TestAccountUpdatedNotification(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)

	tests := []struct {
		name     string
		authMode *codex.AuthMode
	}{
		{
			name:     "apikey_mode",
			authMode: authModePtr(codex.AuthModeAPIKey),
		},
		{
			name:     "chatgpt_mode",
			authMode: authModePtr(codex.AuthModeChatGPT),
		},
		{
			name:     "chatgpt_auth_tokens_mode",
			authMode: authModePtr(codex.AuthModeChatGPTAuthTokens),
		},
		{
			name:     "no_auth_mode",
			authMode: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notificationReceived := false
			var receivedAuthMode *codex.AuthMode

			client.OnAccountUpdated(func(notif codex.AccountUpdatedNotification) {
				notificationReceived = true
				receivedAuthMode = notif.AuthMode
			})

			params := map[string]interface{}{}
			if tt.authMode != nil {
				params["authMode"] = string(*tt.authMode)
			}
			paramsJSON, _ := json.Marshal(params)

			transport.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "account/updated",
				Params:  paramsJSON,
			})

			if !notificationReceived {
				t.Error("notification handler not called")
			}

			if (receivedAuthMode == nil) != (tt.authMode == nil) {
				t.Errorf("authMode presence mismatch: got %v, want %v", receivedAuthMode, tt.authMode)
			}
			if receivedAuthMode != nil && tt.authMode != nil && *receivedAuthMode != *tt.authMode {
				t.Errorf("authMode = %s, want %s", *receivedAuthMode, *tt.authMode)
			}
		})
	}
}

func authModePtr(m codex.AuthMode) *codex.AuthMode {
	return &m
}

func TestAccountLoginCompletedNotification(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)

	tests := []struct {
		name    string
		success bool
		loginId *string
		errMsg  *string
	}{
		{
			name:    "success",
			success: true,
			loginId: ptr("login-123"),
			errMsg:  nil,
		},
		{
			name:    "failure",
			success: false,
			loginId: ptr("login-456"),
			errMsg:  ptr("authentication failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notificationReceived := false
			var receivedSuccess bool
			var receivedLoginId *string
			var receivedError *string

			client.OnAccountLoginCompleted(func(notif codex.AccountLoginCompletedNotification) {
				notificationReceived = true
				receivedSuccess = notif.Success
				receivedLoginId = notif.LoginId
				receivedError = notif.Error
			})

			params := map[string]interface{}{
				"success": tt.success,
			}
			if tt.loginId != nil {
				params["loginId"] = *tt.loginId
			}
			if tt.errMsg != nil {
				params["error"] = *tt.errMsg
			}
			paramsJSON, _ := json.Marshal(params)

			transport.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "account/login/completed",
				Params:  paramsJSON,
			})

			if !notificationReceived {
				t.Error("notification handler not called")
			}

			if receivedSuccess != tt.success {
				t.Errorf("success = %v, want %v", receivedSuccess, tt.success)
			}

			if (receivedLoginId == nil) != (tt.loginId == nil) {
				t.Errorf("loginId presence mismatch")
			}
			if receivedLoginId != nil && tt.loginId != nil && *receivedLoginId != *tt.loginId {
				t.Errorf("loginId = %s, want %s", *receivedLoginId, *tt.loginId)
			}

			if (receivedError == nil) != (tt.errMsg == nil) {
				t.Errorf("error presence mismatch")
			}
			if receivedError != nil && tt.errMsg != nil && *receivedError != *tt.errMsg {
				t.Errorf("error = %s, want %s", *receivedError, *tt.errMsg)
			}
		})
	}
}

func TestAccountRateLimitsUpdatedNotification(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)

	notificationReceived := false
	var receivedRateLimits *codex.RateLimitSnapshot

	client.OnAccountRateLimitsUpdated(func(notif codex.AccountRateLimitsUpdatedNotification) {
		notificationReceived = true
		receivedRateLimits = &notif.RateLimits
	})

	params := map[string]interface{}{
		"rateLimits": map[string]interface{}{
			"limitId":   "codex",
			"limitName": "Codex Rate Limit",
			"planType":  "plus",
			"credits": map[string]interface{}{
				"hasCredits": true,
				"unlimited":  false,
				"balance":    "100",
			},
			"primary": map[string]interface{}{
				"usedPercent":        75,
				"resetsAt":           1234567890,
				"windowDurationMins": 60,
			},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	transport.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "account/rateLimits/updated",
		Params:  paramsJSON,
	})

	if !notificationReceived {
		t.Error("notification handler not called")
	}

	if receivedRateLimits == nil {
		t.Fatal("expected non-nil rateLimits")
	}

	if receivedRateLimits.LimitId == nil || *receivedRateLimits.LimitId != "codex" {
		t.Errorf("limitId mismatch")
	}

	if receivedRateLimits.Primary == nil {
		t.Fatal("expected non-nil primary rate limit window")
	}

	if receivedRateLimits.Primary.UsedPercent != 75 {
		t.Errorf("usedPercent = %d, want 75", receivedRateLimits.Primary.UsedPercent)
	}
}
