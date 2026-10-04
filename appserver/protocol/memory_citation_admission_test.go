package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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
			seed := `{"type":"agentMessage","id":"prior","text":"prior","memoryCitation":{"entries":[{"lineEnd":8,"lineStart":3,"note":"prior-note","path":"prior-path"}],"threadIds":["prior-id"]}}`
			var item protocol.ThreadItemWrapper
			if err := json.Unmarshal([]byte(seed), &item); err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(item)
			if err != nil {
				t.Fatal(err)
			}
			priorAgent := item.Value.(*protocol.AgentMessageThreadItem)
			priorCitation := priorAgent.MemoryCitation
			priorEntry := &priorCitation.Entries[0]
			priorThreadID := &priorCitation.ThreadIDs[0]
			if err := json.Unmarshal([]byte(raw), &item); err == nil {
				t.Fatal("invalid memory citation admitted")
			}
			after, err := json.Marshal(item)
			if err != nil || string(before) != string(after) {
				t.Fatalf("rejection changed prior wrapper value: err=%v before=%s after=%s", err, before, after)
			}
			if item.Value != priorAgent || priorAgent.MemoryCitation != priorCitation || &priorCitation.Entries[0] != priorEntry || &priorCitation.ThreadIDs[0] != priorThreadID {
				t.Fatal("rejection changed a retained wrapper/citation/slice reference")
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
	for name, body := range map[string]string{
		"entries empty then complete": memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":9,"lineStart":4,"note":"old","path":"old"}],"threadIds":["old"]},"memoryCitation":{"entries":[]},"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":"","path":""}],"threadIds":[]}`),
		"citation null then complete": memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":9,"lineStart":4,"note":"old","path":"old"}],"threadIds":["old"]},"memoryCitation":null,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":"","path":""}],"threadIds":[]}`),
	} {
		t.Run(name, func(t *testing.T) {
			var wrapper protocol.ThreadItemWrapper
			if err := json.Unmarshal([]byte(body), &wrapper); err != nil {
				t.Fatalf("complete record after reset rejected: %v", err)
			}
			assertMemoryCitationEqual(t, wrapper.Value.(*protocol.AgentMessageThreadItem).MemoryCitation, &protocol.MemoryCitation{
				Entries: []protocol.MemoryCitationEntry{{LineEnd: 0, LineStart: 0, Note: "", Path: ""}}, ThreadIDs: []string{},
			})
		})
	}
}

func TestMemoryCitationSplitEntryFieldsAndVisibility(t *testing.T) {
	const first = `{"lineEnd":7,"lineStart":2}`
	const second = `{"note":"note","path":"opaque/../path"}`
	for _, tc := range []struct {
		name          string
		first, second string
	}{
		{"line fields then text fields", first, second},
		{"text fields then line fields", second, first},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := memoryCitationItem(`,"memoryCitation":{"entries":[` + tc.first + `]},"memoryCitation":{"entries":[` + tc.second + `],"threadIds":[""]}`)
			var wrapper protocol.ThreadItemWrapper
			if err := json.Unmarshal([]byte(body), &wrapper); err != nil {
				t.Fatalf("split fields rejected: %v", err)
			}
			assertMemoryCitationEqual(t, wrapper.Value.(*protocol.AgentMessageThreadItem).MemoryCitation, &protocol.MemoryCitation{
				Entries:   []protocol.MemoryCitationEntry{{LineEnd: 7, LineStart: 2, Note: "note", Path: "opaque/../path"}},
				ThreadIDs: []string{""},
			})
		})
	}

	t.Run("newly visible incomplete index rejects", func(t *testing.T) {
		body := memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":1,"lineStart":0,"note":"n","path":"p"}],"threadIds":[]},"memoryCitation":{"entries":[{"note":"replacement"},{"note":"incomplete"}]}`)
		var wrapper protocol.ThreadItemWrapper
		if err := json.Unmarshal([]byte(body), &wrapper); err == nil {
			t.Fatal("newly visible incomplete index admitted")
		}
	})
	t.Run("truncation hides incomplete index", func(t *testing.T) {
		body := memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":1,"lineStart":0,"note":"n","path":"p"},{"note":"hidden"}],"threadIds":[]},"memoryCitation":{"entries":[{"note":"visible"}]}`)
		var wrapper protocol.ThreadItemWrapper
		if err := json.Unmarshal([]byte(body), &wrapper); err != nil {
			t.Fatalf("hidden incomplete index affected final visible graph: %v", err)
		}
		assertMemoryCitationEqual(t, wrapper.Value.(*protocol.AgentMessageThreadItem).MemoryCitation, &protocol.MemoryCitation{
			Entries: []protocol.MemoryCitationEntry{{LineEnd: 1, LineStart: 0, Note: "visible", Path: "p"}}, ThreadIDs: []string{},
		})
	})
}

func TestMemoryCitationNullRepairIsStickyForEveryEntryField(t *testing.T) {
	fields := []struct {
		name          string
		nullFragment  string
		validFragment string
	}{
		{"lineEnd", `"lineEnd":null`, `"lineEnd":1`},
		{"lineStart", `"lineStart":null`, `"lineStart":2`},
		{"note", `"note":null`, `"note":"n"`},
		{"path", `"path":null`, `"path":"p"`},
	}
	for _, field := range fields {
		for _, invalidFirst := range []bool{true, false} {
			order := "null then value"
			if !invalidFirst {
				order = "value then null"
			}
			t.Run(field.name+"/"+order, func(t *testing.T) {
				first, second := field.nullFragment, field.validFragment
				if !invalidFirst {
					first, second = second, first
				}
				body := memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":0,"note":"","path":"",` + first + `}],"threadIds":[]},"memoryCitation":{"entries":[{` + second + `}]}`)
				var wrapper protocol.ThreadItemWrapper
				if err := json.Unmarshal([]byte(body), &wrapper); err == nil {
					t.Fatalf("null %s repaired by later value", field.name)
				}
			})
		}
	}
	for _, alias := range []struct{ name, field, key, repair string }{
		{"folded", "path", "PATH", `"path":"p"`},
		{"escaped", "path", `\u0070ath`, `"path":"p"`},
		{"long s", "lineStart", "lineſtart", `"lineStart":1`},
	} {
		t.Run(alias.name, func(t *testing.T) {
			body := memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"` + alias.key + `":null,"note":"n","path":"p"}],"threadIds":[]},"memoryCitation":{"entries":[{` + alias.repair + `}]}`)
			var wrapper protocol.ThreadItemWrapper
			if err := json.Unmarshal([]byte(body), &wrapper); err == nil {
				t.Fatalf("%s null %s occurrence admitted", alias.name, alias.field)
			}
		})
	}
	var reset protocol.ThreadItemWrapper
	if err := json.Unmarshal([]byte(memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":null,"lineStart":0,"note":"n","path":"p"}],"threadIds":[]},"memoryCitation":null`)), &reset); err == nil {
		t.Fatal("citation null reset repaired earlier null scalar")
	}
}

func TestMemoryCitationUnicodeFoldAndBoundsControls(t *testing.T) {
	// encoding/json applies Unicode simple folding to struct field names. The
	// long-s aliases below are recognized by the standard library and matcher.
	for _, tc := range []struct {
		name string
		body string
		want *protocol.MemoryCitation
	}{
		{
			name: "long-s entries and thread IDs",
			body: `{"type":"agentMessage","id":"m","text":"t","memoryCitation":{"entrieſ":[{"lineEnd":1,"lineStart":2,"note":"n","path":"p"}],"threadIdſ":["s"]}}`,
			want: &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 1, LineStart: 2, Note: "n", Path: "p"}}, ThreadIDs: []string{"s"}},
		},
		{
			name: "long-s lineStart",
			body: `{"type":"agentMessage","id":"m","text":"t","memoryCitation":{"entries":[{"lineEnd":1,"lineſtart":2,"note":"n","path":"p"}],"threadIds":[]}}`,
			want: &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 1, LineStart: 2, Note: "n", Path: "p"}}, ThreadIDs: []string{}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var wrapper protocol.ThreadItemWrapper
			var reference struct {
				Type           string `json:"type"`
				ID             string `json:"id"`
				Text           string `json:"text"`
				MemoryCitation *struct {
					Entries []struct {
						LineEnd   uint32 `json:"lineEnd"`
						LineStart uint32 `json:"lineStart"`
						Note      string `json:"note"`
						Path      string `json:"path"`
					} `json:"entries"`
					ThreadIDs []string `json:"threadIds"`
				} `json:"memoryCitation"`
			}
			if err := json.Unmarshal([]byte(tc.body), &reference); err != nil {
				t.Fatalf("native reference rejected unicode fold: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.body), &wrapper); err != nil {
				t.Fatalf("admission rejected unicode fold accepted by native reference: %v", err)
			}
			assertMemoryCitationEqual(t, wrapper.Value.(*protocol.AgentMessageThreadItem).MemoryCitation, tc.want)
		})
	}

	var omitted protocol.ThreadItemWrapper
	if err := json.Unmarshal([]byte(memoryCitationItem("")), &omitted); err != nil {
		t.Fatalf("omitted optional citation rejected: %v", err)
	}
	if omitted.Value.(*protocol.AgentMessageThreadItem).MemoryCitation != nil {
		t.Fatal("omitted optional citation produced a value")
	}
	maxU32 := ^uint32(0)
	maxUint32 := strconv.FormatUint(uint64(maxU32), 10)
	maxBody := memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":` + maxUint32 + `,"lineStart":0,"note":"","path":""}],"threadIds":[""]}`)
	var maxValue protocol.ThreadItemWrapper
	if err := json.Unmarshal([]byte(maxBody), &maxValue); err != nil {
		t.Fatalf("max uint32/empty opaque fields rejected: %v", err)
	}
	got := maxValue.Value.(*protocol.AgentMessageThreadItem).MemoryCitation
	assertMemoryCitationEqual(t, got, &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: maxU32, Note: "", Path: ""}}, ThreadIDs: []string{""}})
	lessBody := memoryCitationItem(`,"memoryCitation":{"entries":[{"lineEnd":0,"lineStart":1,"note":"","path":""}],"threadIds":[]}`)
	var less protocol.ThreadItemWrapper
	if err := json.Unmarshal([]byte(lessBody), &less); err != nil {
		t.Fatalf("schema-valid lineEnd < lineStart rejected: %v", err)
	}
}

type nativeCitationReference struct {
	Entries   []nativeCitationEntry `json:"entries"`
	ThreadIDs []string              `json:"threadIds"`
}

type nativeCitationEntry struct {
	LineEnd   uint32 `json:"lineEnd"`
	LineStart uint32 `json:"lineStart"`
	Note      string `json:"note"`
	Path      string `json:"path"`
}

type nativeAgentMessageReference struct {
	Type           string                   `json:"type"`
	ID             string                   `json:"id"`
	Text           string                   `json:"text"`
	MemoryCitation *nativeCitationReference `json:"memoryCitation"`
}

func marshalCitationFixture(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestMemoryCitationGeneratedNativeReference(t *testing.T) {
	fieldNames := []string{"lineEnd", "lineStart", "note", "path"}
	for seed := 0; seed < 32; seed++ {
		t.Run(fmt.Sprintf("seed-%02d", seed), func(t *testing.T) {
			count := 2 + (seed*7+1)%4
			complete := make([]nativeCitationEntry, count)
			for i := range complete {
				complete[i] = nativeCitationEntry{
					LineEnd: uint32(seed + i + 1), LineStart: uint32(seed + i + 2),
					Note: fmt.Sprintf("note-%d-%d", seed, i), Path: fmt.Sprintf("opaque/%d/%d", seed, i),
				}
			}
			ids := []string{fmt.Sprintf("id-%d", seed), ""}
			occurrences := make([]string, 0, 12)
			appendEntries := func(entries []map[string]any) {
				occurrences = append(occurrences, `"memoryCitation":{"entries":`+marshalCitationFixture(t, entries)+`}`)
			}
			for _, name := range fieldNames {
				entries := make([]map[string]any, count)
				for i, entry := range complete {
					var value any
					switch name {
					case "lineEnd":
						value = entry.LineEnd
					case "lineStart":
						value = entry.LineStart
					case "note":
						value = entry.Note
					case "path":
						value = entry.Path
					}
					entries[i] = map[string]any{name: value}
				}
				appendEntries(entries)
			}
			occurrences = append(occurrences, `"memoryCitation":{"threadIds":`+marshalCitationFixture(t, ids)+`}`)

			switch seed % 4 {
			case 0:
				// A partial object merges over all populated indices.
				entries := make([]map[string]any, count)
				for i := range entries {
					entries[i] = map[string]any{"note": fmt.Sprintf("later-%d-%d", seed, i)}
				}
				appendEntries(entries)
			case 1:
				// Truncation and reextension reuse the hidden same-index values.
				short := make([]map[string]any, count-1)
				for i := range short {
					short[i] = map[string]any{"note": fmt.Sprintf("short-%d-%d", seed, i)}
				}
				appendEntries(short)
				reextended := make([]map[string]any, count)
				for i := range reextended {
					reextended[i] = map[string]any{"path": fmt.Sprintf("reextended/%d/%d", seed, i)}
				}
				appendEntries(reextended)
			case 2:
				occurrences = append(occurrences, `"memoryCitation":{"entries":[]}`)
				appendCompleteOccurrences := func() {
					for _, name := range fieldNames {
						entries := make([]map[string]any, count)
						for i, entry := range complete {
							var value any
							switch name {
							case "lineEnd":
								value = entry.LineEnd
							case "lineStart":
								value = entry.LineStart
							case "note":
								value = entry.Note
							case "path":
								value = entry.Path
							}
							entries[i] = map[string]any{name: value}
						}
						appendEntries(entries)
					}
				}
				appendCompleteOccurrences()
			case 3:
				occurrences = append(occurrences, `"memoryCitation":null`)
				for _, name := range fieldNames {
					entries := make([]map[string]any, count)
					for i, entry := range complete {
						var value any
						switch name {
						case "lineEnd":
							value = entry.LineEnd
						case "lineStart":
							value = entry.LineStart
						case "note":
							value = entry.Note
						case "path":
							value = entry.Path
						}
						entries[i] = map[string]any{name: value}
					}
					appendEntries(entries)
				}
				occurrences = append(occurrences, `"memoryCitation":{"threadIds":`+marshalCitationFixture(t, ids)+`}`)
			}
			raw := `{"type":"agentMessage","id":"generated","text":"reference"}`
			raw = strings.TrimSuffix(raw, `}`) + `,` + strings.Join(occurrences, ",") + `}`
			var reference nativeAgentMessageReference
			if err := json.Unmarshal([]byte(raw), &reference); err != nil {
				t.Fatalf("method-free native reference rejected generated valid sequence: %v\n%s", err, raw)
			}
			if reference.MemoryCitation == nil || len(reference.MemoryCitation.Entries) != count || len(reference.MemoryCitation.ThreadIDs) != len(ids) {
				t.Fatalf("generated native reference was vacuous: %#v", reference.MemoryCitation)
			}
			var actual protocol.ThreadItemWrapper
			if err := json.Unmarshal([]byte(raw), &actual); err != nil {
				t.Fatalf("SDK rejected native-valid generated sequence: %v\n%s", err, raw)
			}
			got := actual.Value.(*protocol.AgentMessageThreadItem).MemoryCitation
			want := &protocol.MemoryCitation{Entries: make([]protocol.MemoryCitationEntry, len(reference.MemoryCitation.Entries)), ThreadIDs: append([]string(nil), reference.MemoryCitation.ThreadIDs...)}
			for i, entry := range reference.MemoryCitation.Entries {
				want.Entries[i] = protocol.MemoryCitationEntry{LineEnd: entry.LineEnd, LineStart: entry.LineStart, Note: entry.Note, Path: entry.Path}
			}
			assertMemoryCitationEqual(t, got, want)
		})
	}
}

func generatedCitationSequence(input []byte) string {
	byteAt := func(index int) byte {
		if len(input) == 0 {
			return 0
		}
		return input[index%len(input)]
	}
	count := 1 + int(byteAt(0)%4)
	full := make([]map[string]any, count)
	for i := range full {
		full[i] = map[string]any{
			"lineEnd":   uint32(byteAt(i+1)) + uint32(i),
			"lineStart": uint32(byteAt(i+5)) + uint32(i),
			"note":      fmt.Sprintf("n-%d-%d", byteAt(i+9), i),
			"path":      fmt.Sprintf("opaque/%d/%d", byteAt(i+13), i),
		}
	}
	var occurrences []string
	appendObject := func(value any, citationKey string) {
		entries, _ := json.Marshal(value)
		occurrences = append(occurrences, `"memoryCitation":{"`+citationKey+`":`+string(entries)+`}`)
	}
	fieldNames := []string{"lineEnd", "lineStart", "note", "path"}
	start := int(byteAt(18) % 4)
	for offset := 0; offset < 4; offset++ {
		field := fieldNames[(start+offset)%4]
		key := field
		if byteAt(offset+19)%2 == 1 {
			key = strings.ToUpper(field)
		}
		partial := make([]map[string]any, count)
		for i := range full {
			partial[i] = map[string]any{key: full[i][field]}
		}
		appendObject(partial, "entries")
	}
	ids := []string{fmt.Sprintf("id-%d", byteAt(23)), ""}
	appendObject(ids, "threadIds")
	operationCount := 1 + int(byteAt(24)%5)
	for operation := 0; operation < operationCount; operation++ {
		choice := byteAt(25+operation) % 4
		switch choice {
		case 0:
			visible := 1 + int(byteAt(30+operation)%byte(count))
			partial := make([]map[string]any, visible)
			for i := range partial {
				partial[i] = map[string]any{"note": fmt.Sprintf("update-%d-%d", operation, i)}
			}
			appendObject(partial, "ENTRIES")
		case 1:
			occurrences = append(occurrences, `"memoryCitation":{"entries":[]}`)
			appendObject(full, "entries")
		case 2:
			occurrences = append(occurrences, `"memoryCitation":null`)
			appendObject(full, "entries")
			appendObject(ids, "THREADIDS")
		case 3:
			visible := 1 + int(byteAt(35+operation)%byte(count))
			partial := make([]map[string]any, visible)
			for i := range partial {
				partial[i] = map[string]any{"path": fmt.Sprintf("alias/%d/%d", operation, i)}
			}
			appendObject(partial, "entries")
		}
	}
	return `{"type":"agentMessage","id":"generated","text":"reference",` + strings.Join(occurrences, ",") + `}`
}

func FuzzMemoryCitationGeneratedNativeReference(f *testing.F) {
	for _, seed := range [][]byte{
		{}, {0}, {1, 2, 3, 4, 5, 6}, {255, 254, 253, 252, 251},
		{4, 0, 3, 8, 1, 7, 2, 6, 9, 5}, {3, 9, 1, 8, 2, 7, 4, 6},
		{2, 12, 29, 44, 51, 73, 98, 121}, {7, 0, 0, 1, 1, 2, 3, 5, 8, 13},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, rawSeed []byte) {
		if len(rawSeed) > 128 {
			rawSeed = rawSeed[:128]
		}
		valid := generatedCitationSequence(rawSeed)
		var reference nativeAgentMessageReference
		if err := json.Unmarshal([]byte(valid), &reference); err != nil {
			t.Fatalf("method-free native reference rejected generated sequence: %v\n%s", err, valid)
		}
		if reference.MemoryCitation == nil || len(reference.MemoryCitation.Entries) == 0 {
			t.Fatalf("generator produced vacuous valid reference: %#v", reference.MemoryCitation)
		}
		var actual protocol.ThreadItemWrapper
		if err := json.Unmarshal([]byte(valid), &actual); err != nil {
			t.Fatalf("SDK rejected native-valid generated sequence: %v\n%s", err, valid)
		}
		got := actual.Value.(*protocol.AgentMessageThreadItem).MemoryCitation
		want := &protocol.MemoryCitation{Entries: make([]protocol.MemoryCitationEntry, len(reference.MemoryCitation.Entries)), ThreadIDs: append([]string(nil), reference.MemoryCitation.ThreadIDs...)}
		for i, entry := range reference.MemoryCitation.Entries {
			want.Entries[i] = protocol.MemoryCitationEntry{LineEnd: entry.LineEnd, LineStart: entry.LineStart, Note: entry.Note, Path: entry.Path}
		}
		assertMemoryCitationEqual(t, got, want)

		// Each generated valid record gets a separate guaranteed forbidden null
		// occurrence, and rejection must preserve an existing public wrapper.
		mutation := int(rawSeedByte(rawSeed, 0) % 4)
		badEntryFields := []string{"lineEnd", "lineStart", "note", "path"}
		var badEntry string
		if rawSeedByte(rawSeed, 1)%2 == 0 {
			badEntry = strings.TrimSuffix(valid, "}") + `,"memoryCitation":{"entries":[{"` + badEntryFields[mutation] + `":null}]}}`
		} else {
			missing := badEntryFields[mutation]
			members := make([]string, 0, len(badEntryFields)-1)
			for _, field := range badEntryFields {
				switch field {
				case missing:
					continue
				}
				value := `"n"`
				switch field {
				case "lineEnd":
					value = `1`
				case "lineStart":
					value = `0`
				}
				members = append(members, `"`+field+`":`+value)
			}
			badEntry = strings.TrimSuffix(valid, "}") + `,"memoryCitation":null,"memoryCitation":{"entries":[{` + strings.Join(members, ",") + `}],"threadIds":[]}}`
		}
		var nativeBad nativeAgentMessageReference
		if err := json.Unmarshal([]byte(badEntry), &nativeBad); err != nil {
			t.Fatalf("native method-free reference rejected null scalar mutation: %v", err)
		}
		priorRaw := memoryCitationItem(`,"memoryCitation":{"entries":[],"threadIds":["prior"]}`)
		var target protocol.ThreadItemWrapper
		if err := json.Unmarshal([]byte(priorRaw), &target); err != nil {
			t.Fatal(err)
		}
		before, err := json.Marshal(target)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(badEntry), &target); err == nil {
			t.Fatal("generated repaired null scalar admitted")
		}
		after, err := json.Marshal(target)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("generated rejection changed prior wrapper: before=%s after=%s err=%v", before, after, err)
		}
	})
}

func rawSeedByte(seed []byte, index int) byte {
	if len(seed) == 0 {
		return 0
	}
	return seed[index%len(seed)]
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
