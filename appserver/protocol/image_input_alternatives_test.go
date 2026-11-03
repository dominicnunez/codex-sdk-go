package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestImageInputAlternatives(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"url", `{"type":"image","url":"https://example.invalid/image"}`, true},
		{"file", `{"type":"image","fileId":"file-1"}`, true},
		{"empty-file", `{"type":"image","fileId":""}`, true},
		{"both-strings", `{"type":"image","url":"https://example.invalid/image","fileId":"file-1"}`, true},
		{"url-with-number-file", `{"type":"image","url":"https://example.invalid/image","fileId":17}`, true},
		{"url-with-object-file", `{"type":"image","url":"https://example.invalid/image","fileId":{}}`, true},
		{"file-with-bool-url", `{"type":"image","url":true,"fileId":"file-1"}`, true},
		{"file-with-object-url", `{"type":"image","url":{},"fileId":"file-1"}`, true},
		{"empty-url", `{"type":"image","url":""}`, true},
		{"no-branch", `{"type":"image"}`, false},
		{"both-invalid", `{"type":"image","url":17,"fileId":false}`, false},
		{"both-null", `{"type":"image","url":null,"fileId":null}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, err := p.UnmarshalUserInput([]byte(tc.body))
			if tc.valid {
				if err != nil {
					t.Errorf("schema-valid image rejected directly: %v", err)
				} else {
					checkImageAlternatives(t, input, tc.body, false)
					encoded, e := json.Marshal(input)
					if e != nil {
						t.Errorf("valid image cannot be marshaled: %v", e)
					} else {
						var members map[string]json.RawMessage
						if e = json.Unmarshal(encoded, &members); e != nil {
							t.Fatal(e)
						}
						if tc.name == "empty-url" && string(members["url"]) != `""` {
							t.Error("empty URL presence erased")
						}
					}
				}
			} else if err == nil {
				t.Error("image without valid alternative accepted")
			}
			thread := validProcessThreadPayload("t")
			thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []any{map[string]any{"type": "userMessage", "id": "i", "content": []any{json.RawMessage(tc.body)}}}}}
			mock := NewMockTransport()
			reports := 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(origin string, err error) {
				if (origin != "item/started" && origin != "item/completed") || err == nil {
					t.Errorf("bad image error origin: %s %v", origin, err)
				}
				reports++
			}))
			t.Cleanup(func() { _ = client.Close() })
			if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
				t.Fatal(err)
			}
			got, readErr := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t", IncludeTurns: p.Ptr(true)})
			if tc.valid {
				if readErr != nil {
					t.Errorf("schema-valid image rejected by Thread.Read: %v", readErr)
				} else if len(got.Thread.Turns) != 1 || len(got.Thread.Turns[0].Items) != 1 {
					t.Error("image carrier disappeared")
				}
			} else if readErr == nil {
				t.Error("invalid image published by Thread.Read")
			}

			calls := 0
			notifications := client
			check := func(item p.ThreadItemWrapper) {
				calls++
				checkImageAlternatives(t, item.Value.(*p.UserMessageThreadItem).Content[0], tc.body, false)
			}
			notifications.OnItemStarted(func(n p.ItemStartedNotification) { check(n.Item) })
			removeStart := notifications.AddItemStartedListener(func(n p.ItemStartedNotification) { check(n.Item) })
			defer removeStart()
			notifications.OnItemCompleted(func(n p.ItemCompletedNotification) { check(n.Item) })
			removeEnd := notifications.AddItemCompletedListener(func(n p.ItemCompletedNotification) { check(n.Item) })
			defer removeEnd()
			for _, method := range []string{"item/started", "item/completed"} {
				mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"threadId":"t","turnId":"u","startedAtMs":0,"completedAtMs":0,"item":{"type":"userMessage","id":"i","content":[` + tc.body + `]}}`)})
			}
			if tc.valid {
				if calls != 4 || reports != 0 {
					t.Fatalf("valid user image callbacks=%d errors=%d", calls, reports)
				}
			} else if calls != 0 || reports != 4 {
				t.Fatalf("invalid user image callbacks=%d errors=%d", calls, reports)
			}
		})
	}
}
func TestUserInputTypedNilAdmission(t *testing.T) {
	inputs := []p.UserInput{nil, (*p.TextUserInput)(nil), (*p.ImageUserInput)(nil), (*p.LocalImageUserInput)(nil), (*p.AudioUserInput)(nil), (*p.LocalAudioUserInput)(nil), (*p.SkillUserInput)(nil), (*p.MentionUserInput)(nil), &p.TextUserInput{Text: "control"}}
	for i, input := range inputs {
		for _, steer := range []bool{false, true} {
			mock := NewMockTransport()
			mock.SetResponse("turn/start", p.Response{Result: json.RawMessage(`{"turn":{"id":"u","status":"inProgress","items":[]}}`)})
			mock.SetResponse("turn/steer", p.Response{Result: json.RawMessage(`{"turnId":"u"}`)})
			client := p.NewClient(mock)
			var err error
			if steer {
				_, err = client.Turn.Steer(context.Background(), p.TurnSteerParams{ThreadID: "t", ExpectedTurnID: "u", Input: []p.UserInput{input}})
			} else {
				_, err = client.Turn.Start(context.Background(), p.TurnStartParams{ThreadID: "t", Input: []p.UserInput{input}})
			}
			if i == len(inputs)-1 {
				if err != nil || mock.GetSentRequest(0) == nil {
					t.Errorf("nonnull input steer=%v rejected: %v", steer, err)
				}
			} else if err == nil || mock.GetSentRequest(0) != nil {
				t.Errorf("nil input %d steer=%v dispatched", i, steer)
			}
			_ = client.Close()
		}
	}
}

// The oracle interprets single-occurrence schema properties independently.
func checkImageAlternatives(t *testing.T, value any, raw string, tool bool) {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatal(err)
	}
	urlName, fileName := "url", "fileId"
	if tool {
		urlName, fileName = "image_url", "file_id"
	}
	var wantURL, wantFile *string
	if text, ok := fields[urlName].(string); ok {
		wantURL = &text
	}
	if text, ok := fields[fileName].(string); ok {
		wantFile = &text
	}
	var url, file *string
	switch v := value.(type) {
	case *p.ImageUserInput:
		url, file = v.URL, v.FileID
	case *p.InputImageFunctionCallOutputContentItem:
		url, file = v.ImageURL, v.FileID
	default:
		t.Fatalf("unexpected image %T", value)
	}
	if !reflect.DeepEqual(url, wantURL) || !reflect.DeepEqual(file, wantFile) {
		t.Fatalf("alternative lost for %s: %v %v", raw, url, file)
	}
}

func TestImageAlternativeMatrix(t *testing.T) {
	values := []string{`""`, `"opaque"`, `null`, `0`, `false`, `[]`, `{}`}
	for _, tool := range []bool{false, true} {
		for _, url := range values {
			for _, file := range values {
				name, urlName, fileName := "image", "url", "fileId"
				if tool {
					name, urlName, fileName = "input_image", "image_url", "file_id"
				}
				raw := fmt.Sprintf(`{"type":%q,%q:%s,%q:%s}`, name, urlName, url, fileName, file)
				valid := url[0] == '"' || file[0] == '"'
				var value any
				var err error
				if tool {
					v := new(p.InputImageFunctionCallOutputContentItem)
					err = v.UnmarshalJSON([]byte(raw))
					value = v
				} else {
					v := new(p.ImageUserInput)
					err = v.UnmarshalJSON([]byte(raw))
					value = v
				}
				if !valid {
					if err == nil {
						t.Fatalf("invalid alternatives accepted: %s", raw)
					}
					continue
				}
				if err != nil {
					t.Fatalf("valid alternatives rejected: %s: %v", raw, err)
				}
				checkImageAlternatives(t, value, raw, tool)
				encoded, e := json.Marshal(value)
				if e != nil {
					t.Fatal(e)
				}
				var round any
				if tool {
					v := new(p.InputImageFunctionCallOutputContentItem)
					e = json.Unmarshal(encoded, v)
					round = v
				} else {
					v := new(p.ImageUserInput)
					e = json.Unmarshal(encoded, v)
					round = v
				}
				if e != nil || !reflect.DeepEqual(value, round) {
					t.Fatalf("roundtrip lost valid alternative: %s %v", encoded, e)
				}
			}
		}
	}
}

func TestImagePresenceRequests(t *testing.T) {
	for _, url := range []string{"", "opaque/../image"} {
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		mock.SetResponse("turn/start", p.Response{Result: json.RawMessage(`{"turn":{"id":"u","status":"inProgress","items":[]}}`)})
		mock.SetResponse("turn/steer", p.Response{Result: json.RawMessage(`{"turnId":"u"}`)})
		input := &p.ImageUserInput{URL: p.Ptr(url)}
		if _, err := client.Turn.Start(context.Background(), p.TurnStartParams{ThreadID: "t", Input: []p.UserInput{input}, ToolOutput: &p.TurnToolOutput{Name: "tool", Output: p.FunctionCallOutputBody{Content: []p.FunctionCallOutputContentItemWrapper{{Value: &p.InputImageFunctionCallOutputContentItem{ImageURL: p.Ptr(url)}}}}}}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.Turn.Steer(context.Background(), p.TurnSteerParams{ThreadID: "t", ExpectedTurnID: "u", Input: []p.UserInput{input}}); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			var sent struct {
				Input      []json.RawMessage `json:"input"`
				ToolOutput *struct {
					Output []json.RawMessage `json:"output"`
				} `json:"toolOutput"`
			}
			if err := json.Unmarshal(mock.GetSentRequest(i).Params, &sent); err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(sent.Input[0], &fields); err != nil {
				t.Fatal(err)
			}
			if got, ok := fields["url"].(string); !ok || got != url {
				t.Fatal("outgoing URL presence lost")
			}
			if i == 0 {
				if sent.ToolOutput == nil || len(sent.ToolOutput.Output) != 1 {
					t.Fatal("tool output lost")
				}
				if err := json.Unmarshal(sent.ToolOutput.Output[0], &fields); err != nil {
					t.Fatal(err)
				}
				if got, ok := fields["image_url"].(string); !ok || got != url {
					t.Fatal("tool-output URL presence lost")
				}
			}
		}
	}
}
