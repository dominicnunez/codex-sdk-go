package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestBedrockAccountRejectsMalformedFlag(t *testing.T) {
	for _, members := range []string{
		`"usesCodexManagedCredentials":null`,
		`"usesCodexManagedCredentials":42`,
		`"usesCodexManagedCredentials":"true"`,
		`"usesCodexManagedCredentials":{}`,
		`"usesCodexManagedCredentials":[]`,
		`"usesCodexManagedCredentials":null,"usesCodexManagedCredentials":false`,
		`"usesCodexManagedCreden\u0074ials":null`,
	} {
		t.Run(members, func(t *testing.T) {
			raw := `{"type":"amazonBedrock",` + members + `}`
			prior := &p.UnknownAccount{Type: "future", Raw: json.RawMessage(`{"type":"future"}`)}
			wrapper := p.AccountWrapper{Value: prior}
			if err := json.Unmarshal([]byte(raw), &wrapper); err == nil {
				t.Error("known Bedrock flag accepted malformed value")
			} else if wrapper.Value != prior {
				t.Error("new rejection replaced prior receiver")
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":false,"account":` + raw + `}`)})
			result, err := client.Account.Get(context.Background(), p.GetAccountParams{})
			if err == nil || result != (p.GetAccountResponse{}) {
				t.Errorf("malformed account published: %v", err)
			}
			mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":false,"account":{"type":"amazonBedrock","usesCodexManagedCredentials":true}}`)})
			if _, err := client.Account.Get(context.Background(), p.GetAccountParams{}); err != nil || len(mock.SentRequests) != 2 {
				t.Fatalf("same-client recovery failed: %v", err)
			}
		})
	}
}

func TestBedrockAccountOtherBranchesRemainOpaque(t *testing.T) {
	for _, raw := range []string{
		`{"type":"future","usesCodexManagedCredentials":null}`,
		`{"type":"apiKey","usesCodexManagedCredentials":null}`,
		`{"type":"gcp","usesCodexManagedCredentials":{}}`,
		`{"type":"chatgpt","email":null,"planType":"free","usesCodexManagedCredentials":[]}`,
	} {
		var wrapper p.AccountWrapper
		if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
			t.Fatalf("unconstrained other branch narrowed: %v", err)
		}
	}
}

func TestBedrockAccountDefaultAndEncoding(t *testing.T) {
	for _, tc := range []struct {
		members string
		want    bool
	}{
		{"", false}, {`,"usesCodexManagedCredentials":false`, false}, {`,"usesCodexManagedCredentials":true`, true},
		{`,"usesCodexManagedCreden\u0074ials":true`, true},
		{`,"usesCodexManagedCredentials":true,"usesCodexManagedCredentials":false`, false},
	} {
		t.Run(tc.members, func(t *testing.T) {
			raw := `{"type":"amazonBedrock"` + tc.members + `}`
			var wrapper p.AccountWrapper
			if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
				t.Fatal(err)
			}
			account, ok := wrapper.Value.(*p.AmazonBedrockAccount)
			if !ok || account.Type != "amazonBedrock" || account.UsesCodexManagedCredentials != tc.want {
				t.Fatalf("typed/default metadata lost: %+v", wrapper.Value)
			}
			for _, value := range []any{account, &wrapper, &p.AmazonBedrockAccount{Type: "ignored", UsesCodexManagedCredentials: tc.want}} {
				encoded, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				wantFlag := "false"
				if tc.want {
					wantFlag = "true"
				}
				if string(fields["type"]) != `"amazonBedrock"` || string(fields["usesCodexManagedCredentials"]) != wantFlag {
					t.Fatalf("wire metadata changed: %s", encoded)
				}
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":false,"account":` + raw + `}`)})
			response, err := client.Account.Get(context.Background(), p.GetAccountParams{})
			if err != nil {
				t.Fatal(err)
			}
			got, ok := response.Account.Value.(*p.AmazonBedrockAccount)
			if !ok || got.UsesCodexManagedCredentials != tc.want {
				t.Fatal("public typed flag/default lost")
			}
		})
	}
}

func TestBedrockAccountDirectReceiverAndDiscriminator(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"type":null}`, `{"type":"gcp"}`, `{"type":"future","type":"amazonBedrock"}`,
		`{"type":"amazonBedrock","TYPE":"gcp"}`, `{"type":"amazonBedrock","usesCodexManagedCredentials":null}`,
		`{"type":"amazonBedrock","usesCodexManagedCredentials":42,"usesCodexManagedCredentials":true}`,
	} {
		prior := p.AmazonBedrockAccount{Type: "amazonBedrock", UsesCodexManagedCredentials: true}
		value := prior
		if err := json.Unmarshal([]byte(raw), &value); err == nil || value != prior {
			t.Fatalf("direct invalid value changed receiver: err=%v value=%+v", err, value)
		}
	}
	var wrapper p.AccountWrapper
	if err := json.Unmarshal([]byte(`{"type":"future","usesCodexManagedCredentials":null,"type":"amazonBedrock"}`), &wrapper); err == nil {
		t.Fatal("earlier discriminator/type occurrence bypassed known branch validation")
	}
	data := []byte(`{"type":"future","usesCodexManagedCredentials":null,"other":{"preserved":true}}`)
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatal(err)
	}
	unknown, ok := wrapper.Value.(*p.UnknownAccount)
	if !ok || string(unknown.Raw) != string(data) {
		t.Fatal("future raw account changed")
	}
	data[2] = 'X'
	if unknown.Raw[2] == 'X' {
		t.Fatal("future raw account borrows input")
	}
}
