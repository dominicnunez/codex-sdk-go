package codex_test

import (
	"encoding/json"
	"fmt"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestByteRange tests ByteRange JSON serialization
func TestByteRange(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.ByteRange
	}{
		{
			name: "basic range",
			data: `{"start":0,"end":10}`,
			want: codex.ByteRange{Start: 0, End: 10},
		},
		{
			name: "large range",
			data: `{"start":1000,"end":2000}`,
			want: codex.ByteRange{Start: 1000, End: 2000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.ByteRange
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}

			// Test marshal round-trip
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}
			var roundtrip codex.ByteRange
			if err := json.Unmarshal(data, &roundtrip); err != nil {
				t.Fatalf("roundtrip unmarshal error: %v", err)
			}
			if roundtrip != tt.want {
				t.Errorf("roundtrip got %+v, want %+v", roundtrip, tt.want)
			}
		})
	}
}

// TestTextElement tests TextElement JSON serialization
func TestTextElement(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.TextElement
	}{
		{
			name: "with placeholder",
			data: `{"byteRange":{"start":5,"end":15},"placeholder":"variable"}`,
			want: codex.TextElement{
				ByteRange:   codex.ByteRange{Start: 5, End: 15},
				Placeholder: ptr("variable"),
			},
		},
		{
			name: "without placeholder",
			data: `{"byteRange":{"start":0,"end":100}}`,
			want: codex.TextElement{
				ByteRange:   codex.ByteRange{Start: 0, End: 100},
				Placeholder: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.TextElement
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got.ByteRange != tt.want.ByteRange {
				t.Errorf("byteRange: got %+v, want %+v", got.ByteRange, tt.want.ByteRange)
			}
			if (got.Placeholder == nil) != (tt.want.Placeholder == nil) {
				t.Errorf("placeholder nil mismatch: got %v, want %v", got.Placeholder, tt.want.Placeholder)
			}
			if got.Placeholder != nil && tt.want.Placeholder != nil && *got.Placeholder != *tt.want.Placeholder {
				t.Errorf("placeholder: got %q, want %q", *got.Placeholder, *tt.want.Placeholder)
			}
		})
	}
}

// TestMessagePhase tests MessagePhase enum serialization
func TestMessagePhase(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.MessagePhase
	}{
		{
			name: "commentary",
			data: `"commentary"`,
			want: codex.MessagePhaseCommentary,
		},
		{
			name: "final_answer",
			data: `"final_answer"`,
			want: codex.MessagePhaseFinalAnswer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.MessagePhase
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}

			// Test marshal
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}
			if string(data) != tt.data {
				t.Errorf("marshal: got %s, want %s", data, tt.data)
			}
		})
	}
}

// TestCommandExecutionStatus tests CommandExecutionStatus enum
func TestCommandExecutionStatus(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.CommandExecutionStatus
	}{
		{"inProgress", `"inProgress"`, codex.CommandExecutionStatusInProgress},
		{"completed", `"completed"`, codex.CommandExecutionStatusCompleted},
		{"failed", `"failed"`, codex.CommandExecutionStatusFailed},
		{"declined", `"declined"`, codex.CommandExecutionStatusDeclined},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.CommandExecutionStatus
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPatchApplyStatus tests PatchApplyStatus enum
func TestPatchApplyStatus(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.PatchApplyStatus
	}{
		{"inProgress", `"inProgress"`, codex.PatchApplyStatusInProgress},
		{"completed", `"completed"`, codex.PatchApplyStatusCompleted},
		{"failed", `"failed"`, codex.PatchApplyStatusFailed},
		{"declined", `"declined"`, codex.PatchApplyStatusDeclined},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.PatchApplyStatus
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMcpToolCallStatus tests McpToolCallStatus enum
func TestMcpToolCallStatus(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.McpToolCallStatus
	}{
		{"inProgress", `"inProgress"`, codex.McpToolCallStatusInProgress},
		{"completed", `"completed"`, codex.McpToolCallStatusCompleted},
		{"failed", `"failed"`, codex.McpToolCallStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.McpToolCallStatus
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDynamicToolCallStatus tests DynamicToolCallStatus enum
func TestDynamicToolCallStatus(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.DynamicToolCallStatus
	}{
		{"inProgress", `"inProgress"`, codex.DynamicToolCallStatusInProgress},
		{"completed", `"completed"`, codex.DynamicToolCallStatusCompleted},
		{"failed", `"failed"`, codex.DynamicToolCallStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.DynamicToolCallStatus
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFileUpdateChange tests FileUpdateChange structure including Kind discriminator
func TestFileUpdateChange(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		wantPath string
		wantDiff string
		wantKind interface{} // expected concrete type for Kind.Value
	}{
		{
			name:     "add file",
			data:     `{"path":"/path/to/file","diff":"+ new content","kind":{"type":"add"}}`,
			wantPath: "/path/to/file",
			wantDiff: "+ new content",
			wantKind: &codex.AddPatchChangeKind{},
		},
		{
			name:     "update file",
			data:     `{"path":"/path/to/file","diff":"@@ -1 +1 @@","kind":{"type":"update","move_path":"/new/path"}}`,
			wantPath: "/path/to/file",
			wantDiff: "@@ -1 +1 @@",
			wantKind: &codex.UpdatePatchChangeKind{MovePath: ptr("/new/path")},
		},
		{
			name:     "delete file",
			data:     `{"path":"/path/to/file","diff":"- old content","kind":{"type":"delete"}}`,
			wantPath: "/path/to/file",
			wantDiff: "- old content",
			wantKind: &codex.DeletePatchChangeKind{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.FileUpdateChange
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got.Path != tt.wantPath {
				t.Errorf("path: got %q, want %q", got.Path, tt.wantPath)
			}
			if got.Diff != tt.wantDiff {
				t.Errorf("diff: got %q, want %q", got.Diff, tt.wantDiff)
			}
			if got.Kind.Value == nil {
				t.Fatal("kind: got nil, want non-nil")
			}
			gotType := fmt.Sprintf("%T", got.Kind.Value)
			wantType := fmt.Sprintf("%T", tt.wantKind)
			if gotType != wantType {
				t.Errorf("kind type: got %s, want %s", gotType, wantType)
			}
		})
	}
}

// TestMcpToolCallResult tests McpToolCallResult structure
func TestMcpToolCallResult(t *testing.T) {
	data := `{"content":[{"type":"text","text":"result"}],"structuredContent":{"key":"value"}}`
	var got codex.McpToolCallResult
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(got.Content) != 1 {
		t.Errorf("content length: got %d, want 1", len(got.Content))
	}
}

// TestMcpToolCallError tests McpToolCallError structure
func TestMcpToolCallError(t *testing.T) {
	data := `{"message":"tool call failed"}`
	var got codex.McpToolCallError
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got.Message != "tool call failed" {
		t.Errorf("message: got %q, want %q", got.Message, "tool call failed")
	}
}

// TestCollabAgentStatus tests CollabAgentStatus enum
func TestCollabAgentStatus(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.CollabAgentStatus
	}{
		{"pendingInit", `"pendingInit"`, codex.CollabAgentStatusPendingInit},
		{"running", `"running"`, codex.CollabAgentStatusRunning},
		{"completed", `"completed"`, codex.CollabAgentStatusCompleted},
		{"errored", `"errored"`, codex.CollabAgentStatusErrored},
		{"shutdown", `"shutdown"`, codex.CollabAgentStatusShutdown},
		{"notFound", `"notFound"`, codex.CollabAgentStatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.CollabAgentStatus
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCollabAgentTool tests CollabAgentTool enum
func TestCollabAgentTool(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.CollabAgentTool
	}{
		{"spawnAgent", `"spawnAgent"`, codex.CollabAgentToolSpawnAgent},
		{"sendInput", `"sendInput"`, codex.CollabAgentToolSendInput},
		{"resumeAgent", `"resumeAgent"`, codex.CollabAgentToolResumeAgent},
		{"wait", `"wait"`, codex.CollabAgentToolWait},
		{"closeAgent", `"closeAgent"`, codex.CollabAgentToolCloseAgent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.CollabAgentTool
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCollabAgentToolCallStatus tests CollabAgentToolCallStatus enum
func TestCollabAgentToolCallStatus(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.CollabAgentToolCallStatus
	}{
		{"inProgress", `"inProgress"`, codex.CollabAgentToolCallStatusInProgress},
		{"completed", `"completed"`, codex.CollabAgentToolCallStatusCompleted},
		{"failed", `"failed"`, codex.CollabAgentToolCallStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.CollabAgentToolCallStatus
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCollabAgentState tests CollabAgentState structure
func TestCollabAgentState(t *testing.T) {
	tests := []struct {
		name string
		data string
		want codex.CollabAgentState
	}{
		{
			name: "with message",
			data: `{"status":"running","message":"processing request"}`,
			want: codex.CollabAgentState{
				Status:  codex.CollabAgentStatusRunning,
				Message: ptr("processing request"),
			},
		},
		{
			name: "without message",
			data: `{"status":"completed"}`,
			want: codex.CollabAgentState{
				Status:  codex.CollabAgentStatusCompleted,
				Message: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.CollabAgentState
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if got.Status != tt.want.Status {
				t.Errorf("status: got %q, want %q", got.Status, tt.want.Status)
			}
			if (got.Message == nil) != (tt.want.Message == nil) {
				t.Errorf("message nil mismatch")
			}
			if got.Message != nil && tt.want.Message != nil && *got.Message != *tt.want.Message {
				t.Errorf("message: got %q, want %q", *got.Message, *tt.want.Message)
			}
		})
	}
}

// TestWebSearchAction tests WebSearchAction discriminated union
func TestWebSearchAction(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "search action",
			data: `{"type":"search","query":"test query"}`,
		},
		{
			name: "open page action",
			data: `{"type":"openPage","url":"https://example.com"}`,
		},
		{
			name: "find in page action",
			data: `{"type":"findInPage","url":"https://example.com","pattern":"test"}`,
		},
		{
			name: "other action",
			data: `{"type":"other"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wrapper codex.WebSearchActionWrapper
			if err := json.Unmarshal([]byte(tt.data), &wrapper); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if wrapper.Value == nil {
				t.Fatal("wrapper.Value is nil")
			}

			// Test marshal round-trip
			data, err := json.Marshal(wrapper)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}
			var roundtrip codex.WebSearchActionWrapper
			if err := json.Unmarshal(data, &roundtrip); err != nil {
				t.Fatalf("roundtrip unmarshal error: %v", err)
			}
		})
	}
}
