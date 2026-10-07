package protocol

import (
	"encoding/json"
	"testing"
)

func TestSpecSyncOpaqueSkillPaths(t *testing.T) {
	t.Run("skill metadata", func(t *testing.T) {
		var got SkillMetadata
		if err := json.Unmarshal([]byte(`{"description":"skill","enabled":true,"name":"demo","path":"skills/demo","scope":"user"}`), &got); err != nil {
			t.Fatalf("decode opaque skill path: %v", err)
		}
		if got.Path != "skills/demo" {
			t.Fatalf("path = %q, want original opaque value", got.Path)
		}
	})

	t.Run("plugin skill summary", func(t *testing.T) {
		var got SkillSummary
		if err := json.Unmarshal([]byte(`{"description":"skill","enabled":true,"name":"demo","path":"skills/demo"}`), &got); err != nil {
			t.Fatalf("decode opaque plugin skill path: %v", err)
		}
		if got.Path == nil || *got.Path != "skills/demo" {
			t.Fatalf("path = %v, want original opaque value", got.Path)
		}
	})
}

func TestSpecSyncModelProviderCapabilitiesMayOmitNamespaceTools(t *testing.T) {
	var got ModelProviderCapabilitiesReadResponse
	if err := json.Unmarshal([]byte(`{"imageGeneration":false,"webSearch":true}`), &got); err != nil {
		t.Fatalf("decode current capability response without legacy namespaceTools: %v", err)
	}
	if got.ImageGeneration || !got.WebSearch || got.NamespaceTools {
		t.Fatalf("decoded capabilities = %+v", got)
	}
}

func TestSpecSyncNullableExclusionsPreserveWirePresence(t *testing.T) {
	values := []struct {
		name  string
		value ThreadListParams
		want  string
	}{
		{name: "absent", want: `{}`},
		{name: "null", value: ThreadListParams{ExcludedThreadIDs: OptionalNullable[[]string]{Present: true}}, want: `{"excludedThreadIds":null}`},
		{name: "empty", value: ThreadListParams{ExcludedThreadIDs: OptionalNullable[[]string]{Present: true, Value: valuePtr([]string{})}}, want: `{"excludedThreadIds":[]}`},
		{name: "populated", value: ThreadListParams{ExcludedThreadIDs: OptionalNullable[[]string]{Present: true, Value: valuePtr([]string{"a", "b"})}}, want: `{"excludedThreadIds":["a","b"]}`},
	}
	for _, tt := range values {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("JSON = %s, want %s", got, tt.want)
			}
			var decoded ThreadListParams
			if err := json.Unmarshal(got, &decoded); err != nil {
				t.Fatal(err)
			}
			reencoded, err := json.Marshal(decoded)
			if err != nil || string(reencoded) != tt.want {
				t.Fatalf("round trip = %s, %v; want %s", reencoded, err, tt.want)
			}
		})
	}
}

func TestSpecSyncAttachmentOwnerListTypes(t *testing.T) {
	var params ThreadAttachmentOwnerListParams
	if err := json.Unmarshal([]byte(`{"attachmentType":"","identityKey":"","archived":false,"limit":0}`), &params); err != nil {
		t.Fatalf("decode schema-valid empty request values: %v", err)
	}
	if params.AttachmentType != "" || params.IdentityKey != "" || params.Archived == nil || *params.Archived || params.Limit == nil || *params.Limit != 0 {
		t.Fatalf("decoded params = %+v", params)
	}
	var response ThreadAttachmentOwnerListResponse
	if err := json.Unmarshal([]byte(`{"data":[{"threadId":"","archived":false}],"nextCursor":null}`), &response); err != nil {
		t.Fatalf("decode owner response preserving false/empty values: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ThreadID != "" || response.Data[0].Archived {
		t.Fatalf("decoded owners = %+v", response.Data)
	}
	for _, payload := range []string{`{}`, `{"data":[{"threadId":"t"}]}`, `{"data":[{"archived":false}]}`} {
		var invalid ThreadAttachmentOwnerListResponse
		if err := json.Unmarshal([]byte(payload), &invalid); err == nil {
			t.Errorf("accepted response missing required data: %s", payload)
		}
	}
}

func TestSpecSyncTurnLineageAndSubagentMetadata(t *testing.T) {
	assertSyncRoundTrip(t, `{"threadId":"t","input":[],"parentTurnId":"","rootTurnId":"root"}`, &TurnStartParams{})
	var params TurnStartParams
	if err := json.Unmarshal([]byte(`{"threadId":"t","input":[],"parentTurnId":"","rootTurnId":"root"}`), &params); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(params)
	var actual map[string]interface{}
	if err != nil || json.Unmarshal(encoded, &actual) != nil || actual["parentTurnId"] != "" || actual["rootTurnId"] != "root" {
		t.Fatalf("custom turn/start marshal = %s, %v", encoded, err)
	}
	var turn Turn
	if err := json.Unmarshal([]byte(`{"id":"t","status":"inProgress","items":[],"rootTurnId":"root"}`), &turn); err != nil {
		t.Fatal(err)
	}
	if turn.RootTurnID == nil || *turn.RootTurnID != "root" {
		t.Fatalf("rootTurnId = %v", turn.RootTurnID)
	}
	var wrapper ThreadItemWrapper
	if err := json.Unmarshal([]byte(`{"type":"subAgentActivity","agentPath":"a","agentThreadId":"t","id":"i","kind":"started","model":"","reasoningEffort":"high"}`), &wrapper); err != nil {
		t.Fatal(err)
	}
	activity := wrapper.Value.(*SubAgentActivityThreadItem)
	if activity.Model == nil || *activity.Model != "" || activity.ReasoningEffort == nil || *activity.ReasoningEffort != ReasoningEffortHigh {
		t.Fatalf("subagent metadata = %+v", activity)
	}
}

func TestSpecSyncPartialAnswerPhaseAndMisalignmentReviewTarget(t *testing.T) {
	var phase MessagePhase
	if err := json.Unmarshal([]byte(`"partial_answer"`), &phase); err != nil || phase != MessagePhasePartialAnswer {
		t.Fatalf("phase = %q, %v", phase, err)
	}
	var details MisalignmentErrorDetails
	if err := json.Unmarshal([]byte(`{"reviewTarget":"opaque:review"}`), &details); err != nil {
		t.Fatal(err)
	}
	if details.ReviewTarget == nil || *details.ReviewTarget != "opaque:review" {
		t.Fatalf("review target = %v", details.ReviewTarget)
	}
}

func TestSpecSyncConfigRequirementMetadata(t *testing.T) {
	var response ConfigRequirementsReadResponse
	if err := json.Unmarshal([]byte(`{"supportsIndependentSpeedModes":false,"requirements":null}`), &response); err != nil {
		t.Fatal(err)
	}
	if response.SupportsIndependentSpeedModes == nil || *response.SupportsIndependentSpeedModes {
		t.Fatalf("supportsIndependentSpeedModes = %v", response.SupportsIndependentSpeedModes)
	}
	var browser BrowserUseRequirements
	if err := json.Unmarshal([]byte(`{"extension":{"requestHeaders":[]}}`), &browser); err != nil {
		t.Fatal(err)
	}
	if browser.Extension == nil || !browser.Extension.RequestHeaders.Present || browser.Extension.RequestHeaders.Value == nil || len(*browser.Extension.RequestHeaders.Value) != 0 {
		t.Fatalf("browser extension requirements = %+v", browser.Extension)
	}
	for _, payload := range []string{`{"extension":{"requestHeaders":null}}`, `{"extension":{"requestHeaders":[{"name":"","value":""}]}}`} {
		var decoded BrowserUseRequirements
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Fatalf("decode requestHeaders %s: %v", payload, err)
		}
	}
	for _, payload := range []string{`{"extension":{"requestHeaders":[null]}}`, `{"extension":{"requestHeaders":[{"name":"x"}]}}`} {
		var decoded BrowserUseRequirements
		if err := json.Unmarshal([]byte(payload), &decoded); err == nil {
			t.Errorf("accepted malformed request header %s", payload)
		}
	}
	header := []RequestHeader{{Name: "X-Test", Value: ""}}
	for _, tt := range []struct {
		name  string
		value OptionalNullable[[]RequestHeader]
		want  string
	}{
		{name: "absent", want: `{}`},
		{name: "null", value: OptionalNullable[[]RequestHeader]{Present: true}, want: `{"requestHeaders":null}`},
		{name: "empty", value: OptionalNullable[[]RequestHeader]{Present: true, Value: valuePtr([]RequestHeader{})}, want: `{"requestHeaders":[]}`},
		{name: "populated", value: OptionalNullable[[]RequestHeader]{Present: true, Value: &header}, want: `{"requestHeaders":[{"name":"X-Test","value":""}]}`},
	} {
		t.Run("request headers "+tt.name, func(t *testing.T) {
			encoded, err := json.Marshal(BrowserUseExtensionRequirements{RequestHeaders: tt.value})
			if err != nil || string(encoded) != tt.want {
				t.Fatalf("marshal = %s, %v; want %s", encoded, err, tt.want)
			}
		})
	}
}

func valuePtr[T any](value T) *T { return &value }
