package protocol

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSyncMetadataFields(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		target    any
	}{
		{"workspace", `{"type":"chatgpt","email":null,"planType":"plus","workspaceName":"Team"}`, &ChatgptAccount{}},
		{"thread hooks", `{"threadId":"t"}`, &HooksListParams{}},
		{"usage", `{"estimatedUsageCreditsMicros":0,"groups":[],"threadId":"t","nativeUsageUsdMicros":0}`, &ThreadUsage{}},
		{"drain", `{"exitCode":0,"processHandle":"p","stderr":"","stderrCapReached":false,"stdout":"","stdoutCapReached":false,"outputDrainTimedOut":false}`, &ProcessExitedNotification{}},
		{"generation", `{"type":"imageGeneration","id":"i","status":"completed","fileId":"f","generationId":"g"}`, &ThreadItemWrapper{}},
		{"times", `{"type":"plan","id":"i","text":"plan","startedAtMs":0,"completedAtMs":-1}`, &ThreadItemWrapper{}},
	} {
		t.Run(tc.name, func(t *testing.T) { assertSyncRoundTrip(t, tc.raw, tc.target) })
	}
}

func TestSyncOtelRetention(t *testing.T) {
	for _, raw := range []string{`{"otel":{"exporter":"statsig","log_user_prompt":false,"tool_result":{"max_bytes":0}}}`, `{"otel":{"exporter":{"otlp-http":{"endpoint":"https://example.invalid","headers":{},"protocol":"json","tls":{"ca-certificate":"opaque"}}}}}`} {
		assertSyncRoundTrip(t, raw, &ConfigRequirements{})
	}
}

func TestSyncLegacyHookPath(t *testing.T) {
	for _, path := range []string{"", "relative/hook", "/tmp/a/../hook"} {
		run := HookRunSummary{Entries: []HookOutputEntry{}, EventName: "sessionStart", ExecutionMode: "sync", HandlerType: "command", Scope: "thread", SourcePath: path, Status: "completed"}
		raw, err := json.Marshal(run)
		if err != nil {
			t.Fatal(err)
		}
		var decoded HookRunSummary
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("schema-valid path %q rejected: %v", path, err)
		}
		if decoded.SourcePath != path {
			t.Fatalf("path changed: %q", decoded.SourcePath)
		}
	}
}

func TestSyncRemovedReviewDelivery(t *testing.T) {
	if _, err := json.Marshal(ReviewDeliveryDetached); err == nil {
		t.Fatal("removed detached delivery still encodes")
	}
}

// Generate each variant's required shape from the checked-in schema, then
// independently assert that its common lifecycle fields survive both union forms.
func TestSyncEveryItemLifecycle(t *testing.T) {
	data, err := os.ReadFile("schema/json/v2/ThreadReadResponse.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	defs := schema["definitions"].(map[string]any)
	variants := defs["ThreadItem"].(map[string]any)["oneOf"].([]any)
	for _, variant := range variants {
		shape := syncSchemaValue(variant.(map[string]any), defs).(map[string]any)
		name := shape["type"].(string)
		t.Run(name, func(t *testing.T) {
			shape["startedAtMs"] = int64(0)
			shape["completedAtMs"] = int64(-1)
			raw, err := json.Marshal(shape)
			if err != nil {
				t.Fatal(err)
			}
			var item ThreadItemWrapper
			if err := json.Unmarshal(raw, &item); err != nil {
				t.Fatalf("valid fixture %s: %v", raw, err)
			}

			for _, value := range []ThreadItem{item.Value, reflect.ValueOf(item.Value).Elem().Interface().(ThreadItem)} {
				out, err := json.Marshal(ThreadItemWrapper{Value: value})
				if err != nil {
					t.Fatal(err)
				}
				var got map[string]json.RawMessage
				if err := json.Unmarshal(out, &got); err != nil {
					t.Fatal(err)
				}
				if string(got["startedAtMs"]) != "0" || string(got["completedAtMs"]) != "-1" {
					t.Fatalf("lifecycle lost: %s", out)
				}
			}
			if _, unknown := item.Value.(*UnknownThreadItem); unknown {
				return
			}
			original := item.Value
			shape["completedAtMs"] = "invalid"
			invalid, err := json.Marshal(shape)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(invalid, &item); err == nil || item.Value != original {
				t.Fatalf("invalid lifecycle replaced receiver: %v", err)
			}
		})
	}
}

func syncSchemaValue(node any, defs map[string]any) any {
	s, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	if ref, ok := s["$ref"].(string); ok {
		return syncSchemaValue(defs[strings.TrimPrefix(ref, "#/definitions/")].(map[string]any), defs)
	}
	if enum, ok := s["enum"].([]any); ok {
		return enum[0]
	}
	for _, union := range []string{"oneOf", "anyOf", "allOf"} {
		if choices, ok := s[union].([]any); ok {
			value := syncSchemaValue(choices[0].(map[string]any), defs)
			if obj, ok := value.(map[string]any); ok {
				for key, v := range syncRequiredProperties(s, defs) {
					obj[key] = v
				}
			}
			return value
		}
	}
	kind := s["type"]
	if choices, ok := kind.([]any); ok {
		kind = choices[0]
	}
	switch kind {
	case "string":
		return "/tmp/value"
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "array":
		return []any{}
	case "object":
		return syncRequiredProperties(s, defs)
	default:
		return nil
	}
}
func syncRequiredProperties(s map[string]any, defs map[string]any) map[string]any {
	out := map[string]any{}
	required, _ := s["required"].([]any)
	props, _ := s["properties"].(map[string]any)
	for _, key := range required {
		name := key.(string)
		out[name] = syncSchemaValue(props[name], defs)
	}
	return out
}

func TestSyncNewRequiredOwnersRejectNullAliases(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		dest any
	}{
		{`{"max_bytes":1,"MAX_BYTES":null}`, &OtelToolResultLogConfig{MaxBytes: 9}},
		{`{"type":"gcp","TYPE":null}`, &GcpAccount{Type: "gcp"}},
		{`{"savedPath":"p","SAVEDPATH":null,"threadId":"t"}`, &FeedbackSaveResponse{SavedPath: "old", ThreadID: "old"}},
		{`{"savedPath":"p","threadId":"t","THREADID":null}`, &FeedbackSaveResponse{SavedPath: "old", ThreadID: "old"}},
	} {
		before := reflect.ValueOf(tc.dest).Elem().Interface()
		err := json.Unmarshal([]byte(tc.raw), tc.dest)
		if err == nil || !reflect.DeepEqual(reflect.ValueOf(tc.dest).Elem().Interface(), before) {
			t.Errorf("invalid alias published for %T: %s, %v", tc.dest, tc.raw, err)
		}
	}
}
