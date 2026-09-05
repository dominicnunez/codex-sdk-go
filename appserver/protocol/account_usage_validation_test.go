package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestGetTokenUsageRequiresSummary(t *testing.T) {
	for _, tc := range []struct {
		payload string
		valid   bool
	}{
		{`{}`, false},
		{`{"summary":null}`, false},
		{`null`, false},
		{`{"summary":{}}`, true},
		{`{"summary":{"lifetimeTokens":0}}`, true},
	} {
		t.Run(tc.payload, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			t.Cleanup(func() { _ = client.Close() })
			if err := transport.SetResponseData("account/usage/read", json.RawMessage(tc.payload)); err != nil {
				t.Fatal(err)
			}
			_, err := client.Account.GetTokenUsage(context.Background(), nil)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
