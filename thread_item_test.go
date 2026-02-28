package codex_test

import (
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestThreadItemRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		checkFn func(*testing.T, codex.ThreadItemWrapper)
	}{
		{
			name:  "userMessage",
			input: `{"type":"userMessage","id":"u1","content":[{"type":"text","text":"hello"}]}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.UserMessageThreadItem)
				if !ok {
					t.Fatalf("expected *UserMessageThreadItem, got %T", w.Value)
				}
				if v.ID != "u1" {
					t.Errorf("ID = %q, want %q", v.ID, "u1")
				}
				if len(v.Content) != 1 {
					t.Fatalf("len(Content) = %d, want 1", len(v.Content))
				}
				txt, ok := v.Content[0].(*codex.TextUserInput)
				if !ok {
					t.Fatalf("Content[0]: expected *TextUserInput, got %T", v.Content[0])
				}
				if txt.Text != "hello" {
					t.Errorf("Content[0].Text = %q, want %q", txt.Text, "hello")
				}
			},
		},
		{
			name:  "agentMessage",
			input: `{"type":"agentMessage","id":"a1","text":"response","phase":"commentary"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.AgentMessageThreadItem)
				if !ok {
					t.Fatalf("expected *AgentMessageThreadItem, got %T", w.Value)
				}
				if v.ID != "a1" || v.Text != "response" {
					t.Errorf("got ID=%q Text=%q", v.ID, v.Text)
				}
				if v.Phase == nil || *v.Phase != codex.MessagePhaseCommentary {
					t.Errorf("Phase = %v, want commentary", v.Phase)
				}
			},
		},
		{
			name:  "agentMessage with null phase",
			input: `{"type":"agentMessage","id":"a2","text":"hi","phase":null}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v := w.Value.(*codex.AgentMessageThreadItem)
				if v.Phase != nil {
					t.Errorf("Phase = %v, want nil", v.Phase)
				}
			},
		},
		{
			name:  "plan",
			input: `{"type":"plan","id":"p1","text":"step 1"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.PlanThreadItem)
				if !ok {
					t.Fatalf("expected *PlanThreadItem, got %T", w.Value)
				}
				if v.ID != "p1" || v.Text != "step 1" {
					t.Errorf("got %+v", v)
				}
			},
		},
		{
			name:  "reasoning",
			input: `{"type":"reasoning","id":"r1","content":["think"],"summary":["result"]}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.ReasoningThreadItem)
				if !ok {
					t.Fatalf("expected *ReasoningThreadItem, got %T", w.Value)
				}
				if v.ID != "r1" {
					t.Errorf("ID = %q", v.ID)
				}
				if len(v.Content) != 1 || v.Content[0] != "think" {
					t.Errorf("Content = %v", v.Content)
				}
				if len(v.Summary) != 1 || v.Summary[0] != "result" {
					t.Errorf("Summary = %v", v.Summary)
				}
			},
		},
		{
			name:  "reasoning minimal",
			input: `{"type":"reasoning","id":"r2"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v := w.Value.(*codex.ReasoningThreadItem)
				if v.ID != "r2" {
					t.Errorf("ID = %q", v.ID)
				}
				if v.Content != nil {
					t.Errorf("Content = %v, want nil", v.Content)
				}
			},
		},
		{
			name:  "commandExecution",
			input: `{"type":"commandExecution","id":"c1","command":"ls -la","commandActions":[{"type":"unknown","command":"ls -la"}],"cwd":"/tmp","status":"completed","exitCode":0,"durationMs":150}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.CommandExecutionThreadItem)
				if !ok {
					t.Fatalf("expected *CommandExecutionThreadItem, got %T", w.Value)
				}
				if v.Command != "ls -la" || v.Cwd != "/tmp" {
					t.Errorf("Command=%q Cwd=%q", v.Command, v.Cwd)
				}
				if v.Status != codex.CommandExecutionStatusCompleted {
					t.Errorf("Status = %q", v.Status)
				}
				if v.ExitCode == nil || *v.ExitCode != 0 {
					t.Errorf("ExitCode = %v", v.ExitCode)
				}
				if v.DurationMs == nil || *v.DurationMs != 150 {
					t.Errorf("DurationMs = %v", v.DurationMs)
				}
				if len(v.CommandActions) != 1 {
					t.Fatalf("len(CommandActions) = %d", len(v.CommandActions))
				}
			},
		},
		{
			name:  "fileChange",
			input: `{"type":"fileChange","id":"f1","changes":[{"path":"a.go","diff":"+line","kind":{"type":"add"}}],"status":"completed"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.FileChangeThreadItem)
				if !ok {
					t.Fatalf("expected *FileChangeThreadItem, got %T", w.Value)
				}
				if v.Status != codex.PatchApplyStatusCompleted {
					t.Errorf("Status = %q", v.Status)
				}
				if len(v.Changes) != 1 || v.Changes[0].Path != "a.go" {
					t.Errorf("Changes = %+v", v.Changes)
				}
			},
		},
		{
			name:  "mcpToolCall",
			input: `{"type":"mcpToolCall","id":"m1","server":"srv","tool":"read","status":"completed","arguments":{"path":"x"},"result":{"content":[]},"durationMs":42}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.McpToolCallThreadItem)
				if !ok {
					t.Fatalf("expected *McpToolCallThreadItem, got %T", w.Value)
				}
				if v.Server != "srv" || v.Tool != "read" {
					t.Errorf("Server=%q Tool=%q", v.Server, v.Tool)
				}
				if v.Result == nil {
					t.Error("Result is nil")
				}
				if v.DurationMs == nil || *v.DurationMs != 42 {
					t.Errorf("DurationMs = %v", v.DurationMs)
				}
			},
		},
		{
			name:  "mcpToolCall with error",
			input: `{"type":"mcpToolCall","id":"m2","server":"srv","tool":"write","status":"failed","arguments":null,"error":{"message":"boom"}}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v := w.Value.(*codex.McpToolCallThreadItem)
				if v.Error == nil || v.Error.Message != "boom" {
					t.Errorf("Error = %+v", v.Error)
				}
			},
		},
		{
			name:  "dynamicToolCall",
			input: `{"type":"dynamicToolCall","id":"d1","tool":"mytool","status":"completed","arguments":{"key":"val"},"contentItems":[{"type":"inputText","text":"output"}],"success":true,"durationMs":99}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.DynamicToolCallThreadItem)
				if !ok {
					t.Fatalf("expected *DynamicToolCallThreadItem, got %T", w.Value)
				}
				if v.Tool != "mytool" {
					t.Errorf("Tool = %q", v.Tool)
				}
				if v.Success == nil || !*v.Success {
					t.Errorf("Success = %v", v.Success)
				}
				if len(v.ContentItems) != 1 {
					t.Fatalf("len(ContentItems) = %d", len(v.ContentItems))
				}
			},
		},
		{
			name:  "collabAgentToolCall",
			input: `{"type":"collabAgentToolCall","id":"ca1","tool":"spawnAgent","status":"completed","agentsStates":{"agent-1":{"status":"running"}},"receiverThreadIds":["t1"],"senderThreadId":"t0","prompt":"do stuff"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.CollabAgentToolCallThreadItem)
				if !ok {
					t.Fatalf("expected *CollabAgentToolCallThreadItem, got %T", w.Value)
				}
				if v.Tool != codex.CollabAgentToolSpawnAgent {
					t.Errorf("Tool = %q", v.Tool)
				}
				if v.SenderThreadId != "t0" {
					t.Errorf("SenderThreadId = %q", v.SenderThreadId)
				}
				if len(v.ReceiverThreadIds) != 1 || v.ReceiverThreadIds[0] != "t1" {
					t.Errorf("ReceiverThreadIds = %v", v.ReceiverThreadIds)
				}
				if v.Prompt == nil || *v.Prompt != "do stuff" {
					t.Errorf("Prompt = %v", v.Prompt)
				}
				state, ok := v.AgentsStates["agent-1"]
				if !ok {
					t.Fatal("missing agent-1 state")
				}
				if state.Status != codex.CollabAgentStatusRunning {
					t.Errorf("agent-1 status = %q", state.Status)
				}
			},
		},
		{
			name:  "webSearch",
			input: `{"type":"webSearch","id":"w1","query":"golang generics","action":{"type":"search","query":"golang generics"}}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.WebSearchThreadItem)
				if !ok {
					t.Fatalf("expected *WebSearchThreadItem, got %T", w.Value)
				}
				if v.Query != "golang generics" {
					t.Errorf("Query = %q", v.Query)
				}
				if v.Action == nil || v.Action.Value == nil {
					t.Fatal("Action is nil")
				}
			},
		},
		{
			name:  "webSearch without action",
			input: `{"type":"webSearch","id":"w2","query":"test"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v := w.Value.(*codex.WebSearchThreadItem)
				if v.Action != nil {
					t.Errorf("Action = %+v, want nil", v.Action)
				}
			},
		},
		{
			name:  "imageView",
			input: `{"type":"imageView","id":"i1","path":"/img/screenshot.png"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.ImageViewThreadItem)
				if !ok {
					t.Fatalf("expected *ImageViewThreadItem, got %T", w.Value)
				}
				if v.Path != "/img/screenshot.png" {
					t.Errorf("Path = %q", v.Path)
				}
			},
		},
		{
			name:  "enteredReviewMode",
			input: `{"type":"enteredReviewMode","id":"e1","review":"pr-123"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.EnteredReviewModeThreadItem)
				if !ok {
					t.Fatalf("expected *EnteredReviewModeThreadItem, got %T", w.Value)
				}
				if v.Review != "pr-123" {
					t.Errorf("Review = %q", v.Review)
				}
			},
		},
		{
			name:  "exitedReviewMode",
			input: `{"type":"exitedReviewMode","id":"x1","review":"pr-123"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.ExitedReviewModeThreadItem)
				if !ok {
					t.Fatalf("expected *ExitedReviewModeThreadItem, got %T", w.Value)
				}
				if v.Review != "pr-123" {
					t.Errorf("Review = %q", v.Review)
				}
			},
		},
		{
			name:  "contextCompaction",
			input: `{"type":"contextCompaction","id":"cc1"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.ContextCompactionThreadItem)
				if !ok {
					t.Fatalf("expected *ContextCompactionThreadItem, got %T", w.Value)
				}
				if v.ID != "cc1" {
					t.Errorf("ID = %q", v.ID)
				}
			},
		},
		{
			name:  "unknown type forward compatibility",
			input: `{"type":"futureType","id":"ft1","extra":"data"}`,
			checkFn: func(t *testing.T, w codex.ThreadItemWrapper) {
				v, ok := w.Value.(*codex.UnknownThreadItem)
				if !ok {
					t.Fatalf("expected *UnknownThreadItem, got %T", w.Value)
				}
				if v.Type != "futureType" {
					t.Errorf("Type = %q", v.Type)
				}
				if v.Raw == nil {
					t.Error("Raw is nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Unmarshal
			var w codex.ThreadItemWrapper
			if err := json.Unmarshal([]byte(tt.input), &w); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			tt.checkFn(t, w)

			// Round-trip: marshal then unmarshal again
			marshaled, err := json.Marshal(w)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var w2 codex.ThreadItemWrapper
			if err := json.Unmarshal(marshaled, &w2); err != nil {
				t.Fatalf("re-Unmarshal failed: %v", err)
			}
			tt.checkFn(t, w2)
		})
	}
}

func TestThreadItemUnmarshalInvalidJSON(t *testing.T) {
	var w codex.ThreadItemWrapper
	if err := json.Unmarshal([]byte(`not json`), &w); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestUnknownThreadItemMarshalNilRaw(t *testing.T) {
	u := &codex.UnknownThreadItem{Type: "x", Raw: nil}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "null" {
		t.Errorf("got %s, want null", b)
	}
}

func TestUserMessageThreadItemEmptyContent(t *testing.T) {
	// Marshal with empty content slice
	item := &codex.UserMessageThreadItem{ID: "u1", Content: nil}
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	// Should round-trip
	var w codex.ThreadItemWrapper
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	v := w.Value.(*codex.UserMessageThreadItem)
	if v.ID != "u1" {
		t.Errorf("ID = %q", v.ID)
	}
}
