package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestHookPromptDuplicateArrayHistory(t *testing.T) {
	for _, tc := range []struct {
		members string
		want    []p.HookPromptFragment
	}{
		{`"fragments":[{"hookRunId":"r","text":"first"}],"fragments":[{"text":"last"}]`, []p.HookPromptFragment{{HookRunID: "r", Text: "last"}}},
		{`"fragments":[{"hookRunId":"one","text":"1"},{"hookRunId":"two","text":"2"}],"fragments":[{"text":"new-one"}],"fragments":[{"text":"final-one"},{"text":"final-two"}]`, []p.HookPromptFragment{{HookRunID: "one", Text: "final-one"}, {HookRunID: "two", Text: "final-two"}}},
		{`"fragments":[{"hookRunId":"r","text":"first"}],"fragments":[]`, []p.HookPromptFragment{}},
	} {
		var wrapper p.ThreadItemWrapper
		if err := json.Unmarshal([]byte(`{"type":"hookPrompt","id":"i",`+tc.members+`}`), &wrapper); err != nil {
			t.Fatal(err)
		}
		item, ok := wrapper.Value.(*p.HookPromptThreadItem)
		if !ok || len(item.Fragments) != len(tc.want) {
			t.Fatal("valid native history not retained")
		}
		for i, want := range tc.want {
			if item.Fragments[i] != want {
				t.Fatalf("slot%d=%+v want%+v", i, item.Fragments[i], want)
			}
		}
	}
	for _, members := range []string{
		`"fragments":[{"hookRunId":"r","text":"first"}],"fragments":[],"fragments":[{"text":"last"}]`,
		`"fragments":[{"hookRunId":"r","text":null}],"fragments":[{"hookRunId":"r","text":"last"}]`,
		`"fragments":[null],"fragments":[{"hookRunId":"r","text":"last"}]`,
	} {
		var wrapper p.ThreadItemWrapper
		if err := json.Unmarshal([]byte(`{"type":"hookPrompt","id":"i",`+members+`}`), &wrapper); err == nil {
			t.Fatal("invalid array history admitted")
		}
	}
}

func TestHookPromptEncodingAndOwnedCopies(t *testing.T) {
	item := p.HookPromptThreadItem{ID: "i", StartedAtMs: p.Ptr(int64(0)), CompletedAtMs: p.Ptr(int64(-1)), Fragments: []p.HookPromptFragment{{HookRunID: "r", Text: "prompt"}}}
	for _, value := range []p.ThreadItem{item, &item} {
		encoded, err := json.Marshal(p.ThreadItemWrapper{Value: value})
		if err != nil {
			t.Fatal(err)
		}
		var wrapper p.ThreadItemWrapper
		if err := json.Unmarshal(encoded, &wrapper); err != nil {
			t.Fatal(err)
		}
		got, ok := wrapper.Value.(*p.HookPromptThreadItem)
		if !ok || got.ID != "i" || got.Fragments[0] != item.Fragments[0] || got.StartedAtMs == nil || *got.StartedAtMs != 0 || got.CompletedAtMs == nil || *got.CompletedAtMs != -1 {
			t.Fatalf("value/pointer encoding lost metadata: %s", encoded)
		}
	}
	mock := NewMockTransport()
	client := p.NewClient(mock)
	defer client.Close()
	client.CacheThreadState(p.Thread{ID: "hook-thread", Turns: []p.Turn{{ID: "turn", Items: []p.ThreadItemWrapper{{Value: &item}}}}})
	item.Fragments[0].Text = "mutated"
	*item.StartedAtMs = 42
	snapshot, ok := client.ThreadStateSnapshot("hook-thread")
	if !ok {
		t.Fatal("snapshot missing")
	}
	got := snapshot.Turns[0].Items[0].Value.(*p.HookPromptThreadItem)
	if got.Fragments[0].Text != "prompt" || *got.StartedAtMs != 0 || got.StartedAtMs == item.StartedAtMs || &got.Fragments[0] == &item.Fragments[0] {
		t.Fatal("cache borrowed caller fragment/time")
	}
	got.Fragments[0].Text = "snapshot-mutated"
	*got.CompletedAtMs = 100
	fresh, _ := client.ThreadStateSnapshot("hook-thread")
	second := fresh.Turns[0].Items[0].Value.(*p.HookPromptThreadItem)
	if second.Fragments[0].Text != "prompt" || *second.CompletedAtMs != -1 {
		t.Fatal("snapshot mutation changed cache")
	}

	calls := 0
	client.OnItemCompleted(func(n p.ItemCompletedNotification) {
		calls++
		v := n.Item.Value.(*p.HookPromptThreadItem)
		v.Fragments[0].Text = "callback-mutated"
		*v.StartedAtMs = 88
	})
	remove := client.AddItemCompletedListener(func(n p.ItemCompletedNotification) {
		calls++
		v := n.Item.Value.(*p.HookPromptThreadItem)
		if v.Fragments[0].Text != "prompt" || *v.StartedAtMs != 0 {
			t.Error("callbacks share fragment/time references")
		}
	})
	defer remove()
	body := `{"threadId":"hook-thread","turnId":"turn","completedAtMs":0,"item":{"type":"hookPrompt","id":"i","fragments":[{"hookRunId":"r","text":"prompt"}],"startedAtMs":0}}`
	mock.InjectServerNotification(context.Background(), p.Notification{Method: "item/completed", Params: json.RawMessage(body)})
	if calls != 2 {
		t.Fatal("owned callback controls did not run")
	}
}

func TestHookPromptPlainFragmentAndFuture(t *testing.T) {
	var envelope struct {
		p.HookPromptFragment
		After string `json:"after"`
	}
	if err := json.Unmarshal([]byte(`{"hookRunId":"r","text":"t","after":"visited"}`), &envelope); err != nil || envelope.After != "visited" {
		t.Fatal("plain fragment embedding changed")
	}
	if _, ok := any(&p.HookPromptFragment{}).(json.Unmarshaler); ok {
		t.Fatal("fragment acquired a promoted decoder")
	}
	data := []byte(`{"type":"future-hook","fragments":null,"completedAtMs":"opaque"}`)
	var wrapper p.ThreadItemWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatal(err)
	}
	unknown, ok := wrapper.Value.(*p.UnknownThreadItem)
	if !ok || string(unknown.Raw) != string(data) {
		t.Fatal("future payload changed")
	}
	data[2] = 'X'
	if unknown.Raw[2] == 'X' {
		t.Fatal("unknown raw payload borrows input")
	}
	// Hook-specific fields stay unconstrained extras on a different known branch.
	if err := json.Unmarshal([]byte(`{"type":"plan","id":"i","text":"plan","fragments":null}`), &wrapper); err != nil {
		t.Fatal("unrelated item branch narrowed")
	}
}

func TestHookPromptNilFragmentsEncodeEmpty(t *testing.T) {
	item := p.HookPromptThreadItem{}
	for _, value := range []p.ThreadItem{item, &item} {
		raw, err := json.Marshal(p.ThreadItemWrapper{Value: value})
		if err != nil {
			t.Fatal(err)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			t.Fatal(err)
		}
		if string(object["fragments"]) != "[]" {
			t.Errorf("nil fragments emitted schema-invalid null: %s", raw)
		}
	}
}
