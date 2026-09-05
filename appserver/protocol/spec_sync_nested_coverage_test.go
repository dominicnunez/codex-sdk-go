package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSyncNestedFieldCoverage(t *testing.T) {
	for _, group := range []struct {
		path  string
		types []interface{}
	}{
		{"schema/json/v2/ConfigReadResponse.json", []interface{}{BrowserUseConfig{}, BrowserUseOriginPolicyConfig{}, ComputerUseConfig{}, ComputerUseMacosConfig{}, ComputerUseWindowsConfig{}, ComputerUseWindowsExeConfig{}}},
		{"schema/json/v2/ConfigRequirementsReadResponse.json", []interface{}{BrowserUseRequirements{}, BrowserUseOriginPolicy{}, ComputerUseRequirements{}, ComputerUseMacosRequirements{}, ComputerUseWindowsRequirements{}, ComputerUseWindowsExeRequirement{}, InAppBrowserRequirements{}}},
		{"schema/json/v2/ThreadReadResponse.json", []interface{}{Thread{}, Turn{}, TurnError{}, MisalignmentErrorDetails{}, MisalignmentSteer{}, AsyncUserInputQuestion{}}},
		{"schema/json/v2/ThreadItemsListResponse.json", []interface{}{ThreadItemEntry{}}},
		{"schema/json/v2/PluginReconcileResponse.json", []interface{}{PluginReconcileChangedPlugin{}}},
		{"schema/json/v2/McpServerEventStreamNotification.json", []interface{}{McpServerEventNotification{}}},
		{"schema/json/v2/TurnStartParams.json", []interface{}{TurnToolOutput{}}},
		{"schema/json/v2/RawResponseCompletedNotification.json", []interface{}{ResponseUsageMetadata{}}},
		{"schema/json/v2/SkillsListResponse.json", []interface{}{SkillMetadata{}}},
		{"schema/json/v2/ListMcpServerStatusResponse.json", []interface{}{McpServerStatus{}}},
	} {
		data, err := readSpecFile(group.path)
		if err != nil {
			t.Fatal(err)
		}
		var schema schemaTopLevel
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatal(err)
		}
		for _, v := range group.types {
			typ := reflect.TypeOf(v)
			t.Run(typ.Name(), func(t *testing.T) {
				raw, ok := schema.Definitions[typ.Name()]
				if !ok {
					t.Fatalf("missing definition %s", typ.Name())
				}
				var definition schemaTopLevel
				if err := json.Unmarshal(raw, &definition); err != nil {
					t.Fatal(err)
				}
				fields := structJSONFields(typ)
				for name := range definition.Properties {
					if _, ok := fields[name]; !ok {
						t.Errorf("missing schema field %s", name)
					}
				}
			})
		}
	}
}
