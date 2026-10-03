package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestConfigImportResponseContract(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		valid bool
	}{
		{"identifier", `{"importId":"import-a"}`, true},
		{"empty identifier", `{"importId":""}`, true},
		{"missing", `{}`, false},
		{"null identifier", `{"importId":null}`, false},
		{"wrong type", `{"importId":1}`, false},
		{"null object", `null`, false},
		{"array", `[]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockTransport()
			if err := mock.SetResponseData("externalAgentConfig/import", json.RawMessage(tc.body)); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(mock)
			defer client.Close()
			response, err := client.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{
				MigrationItems: []codex.ExternalAgentConfigMigrationItem{},
			})
			if !tc.valid {
				if err == nil {
					t.Fatal("invalid required response accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tc.body {
				t.Fatalf("response = %s, want %s", data, tc.body)
			}
		})
	}
}
