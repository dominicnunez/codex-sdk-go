package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func requireWireFields(t *testing.T, value any, expected string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	for key, raw := range want {
		var a, b any
		if err := json.Unmarshal(got[key], &a); err != nil {
			t.Errorf("%s missing: %v", key, err)
			continue
		}
		if err := json.Unmarshal(raw, &b); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s = %s, want %s", key, got[key], raw)
		}
	}
}

func TestConfigCatalogMetadata(t *testing.T) {
	const fields = `{"approvals_reviewer":"auto_review","desktop":{"nested":{"enabled":false}},"model_auto_compact_token_limit_scope":"body_after_prefix","service_tier":"custom-tier"}`
	mock := NewMockTransport()
	if err := mock.SetResponseData("config/read", json.RawMessage(`{"config":`+fields+`,"origins":{}}`)); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(mock)
	defer client.Close()
	response, err := client.Config.Read(context.Background(), codex.ConfigReadParams{})
	if err != nil {
		t.Fatal(err)
	}
	requireWireFields(t, response.Config, fields)
}

func TestModelCatalogMetadata(t *testing.T) {
	const model = `{"id":"model-a","model":"model-a","displayName":"A","description":"test","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[],"additionalSpeedTiers":["legacy"],"defaultServiceTier":"priority","modelSpecialty":"coding","multiAgentVersion":"v2","serviceTiers":[{"id":"priority","name":"Priority","description":"Fast"}],"upgradeInfo":{"model":"model-b","retirementAt":123}}`
	mock := NewMockTransport()
	if err := mock.SetResponseData("model/list", json.RawMessage(`{"data":[`+model+`]}`)); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(mock)
	defer client.Close()
	response, err := client.Model.List(context.Background(), codex.ModelListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 1 {
		t.Fatal("missing model")
	}
	requireWireFields(t, response.Data[0], model)
}

func TestCatalogMetadataValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		body   string
		valid  bool
	}{
		{"config absent", "config/read", `{"config":{},"origins":{}}`, true},
		{"config null", "config/read", `{"config":{"desktop":null,"approvals_reviewer":null,"model_auto_compact_token_limit_scope":null,"service_tier":null},"origins":{}}`, true},
		{"desktop array", "config/read", `{"config":{"desktop":[]},"origins":{}}`, false},
		{"desktop scalar", "config/read", `{"config":{"desktop":1},"origins":{}}`, false},
		{"reviewer enum", "config/read", `{"config":{"approvals_reviewer":"invalid"},"origins":{}}`, false},
		{"compact enum", "config/read", `{"config":{"model_auto_compact_token_limit_scope":"invalid"},"origins":{}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockTransport()
			if err := mock.SetResponseData(tc.method, json.RawMessage(tc.body)); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(mock)
			defer client.Close()
			_, err := client.Config.Read(context.Background(), codex.ConfigReadParams{})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, err=%v", tc.valid, err)
			}
		})
	}
	const base = `"id":"m","model":"m","displayName":"M","description":"d","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[]`
	for _, tc := range []struct {
		fields string
		valid  bool
	}{
		{``, true},
		{`,"additionalSpeedTiers":[],"serviceTiers":[],"multiAgentVersion":null`, true},
		{`,"additionalSpeedTiers":null`, false},
		{`,"serviceTiers":null`, false},
		{`,"serviceTiers":[{"id":"x","name":"X"}]`, false},
		{`,"serviceTiers":[{"id":"x","name":"X","description":null}]`, false},
		{`,"multiAgentVersion":"v3"`, false},
	} {
		t.Run(tc.fields, func(t *testing.T) {
			mock := NewMockTransport()
			if err := mock.SetResponseData("model/list", json.RawMessage(`{"data":[{`+base+tc.fields+`}]}`)); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(mock)
			defer client.Close()
			_, err := client.Model.List(context.Background(), codex.ModelListParams{})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, err=%v", tc.valid, err)
			}
		})
	}
}

func TestCatalogPresentEmptyMetadata(t *testing.T) {
	var config codex.Config
	if err := json.Unmarshal([]byte(`{"desktop":{}}`), &config); err != nil {
		t.Fatal(err)
	}
	requireWireFields(t, config, `{"desktop":{}}`)
	var model codex.Model
	const body = `{"id":"m","model":"m","displayName":"M","description":"d","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[],"serviceTiers":[],"additionalSpeedTiers":[]}`
	if err := json.Unmarshal([]byte(body), &model); err != nil {
		t.Fatal(err)
	}
	requireWireFields(t, model, `{"serviceTiers":[],"additionalSpeedTiers":[]}`)
}
