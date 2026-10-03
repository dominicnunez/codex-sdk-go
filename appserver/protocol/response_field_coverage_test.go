package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Cover the containing definitions, not just the originally reported properties:
// a type-name inventory cannot detect nested response data being discarded.
func TestResponseSchemaFieldCoverage(t *testing.T) {
	data, err := readSpecFile("schema/json/codex_app_server_protocol.v2.schemas.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema schemaTopLevel
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{
		ExternalAgentConfigImportResponse{}, McpToolCallResult{}, AppSummary{}, Config{},
		ExternalAgentConfigMigrationItem{}, FuzzyFileSearchResult{}, HookMetadata{},
		Model{}, ModelUpgradeInfo{}, PluginDetail{}, PluginInterface{}, PluginShareContext{},
		PluginSummary{}, SkillInterface{}, TokenUsageBreakdown{},
		ModelServiceTier{}, MigrationDetails{}, CommandMigration{}, HookMigration{},
		McpServerMigration{}, PluginsMigration{}, SessionMigration{}, SkillMigration{}, SubagentMigration{},
		AppTemplateSummary{}, ScheduledTaskSummary{},
	} {
		typ := reflect.TypeOf(value)
		t.Run(typ.Name(), func(t *testing.T) {
			raw, ok := schema.Definitions[typ.Name()]
			if !ok {
				t.Fatalf("missing schema definition %s", typ.Name())
			}
			var definition schemaTopLevel
			if err := json.Unmarshal(raw, &definition); err != nil {
				t.Fatal(err)
			}
			fields := structJSONFields(typ)
			for name := range definition.Properties {
				if _, ok := fields[name]; !ok {
					t.Errorf("missing schema property %s", name)
				}
			}
			for _, name := range definition.Required {
				if field, ok := fields[name]; ok && field.isOptional {
					t.Errorf("required schema property %s has omitempty", name)
				}
			}
		})
	}
}

func TestResponseMetadataEnumCoverage(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"AutoCompactTokenLimitScope", AutoCompactTokenLimitScope("")},
		{"MultiAgentVersion", MultiAgentVersion("")},
		{"FuzzyFileSearchMatchType", FuzzyFileSearchMatchType("")},
		{"PluginDisabledReason", PluginDisabledReason("")},
		{"PluginInstallPolicySource", PluginInstallPolicySource("")},
		{"AppTemplateUnavailableReason", AppTemplateUnavailableReason("")},
		{"ScheduledTaskWeekday", ScheduledTaskWeekday("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values, err := schemaEnumValues("schema/json/codex_app_server_protocol.v2.schemas.json", tc.name)
			if err != nil || len(values) == 0 {
				t.Fatalf("schema enum: values=%v err=%v", values, err)
			}
			for _, literal := range values {
				data, err := json.Marshal(literal)
				if err != nil {
					t.Fatal(err)
				}
				value := reflect.New(reflect.TypeOf(tc.value))
				if err := json.Unmarshal(data, value.Interface()); err != nil {
					t.Fatalf("%s: %v", literal, err)
				}
				roundTrip, err := json.Marshal(value.Elem().Interface())
				if err != nil || string(roundTrip) != string(data) {
					t.Fatalf("%s: round trip %s, err=%v", literal, roundTrip, err)
				}
			}
			value := reflect.New(reflect.TypeOf(tc.value))
			if err := json.Unmarshal([]byte(`"not-a-schema-value"`), value.Interface()); err == nil {
				t.Fatal("unknown enum accepted")
			}
		})
	}
}

func TestResponseMetadataUnionFields(t *testing.T) {
	data, err := readSpecFile("schema/json/codex_app_server_protocol.v2.schemas.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema schemaTopLevel
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		definition    string
		discriminator string
		variants      map[string]reflect.Type
	}{
		{"HookMetadata", "handlerType", map[string]reflect.Type{
			"command": reflect.TypeOf(HookMetadata{}), "mcpTool": reflect.TypeOf(HookMetadata{}),
			"prompt": reflect.TypeOf(HookMetadata{}), "agent": reflect.TypeOf(HookMetadata{}),
		}},
		{"ScheduledTaskSchedule", "type", map[string]reflect.Type{
			"hourly": reflect.TypeOf(HourlyScheduledTaskSchedule{}), "daily": reflect.TypeOf(DailyScheduledTaskSchedule{}),
			"weekdays": reflect.TypeOf(WeekdaysScheduledTaskSchedule{}), "weekly": reflect.TypeOf(WeeklyScheduledTaskSchedule{}),
		}},
	} {
		t.Run(tc.definition, func(t *testing.T) {
			var definition schemaTopLevel
			if err := json.Unmarshal(schema.Definitions[tc.definition], &definition); err != nil {
				t.Fatal(err)
			}
			if len(definition.OneOf) != len(tc.variants) {
				t.Fatalf("schema branches=%d, mapped branches=%d", len(definition.OneOf), len(tc.variants))
			}
			for _, raw := range definition.OneOf {
				var branch schemaTopLevel
				if err := json.Unmarshal(raw, &branch); err != nil {
					t.Fatal(err)
				}
				var tag struct {
					Enum []string `json:"enum"`
				}
				if err := json.Unmarshal(branch.Properties[tc.discriminator], &tag); err != nil || len(tag.Enum) != 1 {
					t.Fatalf("invalid schema discriminator: %s", raw)
				}
				typ, ok := tc.variants[tag.Enum[0]]
				if !ok {
					t.Fatalf("unmapped branch %s", tag.Enum[0])
				}
				fields := structJSONFields(typ)
				for property := range branch.Properties {
					// Schedule marshalers supply their concrete discriminator;
					// public wire tests verify its value and direct decoding.
					if property == "type" && tc.definition == "ScheduledTaskSchedule" {
						continue
					}
					if _, ok := fields[property]; !ok {
						t.Errorf("%s missing branch property %s", typ.Name(), property)
					}
				}
			}
		})
	}
}
