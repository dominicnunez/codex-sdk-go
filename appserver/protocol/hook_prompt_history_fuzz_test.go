package protocol_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// Pointer presence in a method-free native model is an independent oracle for
// which required strings survive slice reuse, shrink/regrow and empty resets.
func FuzzHookPromptArrayHistory(f *testing.F) {
	for _, seed := range [][]byte{{}, {3, 1, 3}, {3, 0, 1}, {15, 5, 13}, {255, 0, 255}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, history []byte) {
		if len(history) > 16 {
			history = history[:16]
		}
		members := []string{`"fragments":[{"hookRunId":"initial","text":"initial"}]`}
		for step, op := range history {
			entries := make([]map[string]string, int(op%4))
			for index := range entries {
				entry := map[string]string{}
				if op&4 != 0 {
					entry["hookRunId"] = fmt.Sprintf("run-%d-%d", step, index)
				}
				if op&8 != 0 {
					entry["text"] = fmt.Sprintf("text-%d-%d", step, index)
				}
				entries[index] = entry
			}
			raw, err := json.Marshal(entries)
			if err != nil {
				t.Fatal(err)
			}
			members = append(members, `"fragments":`+string(raw))
		}
		raw := `{"type":"hookPrompt","id":"i",` + strings.Join(members, ",") + `}`
		var reference struct {
			Fragments []struct {
				HookRunID *string `json:"hookRunId"`
				Text      *string `json:"text"`
			} `json:"fragments"`
		}
		if err := json.Unmarshal([]byte(raw), &reference); err != nil {
			t.Fatal(err)
		}
		valid := true
		for _, entry := range reference.Fragments {
			valid = valid && entry.HookRunID != nil && entry.Text != nil
		}
		var wrapper p.ThreadItemWrapper
		err := json.Unmarshal([]byte(raw), &wrapper)
		if !valid {
			if err == nil {
				t.Fatal("missing native fragment presence admitted")
			}
			return
		}
		if err != nil {
			t.Fatalf("valid native history rejected: %v input=%s", err, raw)
		}
		item, ok := wrapper.Value.(*p.HookPromptThreadItem)
		if !ok || len(item.Fragments) != len(reference.Fragments) {
			t.Fatal("typed history shape changed")
		}
		for i, entry := range reference.Fragments {
			if item.Fragments[i].HookRunID != *entry.HookRunID || item.Fragments[i].Text != *entry.Text {
				t.Fatal("native history values changed")
			}
		}
		// A forbidden earlier null remains invalid even after a complete final
		// array supplies valid values, independently of the model's final state.
		bad := `{"type":"hookPrompt","id":"i","fragments":[{"hookRunId":"r","text":null}],"fragments":[{"hookRunId":"r","text":"valid"}]}`
		if err := json.Unmarshal([]byte(bad), &wrapper); err == nil {
			t.Fatal("later array erased earlier forbidden null")
		}
	})
}
