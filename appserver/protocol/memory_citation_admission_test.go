package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	protocol "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func memoryCitationItem(citation string) string {
	return `{"type":"agentMessage","id":"m1","text":"hello"` + citation + `}`
}

func invalidMemoryCitationItems() map[string]string {
	return map[string]string{
		"missing entries":    memoryCitationItem(`,"memoryCitation":{"threadIds":[]}`),
		"null entries":       memoryCitationItem(`,"memoryCitation":{"entries":null,"threadIds":[]}`),
		"missing thread IDs": memoryCitationItem(`,"memoryCitation":{"entries":[]}`),
		"null thread IDs":    memoryCitationItem(`,"memoryCitation":{"entries":[],"threadIds":null}`),
		"null entry":         memoryCitationItem(`,"memoryCitation":{"entries":[null],"threadIds":[]}`),
		"empty entry":        memoryCitationItem(`,"memoryCitation":{"entries":[{}],"threadIds":[]}`),
		"missing lineEnd":    memoryCitationItem(`,"memoryCitation":{"entries":[{"lineStart":0,"note":"n","path":"p"}],"threadIds":[]}`),
		"null lineEnd":       memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":null,"lineStart":0,"note":"n","path":"p"}],"threadIds":[]}`),
		"missing lineStart":  memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"note":"n","path":"p"}],"threadIds":[]}`),
		"null lineStart":     memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":null,"note":"n","path":"p"}],"threadIds":[]}`),
		"missing note":       memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"path":"p"}],"threadIds":[]}`),
		"null note":          memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":null,"path":"p"}],"threadIds":[]}`),
		"missing path":       memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":"n"}],"threadIds":[]}`),
		"null path":          memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":"n","path":null}],"threadIds":[]}`),
	}
}

func validMemoryCitationItems() []string {
	return []string{
		memoryCitationItem(`,"memoryCitation":{"entries":[],"threadIds":[]}`),
		memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":"","path":""}],"threadIds":[""]}`),
		memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":"relative","path":"relative/path"}],"threadIds":[]}`),
	}
}

func expectedMemoryCitation(index int) *protocol.MemoryCitation {
	switch index {
	case 0:
		return &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{}, ThreadIDs: []string{}}
	case 1:
		return &protocol.MemoryCitation{
			Entries:   []protocol.MemoryCitationEntry{{LineEnd: 0, LineStart: 0, Note: "", Path: ""}},
			ThreadIDs: []string{""},
		}
	default:
		return &protocol.MemoryCitation{
			Entries:   []protocol.MemoryCitationEntry{{LineEnd: 0, LineStart: 0, Note: "relative", Path: "relative/path"}},
			ThreadIDs: []string{},
		}
	}
}

func assertMemoryCitationEqual(t *testing.T, got, want *protocol.MemoryCitation) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("citation = %#v, want %#v", got, want)
	}
}

func TestMemoryCitationRecordAdmission(t *testing.T) {
	for name, raw := range invalidMemoryCitationItems() {
		t.Run(name, func(t *testing.T) {
			seed := `{"type":"agentMessage","id":"prior","text":"prior","memoryCitation":{"entries":[],"threadIds":["prior"]}}`
			var item protocol.ThreadItemWrapper
			if err := json.Unmarshal([]byte(seed), &item); err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(item)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(raw), &item); err == nil {
				t.Fatal("invalid memory citation admitted")
			}
			after, err := json.Marshal(item)
			if err != nil || string(before) != string(after) {
				t.Fatalf("rejection changed prior wrapper value: err=%v before=%s after=%s", err, before, after)
			}
			if err := json.Unmarshal([]byte(validMemoryCitationItems()[0]), &item); err != nil {
				t.Fatalf("valid recovery: %v", err)
			}
		})
	}
	for i, raw := range validMemoryCitationItems() {
		t.Run(fmt.Sprintf("valid/%d", i), func(t *testing.T) {
			var item protocol.ThreadItemWrapper
			if err := json.Unmarshal([]byte(raw), &item); err != nil {
				t.Fatalf("valid citation rejected: %v", err)
			}
			assertMemoryCitationEqual(t, item.Value.(*protocol.AgentMessageThreadItem).MemoryCitation, expectedMemoryCitation(i))
		})
	}
}

func TestMemoryCitationItemCompletedAdmissionAndRecovery(t *testing.T) {
	for name, raw := range invalidMemoryCitationItems() {
		t.Run(name, func(t *testing.T) {
			transport := NewMockTransport()
			var calls, reported int
			var gotCitation *protocol.MemoryCitation
			client := protocol.NewClient(transport, protocol.WithHandlerErrorCallback(func(method string, err error) {
				if method != "item/completed" || err == nil {
					t.Errorf("handler error origin=%q error=%v", method, err)
				}
				reported++
			}))
			t.Cleanup(func() { _ = client.Close() })
			client.OnItemCompleted(func(value protocol.ItemCompletedNotification) {
				calls++
				gotCitation = value.Item.Value.(*protocol.AgentMessageThreadItem).MemoryCitation
			})
			inject := func(item string) {
				body := fmt.Sprintf(`{"completedAtMs":1,"threadId":"t","turnId":"u","item":%s}`, item)
				transport.InjectServerNotification(context.Background(), protocol.Notification{
					JSONRPC: "2.0", Method: "item/completed", Params: json.RawMessage(body),
				})
			}
			inject(raw)
			if calls != 0 || reported != 1 {
				t.Fatalf("invalid item publication: calls=%d handlerErrors=%d", calls, reported)
			}
			inject(validMemoryCitationItems()[0])
			if calls != 1 || reported != 1 {
				t.Fatalf("valid recovery: calls=%d handlerErrors=%d", calls, reported)
			}
			assertMemoryCitationEqual(t, gotCitation, expectedMemoryCitation(0))
		})
	}
	for i, raw := range validMemoryCitationItems() {
		t.Run(fmt.Sprintf("valid/%d", i), func(t *testing.T) {
			transport := NewMockTransport()
			var calls, reported int
			var gotCitation *protocol.MemoryCitation
			client := protocol.NewClient(transport, protocol.WithHandlerErrorCallback(func(string, error) { reported++ }))
			t.Cleanup(func() { _ = client.Close() })
			client.OnItemCompleted(func(value protocol.ItemCompletedNotification) {
				calls++
				gotCitation = value.Item.Value.(*protocol.AgentMessageThreadItem).MemoryCitation
			})
			body := fmt.Sprintf(`{"completedAtMs":1,"threadId":"t","turnId":"u","item":%s}`, raw)
			transport.InjectServerNotification(context.Background(), protocol.Notification{
				JSONRPC: "2.0", Method: "item/completed", Params: json.RawMessage(body),
			})
			if calls != 1 || reported != 0 {
				t.Fatalf("valid item not published: calls=%d handlerErrors=%d", calls, reported)
			}
			assertMemoryCitationEqual(t, gotCitation, expectedMemoryCitation(i))
		})
	}
}

func TestMemoryCitationDuplicateMergePresence(t *testing.T) {
	tests := []struct {
		name string
		body string
		want *protocol.MemoryCitation
	}{
		{
			name: "entries then thread IDs",
			body: memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":2,"lineStart":1,"note":"n","path":"p"}]},"memoryCitation":{"threadIds":["t"]}`),
			want: &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 2, LineStart: 1, Note: "n", Path: "p"}}, ThreadIDs: []string{"t"}},
		},
		{
			name: "thread IDs then entries",
			body: memoryCitationItem(`,"memoryCitation":{"threadIds":["t"]},"memoryCitation":{"entries":[{"lineEnd":2,"lineStart":1,"note":"n","path":"p"}]}`),
			want: &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 2, LineStart: 1, Note: "n", Path: "p"}}, ThreadIDs: []string{"t"}},
		},
		{
			name: "full then empty object keeps fields",
			body: memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":9,"lineStart":4,"note":"old","path":"old/path"}],"threadIds":["old"]},"memoryCitation":{"entries":[{}]}`),
			want: &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 9, LineStart: 4, Note: "old", Path: "old/path"}}, ThreadIDs: []string{"old"}},
		},
		{
			name: "truncate and reextend preserves hidden presence",
			body: memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":9,"lineStart":4,"note":"old0","path":"path0"},{"lineEnd":19,"lineStart":14,"note":"old1","path":"path1"}],"threadIds":[]},"memoryCitation":{"entries":[{"note":"middle"}]},"memoryCitation":{"entries":[{"note":"new0"},{"path":"new1"}]}`),
			want: &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 9, LineStart: 4, Note: "new0", Path: "path0"}, {LineEnd: 19, LineStart: 14, Note: "old1", Path: "new1"}}, ThreadIDs: []string{}},
		},
		{
			name: "folded and escaped names",
			body: `{"type":"agentMessage","id":"m1","text":"hello","MEMORYCITATION":{"ENTRIES":[{"LINE\u0045ND":2,"LINESTART":1,"NOTE":"n","PATH":"p"}],"THREADIDS":["t"]}}`,
			want: &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 2, LineStart: 1, Note: "n", Path: "p"}}, ThreadIDs: []string{"t"}},
		},
		{
			name: "folded scalar duplicates last wins",
			body: `{"type":"agentMessage","id":"m1","text":"hello","memoryCitation":{"entries":[{"lineEnd":1,"LINEEND":2,"lineStart":3,"note":"first","path":"a","PATH":"b"}],"threadIds":["first"],"THREADIDS":["last"]}}`,
			want: &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 2, LineStart: 3, Note: "first", Path: "b"}}, ThreadIDs: []string{"last"}},
		},
		{
			name: "folded scalar duplicates reverse order",
			body: `{"type":"agentMessage","id":"m1","text":"hello","memoryCitation":{"entries":[{"LINEEND":2,"lineEnd":1,"lineStart":3,"note":"first","PATH":"b","path":"a"}],"threadIds":["last"],"THREADIDS":["first"]}}`,
			want: &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 1, LineStart: 3, Note: "first", Path: "a"}}, ThreadIDs: []string{"first"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var item protocol.ThreadItemWrapper
			if err := json.Unmarshal([]byte(tc.body), &item); err != nil {
				t.Fatalf("valid duplicate merge rejected: %v", err)
			}
			assertMemoryCitationEqual(t, item.Value.(*protocol.AgentMessageThreadItem).MemoryCitation, tc.want)
		})
	}
}

func TestMemoryCitationDuplicateNullsRemainRejected(t *testing.T) {
	for name, body := range map[string]string{
		"null entries repaired": memoryCitationItem(`,"memoryCitation":{"entries":null,"threadIds":[]},"memoryCitation":{"entries":[],"threadIds":[]}`),
		"null entry repaired":   memoryCitationItem(`,"memoryCitation":{"entries":[null],"threadIds":[]},"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":"n","path":"p"}]}`),
		"null scalar repaired":  memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":null,"lineStart":0,"note":"n","path":"p"}],"threadIds":[]},"memoryCitation":{"entries":[{"lineEnd":0}]}`),
	} {
		t.Run(name, func(t *testing.T) {
			var item protocol.ThreadItemWrapper
			if err := json.Unmarshal([]byte(body), &item); err == nil {
				t.Fatal("duplicate citation repaired an explicitly null occurrence")
			}
		})
	}
}

func TestMemoryCitationFinalResetsValidateVisibleGraph(t *testing.T) {
	for name, body := range map[string]string{
		"empty entries reset": memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":"n","path":"p"}],"threadIds":[]},"memoryCitation":{"entries":[],"threadIds":[]},"memoryCitation":{"entries":[{"note":"new"}]}`),
		"citation null reset": memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":"n","path":"p"}],"threadIds":[]},"memoryCitation":null,"memoryCitation":{"entries":[{"note":"new"}],"threadIds":[]}`),
	} {
		t.Run(name, func(t *testing.T) {
			var item protocol.ThreadItemWrapper
			if err := json.Unmarshal([]byte(body), &item); err == nil {
				t.Fatal("incomplete visible citation admitted after reset")
			}
		})
	}
	var finalNull protocol.ThreadItemWrapper
	if err := json.Unmarshal([]byte(memoryCitationItem(`,"memoryCitation":{"entries":[{"note":"missing"}]},"memoryCitation":null`)), &finalNull); err != nil {
		t.Fatalf("final nullable citation reset rejected: %v", err)
	}
	if finalNull.Value.(*protocol.AgentMessageThreadItem).MemoryCitation != nil {
		t.Fatal("final null citation did not reset public value")
	}
}

func TestMemoryCitationPersistedThreadReadAdmission(t *testing.T) {
	transport := NewMockTransport()
	client := protocol.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	thread := validProcessThreadPayload("thread-citation")
	prior := protocol.Thread{ID: "thread-citation", Preview: "prior"}
	client.CacheThreadState(prior)
	updates := 0
	remove := client.AddThreadStateListener(prior.ID, func(protocol.Thread) { updates++ }, nil)
	defer remove()
	initialUpdates := updates
	turn := map[string]interface{}{"id": "turn-1", "status": "completed"}
	thread["turns"] = []interface{}{turn}
	items := []interface{}{json.RawMessage(invalidMemoryCitationItems()["empty entry"])}
	turn["items"] = items
	if err := transport.SetResponseData("thread/read", map[string]interface{}{"thread": thread}); err != nil {
		t.Fatal(err)
	}
	if result, err := client.Thread.Read(context.Background(), protocol.ThreadReadParams{ThreadID: "thread-citation"}); err == nil || result.Thread.ID != "" {
		t.Fatal("Thread.Read admitted an invalid persisted citation")
	}
	state, ok := client.ThreadStateSnapshot(prior.ID)
	if !ok || !reflect.DeepEqual(prior, state) || updates != initialUpdates {
		t.Fatalf("invalid persisted citation changed cache/listener state: cached=%+v updates=%d", state, updates)
	}
	turn["items"] = []interface{}{json.RawMessage(validMemoryCitationItems()[0])}
	if err := transport.SetResponseData("thread/read", map[string]interface{}{"thread": thread}); err != nil {
		t.Fatal(err)
	}
	got, err := client.Thread.Read(context.Background(), protocol.ThreadReadParams{ThreadID: "thread-citation"})
	if err != nil {
		t.Fatalf("valid persisted recovery: %v", err)
	}
	if len(got.Thread.Turns) != 1 || len(got.Thread.Turns[0].Items) != 1 {
		t.Fatalf("valid persisted recovery returned unexpected turns/items: %#v", got.Thread.Turns)
	}
	if _, ok := got.Thread.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem); !ok {
		t.Fatalf("valid persisted item has type %T", got.Thread.Turns[0].Items[0].Value)
	}
	state, ok = client.ThreadStateSnapshot(prior.ID)
	if !ok || len(state.Turns) != 1 || updates != initialUpdates+1 {
		t.Fatalf("valid recovery not cached/notified: cached=%+v updates=%d", state, updates)
	}
	citation := state.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation
	if citation == nil || !reflect.DeepEqual(citation.Entries, []protocol.MemoryCitationEntry{}) || !reflect.DeepEqual(citation.ThreadIDs, []string{}) {
		t.Fatalf("valid recovery returned unexpected citation: %#v", citation)
	}
}
