package protocol

import (
	"encoding/json"
	"testing"
)

func TestSyncFea1Fields(t *testing.T) {
	for _, retain := range []string{"false", "true"} {
		assertSyncRoundTrip(t, `{"threadId":"t","input":[],"toolOutput":{"name":"tool","output":"done","retain":`+retain+`}}`, &TurnStartParams{})
	}
	for _, meta := range []string{`null`, `{}`, `[]`, `false`, `0`, `""`, `{"cursor":9007199254740993}`} {
		assertSyncRoundTrip(t, `{"authStatus":"notLoggedIn","name":"mcp","resourceTemplates":[],"resources":[],"tools":{},"toolListMeta":`+meta+`}`, &McpServerStatus{})
	}
	for _, plan := range []string{"sci", "sci_plus"} {
		assertSyncRoundTrip(t, `{"planType":"`+plan+`"}`, &RateLimitSnapshot{})
	}
	for _, event := range []string{"preModelRequest", "postModelRequest"} {
		var got HookEventName
		assertSyncRoundTrip(t, `"`+event+`"`, &got)
	}
}

func TestSyncFea1ImageGeneration(t *testing.T) {
	for _, payload := range []string{
		`{"type":"imageGeneration","id":"i","status":"completed","result":""}`,
		`{"type":"imageGeneration","id":"i","status":"completed","fileId":"file","inputImages":[]}`,
		`{"type":"imageGeneration","id":"i","status":"completed","fileId":"file","result":""}`,
		`{"type":"imageGeneration","id":"i","status":"failed","result":"","failure":{"type":"usageLimitExceeded","limitId":"","resetsAt":0}}`,
		`{"type":"imageGeneration","id":"i","status":"completed","fileId":"file","inputImages":[{"path":"/tmp/input.png","sha256":"digest"}]}`,
	} {
		var item ThreadItemWrapper
		assertSyncRoundTrip(t, payload, &item)
		if _, ok := item.Value.(*UnknownThreadItem); ok {
			t.Fatal("imageGeneration still lacks a typed representation")
		}
	}
	for _, payload := range []string{
		`{"type":"imageGeneration","id":"i","status":"completed"}`,
		`{"type":"imageGeneration","id":"i","status":"completed","result":null}`,
		`{"type":"imageGeneration","id":"i","status":"completed","fileId":"file","inputImages":null}`,
		`{"type":"imageGeneration","id":"i","status":"completed","fileId":"file","inputImages":[null]}`,
		`{"type":"imageGeneration","id":"i","status":"completed","fileId":"file","inputImages":[{"path":"relative","sha256":"digest"}]}`,
		`{"type":"imageGeneration","id":"i","status":"completed","fileId":"file","inputImages":[{"path":"/tmp/input.png"}]}`,
		`{"type":"imageGeneration","id":"i","status":"failed","result":"","failure":7}`,
		`{"type":"imageGeneration","id":"i","status":"failed","result":"","failure":{}}`,
		`{"type":"imageGeneration","id":"i","status":"failed","result":"","failure":{"type":"future"}}`,
		`{"type":"imageGeneration","id":"i","status":"failed","result":"","failure":{"type":"usageLimitExceeded"}}`,
		`{"type":"imageGeneration","id":"i","status":"failed","result":"","failure":{"type":"usageLimitExceeded","limitId":null}}`,
	} {
		var item ThreadItemWrapper
		if err := json.Unmarshal([]byte(payload), &item); err == nil {
			t.Errorf("accepted invalid image generation item: %s", payload)
		}
	}
}

func TestSyncFea1ConstructedUnions(t *testing.T) {
	for _, failure := range []ImageGenerationFailure{UsageLimitExceededImageGenerationFailure{LimitID: "", ResetsAt: Ptr(int64(0))}, &UsageLimitExceededImageGenerationFailure{}} {
		data, err := json.Marshal(ImageGenerationFailureWrapper{Value: failure})
		if err != nil {
			t.Fatal(err)
		}
		var decoded ImageGenerationFailureWrapper
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
	}
	for _, position := range []ThreadUnreadPosition{ThreadStartThreadUnreadPosition{}, &ThreadStartThreadUnreadPosition{}, TurnThreadUnreadPosition{}, &TurnThreadUnreadPosition{}} {
		data, err := json.Marshal(ThreadUnreadPositionWrapper{Value: position})
		if err != nil {
			t.Fatal(err)
		}
		var decoded ThreadUnreadPositionWrapper
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
	}
	for _, position := range []ThreadUnreadPosition{nil, (*ThreadStartThreadUnreadPosition)(nil), (*TurnThreadUnreadPosition)(nil)} {
		if _, err := json.Marshal(ThreadUnreadPositionWrapper{Value: position}); err == nil {
			t.Fatal("accepted missing unread position")
		}
	}
	var images []ImageGenerationInputImage
	if _, err := json.Marshal(&ImageGenerationThreadItem{FileID: Ptr(""), InputImages: &images}); err == nil {
		t.Fatal("constructed inputImages emitted forbidden null")
	}
}

func TestSyncFea1ReadState(t *testing.T) {
	for _, first := range []string{"", `,"firstUnread":null`, `,"firstUnread":{"type":"threadStart"}`, `,"firstUnread":{"type":"turn","turnId":""}`} {
		assertSyncRoundTrip(t, `{"threadId":"t","readState":{"revision":""`+first+`}}`, &ThreadReadStateChangedNotification{})
	}
	for _, payload := range []string{
		`{}`, `{"threadId":"t"}`, `{"threadId":null,"readState":{"revision":"r"}}`,
		`{"threadId":"t","readState":null}`, `{"threadId":"t","readState":{}}`,
		`{"threadId":"t","readState":{"revision":null}}`,
		`{"threadId":"t","readState":{"revision":"r","firstUnread":{"type":"turn"}}}`,
		`{"threadId":"t","readState":{"revision":"r","firstUnread":{"type":"future"}}}`,
	} {
		var n ThreadReadStateChangedNotification
		if err := json.Unmarshal([]byte(payload), &n); err == nil {
			t.Errorf("accepted invalid read-state notification: %s", payload)
		}
	}
}
