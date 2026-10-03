package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestSyncedNestedWrapperErrorBoundary(t *testing.T) {
	type wrapper struct {
		App   InstalledApp `json:"app"`
		After string       `json:"after"`
	}
	value := wrapper{App: InstalledApp{ID: "prior", Callable: true}, After: "prior"}
	err := json.Unmarshal([]byte(`{"app":{"id":3,"callable":false,"enabled":false},"after":"later"}`), &value)
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) || typeErr.Type != reflect.TypeFor[string]() || typeErr.Value != "number" || typeErr.Offset != 1 {
		t.Fatalf("nested field-local diagnostic: %T %v", err, err)
	}
	if value.App.ID != "prior" || !value.App.Callable || value.After != "prior" {
		t.Fatalf("failed nested admission published partial value: %+v", value)
	}
	if err := json.Unmarshal([]byte(`{"app":{"id":"","callable":false,"enabled":false},"after":"later"}`), &value); err != nil || value.After != "later" {
		t.Fatalf("wrapper recovery: %+v %v", value, err)
	}
}

func TestSyncedAdmissionVariants(t *testing.T) {
	for _, tc := range []struct {
		value   any
		invalid []string
		valid   []string
	}{
		{AppToolSummary{},
			[]string{`{"name":"","description":"","isEnabled":"false"}`, `{"name":"","description":"","isEnabled":null,"isEnabled":true}`, `{"name":"","description":"","isReadOnly":1}`},
			[]string{`{"name":"","description":"","isEnabled":true,"isEnabled":false,"isReadOnly":true,"isReadOnly":false}`}},
		{AppsReadResponse{},
			[]string{`{"apps":[null],"missingAppIds":[]}`, `{"apps":[],"missingAppIds":[null]}`, `{"apps":[],"missingAppIds":{},"missingAppIds":[]}`, `{"apps":[],"MissingAppIds":[]}`},
			[]string{`{"apps":[],"missingAppIds":[],"unknown":{"apps":null}}`, `{"apps":[],"missingApp\u0049ds":[]}`}},
		{ConnectorMetadata{},
			[]string{`{"id":"","name":"","pluginDisplayNames":[null]}`, `{"id":"","name":"","toolSummaries":[null]}`, `{"id":"","name":"","toolSummaries":[{}]}`},
			[]string{`{"id":"","name":"","toolSummaries":null,"description":null}`, `{"id":"","name":"","pluginDisplayNames":[],"toolSummaries":[{"name":"","description":"","isEnabled":false,"isReadOnly":false}]}`}},
		{WorkspaceMessage{},
			[]string{`{"messageBody":"","messageId":"","messageType":"other"}`, `{"messageBody":"","messageId":"","messageType":null}`, `{"messageBody":"","messageId":"","messageType":"bad","messageType":"unknown"}`, `{"messageBody":"","messageId":"","messageType":"unknown","createdAt":1.5}`},
			[]string{`{"messageBody":"","messageId":"","messageType":"headline","createdAt":-1,"archivedAt":null}`, `{"messageBody":"","messageId":"","messageType":"announcement"}`}},
		{ThreadGoal{},
			[]string{`{"createdAt":0,"objective":"","status":"bad","threadId":"","timeUsedSeconds":0,"tokensUsed":0,"updatedAt":0}`, `{"createdAt":0,"objective":"","status":"active","threadId":"","timeUsedSeconds":9223372036854775808,"tokensUsed":0,"updatedAt":0}`},
			[]string{`{"createdAt":-1,"objective":"","status":"paused","threadId":"","timeUsedSeconds":-1,"tokensUsed":-1,"updatedAt":-1,"tokenBudget":null}`}},
		{ExternalAgentConfigImportTypeResult{},
			[]string{`{"failures":[null],"successes":[],"itemType":"CONFIG"}`, `{"failures":[],"successes":[{}],"itemType":"CONFIG"}`, `{"failures":[],"successes":[],"itemType":"bad"}`},
			[]string{`{"failures":[{"failureStage":"","message":"","itemType":"CONFIG","cwd":null}],"successes":[{"itemType":"CONFIG","title":""}],"itemType":"CONFIG"}`}},
		{ExternalAgentConfigImportHistory{},
			[]string{`{"completedAtMs":"0","failures":[],"successes":[],"importId":""}`, `{"completedAtMs":0,"failures":[],"successes":[null],"importId":""}`},
			[]string{`{"completedAtMs":-1,"failures":[],"successes":[],"importId":"","providerId":null}`}},
		{ExternalAgentImportedConnectorCandidate{},
			[]string{`{"name":"","sessionCount":-1,"source":"remoteMcpServersConfig"}`, `{"name":"","sessionCount":4294967296,"source":"remoteMcpServersConfig"}`, `{"name":"","sessionCount":0,"source":"bad"}`},
			[]string{`{"name":"","sessionCount":4294967295,"source":"remoteMcpServersConfig"}`}},
		{McpToolCallAppContext{},
			[]string{`{"ConnectorID":""}`, `{"connectorId":null,"connectorId":""}`},
			[]string{`{"connectorId":"","appName":null,"actionName":""}`}},
		{RawResponseCompletedNotification{},
			[]string{`{"responseId":"","threadId":"","turnId":"","usage":{}}`},
			[]string{`{"responseId":"","threadId":"","turnId":"","usage":null,"usageMetadata":{"metadata":[1]}}`, `{"responseId":"","threadId":"","turnId":"","usageMetadata":null}`}},
		{ModelSafetyBufferingUpdatedNotification{},
			[]string{`{"threadId":"","turnId":"","model":"","reasons":[],"useCases":[null],"showBufferingUi":false}`},
			[]string{`{"threadId":"","turnId":"","model":"","reasons":[],"useCases":[""],"showBufferingUi":false,"fasterModel":null}`}},
	} {
		typ := reflect.TypeOf(tc.value)
		t.Run(typ.Name(), func(t *testing.T) {
			receiver := reflect.New(typ).Interface()
			for _, payload := range tc.valid {
				if err := json.Unmarshal([]byte(payload), receiver); err != nil {
					t.Fatalf("valid %s: %v", payload, err)
				}
			}
			before, err := json.Marshal(receiver)
			if err != nil {
				t.Fatal(err)
			}
			for _, payload := range tc.invalid {
				if err := json.Unmarshal([]byte(payload), receiver); err == nil {
					t.Fatalf("invalid %s admitted", payload)
				}
				after, err := json.Marshal(receiver)
				if err != nil || string(after) != string(before) {
					t.Fatalf("failed decode mutated prior record: %s: %v", after, err)
				}
			}
			if err := json.Unmarshal([]byte(tc.valid[0]), receiver); err != nil {
				t.Fatalf("recovery: %v", err)
			}
		})
	}
}

func TestSyncedSuccessfulDecodeReplacesOptionalFields(t *testing.T) {
	var app ConnectorMetadata
	if err := json.Unmarshal([]byte(`{"id":"old","name":"old","description":"old","pluginDisplayNames":["old"]}`), &app); err != nil {
		t.Fatal(err)
	}
	priorDescription, priorNames := app.Description, app.PluginDisplayNames
	if err := json.Unmarshal([]byte(`{"id":"","name":""}`), &app); err != nil {
		t.Fatal(err)
	}
	if app.Description != nil || app.PluginDisplayNames != nil || app.ID != "" || app.Name != "" {
		t.Fatalf("stale fields: %+v", app)
	}
	if *priorDescription != "old" || priorNames[0] != "old" {
		t.Fatal("replacement mutated retained references")
	}
}
