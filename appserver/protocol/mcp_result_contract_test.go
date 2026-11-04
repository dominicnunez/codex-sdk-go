package protocol_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestMcpResultHistoryAndOwnership(t *testing.T) {
	for _, tc := range []struct {
		raw                 string
		content, structured any
	}{
		{`{"content":[null,true,9007199254740993]}`, []any{nil, true, json.Number("9007199254740993")}, nil},
		{`{"content":[1],"content":[]}`, []any{}, nil},
		{`{"content":[],"CONTENT":[18446744073709551615]}`, []any{json.Number("18446744073709551615")}, nil},
		{`{"content":[],"structuredContent":{"old":1},"structuredContent":{"new":1e400}}`, []any{}, map[string]any{"new": json.Number("1e400")}},
		{`{"content":[],"structuredContent":{"old":1},"structuredContent":null}`, []any{}, nil},
		{`{"content":[],"structuredContent":null,"structuredContent":[]}`, []any{}, []any{}},
		{`{"content":[],"structuredCont\u0065nt":-0}`, []any{}, json.Number("-0")},
	} {
		var result p.McpToolCallResult
		if err := json.Unmarshal([]byte(tc.raw), &result); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result.Content, tc.content) || !reflect.DeepEqual(result.StructuredContent, tc.structured) {
			t.Fatalf("result history changed: %s: %#v", tc.raw, result)
		}
	}
	content := []any{map[string]any{"n": json.Number("9007199254740993")}}
	structured := map[string]any{"prior": json.Number("1e400")}
	meta := json.RawMessage(`{"n":9007199254740993}`)
	result := p.McpToolCallResult{Content: content, StructuredContent: structured, Meta: meta}
	before := result
	for _, raw := range []string{`{}`, `{"content":null}`, `{"content":null,"content":[]}`, `{"content":42}`, `{"content":{}}`, `{"content":"bad"}`, `{"content":true}`} {
		if err := result.UnmarshalJSON([]byte(raw)); err == nil {
			t.Fatalf("invalid content admitted: %s", raw)
		}
		if !reflect.DeepEqual(result, before) || &result.Content[0] != &content[0] || reflect.ValueOf(result.StructuredContent).Pointer() != reflect.ValueOf(structured).Pointer() || &result.Meta[0] != &meta[0] {
			t.Fatal("failed result admission replaced prior references")
		}
	}
	raw := []byte(`{"content":[9007199254740993],"_meta": {"n": 1e400,"b": [null]}}`)
	if err := result.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	wantMeta := []byte(`{"n": 1e400,"b": [null]}`)
	if !bytes.Equal(result.Meta, wantMeta) {
		t.Fatalf("raw meta rewritten: %s", result.Meta)
	}
	for i := range raw {
		raw[i] = ' '
	}
	if result.Content[0] != json.Number("9007199254740993") || !bytes.Equal(result.Meta, wantMeta) {
		t.Fatal("result retains mutable input storage")
	}
	if err := result.UnmarshalJSON([]byte(`{"content":[]}`)); err != nil || result.Content == nil || len(result.Content) != 0 || result.StructuredContent != nil || result.Meta != nil {
		t.Fatalf("successful reuse retained optional data: %#v %v", result, err)
	}
}

func FuzzMcpResultNumbers(f *testing.F) {
	for mode := uint8(0); mode < 4; mode++ {
		f.Add(int64(9007199254740993), int16(400), mode)
	}
	f.Fuzz(func(t *testing.T, n int64, exponent int16, mode uint8) {
		integer := json.Number(fmt.Sprint(n))
		exp := json.Number(fmt.Sprintf("1e%d", exponent))
		content := []any{integer, map[string]any{"n": exp}, nil, true}
		var structured any
		switch mode % 4 {
		case 0:
			structured = integer
		case 1:
			structured = map[string]any{"ids": []any{integer, exp}}
		case 2:
			structured = []any{}
		case 3:
			structured = nil
		}
		input := map[string]any{"content": content, "structuredContent": structured, "_meta": map[string]any{"n": integer}}
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		var got p.McpToolCallResult
		if err = json.Unmarshal(raw, &got); err != nil || !reflect.DeepEqual(got.Content, content) || !reflect.DeepEqual(got.StructuredContent, structured) {
			t.Fatalf("exact result changed: %v %#v", err, got)
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var round p.McpToolCallResult
		if err = json.Unmarshal(encoded, &round); err != nil || !reflect.DeepEqual(round, got) {
			t.Fatalf("roundtrip changed exact result: %v", err)
		}
	})
}

func TestMcpResultPublicRejectionAndRecovery(t *testing.T) {
	for _, result := range []string{`{}`, `{"content":null}`, `{"content":17}`, `{"content":null,"content":[]}`} {
		mock := NewMockTransport()
		reports, calls := 0, 0
		client := p.NewClient(mock, p.WithHandlerErrorCallback(func(origin string, err error) {
			if (origin != "item/started" && origin != "item/completed") || err == nil {
				t.Errorf("bad diagnostic origin: %s %v", origin, err)
			}
			reports++
		}))
		t.Cleanup(func() { _ = client.Close() })
		check := func(p.ThreadItemWrapper) { calls++ }
		client.OnItemStarted(func(n p.ItemStartedNotification) { check(n.Item) })
		removeStart := client.AddItemStartedListener(func(n p.ItemStartedNotification) { check(n.Item) })
		defer removeStart()
		client.OnItemCompleted(func(n p.ItemCompletedNotification) { check(n.Item) })
		removeEnd := client.AddItemCompletedListener(func(n p.ItemCompletedNotification) { check(n.Item) })
		defer removeEnd()
		publish := func(result string) (p.ThreadReadResponse, error) {
			item := json.RawMessage(`{"type":"mcpToolCall","id":"i","server":"s","tool":"fixture","status":"completed","arguments":null,"result":` + result + `}`)
			thread := validProcessThreadPayload("t")
			thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []any{item}}}
			if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
				t.Fatal(err)
			}
			got, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
			for _, method := range []string{"item/started", "item/completed"} {
				mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"threadId":"t","turnId":"u","startedAtMs":0,"completedAtMs":0,"item":` + string(item) + `}`)})
			}
			return got, err
		}
		got, err := publish(result)
		if err == nil || !reflect.DeepEqual(got, p.ThreadReadResponse{}) || calls != 0 || reports != 4 {
			t.Fatalf("bad result published: %s %v calls=%d reports=%d", result, err, calls, reports)
		}
		if _, err = publish(`{"content":[null,9007199254740993]}`); err != nil || calls != 4 || reports != 4 {
			t.Fatalf("same-client recovery failed: %v", err)
		}
	}
}

func TestMcpResultCachedOwnership(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	raw := json.RawMessage(`{"type":"mcpToolCall","id":"i","server":"s","tool":"fixture","status":"completed","arguments":null,"result":{"content":[{"n":9007199254740993}],"structuredContent":{"ids":[1e400]},"_meta":{"n":9007199254740993}}}`)
	thread := validProcessThreadPayload("t")
	thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []any{raw}}}
	if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
		t.Fatal(err)
	}
	response, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	client.CacheThreadState(response.Thread)
	result := func(thread p.Thread) *p.McpToolCallResult {
		return thread.Turns[0].Items[0].Value.(*p.McpToolCallThreadItem).Result
	}
	original := result(response.Thread)
	original.Content[0].(map[string]any)["n"] = json.Number("2")
	original.StructuredContent.(map[string]any)["ids"].([]any)[0] = json.Number("3")
	original.Meta[0] = ' '
	first, ok := client.ThreadStateSnapshot("t")
	if !ok {
		t.Fatal("missing cached snapshot")
	}
	snapshot := result(first)
	if snapshot.Content[0].(map[string]any)["n"] != json.Number("9007199254740993") || snapshot.StructuredContent.(map[string]any)["ids"].([]any)[0] != json.Number("1e400") || snapshot.Meta[0] != '{' {
		t.Fatal("source mutation reached cached result")
	}
	snapshot.Content[0].(map[string]any)["n"] = json.Number("4")
	snapshot.Meta[0] = ' '
	second, ok := client.ThreadStateSnapshot("t")
	if !ok || result(second).Content[0].(map[string]any)["n"] != json.Number("9007199254740993") || result(second).Meta[0] != '{' {
		t.Fatal("snapshot mutation reached cache")
	}
}
