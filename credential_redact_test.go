package codex_test

import (
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
