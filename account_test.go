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
					t.Errorf("expected method account/read, got %s", req.Method)
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
		t.Errorf("expected method account/rateLimits/read, got %s", req.Method)
	}

	if resp.RateLimits.Primary == nil {
		t.Fatal("expected non-nil primary rate limit window")
	}
	if resp.RateLimits.Primary.UsedPercent != 50 {
		t.Errorf("expected usedPercent = 50, got %d", resp.RateLimits.Primary.UsedPercent)
	}
}

func TestLoginParamsMarshalJSONHardcodesType(t *testing.T) {
	t.Run("ApiKey_redacted_uses_correct_type", func(t *testing.T) {
		p := &codex.ApiKeyLoginAccountParams{ApiKey: "sk-xxx"} // Type intentionally omitted
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]string
		_ = json.Unmarshal(b, &out)
		if out["type"] != "apiKey" {
			t.Errorf("redacted type = %q, want %q", out["type"], "apiKey")
		}
		if out["apiKey"] != "[REDACTED]" {
			t.Errorf("apiKey should be redacted, got %q", out["apiKey"])
		}
	})
	t.Run("ChatgptAuthTokens_redacted_uses_correct_type", func(t *testing.T) {
		p := &codex.ChatgptAuthTokensLoginAccountParams{AccessToken: "tok", ChatgptAccountId: "acct"}
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]string
		_ = json.Unmarshal(b, &out)
		if out["type"] != "chatgptAuthTokens" {
			t.Errorf("redacted type = %q, want %q", out["type"], "chatgptAuthTokens")
		}
	})
}

func TestLoginParamsHardcodeTypeDiscriminator(t *testing.T) {
	tests := []struct {
		name     string
		params   codex.LoginAccountParams
		wantType string
	}{
		{
			name:     "ApiKey_without_Type_set",
			params:   &codex.ApiKeyLoginAccountParams{ApiKey: "sk-xxx"},
			wantType: "apiKey",
		},
		{
			name:     "ApiKey_with_wrong_Type",
			params:   &codex.ApiKeyLoginAccountParams{Type: "wrong", ApiKey: "sk-xxx"},
			wantType: "apiKey",
		},
		{
			name:     "Chatgpt_without_Type_set",
			params:   &codex.ChatgptLoginAccountParams{},
			wantType: "chatgpt",
		},
		{
			name:     "ChatgptAuthTokens_without_Type_set",
			params:   &codex.ChatgptAuthTokensLoginAccountParams{AccessToken: "tok", ChatgptAccountId: "acct"},
			wantType: "chatgptAuthTokens",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)

			_ = transport.SetResponseData("account/login/start", map[string]interface{}{
				"type": tt.wantType,
			})

			_, _ = client.Account.Login(context.Background(), tt.params)

			req := transport.GetSentRequest(0)
			var envelope struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(req.Params, &envelope); err != nil {
				t.Fatalf("unmarshal params: %v", err)
			}
			if envelope.Type != tt.wantType {
				t.Errorf("wire type = %q, want %q", envelope.Type, tt.wantType)
			}
		})
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
				Type:             "chatgptAuthTokens",
				AccessToken:      "token-123",
				ChatgptAccountId: "account-456",
				ChatgptPlanType:  ptr("plus"),
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
					t.Errorf("expected method account/login/start, got %s", req.Method)
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

func TestAccountMarshalJSONInjectsTypeDiscriminator(t *testing.T) {
	tests := []struct {
		name    string
		account codex.Account
		want    string
	}{
		{
			name:    "ApiKeyAccount",
			account: &codex.ApiKeyAccount{},
			want:    `{"type":"apiKey"}`,
		},
		{
			name:    "ChatgptAccount_with_fields",
			account: &codex.ChatgptAccount{Email: "a@b.com", PlanType: codex.PlanTypePlus},
			want:    `{"type":"chatgpt","email":"a@b.com","planType":"plus"}`,
		},
		{
			name:    "ChatgptAccount_zero_value",
			account: &codex.ChatgptAccount{},
			want:    `{"type":"chatgpt","email":"","planType":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.account)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestAccountWrapperRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		wrapper *codex.AccountWrapper
		want    string
	}{
		{
			name:    "apiKey",
			wrapper: &codex.AccountWrapper{Value: &codex.ApiKeyAccount{}},
			want:    `{"type":"apiKey"}`,
		},
		{
			name:    "chatgpt",
			wrapper: &codex.AccountWrapper{Value: &codex.ChatgptAccount{Email: "a@b.com", PlanType: codex.PlanTypePro}},
			want:    `{"type":"chatgpt","email":"a@b.com","planType":"pro"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.wrapper)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}

			var roundTripped codex.AccountWrapper
			if err := json.Unmarshal(got, &roundTripped); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
		})
	}
}
