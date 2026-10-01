package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
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
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "automatic", true: "explicit"}[explicit], func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			t.Cleanup(func() { _ = client.Close() })
			transport.SetResponse("initialize", codex.Response{Result: json.RawMessage(`{"codexHome":"/tmp","platformFamily":"unix","platformOs":"linux","userAgent":"test"}`)})
			params := codex.InitializeParams{
				ClientInfo:   codex.ClientInfo{Name: "test", Version: "1"},
				Capabilities: &codex.InitializeCapabilities{ExplicitGatewayOAuth: explicit},
			}
			if _, err := client.Initialize(context.Background(), params); err != nil {
				t.Fatal(err)
			}
			var sent codex.InitializeParams
			if err := json.Unmarshal(transport.GetSentRequest(0).Params, &sent); err != nil {
				t.Fatal(err)
			}
			if explicit && (sent.Capabilities == nil || !sent.Capabilities.ExplicitGatewayOAuth) {
				t.Fatalf("explicit login capability lost on wire: %s", transport.GetSentRequest(0).Params)
			}
			latched, ok := client.InitializedParams()
			if !ok || explicit && (latched.Capabilities == nil || !latched.Capabilities.ExplicitGatewayOAuth) {
				t.Fatalf("explicit login capability lost in session: %+v", latched)
			}
			if _, err := client.Initialize(context.Background(), params); err != nil {
				t.Fatalf("same handshake failed: %v", err)
			}
			params.Capabilities.ExplicitGatewayOAuth = !explicit
			_, err := client.Initialize(context.Background(), params)
			var mismatch *codex.InitializeParamsMismatchError
			if !errors.As(err, &mismatch) {
				t.Fatalf("changed login mode error = %v, want handshake mismatch", err)
			}
		})
	}
}

func TestThreadItemsAnchorInjectsDiscriminator(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	transport.SetResponse("thread/items/list", codex.Response{Result: json.RawMessage(`{"data":[]}`)})
	turnID := "turn-1"
	_, err := client.Thread.ItemsList(context.Background(), codex.ThreadItemsListParams{
		ThreadID: "thread-1", TurnID: &turnID, CursorAnchor: &codex.ThreadItemsListAnchor{ItemID: "item-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
		t.Fatal(err)
	}
	if string(params["cursor"]) != `{"type":"item","itemId":"item-1"}` {
		t.Fatalf("cursor = %s", params["cursor"])
	}
	_, err = client.Thread.ItemsList(context.Background(), codex.ThreadItemsListParams{ThreadID: "thread-1", CursorAnchor: &codex.ThreadItemsListAnchor{ItemID: "item-1"}})
	if err == nil {
		t.Fatal("expected anchor without turnId to fail")
	}
}

func TestThreadItemsParamsCursorVariantsRoundTrip(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
	}{
		{name: "string", data: `{"threadId":"thread-1","cursor":"next"}`},
		{name: "anchor", data: `{"threadId":"thread-1","turnId":"turn-1","cursor":{"type":"item","itemId":"item-1"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var params codex.ThreadItemsListParams
			if err := json.Unmarshal([]byte(test.data), &params); err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(params)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.data {
				t.Fatalf("round trip = %s, want %s", got, test.data)
			}
		})
	}
}

func TestThreadItemEntryAllowsMissingStartedAt(t *testing.T) {
	var entry codex.ThreadItemEntry
	if err := json.Unmarshal([]byte(`{"item":{"type":"agentMessage","id":"item-1","text":"ok"},"turnId":"turn-1"}`), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.StartedAtMs != nil {
		t.Fatalf("startedAtMs = %v, want nil", entry.StartedAtMs)
	}
}

func TestTurnStartDisabledPluginIDsRoundTrip(t *testing.T) {
	data := []byte(`{"threadId":"thread-1","input":[],"disabledPluginIds":[]}`)
	var params codex.TurnStartParams
	if err := json.Unmarshal(data, &params); err != nil {
		t.Fatal(err)
	}
	if params.DisabledPluginIDs == nil || len(*params.DisabledPluginIDs) != 0 {
		t.Fatalf("disabledPluginIds = %#v, want non-nil empty slice", params.DisabledPluginIDs)
	}
	got, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if !containsJSONField(t, got, "disabledPluginIds") {
		t.Fatalf("round trip omitted disabledPluginIds: %s", got)
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
