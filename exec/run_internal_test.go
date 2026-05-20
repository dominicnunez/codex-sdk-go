package exec

import "testing"

func TestBuildTurnParamsIncludesOutputSchema(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"decision": map[string]interface{}{"type": "string"},
		},
		"required": []string{"decision"},
	}

	params := buildTurnParams(RunOptions{Prompt: "decide", OutputSchema: schema}, "thread-1")
	if params.OutputSchema == nil {
		t.Fatal("OutputSchema is nil")
	}
}

func TestConversationBuildTurnParamsIncludesOutputSchema(t *testing.T) {
	conv := &Conversation{threadID: "thread-1"}
	schema := map[string]interface{}{"type": "object"}

	params := conv.buildTurnParams(TurnOptions{Prompt: "decide", OutputSchema: schema})
	if params.OutputSchema == nil {
		t.Fatal("OutputSchema is nil")
	}
}
