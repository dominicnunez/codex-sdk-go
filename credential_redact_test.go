package codex_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestCredentialTypesRedactWithAllFormatVerbs(t *testing.T) {
	secret := "sk-live-super-secret-token-12345"

	tests := []struct {
		name  string
		value fmt.Formatter
	}{
		{
			name: "ApiKeyLoginAccountParams",
			value: &codex.ApiKeyLoginAccountParams{
				Type:   "apiKey",
				ApiKey: secret,
			},
		},
		{
			name: "ChatgptAuthTokensLoginAccountParams",
			value: &codex.ChatgptAuthTokensLoginAccountParams{
				Type:             "chatgptAuthTokens",
				AccessToken:      secret,
				ChatgptAccountId: "acct-123",
			},
		},
		{
			name: "ChatgptAuthTokensRefreshResponse",
			value: &codex.ChatgptAuthTokensRefreshResponse{
				AccessToken:      secret,
				ChatgptAccountID: "acct-456",
			},
		},
	}

	// Verify json.Marshal also redacts
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/json.Marshal", tt.name), func(t *testing.T) {
			data, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			output := string(data)
			if strings.Contains(output, secret) {
				t.Errorf("%s leaked credential via json.Marshal: %s", tt.name, output)
			}
			if !strings.Contains(output, "[REDACTED]") {
				t.Errorf("%s json.Marshal did not include [REDACTED]: %s", tt.name, output)
			}
		})
	}

	// Verify handleApproval sends the unredacted token on the wire
	t.Run("ChatgptAuthTokensRefresh/wireProtocol", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		secret := "sk-live-super-secret-token-12345"
		client.SetApprovalHandlers(codex.ApprovalHandlers{
			OnChatgptAuthTokensRefresh: func(ctx context.Context, p codex.ChatgptAuthTokensRefreshParams) (codex.ChatgptAuthTokensRefreshResponse, error) {
				return codex.ChatgptAuthTokensRefreshResponse{
					AccessToken:      secret,
					ChatgptAccountID: "acct-wire",
				}, nil
			},
		})

		req := codex.Request{
			JSONRPC: "2.0",
			Method:  "account/chatgptAuthTokens/refresh",
			ID:      codex.RequestID{Value: float64(1)},
			Params:  json.RawMessage(`{"reason":"expired"}`),
		}

		resp, err := mock.InjectServerRequest(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected RPC error: %v", resp.Error)
		}

		wireJSON := string(resp.Result)
		if !strings.Contains(wireJSON, secret) {
			t.Errorf("wire response must contain unredacted token, got: %s", wireJSON)
		}
		if strings.Contains(wireJSON, "[REDACTED]") {
			t.Errorf("wire response must not contain [REDACTED], got: %s", wireJSON)
		}
	})

	verbs := []string{"%v", "%+v", "%#v", "%s"}

	for _, tt := range tests {
		for _, verb := range verbs {
			t.Run(fmt.Sprintf("%s/%s", tt.name, verb), func(t *testing.T) {
				output := fmt.Sprintf(verb, tt.value)
				if strings.Contains(output, secret) {
					t.Errorf("%s leaked credential with format verb %s: %s", tt.name, verb, output)
				}
				if !strings.Contains(output, "[REDACTED]") {
					t.Errorf("%s did not include [REDACTED] marker with format verb %s: %s", tt.name, verb, output)
				}
			})
		}
	}
}

func TestLoginWireSerialization_ApiKey_SendsUnredactedCredentials(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)

	apiKey := "sk-live-actual-key"
	_ = transport.SetResponseData("account/login/start", map[string]interface{}{
		"type": "apiKey",
	})

	ctx := context.Background()
	_, err := client.Account.Login(ctx, &codex.ApiKeyLoginAccountParams{
		Type:   "apiKey",
		ApiKey: apiKey,
	})
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}

	req := transport.GetSentRequest(0)
	if req == nil {
		t.Fatal("no request was sent")
	}
	if req.Method != "account/login/start" {
		t.Fatalf("method = %q, want account/login/start", req.Method)
	}

	wireJSON := string(req.Params)
	if !strings.Contains(wireJSON, apiKey) {
		t.Errorf("wire request must contain unredacted apiKey, got: %s", wireJSON)
	}
	if strings.Contains(wireJSON, "[REDACTED]") {
		t.Errorf("wire request must not contain [REDACTED], got: %s", wireJSON)
	}
}

func TestLoginWireSerialization_ChatgptAuthTokens_SendsUnredactedCredentials(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)

	accessToken := "access-token-123"
	_ = transport.SetResponseData("account/login/start", map[string]interface{}{
		"type": "chatgptAuthTokens",
	})

	ctx := context.Background()
	_, err := client.Account.Login(ctx, &codex.ChatgptAuthTokensLoginAccountParams{
		Type:             "chatgptAuthTokens",
		AccessToken:      accessToken,
		ChatgptAccountId: "acct-1",
	})
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}

	req := transport.GetSentRequest(0)
	if req == nil {
		t.Fatal("no request was sent")
	}
	if req.Method != "account/login/start" {
		t.Fatalf("method = %q, want account/login/start", req.Method)
	}

	wireJSON := string(req.Params)
	if !strings.Contains(wireJSON, accessToken) {
		t.Errorf("wire request must contain unredacted accessToken, got: %s", wireJSON)
	}
	if strings.Contains(wireJSON, "[REDACTED]") {
		t.Errorf("wire request must not contain [REDACTED], got: %s", wireJSON)
	}
	if !strings.Contains(wireJSON, "acct-1") {
		t.Errorf("wire request must contain chatgptAccountId, got: %s", wireJSON)
	}
}
