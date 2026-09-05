package protocol

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func assertSyncRoundTrip(t *testing.T, payload string, target interface{}) {
	t.Helper()
	if err := json.Unmarshal([]byte(payload), target); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(target)
	if err != nil {
		t.Fatal(err)
	}
	var want, got interface{}
	if err := json.Unmarshal([]byte(payload), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip\n got %s\nwant %s", encoded, payload)
	}
}

func TestSyncRealtimeUnionRoundTrips(t *testing.T) {
	for _, payload := range []string{
		`{"id":"i1","realtimeSessionId":"r1","type":"realtimeSessionStarted"}`,
		`{"id":"i1","realtimeSessionId":"r1","type":"transcriptSegment","role":"user","text":""}`,
		`{"id":"i1","realtimeSessionId":"r1","type":"transcriptSegment","role":"assistant","text":"hello"}`,
		`{"id":"i1","realtimeSessionId":"r1","type":"bemItemPromoted","item_id":"i2","turn_id":"tu1","presentation":{"type":"wholeItem"}}`,
		`{"id":"i1","realtimeSessionId":"r1","type":"bemItemPromoted","item_id":"i2","turn_id":"tu1","presentation":{"type":"inlineMarkdown"}}`,
		`{"id":"i1","realtimeSessionId":"r1","type":"bemItemPromoted","item_id":"i2","turn_id":"tu1","presentation":{"type":"inlineVisualization","index":0}}`,
		`{"id":"i1","realtimeSessionId":"r1","type":"realtimeSessionClosed","outcome":"ended"}`,
		`{"id":"i1","realtimeSessionId":"r1","type":"realtimeSessionClosed","outcome":"failed"}`,
		`{"type":"futureItem","id":"i1","realtimeSessionId":"r1","future":42}`,
	} {
		assertSyncRoundTrip(t, payload, &ThreadRealtimeItemWrapper{})
	}
	for _, payload := range []string{
		`{"type":"realtimeSessionStarted","realtimeSessionId":"r1"}`,
		`{"type":"transcriptSegment","id":"i1","realtimeSessionId":"r1","role":"invalid","text":"hello"}`,
		`{"type":"transcriptSegment","id":"i1","realtimeSessionId":"r1","role":"user"}`,
		`{"type":"bemItemPromoted","id":"i1","realtimeSessionId":"r1","item_id":"i2","turn_id":"tu1","presentation":{"type":"inlineVisualization"}}`,
		`{"type":"bemItemPromoted","id":"i1","realtimeSessionId":"r1","item_id":"i2","turn_id":"tu1","presentation":{"type":"inlineVisualization","index":-1}}`,
		`{"type":"realtimeSessionClosed","id":"i1","realtimeSessionId":"r1","outcome":"invalid"}`,
	} {
		var item ThreadRealtimeItemWrapper
		if err := json.Unmarshal([]byte(payload), &item); err == nil {
			t.Fatalf("accepted invalid item %s", payload)
		}
	}
}

func TestSyncToolOutputRoundTrips(t *testing.T) {
	for _, payload := range []string{
		`""`, `"result"`, `[]`,
		`[{"type":"input_text","text":"result"},{"type":"input_image","image_url":"https://example.com/image","detail":"original"},{"type":"input_audio","audio_url":"https://example.com/audio"},{"type":"encrypted_content","encrypted_content":"opaque"}]`,
	} {
		assertSyncRoundTrip(t, payload, &FunctionCallOutputBody{})
	}
	for _, payload := range []string{`null`, `{}`, `true`, `42`, `[null]`, `[{"type":"input_text"}]`} {
		var output FunctionCallOutputBody
		if err := json.Unmarshal([]byte(payload), &output); err == nil {
			t.Fatalf("accepted %s", payload)
		}
	}
	assertSyncRoundTrip(t, `{"type":"functionCallOutput","id":"i1","name":"lookup","namespace":"tools","output":"done"}`, &ThreadItemWrapper{})
	assertSyncRoundTrip(t, `{"threadId":"t1","input":[],"serviceTierForTurn":"priority","turnTrigger":"external","toolOutput":{"name":"lookup","namespace":"tools","output":"done"}}`, &TurnStartParams{})
}

func TestSyncOpenAIFormPreservesArbitrarySchema(t *testing.T) {
	for _, mode := range []string{"openaiForm", "openai/form"} {
		for _, schema := range []string{`true`, `null`, `{"type":"object","properties":{"choice":{"enum":[1,2]}}}`, `{"oneOf":[{"type":"string"},{"type":"number"}]}`} {
			payload := `{"serverName":"mcp","threadId":"t1","message":"Choose","mode":"` + mode + `","requestedSchema":` + schema + `}`
			assertSyncRoundTrip(t, payload, &McpServerElicitationRequestParams{})
		}
		var p McpServerElicitationRequestParams
		if err := json.Unmarshal([]byte(`{"serverName":"mcp","threadId":"t1","message":"Choose","mode":"`+mode+`"}`), &p); err == nil {
			t.Fatal("accepted missing requestedSchema")
		}
	}
}

func TestSyncBedrockCredentialsOnlyOnWire(t *testing.T) {
	p := &AmazonBedrockAccessKeysLoginAccountParams{AccessKeyID: "access-secret", SecretAccessKey: "private-secret", SessionToken: Ptr("session-secret"), Region: "us-east-1"}
	wire, err := marshalForWire(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{p.AccessKeyID, p.SecretAccessKey, *p.SessionToken} {
		if !strings.Contains(string(wire), secret) {
			t.Fatal("missing wire credential")
		}
		encoded, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, redacted := range []string{string(encoded), fmt.Sprint(p), fmt.Sprintf("%#v", p), fmt.Sprintf("%+v", p)} {
			if strings.Contains(redacted, secret) {
				t.Fatal("credential leaked in formatted output")
			}
		}
		valueJSON, err := json.Marshal(*p)
		if err != nil {
			t.Fatal(err)
		}
		for _, redacted := range []string{string(valueJSON), fmt.Sprint(*p), fmt.Sprintf("%#v", *p), fmt.Sprintf("%+v", *p)} {
			if strings.Contains(redacted, secret) {
				t.Fatal("credential leaked from copied parameter value")
			}
		}
	}
	for _, p := range []*AmazonBedrockAccessKeysLoginAccountParams{nil, {}, {AccessKeyID: "key", Region: "region"}, {SecretAccessKey: "secret", Region: "region"}, {AccessKeyID: "key", SecretAccessKey: "secret"}} {
		if _, err := marshalForWire(p); err == nil {
			t.Fatal("invalid credentials accepted")
		}
	}
}

func TestSyncAdditionalFieldsRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		payload string
		target  interface{}
	}{
		{`{"browser_use":{"allow_history_access":false,"origins":{"https://example.com":{"access":"allow","uploads":"deny"}}},"computer_use":{"default_app_access":"deny","windows":{"aumids":{"app":"allow"},"exes":[{"access":"allow","product_name":"Product","publisher_name":"Publisher","binary_name":"app.exe"}]}}}`, &Config{}},
		{`{"additionalDeveloperInstructions":"instructions","allowBrowserAndComputerUse":false,"inAppBrowser":{"allowExternalBrowserSettingsImport":false},"computerUse":{"allowLockedComputerUse":false,"allowPersistentApproval":false,"defaultAppAccess":"deny","macos":{"bundleIds":{"com.example":"allow"}},"windows":{"aumids":{"app":"deny"}}}}`, &ConfigRequirements{}},
		{`{"allowGlobalPersistentApproval":false,"allowHistoryAccess":false,"defaultOriginPolicy":{"access":"deny","accessApprovalLifetime":"thread","fullCdpAccess":"deny","persistentApproval":false}}`, &BrowserUseRequirements{}},
		{`{"message":"retrying","provider":"provider","threadId":"t1","turnId":"tu1"}`, &AuthRecoveryNotification{}},
		{`{"notification":{"method":"update","params":null},"subscriptionId":"s1"}`, &McpServerEventStreamNotification{}},
		{`{"threadId":"t1","itemId":"i1","delta":"text"}`, &ThreadRealtimeItemTranscriptDeltaNotification{}},
		{`{"type":"agentMessage","id":"i1","text":"question","questions":[{"title":"Choose","options":["A","B"]}]}`, &ThreadItemWrapper{}},
		{`{"message":"blocked","misalignment":{"errorType":"policy","detailedExplanation":"details","steer":{"message":"adjust"}}}`, &TurnError{}},
		{`{"amount":"1.25","metadata":{"currency":"credits"}}`, &ResponseUsageMetadata{}},
	} {
		assertSyncRoundTrip(t, tc.payload, tc.target)
	}
}
