package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOtelExporterValidation(t *testing.T) {
	for _, raw := range []string{`null`, `false`, `"future"`, `{}`, `{"otlp-http":null}`, `{"otlp-http":{"endpoint":"e","headers":{}}}`, `{"otlp-grpc":{"endpoint":"e","headers":null}}`, `{"otlp-grpc":{"endpoint":"e","headers":{"key":null}}}`, `{"otlp-http":{"endpoint":"e","headers":{},"protocol":"future"}}`, `{"otlp-grpc":{"endpoint":"e","headers":{}},"extra":0}`} {
		original := OtelExporter{Kind: "none"}
		value := original
		if err := json.Unmarshal([]byte(raw), &value); err == nil || !reflect.DeepEqual(value, original) {
			t.Fatalf("invalid exporter %s: %+v", raw, value)
		}
	}
	for _, raw := range []string{`{"span_attributes":{"key":null}}`, `{"tracestate":{"key":null}}`, `{"tracestate":{"key":{"inner":null}}}`, `{"tool_result":{}}`, `{"tool_result":{"max_bytes":null}}`, `{"tool_result":{"max_bytes":-1}}}`} {
		var value OtelRequirements
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			t.Fatalf("invalid requirements accepted: %s", raw)
		}
	}
	for _, raw := range []string{`{"span_attributes":{},"tracestate":{}}`, `{"exporter":"none","metrics_exporter":"statsig","trace_exporter":{"otlp-grpc":{"endpoint":"opaque","headers":{}}},"log_agent_responses":false,"log_guardian_assessments":true}`} {
		assertSyncRoundTrip(t, raw, &OtelRequirements{})
	}
	for _, value := range []OtelExporter{{}, {Kind: "none", GRPC: &OtelGRPCExporter{}}, {HTTP: &OtelHTTPExporter{Protocol: OtelHttpProtocolJSON}}, {GRPC: &OtelGRPCExporter{Headers: map[string]string{}}, HTTP: &OtelHTTPExporter{}}} {
		if _, err := json.Marshal(value); err == nil {
			t.Fatalf("invalid constructed exporter accepted: %+v", value)
		}
	}
}

func FuzzOtelExporterContract(f *testing.F) {
	for branch := uint8(0); branch < 5; branch++ {
		for mutation := uint8(0); mutation < 4; mutation++ {
			f.Add(branch, mutation, "endpoint", "value")
		}
	}
	f.Fuzz(func(t *testing.T, branch, mutation uint8, endpoint, header string) {
		if len(endpoint) > 128 || len(header) > 128 {
			return
		}
		branch %= 5
		mutation %= 4
		valid := mutation == 0
		var input any
		if branch < 2 {
			input = []string{"none", "statsig"}[branch]
			if !valid {
				input = "future"
			}
		} else {
			payload := map[string]any{"endpoint": endpoint, "headers": map[string]any{"key": header}}
			tag := "otlp-grpc"
			if branch < 4 {
				tag = "otlp-http"
				payload["protocol"] = []string{"binary", "json"}[branch-2]
			}
			object := map[string]any{tag: payload}
			switch mutation {
			case 1:
				payload["headers"] = nil
			case 2:
				payload["endpoint"] = 7
			case 3:
				object["extra"] = true
			}
			input = object
		}
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		value := OtelExporter{Kind: "none"}
		err = json.Unmarshal(raw, &value)
		if valid {
			if err != nil {
				t.Fatalf("valid generated exporter rejected: %s: %v", raw, err)
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var expected, actual any
			if err := json.Unmarshal(raw, &expected); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(expected, actual) {
				t.Fatalf("exporter changed: %s => %s", raw, encoded)
			}
		} else if err == nil || value.Kind != "none" || value.HTTP != nil || value.GRPC != nil {
			t.Fatalf("invalid exporter published: %s => %+v, %v", raw, value, err)
		}
	})
}

func TestOtelRejectsHiddenInvalidOccurrences(t *testing.T) {
	for _, raw := range []string{
		`{"exporter":{"otlp-http":null,"otlp-http":{"endpoint":"e","headers":{},"protocol":"json"}}}`,
		`{"exporter":{"otlp-grpc":{"endpoint":"e","headers":{"key":null,"key":"ok"}}}}`,
		`{"span_attributes":{"key":null,"key":"ok"}}`,
		`{"tracestate":{"outer":{"key":null,"key":"ok"}}}`,
		`{"tracestate":{"outer":null,"outer":{"key":"ok"}}}`,
		`{"exporter":{"otlp-grpc":{"endpoint":"e","ENDPOINT":null,"headers":{}}}}`,
		`{"exporter":{"otlp-grpc":{"endpoint":"e","headers":{},"HEADERS":null}}}`,
	} {
		value := OtelRequirements{Environment: Ptr("old")}
		old := value.Environment
		if err := json.Unmarshal([]byte(raw), &value); err == nil || value.Environment != old {
			t.Errorf("hidden invalid occurrence published: %s, %+v", raw, value)
		}
	}
}
