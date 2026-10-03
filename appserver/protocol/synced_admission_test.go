package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const admissionGoal = `{"createdAt":0,"objective":"","status":"active","threadId":"","timeUsedSeconds":0,"tokensUsed":0,"updatedAt":0}`

func TestSyncedRecordAdmission(t *testing.T) {
	tests := []struct {
		value any
		valid string
	}{
		{AppsReadResponse{}, `{"apps":[],"missingAppIds":[]}`},
		{AppsInstalledResponse{}, `{"apps":[]}`},
		{GetWorkspaceMessagesResponse{}, `{"featureEnabled":false,"messages":[]}`},
		{ThreadGoalSetResponse{}, `{"goal":` + admissionGoal + `}`},
		{ThreadGoalClearResponse{}, `{"cleared":false}`},
		{ThreadSectionCreateResponse{}, `{"section":{"id":"","name":""}}`},
		{ThreadSectionListResponse{}, `{"data":[]}`},
		{ThreadSectionUpdateResponse{}, `{"section":{"id":"","name":""}}`},
		{ExternalAgentConfigImportHistoryRecordResponse{}, `{"importId":""}`},
		{ExternalAgentConfigImportHistoriesReadResponse{}, `{"connectors":[],"data":[]}`},
		{ConnectorMetadata{}, `{"id":"","name":""}`},
		{AppToolSummary{}, `{"description":"","name":""}`},
		{InstalledApp{}, `{"callable":false,"enabled":false,"id":""}`},
		{WorkspaceMessage{}, `{"messageBody":"","messageId":"","messageType":"unknown"}`},
		{AccountTokenUsageDailyBucket{}, `{"startDate":"","tokens":0}`},
		{ThreadUsage{}, `{"estimatedUsageCreditsMicros":0,"groups":[],"threadId":""}`},
		{ThreadUsageBreakdownGroup{}, `{"estimatedUsageCreditsMicros":0}`},
		{ThreadGoal{}, admissionGoal},
		{ExternalAgentConfigImportItemTypeFailure{}, `{"failureStage":"","itemType":"CONFIG","message":""}`},
		{ExternalAgentConfigImportItemTypeSuccess{}, `{"itemType":"CONFIG"}`},
		{ExternalAgentConfigImportTypeResult{}, `{"failures":[],"itemType":"CONFIG","successes":[]}`},
		{ExternalAgentConfigImportHistory{}, `{"completedAtMs":0,"failures":[],"importId":"","successes":[]}`},
		{ExternalAgentImportedConnectorCandidate{}, `{"name":"","sessionCount":0,"source":"remoteMcpServersConfig"}`},
		{McpToolCallAppContext{}, `{"connectorId":""}`},
		{RawResponseCompletedNotification{}, `{"responseId":"","threadId":"","turnId":""}`},
	}
	data, err := readSpecFile("schema/json/codex_app_server_protocol.v2.schemas.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema schemaTopLevel
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	for _, tc := range tests {
		typ := reflect.TypeOf(tc.value)
		t.Run(typ.Name(), func(t *testing.T) {
			var definition struct {
				Required   []string                   `json:"required"`
				Properties map[string]json.RawMessage `json:"properties"`
			}
			if err := json.Unmarshal(schema.Definitions[typ.Name()], &definition); err != nil {
				t.Fatal(err)
			}
			var valid map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tc.valid), &valid); err != nil {
				t.Fatal(err)
			}
			receiver := reflect.New(typ).Interface()
			if err := json.Unmarshal([]byte(tc.valid), receiver); err != nil {
				t.Fatalf("valid zero/empty fixture rejected: %v", err)
			}
			before := reflect.New(typ).Elem()
			before.Set(reflect.ValueOf(receiver).Elem())
			for _, field := range definition.Required {
				t.Run("missing/"+field, func(t *testing.T) {
					payload := make(map[string]json.RawMessage)
					for name, value := range valid {
						if name != field {
							payload[name] = value
						}
					}
					encoded, _ := json.Marshal(payload)
					if err := json.Unmarshal(encoded, receiver); err == nil {
						t.Errorf("missing %s accepted", field)
					}
				})
			}
			for field, raw := range definition.Properties {
				if admissionSchemaNullable(raw, schema.Definitions) {
					continue
				}
				t.Run("null/"+field, func(t *testing.T) {
					payload := make(map[string]json.RawMessage)
					for name, value := range valid {
						payload[name] = value
					}
					payload[field] = json.RawMessage("null")
					encoded, _ := json.Marshal(payload)
					if err := json.Unmarshal(encoded, receiver); err == nil {
						t.Errorf("non-null %s accepted", field)
					}
				})
			}
			if !reflect.DeepEqual(before.Interface(), reflect.ValueOf(receiver).Elem().Interface()) {
				t.Error("rejected record changed reused receiver")
			}
			if err := json.Unmarshal([]byte(tc.valid), receiver); err != nil {
				t.Fatalf("receiver did not recover: %v", err)
			}
		})
	}
}

func admissionSchemaNullable(raw json.RawMessage, definitions map[string]json.RawMessage) bool {
	if strings.TrimSpace(string(raw)) == "true" {
		return true
	}
	var node map[string]json.RawMessage
	if json.Unmarshal(raw, &node) != nil {
		return false
	}
	var ref string
	if json.Unmarshal(node["$ref"], &ref) == nil && strings.HasPrefix(ref, "#/definitions/") {
		return admissionSchemaNullable(definitions[strings.TrimPrefix(ref, "#/definitions/")], definitions)
	}
	var kind string
	if json.Unmarshal(node["type"], &kind) == nil && kind == "null" {
		return true
	}
	var kinds []string
	if json.Unmarshal(node["type"], &kinds) == nil {
		for _, kind := range kinds {
			if kind == "null" {
				return true
			}
		}
	}
	for _, union := range []string{"anyOf", "oneOf"} {
		var alternatives []json.RawMessage
		if json.Unmarshal(node[union], &alternatives) == nil {
			for _, alternative := range alternatives {
				if admissionSchemaNullable(alternative, definitions) {
					return true
				}
			}
		}
	}
	return false
}

func TestSyncedEnumsAgainstSchema(t *testing.T) {
	data, err := readSpecFile("schema/json/codex_app_server_protocol.v2.schemas.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema schemaTopLevel
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"WorkspaceMessageType", WorkspaceMessageType("")},
		{"ThreadGoalStatus", ThreadGoalStatus("")},
		{"ProjectChangeType", ProjectChangeType("")},
		{"ExternalAgentImportedConnectorSource", ExternalAgentImportedConnectorSource("")},
		{"ExternalAgentDetectedConnectorSource", detectedConnectorSource("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var definition struct {
				Enum []string `json:"enum"`
			}
			if err := json.Unmarshal(schema.Definitions[tc.name], &definition); err != nil || len(definition.Enum) == 0 {
				t.Fatalf("enum oracle absent: %v", err)
			}
			receiver := reflect.New(reflect.TypeOf(tc.value)).Interface()
			for _, value := range definition.Enum {
				encoded, _ := json.Marshal(value)
				if err := json.Unmarshal(encoded, receiver); err != nil || reflect.ValueOf(receiver).Elem().String() != value {
					t.Fatalf("schema enum %s rejected: %v", encoded, err)
				}
			}
			before := reflect.ValueOf(receiver).Elem().String()
			for _, payload := range []string{`"unsupported"`, `null`, `1`, `false`, `{}`, `[]`} {
				if err := json.Unmarshal([]byte(payload), receiver); err == nil {
					t.Fatalf("invalid enum admitted: %s", payload)
				}
				if reflect.ValueOf(receiver).Elem().String() != before {
					t.Fatal("rejected enum changed receiver")
				}
			}
		})
	}
}
