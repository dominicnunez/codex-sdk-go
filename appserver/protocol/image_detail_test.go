package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestImageDetailCarriers(t *testing.T) {
	for _, base := range []string{`"type":"image","url":"https://example.invalid/image"`, `"type":"image","fileId":"file"`, `"type":"localImage","path":"relative/image"`} {
		for _, detail := range []string{"auto", "low", "high", "original"} {
			payload := "{" + base + `,"detail":"` + detail + `"}`
			t.Run(base+"/"+detail, func(t *testing.T) {
				check := func(input p.UserInput) {
					t.Helper()
					encoded, err := json.Marshal(input)
					if err != nil {
						t.Fatal(err)
					}
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(encoded, &fields); err != nil {
						t.Fatal(err)
					}
					if string(fields["detail"]) != `"`+detail+`"` {
						t.Fatalf("detail lost: %s", encoded)
					}
				}
				input, err := p.UnmarshalUserInput([]byte(payload))
				if err != nil {
					t.Fatal(err)
				}
				check(input)
				var start p.TurnStartParams
				if err := json.Unmarshal([]byte(`{"threadId":"t","input":[`+payload+`]}`), &start); err != nil {
					t.Fatal(err)
				}
				check(start.Input[0])
				var steer p.TurnSteerParams
				if err := json.Unmarshal([]byte(`{"threadId":"t","expectedTurnId":"u","input":[`+payload+`]}`), &steer); err != nil {
					t.Fatal(err)
				}
				check(steer.Input[0])
				var item p.ThreadItemWrapper
				if err := json.Unmarshal([]byte(`{"type":"userMessage","id":"i","content":[`+payload+`]}`), &item); err != nil {
					t.Fatal(err)
				}
				check(item.Value.(*p.UserMessageThreadItem).Content[0])

				mock := NewMockTransport()
				client := p.NewClient(mock)
				t.Cleanup(func() { _ = client.Close() })
				mock.SetResponse("turn/start", p.Response{Result: json.RawMessage(`{"turn":{"id":"u","status":"inProgress","items":[]}}`)})
				mock.SetResponse("turn/steer", p.Response{Result: json.RawMessage(`{"turnId":"u"}`)})
				if _, err := client.Turn.Start(context.Background(), start); err != nil {
					t.Fatal(err)
				}
				if _, err := client.Turn.Steer(context.Background(), steer); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 2; i++ {
					var sent struct {
						Input []json.RawMessage `json:"input"`
					}
					if err := json.Unmarshal(mock.GetSentRequest(i).Params, &sent); err != nil {
						t.Fatal(err)
					}
					decoded, err := p.UnmarshalUserInput(sent.Input[0])
					if err != nil {
						t.Fatal(err)
					}
					check(decoded)
				}
				thread := validThreadPayload("t")
				thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []any{map[string]any{"type": "userMessage", "id": "i", "content": []json.RawMessage{json.RawMessage(payload)}}}}}
				if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
					t.Fatal(err)
				}
				read, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
				if err != nil {
					t.Fatal(err)
				}
				check(read.Thread.Turns[0].Items[0].Value.(*p.UserMessageThreadItem).Content[0])
				callbacks := 0
				client.OnItemStarted(func(n p.ItemStartedNotification) {
					callbacks++
					check(n.Item.Value.(*p.UserMessageThreadItem).Content[0])
				})
				mock.InjectServerNotification(context.Background(), p.Notification{Method: "item/started", Params: json.RawMessage(`{"threadId":"t","turnId":"u","startedAtMs":0,"item":{"type":"userMessage","id":"i","content":[` + payload + `]}}`)})
				if callbacks != 1 {
					t.Fatal("typed image-detail notification lost")
				}
			})
		}
	}
}

func TestImageDetailConstructionAndValidation(t *testing.T) {
	for _, detail := range []p.ImageDetail{p.ImageDetailAuto, p.ImageDetailLow, p.ImageDetailHigh, p.ImageDetailOriginal} {
		for _, input := range []p.UserInput{&p.ImageUserInput{URL: "https://example.invalid/image", Detail: &detail}, &p.ImageUserInput{FileID: strPtr(""), Detail: &detail}, &p.LocalImageUserInput{Path: "", Detail: &detail}} {
			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := p.UnmarshalUserInput(encoded)
			if err != nil {
				t.Fatal(err)
			}
			encodedAgain, err := json.Marshal(decoded)
			if err != nil || string(encodedAgain) != string(encoded) {
				t.Fatalf("constructed detail roundtrip: %s -> %s: %v", encoded, encodedAgain, err)
			}
		}
		var sibling p.FunctionCallOutputContentItemWrapper
		payload := `{"type":"input_image","image_url":"https://example.invalid/image","detail":"` + string(detail) + `"}`
		if err := json.Unmarshal([]byte(payload), &sibling); err != nil {
			t.Fatal(err)
		}
		got := sibling.Value.(*p.InputImageFunctionCallOutputContentItem)
		if got.Detail == nil || *got.Detail != detail {
			t.Fatal("function-output sibling lost detail")
		}
	}
	for _, base := range []string{`"type":"image","fileId":"file"`, `"type":"localImage","path":"relative"`} {
		for _, suffix := range []string{"", `,"detail":null`} {
			input, err := p.UnmarshalUserInput([]byte("{" + base + suffix + "}"))
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if _, present := fields["detail"]; present {
				t.Fatalf("nil detail unexpectedly emitted: %s", encoded)
			}
		}
		for _, bad := range []string{`"bad"`, `""`, `3`, `false`, `[]`, `{}`} {
			if _, err := p.UnmarshalUserInput([]byte("{" + base + `,"detail":` + bad + "}")); err == nil {
				t.Fatalf("bad detail admitted: %s", bad)
			}
		}
		if _, err := p.UnmarshalUserInput([]byte("{" + base + `,"detail":"bad","detail":"low"}`)); err == nil {
			t.Fatal("invalid enum duplicate repaired")
		}
	}
	bad := p.ImageDetail("bad")
	for _, input := range []p.UserInput{&p.ImageUserInput{FileID: strPtr("file"), Detail: &bad}, &p.LocalImageUserInput{Path: "relative", Detail: &bad}} {
		if _, err := json.Marshal(input); err == nil {
			t.Fatal("constructed invalid detail serialized")
		}
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		if _, err := client.Turn.Start(context.Background(), p.TurnStartParams{ThreadID: "t", Input: []p.UserInput{input}}); err == nil {
			t.Fatal("invalid start sent")
		}
		if _, err := client.Turn.Steer(context.Background(), p.TurnSteerParams{ThreadID: "t", ExpectedTurnID: "u", Input: []p.UserInput{input}}); err == nil {
			t.Fatal("invalid steer sent")
		}
		if len(mock.SentRequests) != 0 {
			t.Fatal("invalid detail reached transport")
		}
	}
}

func TestImageDetailReceiverAndCacheOwnership(t *testing.T) {
	var image p.ImageUserInput
	if err := json.Unmarshal([]byte(`{"fileId":"file","detail":"low"}`), &image); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"fileId":"file"}`), &image); err != nil || image.Detail != nil {
		t.Fatalf("image stale optional detail: %+v %v", image, err)
	}
	var local p.LocalImageUserInput
	if err := json.Unmarshal([]byte(`{"path":"relative","detail":"original"}`), &local); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"path":"next"}`), &local); err != nil || local.Detail == nil || *local.Detail != p.ImageDetailOriginal {
		t.Fatal("local plain receiver merge changed")
	}
	if err := json.Unmarshal([]byte(`{"path":"next","detail":null}`), &local); err != nil || local.Detail != nil {
		t.Fatal("local explicit null failed to clear")
	}
	for _, input := range []p.UserInput{&p.ImageUserInput{FileID: strPtr("file"), Detail: detailPtr(p.ImageDetailLow)}, &p.LocalImageUserInput{Path: "relative", Detail: detailPtr(p.ImageDetailLow)}} {
		client := p.NewClient(NewMockTransport())
		t.Cleanup(func() { _ = client.Close() })
		client.CacheThreadState(p.Thread{ID: "t", Turns: []p.Turn{{ID: "u", Items: []p.ThreadItemWrapper{{Value: &p.UserMessageThreadItem{ID: "i", Content: []p.UserInput{input}}}}}}})
		*imageDetail(input) = p.ImageDetailHigh
		snapshot, ok := client.ThreadStateSnapshot("t")
		if !ok {
			t.Fatal("cache missing")
		}
		copied := snapshot.Turns[0].Items[0].Value.(*p.UserMessageThreadItem).Content[0]
		if *imageDetail(copied) != p.ImageDetailLow {
			t.Fatal("source mutation reached cache")
		}
		*imageDetail(copied) = p.ImageDetailOriginal
		again, _ := client.ThreadStateSnapshot("t")
		if *imageDetail(again.Turns[0].Items[0].Value.(*p.UserMessageThreadItem).Content[0]) != p.ImageDetailLow {
			t.Fatal("snapshot mutation reached cache")
		}
	}
}

func detailPtr(value p.ImageDetail) *p.ImageDetail { return &value }
func imageDetail(input p.UserInput) *p.ImageDetail {
	switch v := input.(type) {
	case *p.ImageUserInput:
		return v.Detail
	case *p.LocalImageUserInput:
		return v.Detail
	default:
		panic("not image input")
	}
}
