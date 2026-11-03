package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func configSourceCall(t *testing.T, client *p.Client, mock *MockTransport, route, source string) (any, error) {
	t.Helper()
	ctx := context.Background()
	if route == "origins" || route == "layers" {
		body := `{"config":{},"origins":{"model":{"name":` + source + `,"version":"v"}}}`
		if route == "layers" {
			body = `{"config":{},"origins":{},"layers":[{"config":{},"name":` + source + `,"version":"v"}]}`
		}
		mock.SetResponse("config/read", p.Response{Result: json.RawMessage(body)})
		return client.Config.Read(ctx, p.ConfigReadParams{})
	}
	body := `{"filePath":"/tmp/config.toml","status":"okOverridden","version":"v","overriddenMetadata":{"effectiveValue":null,"message":"","overridingLayer":{"name":` + source + `,"version":"v"}}}`
	if route == "write" {
		mock.SetResponse("config/value/write", p.Response{Result: json.RawMessage(body)})
		return client.Config.Write(ctx, p.ConfigValueWriteParams{KeyPath: "model", MergeStrategy: p.MergeStrategyReplace, Value: json.RawMessage(`null`)})
	}
	mock.SetResponse("config/batchWrite", p.Response{Result: json.RawMessage(body)})
	return client.Config.BatchWrite(ctx, p.ConfigBatchWriteParams{Edits: []p.ConfigEdit{}})
}

func TestKnownConfigSourcesRejectMalformed(t *testing.T) {
	for _, raw := range []string{
		`{"type":"packagedDefaults"}`,
		`{"type":"packagedDefaults","file":null}`,
		`{"type":"packagedDefaults","file":42}`,
		`{"type":"packagedDefaults","file":"relative/defaults.toml"}`,
		`{"type":"packagedDefaults","file":"/tmp/../defaults.toml"}`,
		`{"type":"enterpriseManaged","name":"policy"}`,
		`{"type":"enterpriseManaged","id":null,"name":"policy"}`,
		`{"type":"enterpriseManaged","id":"id"}`,
		`{"type":"enterpriseManaged","id":"id","name":null}`,
		`{"type":"enterpriseManaged","id":42,"name":"policy"}`,
		`{"type":"enterpriseManaged","id":null,"id":"id","name":"policy"}`,
	} {
		t.Run(raw, func(t *testing.T) {
			prior := p.UnknownConfigLayerSource{Type: "future", Raw: json.RawMessage(`{"type":"future"}`)}
			wrapper := p.ConfigLayerSourceWrapper{Value: prior}
			if err := json.Unmarshal([]byte(raw), &wrapper); err == nil {
				t.Error("malformed known config source admitted")
			} else if got, ok := wrapper.Value.(p.UnknownConfigLayerSource); !ok || &got.Raw[0] != &prior.Raw[0] {
				t.Error("new rejection changed prior receiver reference")
			}
			for _, route := range []string{"origins", "layers", "write", "batch"} {
				t.Run(route, func(t *testing.T) {
					mock := NewMockTransport()
					client := p.NewClient(mock)
					defer client.Close()
					result, err := configSourceCall(t, client, mock, route, raw)
					if err == nil || !reflect.ValueOf(result).IsZero() {
						t.Errorf("malformed source published: %v", err)
					}
					if _, err := configSourceCall(t, client, mock, route, `{"type":"packagedDefaults","file":"/unopened-fixture/defaults.toml"}`); err != nil || len(mock.SentRequests) != 2 {
						t.Fatalf("same-client valid recovery: %v", err)
					}
				})
			}
		})
	}
}

func TestKnownConfigSourceValidControls(t *testing.T) {
	for _, raw := range []string{
		`{"type":"packagedDefaults","file":"/unopened-fixture/defaults.toml"}`,
		`{"type":"enterpriseManaged","id":"","name":""}`,
		`{"type":"enterpriseManaged","id":"id","name":"Policy","file":"relative-extra"}`,
		`{"type":"sessionFlags","file":null,"id":null,"name":null}`,
		`{"type":"future","file":null,"id":null,"name":null}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var wrapper p.ConfigLayerSourceWrapper
			if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
				t.Fatal(err)
			}
			for _, route := range []string{"origins", "layers", "write", "batch"} {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				_, err := configSourceCall(t, client, mock, route, raw)
				client.Close()
				if err != nil {
					t.Fatalf("valid %s source rejected: %v", route, err)
				}
			}
		})
	}
}
