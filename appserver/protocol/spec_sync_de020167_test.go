package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestGatewayOAuthServicesAndNotification(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	transport.SetResponse("account/gatewayOAuth/read", codex.Response{Result: json.RawMessage(`{"providerId":"p","providerName":"Provider","required":true,"status":"started"}`)})
	transport.SetResponse("account/gatewayOAuth/login", codex.Response{Result: json.RawMessage(`{}`)})
	transport.SetResponse("account/gatewayOAuth/cancel", codex.Response{Result: json.RawMessage(`{}`)})

	read, err := client.Account.GatewayOAuthRead(context.Background())
	if err != nil || read.ProviderID != "p" || read.Status == nil || *read.Status != codex.GatewayOAuthStatusStarted {
		t.Fatalf("read = %+v, err = %v", read, err)
	}
	if _, err := client.Account.GatewayOAuthLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Account.GatewayOAuthCancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i, method := range []string{"account/gatewayOAuth/read", "account/gatewayOAuth/login", "account/gatewayOAuth/cancel"} {
		if got := transport.GetSentRequest(i).Method; got != method {
			t.Fatalf("request %d method = %q", i, got)
		}
	}

	var changed codex.GatewayOAuthChangedNotification
	client.OnGatewayOAuthChanged(func(n codex.GatewayOAuthChangedNotification) { changed = n })
	transport.InjectServerNotification(context.Background(), codex.Notification{Method: "account/gatewayOAuth/changed", Params: json.RawMessage(`{"providerId":"p","status":"succeeded"}`)})
	if changed.ProviderID != "p" || changed.Status != codex.GatewayOAuthStatusSucceeded {
		t.Fatalf("notification = %+v", changed)
	}
}

func TestThreadAttachmentServicesPreservePayload(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	transport.SetResponse("thread/attachment/add", codex.Response{Result: json.RawMessage(`{"attachment":{"attachmentType":"note","createdAt":1,"id":"a1","identityKey":"k","payload":null},"outcome":"created"}`)})
	transport.SetResponse("thread/attachment/list", codex.Response{Result: json.RawMessage(`{"data":[{"attachmentType":"note","createdAt":1,"id":"a1","identityKey":"k","payload":{"x":1}}]}`)})
	transport.SetResponse("thread/attachment/remove", codex.Response{Result: json.RawMessage(`{}`)})

	added, err := client.Thread.AttachmentAdd(context.Background(), codex.ThreadAttachmentAddParams{ThreadID: "t1", AttachmentType: "note", IdentityKey: "k", Payload: json.RawMessage(`null`)})
	if err != nil || added.Outcome != codex.ThreadAttachmentAddOutcomeCreated || string(added.Attachment.Payload) != "null" {
		t.Fatalf("added = %+v, err = %v", added, err)
	}
	listed, err := client.Thread.AttachmentList(context.Background(), codex.ThreadAttachmentListParams{ThreadID: "t1"})
	if err != nil || len(listed.Data) != 1 || string(listed.Data[0].Payload) != `{"x":1}` {
		t.Fatalf("listed = %+v, err = %v", listed, err)
	}
	if _, err := client.Thread.AttachmentRemove(context.Background(), codex.ThreadAttachmentRemoveParams{ThreadID: "t1", AttachmentType: "note", IdentityKey: "k"}); err != nil {
		t.Fatal(err)
	}
}

func TestNewImageReferencesRoundTrip(t *testing.T) {
	fileID := "file-1"
	input := &codex.ImageUserInput{FileID: &fileID}
	b, err := json.Marshal(input)
	if err != nil || string(b) != `{"type":"image","fileId":"file-1"}` {
		t.Fatalf("image = %s, err = %v", b, err)
	}

	content := &codex.InputImageFunctionCallOutputContentItem{FileID: &fileID}
	b, err = json.Marshal(content)
	if err != nil || string(b) != `{"type":"input_image","file_id":"file-1"}` {
		t.Fatalf("content = %s, err = %v", b, err)
	}
	var decoded codex.InputImageFunctionCallOutputContentItem
	if err := json.Unmarshal(b, &decoded); err != nil || decoded.FileID == nil || *decoded.FileID != fileID {
		t.Fatalf("decoded = %+v, err = %v", decoded, err)
	}
}

func TestThreadPredictionResultVariants(t *testing.T) {
	var completed codex.ThreadPredictionUpdatedNotification
	if err := json.Unmarshal([]byte(`{"result":{"type":"completed","text":"next"},"sourceTurnId":"s","threadId":"t"}`), &completed); err != nil {
		t.Fatal(err)
	}
	result, ok := completed.Result.Value.(*codex.CompletedThreadPredictionResult)
	if !ok || result.Text == nil || *result.Text != "next" {
		t.Fatalf("result = %#v", completed.Result.Value)
	}
	var failed codex.ThreadPredictionUpdatedNotification
	if err := json.Unmarshal([]byte(`{"result":{"type":"failed"},"sourceTurnId":"s","threadId":"t"}`), &failed); err != nil {
		t.Fatal(err)
	}
	if _, ok := failed.Result.Value.(*codex.FailedThreadPredictionResult); !ok {
		t.Fatalf("result = %#v", failed.Result.Value)
	}
	if err := json.Unmarshal([]byte(`{"result":{"type":"unknown"},"sourceTurnId":"s","threadId":"t"}`), &failed); err == nil {
		t.Fatal("expected unknown prediction type error")
	}
}

func TestInitializeExplicitGatewayOAuthAffectsIdentity(t *testing.T) {
	a := codex.InitializeCapabilities{ExplicitGatewayOAuth: true}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) || !containsJSONField(t, b, "explicitGatewayOauth") {
		t.Fatalf("capabilities = %s", b)
	}
}

func containsJSONField(t *testing.T, data []byte, field string) bool {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	_, ok := object[field]
	return ok
}
