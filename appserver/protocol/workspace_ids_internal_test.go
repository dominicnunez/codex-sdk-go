package protocol

import (
	"encoding/json"
	"testing"
)

func TestConfigWorkspaceOwnedValues(t *testing.T) {
	raw := []byte(`{"forced_chatgpt_workspace_id":["a","b"]}`)
	var config Config
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	for i := range raw {
		raw[i] = '!'
	}
	if (*config.ForcedChatgptWorkspaceIDs)[0] != "a" {
		t.Fatal("decoded workspace IDs alias input storage")
	}
	snapshot := cloneArbitraryValue(config)
	(*config.ForcedChatgptWorkspaceIDs)[0] = "source"
	if (*snapshot.ForcedChatgptWorkspaceIDs)[0] != "a" {
		t.Fatal("copied restriction aliases source")
	}
	(*snapshot.ForcedChatgptWorkspaceIDs)[1] = "snapshot"
	if (*config.ForcedChatgptWorkspaceIDs)[1] != "b" {
		t.Fatal("source restriction aliases copy")
	}
	var nilList []string
	source := Config{ForcedChatgptWorkspaceIDs: &nilList}
	snapshot = cloneArbitraryValue(source)
	if snapshot.ForcedChatgptWorkspaceIDs == nil || *snapshot.ForcedChatgptWorkspaceIDs != nil {
		t.Fatal("copy changed constructed array presence or nil storage")
	}
	*snapshot.ForcedChatgptWorkspaceIDs = append(*snapshot.ForcedChatgptWorkspaceIDs, "snapshot")
	if nilList != nil {
		t.Fatal("copy shares constructed slice pointer")
	}
}
