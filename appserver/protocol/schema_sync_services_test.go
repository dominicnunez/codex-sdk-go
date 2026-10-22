package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestReadStateNotificationLifecycle(t *testing.T) {
	transport := NewMockTransport()
	errors := 0
	client := codex.NewClient(transport, codex.WithHandlerErrorCallback(func(method string, _ error) {
		if method != codex.NotifyThreadReadStateChanged {
			t.Errorf("unexpected error method: %s", method)
		}
		errors++
	}))
	t.Cleanup(func() { _ = client.Close() })
	primary, listener := 0, 0
	client.OnThreadReadStateChanged(func(n codex.ThreadReadStateChangedNotification) {
		primary++
		if n.ThreadID != "t" || n.ReadState.Revision != "r" {
			t.Errorf("notification = %+v", n)
		}
	})
	remove := client.AddThreadReadStateChangedListener(func(n codex.ThreadReadStateChangedNotification) {
		listener++
		if n.ThreadID != "t" {
			t.Errorf("listener owner = %q", n.ThreadID)
		}
	})
	inject := func(body string) {
		transport.InjectServerNotification(context.Background(), codex.Notification{Method: codex.NotifyThreadReadStateChanged, Params: json.RawMessage(body)})
	}
	valid := `{"threadId":"t","ThreadID":"other","readState":{"revision":"r","firstUnread":{"type":"turn","turnId":"u"}}}`
	inject(valid)
	if primary != 1 || listener != 1 {
		t.Fatalf("callbacks = %d, %d", primary, listener)
	}
	inject(`{"threadId":"t","readState":{"revision":"r","firstUnread":{"type":"turn"}}}`)
	if primary != 1 || listener != 1 || errors != 2 {
		t.Fatalf("invalid callback admission: primary=%d listener=%d errors=%d", primary, listener, errors)
	}
	remove()
	inject(valid)
	client.OnThreadReadStateChanged(nil)
	inject(valid)
	if primary != 2 || listener != 1 {
		t.Fatalf("removed callbacks = %d, %d", primary, listener)
	}
}

func TestSyncedFieldsThroughServices(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	transport.SetResponse("turn/start", codex.Response{Result: json.RawMessage(`{"turn":{"id":"u","items":[],"status":"inProgress"}}`)})
	for _, retain := range []bool{false, true} {
		_, err := client.Turn.Start(ctx, codex.TurnStartParams{ThreadID: "t", Input: []codex.UserInput{}, ToolOutput: &codex.TurnToolOutput{
			Name: "tool", Output: codex.FunctionCallOutputBody{Text: codex.Ptr("done")}, Retain: &retain,
		}})
		if err != nil {
			t.Fatal(err)
		}
		var sent struct {
			ToolOutput struct {
				Retain *bool `json:"retain"`
			} `json:"toolOutput"`
		}
		if err := json.Unmarshal(transport.GetSentRequest(transport.CallCount()-1).Params, &sent); err != nil {
			t.Fatal(err)
		}
		if sent.ToolOutput.Retain == nil || *sent.ToolOutput.Retain != retain {
			t.Fatalf("retain lost on request: %+v", sent)
		}
	}
	for _, meta := range []string{`null`, `[]`, `{}`, `false`, `0`, `""`, `9007199254740993`} {
		transport.SetResponse("mcpServerStatus/list", codex.Response{Result: json.RawMessage(`{"data":[{"authStatus":"notLoggedIn","name":"mcp","resourceTemplates":[],"resources":[],"tools":{},"toolListMeta":` + meta + `}]}`)})
		response, err := client.Mcp.ListServerStatus(ctx, codex.ListMcpServerStatusParams{})
		if err != nil || len(response.Data) != 1 || string(response.Data[0].ToolListMeta) != meta {
			t.Fatalf("metadata=%s: response=%+v err=%v", meta, response, err)
		}
	}
	for _, plan := range []string{"sci", "sci_plus"} {
		transport.SetResponse("account/read", codex.Response{Result: json.RawMessage(`{"account":{"type":"chatgpt","email":"test@example.com","planType":"` + plan + `"},"requiresOpenaiAuth":true}`)})
		if _, err := client.Account.Get(ctx, codex.GetAccountParams{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGeneratedImageFailureAdmission(t *testing.T) {
	transport := NewMockTransport()
	errors := 0
	client := codex.NewClient(transport, codex.WithHandlerErrorCallback(func(_ string, _ error) { errors++ }))
	t.Cleanup(func() { _ = client.Close() })
	called := 0
	client.OnItemCompleted(func(n codex.ItemCompletedNotification) {
		called++
		image, ok := n.Item.Value.(*codex.ImageGenerationThreadItem)
		if !ok || image.Failure == nil {
			t.Fatalf("image = %#v", n.Item.Value)
		}
		failure, ok := image.Failure.Value.(codex.UsageLimitExceededImageGenerationFailure)
		if !ok || failure.LimitID != "limit" {
			t.Fatalf("failure = %#v", image.Failure.Value)
		}
	})
	for index, failure := range []string{`{"type":"usageLimitExceeded","limitId":"limit","resetsAt":null}`, `7`, `{}`, `{"type":"usageLimitExceeded"}`} {
		item := `{"type":"imageGeneration","id":"i","status":"failed","result":"","failure":` + failure + `}`
		transport.InjectServerNotification(context.Background(), codex.Notification{Method: "item/completed", Params: json.RawMessage(`{"threadId":"t","turnId":"u","completedAtMs":0,"item":` + item + `}`)})
		transport.SetResponse("thread/items/list", codex.Response{Result: json.RawMessage(`{"data":[{"turnId":"u","item":` + item + `}]}`)})
		_, err := client.Thread.ItemsList(context.Background(), codex.ThreadItemsListParams{ThreadID: "t"})
		if (err == nil) != (index == 0) {
			t.Fatalf("history failure=%s err=%v", failure, err)
		}
	}
	if called != 1 || errors != 3 {
		t.Fatalf("callback admission: calls=%d errors=%d", called, errors)
	}
}

func TestSyncedRawResponseFieldsSurviveInjection(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	transport.SetResponse("thread/inject_items", codex.Response{Result: json.RawMessage(`{}`)})
	item := json.RawMessage(`{"type":"message","role":"assistant","status":"completed","encrypted_content":"opaque","content":[{"type":"output_text","text":"done","annotations":[],"logprobs":[9007199254740993]}]}`)
	if _, err := client.Thread.InjectItems(context.Background(), codex.ThreadInjectItemsParams{ThreadID: "t", Items: []json.RawMessage{item}}); err != nil {
		t.Fatal(err)
	}
	var sent struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(transport.GetSentRequest(0).Params, &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent.Items) != 1 || string(sent.Items[0]) != string(item) {
		t.Fatalf("raw fields changed: %s", transport.GetSentRequest(0).Params)
	}
}
