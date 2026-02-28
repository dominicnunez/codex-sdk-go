package codex_test

import (
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
