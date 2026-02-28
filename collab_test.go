package codex_test

import (
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestCollaborationModeJSON(t *testing.T) {
	effort := codex.ReasoningEffortHigh
	mode := codex.CollaborationMode{
		Mode: codex.ModeKindPlan,
		Settings: codex.CollaborationModeSettings{
			Model:                 "o3",
			DeveloperInstructions: codex.Ptr("Be concise"),
			ReasoningEffort:       &effort,
		},
	}

	data, err := json.Marshal(mode)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got codex.CollaborationMode
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Mode != codex.ModeKindPlan {
		t.Errorf("Mode = %q, want %q", got.Mode, codex.ModeKindPlan)
	}
	if got.Settings.Model != "o3" {
		t.Errorf("Settings.Model = %q, want %q", got.Settings.Model, "o3")
	}
	if got.Settings.DeveloperInstructions == nil || *got.Settings.DeveloperInstructions != "Be concise" {
		t.Errorf("Settings.DeveloperInstructions = %v, want 'Be concise'", got.Settings.DeveloperInstructions)
	}
	if got.Settings.ReasoningEffort == nil || *got.Settings.ReasoningEffort != codex.ReasoningEffortHigh {
		t.Errorf("Settings.ReasoningEffort = %v, want %q", got.Settings.ReasoningEffort, codex.ReasoningEffortHigh)
	}
}

func TestCollaborationModeMinimal(t *testing.T) {
	mode := codex.CollaborationMode{
		Mode:     codex.ModeKindDefault,
		Settings: codex.CollaborationModeSettings{Model: "gpt-4"},
	}

	data, err := json.Marshal(mode)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	settings := raw["settings"].(map[string]interface{})
	if _, ok := settings["developer_instructions"]; ok {
		t.Error("omitempty developer_instructions should be absent")
	}
	if _, ok := settings["reasoning_effort"]; ok {
		t.Error("omitempty reasoning_effort should be absent")
	}
}

func TestTurnStartParamsWithCollaborationMode(t *testing.T) {
	mode := codex.CollaborationMode{
		Mode:     codex.ModeKindPlan,
		Settings: codex.CollaborationModeSettings{Model: "o3"},
	}

	params := codex.TurnStartParams{
		ThreadID:          "thread-1",
		Input:             []codex.UserInput{&codex.TextUserInput{Text: "hello"}},
		CollaborationMode: &mode,
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	if _, ok := raw["collaborationMode"]; !ok {
		t.Fatal("collaborationMode field missing from marshaled TurnStartParams")
	}
}

func TestThreadItemWrapperCollabHelpers(t *testing.T) {
	collab := &codex.CollabAgentToolCallThreadItem{
		ID:             "tc-1",
		Tool:           codex.CollabAgentToolSpawnAgent,
		Status:         codex.CollabAgentToolCallStatusInProgress,
		AgentsStates:   map[string]codex.CollabAgentState{},
		SenderThreadId: "parent",
	}

	wrapper := codex.ThreadItemWrapper{Value: collab}

	if !wrapper.IsCollabToolCall() {
		t.Error("IsCollabToolCall() = false, want true")
	}
	if wrapper.CollabToolCall() != collab {
		t.Error("CollabToolCall() did not return the expected item")
	}

	// Non-collab item.
	agentMsg := codex.ThreadItemWrapper{
		Value: &codex.AgentMessageThreadItem{ID: "msg-1", Text: "hi"},
	}
	if agentMsg.IsCollabToolCall() {
		t.Error("IsCollabToolCall() = true for AgentMessageThreadItem")
	}
	if agentMsg.CollabToolCall() != nil {
		t.Error("CollabToolCall() should return nil for non-collab item")
	}
}
