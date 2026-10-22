package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	protocol "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestImageGenerationAlternatives(t *testing.T) {
	// Each branch constrains only its own property. The other property may be
	// any JSON value. The expected admissions come from this schema contract.
	values := []struct {
		json string
		text *string
	}{
		{`""`, protocol.Ptr("")}, {`"image"`, protocol.Ptr("image")},
		{`null`, nil}, {`false`, nil}, {`0`, nil}, {`[]`, nil}, {`{}`, nil},
	}
	for _, file := range values {
		for _, result := range values {
			t.Run(fmt.Sprintf("file=%s/result=%s", file.json, result.json), func(t *testing.T) {
				payload := `{"type":"imageGeneration","id":"i","status":"completed","fileId":` + file.json + `,"result":` + result.json + `}`
				var wrapper protocol.ThreadItemWrapper
				err := json.Unmarshal([]byte(payload), &wrapper)
				if file.text == nil && result.text == nil {
					if err == nil {
						t.Fatal("accepted item without a valid alternative")
					}
					return
				}
				if err != nil {
					t.Fatalf("rejected valid alternative: %v", err)
				}
				item, ok := wrapper.Value.(*protocol.ImageGenerationThreadItem)
				if !ok {
					t.Fatalf("decoded %T", wrapper.Value)
				}
				if (item.FileID == nil) != (file.text == nil) || (item.Result == nil) != (result.text == nil) {
					t.Fatalf("alternative fields = %+v", item)
				}
				if file.text != nil && *item.FileID != *file.text || result.text != nil && *item.Result != *result.text {
					t.Fatalf("alternative values = %+v", item)
				}
			})
		}
	}
}

func TestImageGenerationAlternativesThroughCarriers(t *testing.T) {
	transport := NewMockTransport()
	errors := 0
	client := protocol.NewClient(transport, protocol.WithHandlerErrorCallback(func(_ string, _ error) { errors++ }))
	t.Cleanup(func() { _ = client.Close() })
	started, completed := 0, 0
	client.OnItemStarted(func(n protocol.ItemStartedNotification) { started++ })
	client.OnItemCompleted(func(n protocol.ItemCompletedNotification) { completed++ })
	for _, alternatives := range []string{
		`"fileId":"file","result":{}`,
		`"fileId":false,"result":"inline"`,
		`"fileId":"file","result":[]`,
		`"fileId":0,"result":""`,
	} {
		item := `{"type":"imageGeneration","id":"i","status":"completed",` + alternatives + `}`
		transport.InjectServerNotification(context.Background(), protocol.Notification{Method: "item/started", Params: json.RawMessage(`{"threadId":"t","turnId":"u","startedAtMs":0,"item":` + item + `}`)})
		transport.InjectServerNotification(context.Background(), protocol.Notification{Method: "item/completed", Params: json.RawMessage(`{"threadId":"t","turnId":"u","completedAtMs":0,"item":` + item + `}`)})
		transport.SetResponse("thread/items/list", protocol.Response{Result: json.RawMessage(`{"data":[{"turnId":"u","item":` + item + `}]}`)})
		if _, err := client.Thread.ItemsList(context.Background(), protocol.ThreadItemsListParams{ThreadID: "t"}); err != nil {
			t.Errorf("history rejected %s: %v", alternatives, err)
		}
		transport.SetResponse("thread/read", protocol.Response{Result: json.RawMessage(`{"thread":{"id":"t","projectId":null,"sessionId":"s","cliVersion":"1","createdAt":0,"cwd":"/tmp","ephemeral":false,"modelProvider":"p","preview":"","source":"cli","status":{"type":"idle"},"turns":[{"id":"u","status":"completed","items":[` + item + `]}],"updatedAt":0}}`)})
		if _, err := client.Thread.Read(context.Background(), protocol.ThreadReadParams{ThreadID: "t"}); err != nil {
			t.Errorf("thread read rejected %s: %v", alternatives, err)
		}
	}
	if started != 4 || completed != 4 || errors != 0 {
		t.Fatalf("delivery: started=%d completed=%d errors=%d", started, completed, errors)
	}
}

func TestImageGenerationAlternativeHistory(t *testing.T) {
	for _, tc := range []struct {
		fields       string
		file, inline *string
	}{
		{`"fileId":false,"fileId":"repaired","result":"inline"`, nil, protocol.Ptr("inline")},
		{`"fileId":"file","result":[],"result":"repaired"`, protocol.Ptr("file"), nil},
		{`"fileId":"old","fileId":null,"result":"inline"`, nil, protocol.Ptr("inline")},
		{`"fileId":"file","result":"old","result":null`, protocol.Ptr("file"), nil},
	} {
		var wrapper protocol.ThreadItemWrapper
		if err := json.Unmarshal([]byte(`{"type":"imageGeneration","id":"i","status":"completed",`+tc.fields+`}`), &wrapper); err != nil {
			t.Fatalf("%s: %v", tc.fields, err)
		}
		item := wrapper.Value.(*protocol.ImageGenerationThreadItem)
		if (item.FileID == nil) != (tc.file == nil) || (item.Result == nil) != (tc.inline == nil) {
			t.Fatalf("wrong branch admitted for %s: %+v", tc.fields, item)
		}
	}
	for _, fields := range []string{
		`"fileId":"file","result":{},"failure":7`,
		`"fileId":false,"result":"inline","inputImages":null`,
		`"fileId":"file","result":{},"savedPath":"relative"`,
	} {
		var wrapper protocol.ThreadItemWrapper
		if err := json.Unmarshal([]byte(`{"type":"imageGeneration","id":"i","status":"completed",`+fields+`}`), &wrapper); err == nil {
			t.Errorf("selected alternative bypassed common validation: %s", fields)
		}
	}
}
