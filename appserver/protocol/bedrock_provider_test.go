package protocol_test

import (
	"context"
	"encoding/json"
	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"strings"
	"testing"
)

func TestBedrockProviderSelection(t *testing.T) {
	for _, body := range []string{
		`{"type":"amazonBedrock","apiKey":"sentinel-api","region":"r","providerId":"amazon-bedrock-runtime"}`,
		`{"type":"amazonBedrockAccessKeys","accessKeyId":"sentinel-access","secretAccessKey":"sentinel-secret","sessionToken":"sentinel-session","region":"r","providerId":"amazon-bedrock-runtime"}`,
	} {
		var params p.LoginAccountParams = &p.AmazonBedrockLoginAccountParams{}
		if strings.Contains(body, "amazonBedrockAccessKeys") {
			params = &p.AmazonBedrockAccessKeysLoginAccountParams{}
		}
		if err := json.Unmarshal([]byte(body), params); err != nil {
			t.Fatal(err)
		}
		tr := NewMockTransport()
		c := p.NewClient(tr)
		defer c.Close()
		tr.SetResponse("account/login/start", p.Response{Result: json.RawMessage(`{"type":"amazonBedrock"}`)})
		if _, err := c.Account.Login(context.Background(), params); err != nil {
			t.Fatal(err)
		}
		wire := string(tr.GetSentRequest(0).Params)
		if !strings.Contains(wire, `"providerId":"amazon-bedrock-runtime"`) || !strings.Contains(wire, "sentinel-") {
			t.Errorf("wire lost provider/credentials: provider present=%v", strings.Contains(wire, `"providerId":"amazon-bedrock-runtime"`))
		}
		redacted, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(redacted), "sentinel-") || !strings.Contains(string(redacted), `"providerId":"amazon-bedrock-runtime"`) {
			t.Errorf("redacted login does not preserve provider safely")
		}
	}
}
