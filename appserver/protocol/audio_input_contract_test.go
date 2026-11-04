package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestAudioInputAlternatives(t *testing.T) {
	for _, tc := range []struct {
		name, body, field, value string
		valid                    bool
	}{
		{"url", `{"type":"audio","url":"opaque:audio"}`, "url", `"opaque:audio"`, true},
		{"file", `{"type":"audio","fileId":"f"}`, "fileId", `"f"`, true},
		{"empty URL", `{"type":"audio","url":""}`, "url", `""`, true},
		{"empty file", `{"type":"audio","fileId":""}`, "fileId", `""`, true},
		{"both", `{"type":"audio","url":"u","fileId":"f"}`, "fileId", `"f"`, true},
		{"inactive file", `{"type":"audio","url":"u","fileId":{}}`, "url", `"u"`, true},
		{"inactive URL", `{"type":"audio","url":false,"fileId":"f"}`, "fileId", `"f"`, true},
		{"missing", `{"type":"audio"}`, "", "", false},
		{"null", `{"type":"audio","url":null,"fileId":null}`, "", "", false},
		{"invalid", `{"type":"audio","url":3,"fileId":[]}`, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, err := p.UnmarshalUserInput([]byte(tc.body))
			if (err == nil) != tc.valid {
				t.Fatalf("schema admission: %v", err)
			}
			if !tc.valid {
				return
			}
			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields[tc.field]) != tc.value {
				t.Fatalf("lost alternative: %s", body)
			}
			if _, err := p.UnmarshalUserInput(body); err != nil {
				t.Fatalf("round trip: %v", err)
			}
		})
	}
}

func TestAudioInputPublicRoutes(t *testing.T) {
	for _, body := range []string{`{"type":"audio","fileId":"f"}`, `{"type":"audio","url":[],"fileId":"f"}`, `{"type":"audio","fileId":null}`} {
		valid := body != `{"type":"audio","fileId":null}`
		mock := NewMockTransport()
		errors, calls := 0, 0
		client := p.NewClient(mock, p.WithHandlerErrorCallback(func(_ string, _ error) { errors++ }))
		t.Cleanup(func() { _ = client.Close() })
		item := `{"type":"userMessage","id":"i","content":[` + body + `]}`
		thread := validProcessThreadPayload("t")
		thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []any{json.RawMessage(item)}}}
		if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
			t.Fatal(err)
		}
		_, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t", IncludeTurns: p.Ptr(true)})
		if (err == nil) != valid {
			t.Fatalf("history %s: %v", body, err)
		}
		check := func(item p.ThreadItemWrapper) {
			calls++
			data, err := json.Marshal(item.Value.(*p.UserMessageThreadItem).Content[0])
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if string(got["fileId"]) != `"f"` {
				t.Fatalf("callback lost audio: %s", data)
			}
		}
		client.OnItemStarted(func(n p.ItemStartedNotification) { check(n.Item) })
		client.AddItemStartedListener(func(n p.ItemStartedNotification) { check(n.Item) })
		client.OnItemCompleted(func(n p.ItemCompletedNotification) { check(n.Item) })
		client.AddItemCompletedListener(func(n p.ItemCompletedNotification) { check(n.Item) })
		for _, method := range []string{"item/started", "item/completed"} {
			mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"threadId":"t","turnId":"u","startedAtMs":0,"completedAtMs":0,"item":` + item + `}`)})
		}
		if valid && (calls != 4 || errors != 0) || !valid && (calls != 0 || errors != 4) {
			t.Fatalf("callbacks=%d errors=%d", calls, errors)
		}
		if !valid {
			continue
		}
		input, err := p.UnmarshalUserInput([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		mock.SetResponse("turn/start", p.Response{Result: json.RawMessage(`{"turn":{"id":"u","status":"inProgress","items":[]}}`)})
		mock.SetResponse("turn/steer", p.Response{Result: json.RawMessage(`{"turnId":"u"}`)})
		if _, err := client.Turn.Start(context.Background(), p.TurnStartParams{ThreadID: "t", Input: []p.UserInput{input}}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.Turn.Steer(context.Background(), p.TurnSteerParams{ThreadID: "t", ExpectedTurnID: "u", Input: []p.UserInput{input}}); err != nil {
			t.Fatal(err)
		}
		for _, n := range []int{1, 2} {
			var sent struct {
				Input []map[string]json.RawMessage `json:"input"`
			}
			if err := json.Unmarshal(mock.GetSentRequest(n).Params, &sent); err != nil {
				t.Fatal(err)
			}
			if len(sent.Input) != 1 || string(sent.Input[0]["fileId"]) != `"f"` {
				t.Fatalf("request lost file ID: %+v", sent)
			}
		}
	}
}

func TestConstructedAudioInputAlternatives(t *testing.T) {
	for _, input := range []*p.AudioUserInput{
		{URL: p.Ptr("")}, {FileID: p.Ptr("")}, {URL: p.Ptr("u"), FileID: p.Ptr("f")}, {},
	} {
		data, err := json.Marshal(input)
		valid := input.URL != nil || input.FileID != nil
		if (err == nil) != valid {
			t.Fatalf("constructed branch validity: %v", err)
		}
		if !valid {
			continue
		}
		got, err := p.UnmarshalUserInput(data)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(got)
		if err != nil || string(encoded) != string(data) {
			t.Fatalf("constructed branch changed: %s -> %s (%v)", data, encoded, err)
		}
	}
}

func FuzzAudioInputAlternatives(f *testing.F) {
	for i := range 7 {
		for j := range 7 {
			f.Add(uint8(i), uint8(j), "opaque:audio")
		}
	}
	f.Fuzz(func(t *testing.T, a, b uint8, text string) {
		choices := []any{nil, 0, true, []any{}, map[string]any{}, "", text}
		url, file := choices[int(a)%len(choices)], choices[int(b)%len(choices)]
		body, err := json.Marshal(map[string]any{"type": "audio", "url": url, "fileId": file})
		if err != nil {
			t.Fatal(err)
		}
		_, urlValid := url.(string)
		_, fileValid := file.(string)
		got, err := p.UnmarshalUserInput(body)
		if (err == nil) != (urlValid || fileValid) {
			t.Fatalf("anyOf admission for %s: %v", body, err)
		}
		if err != nil {
			return
		}
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"url", "fileId"} {
			var source map[string]json.RawMessage
			if err := json.Unmarshal(body, &source); err != nil {
				t.Fatal(err)
			}
			var value string
			if json.Unmarshal(source[field], &value) == nil && string(source[field]) != "null" && string(fields[field]) != string(source[field]) {
				t.Fatalf("valid string alternative changed: %s -> %s", body, data)
			}
		}
	})
}

func TestFileBackedRawResponseAudio(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	mock.SetResponse("thread/inject_items", p.Response{Result: json.RawMessage(`{}`)})
	item := json.RawMessage(`{"type":"message","role":"user","content":[{"type":"input_audio","file_id":""}]}`)
	if _, err := client.Thread.InjectItems(context.Background(), p.ThreadInjectItemsParams{ThreadID: "t", Items: []json.RawMessage{item}}); err != nil {
		t.Fatal(err)
	}
	var sent struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(mock.GetSentRequest(0).Params, &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent.Items) != 1 || string(sent.Items[0]) != string(item) {
		t.Fatalf("raw audio changed: %s", mock.GetSentRequest(0).Params)
	}
}

func TestAudioRecordDecodeComposition(t *testing.T) {
	var envelope struct {
		p.AudioUserInput
		Extra string `json:"extra"`
	}
	if err := json.Unmarshal([]byte(`{"url":"u","extra":"kept"}`), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Extra != "kept" {
		t.Fatal("audio decoder consumed application envelope")
	}
	type reference struct {
		URL    *string `json:"url,omitempty"`
		FileID *string `json:"fileId,omitempty"`
	}
	for _, body := range []string{`{}`, `{"fileId":"f"}`, `{"url":3,"fileId":"updated"}`, `{"url":null}`} {
		actual := p.AudioUserInput{URL: p.Ptr("prior")}
		want := reference{URL: p.Ptr("prior")}
		gotErr := json.Unmarshal([]byte(body), &actual)
		wantErr := json.Unmarshal([]byte(body), &want)
		if (gotErr == nil) != (wantErr == nil) {
			t.Errorf("direct record behavior differs for %s: %v vs %v", body, gotErr, wantErr)
		}
		gotJSON, e := json.Marshal(struct {
			URL    *string
			FileID *string
		}{actual.URL, actual.FileID})
		if e != nil {
			t.Fatal(e)
		}
		wantJSON, e := json.Marshal(struct {
			URL    *string
			FileID *string
		}{want.URL, want.FileID})
		if e != nil {
			t.Fatal(e)
		}
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("native receiver state changed for %s: %s vs %s", body, gotJSON, wantJSON)
		}
	}
}
