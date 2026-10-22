package protocol_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func assertUnionJSON(t *testing.T, value any, want string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var gotJSON, wantJSON any
	if err := json.Unmarshal(data, &gotJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &wantJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Errorf("JSON = %s, want %s", data, want)
	}
}

// Embedding preserves the public interfaces' application extension behavior.
type customEncodingUnion struct {
	p.ThreadItem      `json:"-"`
	p.PatchChangeKind `json:"-"`
	p.WebSearchAction `json:"-"`
	Err               error `json:"-"`
}

func (v customEncodingUnion) MarshalJSON() ([]byte, error) {
	if v.Err != nil {
		return nil, v.Err
	}
	return []byte(`{"type":"application","payload":42}`), nil
}

func TestUnionValueEncodingCompatibility(t *testing.T) {
	for _, wrapper := range []func(customEncodingUnion) any{
		func(v customEncodingUnion) any { return p.ThreadItemWrapper{Value: v} },
		func(v customEncodingUnion) any { return p.PatchChangeKindWrapper{Value: v} },
		func(v customEncodingUnion) any { return p.WebSearchActionWrapper{Value: v} },
	} {
		assertUnionJSON(t, wrapper(customEncodingUnion{}), `{"type":"application","payload":42}`)
		want := errors.New("application encoder failure")
		if _, err := json.Marshal(wrapper(customEncodingUnion{Err: want})); !errors.Is(err, want) {
			t.Errorf("application error lost: %v", err)
		}
	}
	for _, wrapper := range []any{
		p.ThreadItemWrapper{}, p.PatchChangeKindWrapper{}, p.WebSearchActionWrapper{},
		p.ThreadItemWrapper{Value: (*p.PlanThreadItem)(nil)},
		p.PatchChangeKindWrapper{Value: (*p.AddPatchChangeKind)(nil)},
		p.WebSearchActionWrapper{Value: (*p.OtherWebSearchAction)(nil)},
		p.ThreadItemWrapper{Value: p.UnknownThreadItem{}},
		p.PatchChangeKindWrapper{Value: p.UnknownPatchChangeKind{}},
		p.WebSearchActionWrapper{Value: p.UnknownWebSearchAction{}},
	} {
		assertUnionJSON(t, wrapper, `null`)
	}
}

func TestThreadItemValueEncodingEnforcesCodec(t *testing.T) {
	for _, value := range []p.ImageGenerationThreadItem{
		{ID: "i", Status: "completed"},
		{ID: "i", Status: "completed", FileID: ptr("file"), InputImages: new([]p.ImageGenerationInputImage)},
	} {
		for _, item := range []p.ThreadItem{value, &value} {
			if _, err := json.Marshal(p.ThreadItemWrapper{Value: item}); err == nil {
				t.Errorf("%T bypassed image codec validation", item)
			}
		}
	}
}

func TestThreadItemValueEncodingInCarriers(t *testing.T) {
	item := p.ThreadItemWrapper{Value: p.WebSearchThreadItem{ID: "i", Query: "q", Action: &p.WebSearchActionWrapper{Value: p.OtherWebSearchAction{}}}}
	assertUnionJSON(t, p.ItemCompletedNotification{ThreadID: "t", TurnID: "u", Item: item}, `{"completedAtMs":0,"threadId":"t","turnId":"u","item":{"type":"webSearch","id":"i","query":"q","action":{"type":"other"}}}`)
	// A cache snapshot must retain the value variant's wire representation.
	client := p.NewClient(NewMockTransport())
	client.CacheThreadState(p.Thread{ID: "t", Turns: []p.Turn{{ID: "u", Items: []p.ThreadItemWrapper{item}}}})
	thread, ok := client.ThreadStateSnapshot("t")
	if !ok {
		t.Fatal("missing cached thread")
	}
	assertUnionJSON(t, thread.Turns[0].Items[0], `{"type":"webSearch","id":"i","query":"q","action":{"type":"other"}}`)
}

func TestThreadItemValueEncoding(t *testing.T) {
	for _, tc := range []struct {
		value p.ThreadItem
		wire  string
	}{
		{p.UserMessageThreadItem{ID: "i", Content: []p.UserInput{&p.TextUserInput{Text: "hello"}}}, `{"type":"userMessage","id":"i","content":[{"type":"text","text":"hello"}]}`},
		{p.AgentMessageThreadItem{ID: "i", Text: "hello"}, `{"type":"agentMessage","id":"i","text":"hello"}`},
		{p.PlanThreadItem{ID: "i", Text: "step"}, `{"type":"plan","id":"i","text":"step"}`},
		{p.ReasoningThreadItem{ID: "i", Summary: []string{"why"}}, `{"type":"reasoning","id":"i","summary":["why"]}`},
		{p.CommandExecutionThreadItem{ID: "i", Command: "pwd", CommandActions: []p.CommandActionWrapper{}, Cwd: "/work", Status: "completed"}, `{"type":"commandExecution","id":"i","command":"pwd","commandActions":[],"cwd":"/work","status":"completed"}`},
		{p.FileChangeThreadItem{ID: "i", Changes: []p.FileUpdateChange{{Path: "/work/file", Diff: "+hello", Kind: p.PatchChangeKindWrapper{Value: p.AddPatchChangeKind{}}}}, Status: "completed"}, `{"type":"fileChange","id":"i","changes":[{"path":"/work/file","diff":"+hello","kind":{"type":"add"}}],"status":"completed"}`},
		{p.McpToolCallThreadItem{ID: "i", Server: "server", Tool: "tool", Status: "completed", Arguments: map[string]any{"q": "v"}}, `{"type":"mcpToolCall","id":"i","server":"server","tool":"tool","status":"completed","arguments":{"q":"v"}}`},
		{p.DynamicToolCallThreadItem{ID: "i", Tool: "tool", Status: "completed", Arguments: map[string]any{}}, `{"type":"dynamicToolCall","id":"i","tool":"tool","status":"completed","arguments":{}}`},
		{p.CollabAgentToolCallThreadItem{ID: "i", Tool: "spawnAgent", Status: "completed", AgentsStates: map[string]p.CollabAgentState{}, ReceiverThreadIds: []string{}, SenderThreadId: "sender"}, `{"type":"collabAgentToolCall","id":"i","tool":"spawnAgent","status":"completed","agentsStates":{},"receiverThreadIds":[],"senderThreadId":"sender"}`},
		{p.SubAgentActivityThreadItem{ID: "i", AgentPath: "agent", AgentThreadID: "child", Kind: "completed"}, `{"type":"subAgentActivity","id":"i","agentPath":"agent","agentThreadId":"child","kind":"completed"}`},
		{p.SleepThreadItem{ID: "i", DurationMs: 0}, `{"type":"sleep","id":"i","durationMs":0}`},
		{p.WebSearchThreadItem{ID: "i", Query: "query", Action: &p.WebSearchActionWrapper{Value: p.OtherWebSearchAction{}}}, `{"type":"webSearch","id":"i","query":"query","action":{"type":"other"}}`},
		{p.ImageViewThreadItem{ID: "i", Path: "/image"}, `{"type":"imageView","id":"i","path":"/image"}`},
		{p.EnteredReviewModeThreadItem{ID: "i", Review: "review"}, `{"type":"enteredReviewMode","id":"i","review":"review"}`},
		{p.ExitedReviewModeThreadItem{ID: "i", Review: "review"}, `{"type":"exitedReviewMode","id":"i","review":"review"}`},
		{p.ContextCompactionThreadItem{ID: "i"}, `{"type":"contextCompaction","id":"i"}`},
		{p.FunctionCallOutputThreadItem{ID: "i", Name: "tool", Output: p.FunctionCallOutputBody{Text: ptr("done")}}, `{"type":"functionCallOutput","id":"i","name":"tool","output":"done"}`},
		{p.ImageGenerationThreadItem{ID: "i", Status: "completed", FileID: ptr("file")}, `{"type":"imageGeneration","id":"i","status":"completed","fileId":"file"}`},
		{p.UnknownThreadItem{Type: "future", Raw: json.RawMessage(`{"type":"future","payload":{"answer":42}}`)}, `{"type":"future","payload":{"answer":42}}`},
	} {
		t.Run(reflect.TypeOf(tc.value).Name(), func(t *testing.T) {
			assertUnionJSON(t, p.ThreadItemWrapper{Value: tc.value}, tc.wire)
			pointer := reflect.New(reflect.TypeOf(tc.value))
			pointer.Elem().Set(reflect.ValueOf(tc.value))
			assertUnionJSON(t, p.ThreadItemWrapper{Value: pointer.Interface().(p.ThreadItem)}, tc.wire)
			var decoded p.ThreadItemWrapper
			if err := json.Unmarshal([]byte(tc.wire), &decoded); err != nil {
				t.Fatalf("invalid fixture: %v", err)
			}
		})
	}
}

func TestNestedUnionValueEncoding(t *testing.T) {
	for _, tc := range []struct {
		value any
		wire  string
	}{
		{p.PatchChangeKindWrapper{Value: p.AddPatchChangeKind{}}, `{"type":"add"}`},
		{p.PatchChangeKindWrapper{Value: p.DeletePatchChangeKind{}}, `{"type":"delete"}`},
		{p.PatchChangeKindWrapper{Value: p.UpdatePatchChangeKind{MovePath: ptr("target")}}, `{"type":"update","move_path":"target"}`},
		{p.PatchChangeKindWrapper{Value: p.UnknownPatchChangeKind{Type: "future", Raw: json.RawMessage(`{"type":"future","payload":42}`)}}, `{"type":"future","payload":42}`},
		{p.WebSearchActionWrapper{Value: p.SearchWebSearchAction{Query: ptr("query"), Queries: &[]string{}}}, `{"type":"search","query":"query","queries":[]}`},
		{p.WebSearchActionWrapper{Value: p.OpenPageWebSearchAction{URL: ptr("https://example.test")}}, `{"type":"openPage","url":"https://example.test"}`},
		{p.WebSearchActionWrapper{Value: p.FindInPageWebSearchAction{URL: ptr("https://example.test"), Pattern: ptr("text")}}, `{"type":"findInPage","url":"https://example.test","pattern":"text"}`},
		{p.WebSearchActionWrapper{Value: p.OtherWebSearchAction{}}, `{"type":"other"}`},
		{p.WebSearchActionWrapper{Value: p.UnknownWebSearchAction{Type: "future", Raw: json.RawMessage(`{"type":"future","payload":42}`)}}, `{"type":"future","payload":42}`},
	} {
		t.Run(tc.wire, func(t *testing.T) { assertUnionJSON(t, tc.value, tc.wire) })
	}
}
