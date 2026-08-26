package protocol

import (
	"encoding/json"
	"testing"
)

func TestSpecSyncReviewUnionVariants(t *testing.T) {
	clientID := "client-1"
	var item ThreadItemWrapper
	if err := json.Unmarshal([]byte(`{"type":"userMessage","id":"u1","content":[],"clientId":"client-1"}`), &item); err != nil {
		t.Fatal(err)
	}
	if got := item.Value.(*UserMessageThreadItem).ClientID; got == nil || *got != clientID {
		t.Fatalf("clientId = %v", got)
	}

	for _, tc := range []struct {
		payload string
		check   func(ThreadItem) bool
	}{
		{`{"type":"subAgentActivity","agentPath":"root/worker","agentThreadId":"t2","id":"i1","kind":"started"}`, func(v ThreadItem) bool { _, ok := v.(*SubAgentActivityThreadItem); return ok }},
		{`{"type":"sleep","durationMs":25,"id":"i2"}`, func(v ThreadItem) bool { _, ok := v.(*SleepThreadItem); return ok }},
	} {
		if err := json.Unmarshal([]byte(tc.payload), &item); err != nil {
			t.Fatal(err)
		}
		if !tc.check(item.Value) {
			t.Fatalf("decoded %T", item.Value)
		}
	}

	for _, payload := range []string{
		`{"type":"audio","url":"https://example.com/a.wav"}`,
		`{"type":"localAudio","path":"audio/a.wav"}`,
	} {
		input, err := UnmarshalUserInput([]byte(payload))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := json.Marshal(input); err != nil {
			t.Fatal(err)
		}
	}

	var output DynamicToolCallOutputContentItemWrapper
	if err := json.Unmarshal([]byte(`{"type":"inputAudio","audioUrl":"https://example.com/a.wav"}`), &output); err != nil {
		t.Fatal(err)
	}
	if _, ok := output.Value.(*InputAudioDynamicToolCallOutputContentItem); !ok {
		t.Fatalf("decoded %T", output.Value)
	}
	if err := output.validateForResponse(); err != nil {
		t.Fatal(err)
	}
}

func TestSpecSyncReviewPreservesTurnClientID(t *testing.T) {
	var params TurnStartParams
	if err := json.Unmarshal([]byte(`{"threadId":"t1","input":[],"clientUserMessageId":"c1"}`), &params); err != nil {
		t.Fatal(err)
	}
	if params.ClientUserMessageID == nil || *params.ClientUserMessageID != "c1" {
		t.Fatalf("clientUserMessageId = %v", params.ClientUserMessageID)
	}
}

func TestSpecSyncReviewApprovalDecisions(t *testing.T) {
	denied := ReviewDecisionWrapper{Value: DeniedReviewDecision{Rejection: "not allowed"}}
	b, err := json.Marshal(denied)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"denied":{"rejection":"not allowed"}}` {
		t.Fatalf("denied = %s", b)
	}
	if err := validateReviewDecisionWrapper(ReviewDecisionWrapper{Value: ReviewDecisionApprovedMCPPolicyAmendment}); err != nil {
		t.Fatal(err)
	}
	if _, err := json.Marshal(ReviewDecisionWrapper{Value: "denied"}); err == nil {
		t.Fatal("legacy denied string was accepted")
	}
}

func TestSpecSyncReviewRequiredAndEnumValues(t *testing.T) {
	var params ToolRequestUserInputParams
	if err := json.Unmarshal([]byte(`{"itemId":"i1","threadId":"t1","turnId":"tu1","questions":[]}`), &params); err == nil {
		t.Fatal("missing isBlocking was accepted")
	}
	values := map[string]string{
		"marketplace": string(PluginListMarketplaceKindCreatedByMeRemote),
		"goalBlocked": string(ThreadGoalStatusBlocked),
		"goalUsage":   string(ThreadGoalStatusUsageLimited),
		"hookSource":  string(HookSourceCloudManagedConfig),
	}
	want := map[string]string{
		"marketplace": "created-by-me-remote",
		"goalBlocked": "blocked",
		"goalUsage":   "usageLimited",
		"hookSource":  "cloudManagedConfig",
	}
	for name, value := range values {
		if value != want[name] {
			t.Fatalf("%s = %q; want %q", name, value, want[name])
		}
	}
}

func TestSpecSyncReviewLoginVariants(t *testing.T) {
	brand := LoginAppBrandCodex
	yes := true
	chatgpt := &ChatgptLoginAccountParams{AppBrand: &brand, CodexStreamlinedLogin: &yes, UseHostedLoginSuccessPage: &yes}
	b, err := chatgpt.marshalWire()
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"appBrand", "codexStreamlinedLogin", "useHostedLoginSuccessPage"} {
		if !json.Valid(b) || !containsJSONField(b, field) {
			t.Fatalf("missing %s in %s", field, b)
		}
	}

	bedrock := &AmazonBedrockLoginAccountParams{ApiKey: "secret", Region: "us-east-1"}
	wire, err := bedrock.marshalWire()
	if err != nil {
		t.Fatal(err)
	}
	if !containsJSONField(wire, "apiKey") {
		t.Fatalf("wire = %s", wire)
	}
	redacted, err := json.Marshal(bedrock)
	if err != nil {
		t.Fatal(err)
	}
	if string(redacted) == string(wire) {
		t.Fatal("bedrock API key was not redacted")
	}
	resp, err := UnmarshalLoginAccountResponse([]byte(`{"type":"amazonBedrock"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resp.(*AmazonBedrockLoginAccountResponse); !ok {
		t.Fatalf("response = %T", resp)
	}
}

func TestSpecSyncReviewLegacyPermissionPaths(t *testing.T) {
	got, err := normalizeAdditionalFileSystemPermissionsField("permissions", &AdditionalFileSystemPermissions{Read: []string{"relative/path"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Read[0] != "relative/path" {
		t.Fatalf("read = %v", got.Read)
	}
}

func containsJSONField(data []byte, field string) bool {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return false
	}
	_, ok := object[field]
	return ok
}
