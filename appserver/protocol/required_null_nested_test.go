package protocol_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestNestedRequiredNullOccurrences(t *testing.T) {
	for _, owner := range []struct {
		name, final string
		fields      []string
	}{
		{"granular", `{"mcp_elicitations":false,"rules":true,"sandbox_approval":false}`, []string{"mcp_elicitations", "rules", "sandbox_approval"}},
		{"thread_spawn", `{"depth":0,"parent_thread_id":""}`, []string{"depth", "parent_thread_id"}},
	} {
		for _, field := range owner.fields {
			for _, escaped := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/escaped=%t", owner.name, field, escaped), func(t *testing.T) {
					key := field
					outer := owner.name
					if escaped {
						key = fmt.Sprintf(`\u%04x%s`, field[0], field[1:])
						outer = fmt.Sprintf(`\u%04x%s`, owner.name[0], owner.name[1:])
					}
					prefix := `{"` + outer + `":{"` + key + `":null},"` + owner.name + `":` + owner.final + `}`
					valid := `{"` + outer + `":{"` + field + `":` + map[string]string{"mcp_elicitations": "true", "rules": "false", "sandbox_approval": "true", "depth": "2", "parent_thread_id": `"prior"`}[field] + `},"` + owner.name + `":` + owner.final + `}`
					if owner.name == "granular" {
						var policy p.AskForApprovalWrapper
						if err := json.Unmarshal([]byte(valid), &policy); err != nil {
							t.Fatalf("valid partial duplicate: %v", err)
						}
						if err := json.Unmarshal([]byte(prefix), &policy); !errors.Is(err, p.ErrNullResultField) {
							t.Errorf("nested null hidden: %v", err)
						}
						if err := closedApprovalRead(t, prefix); !errors.Is(err, p.ErrNullResultField) {
							t.Errorf("nested null published: %v", err)
						}
					} else {
						var source p.SessionSourceWrapper
						valid = `{"subAgent":` + valid + `}`
						prefix = `{"subAgent":` + prefix + `}`
						if err := json.Unmarshal([]byte(valid), &source); err != nil {
							t.Fatalf("valid partial duplicate: %v", err)
						}
						if err := json.Unmarshal([]byte(prefix), &source); !errors.Is(err, p.ErrNullResultField) {
							t.Errorf("nested null hidden: %v", err)
						}
						if err := closedSourceRead(t, prefix); !errors.Is(err, p.ErrNullResultField) {
							t.Errorf("nested null published: %v", err)
						}
					}
				})
			}
		}
	}
}

func TestRequiredNullAcrossSubAgentDuplicates(t *testing.T) {
	for _, field := range []string{"depth", "parent_thread_id"} {
		t.Run(field, func(t *testing.T) {
			bad := `{"sub\u0041gent":{"thread_\u0073pawn":{"` + field + `":null}},"subAgent":{"thread_spawn":{"depth":0,"parent_thread_id":""}}}`
			var source p.SessionSourceWrapper
			if err := json.Unmarshal([]byte(bad), &source); !errors.Is(err, p.ErrNullResultField) {
				t.Fatalf("outer duplicate hid required nested null: %v", err)
			}
			if err := closedSourceRead(t, bad); !errors.Is(err, p.ErrNullResultField) {
				t.Fatalf("outer duplicate published required nested null: %v", err)
			}
		})
	}
	for _, valid := range []string{
		`{"subAgent":{"thread_spawn":{"depth":1}},"subAgent":{"thread_spawn":{"depth":0,"parent_thread_id":""}}}`,
		`{"subAgent":{"future":{"depth":null}},"subAgent":"review"}`,
	} {
		var source p.SessionSourceWrapper
		if err := json.Unmarshal([]byte(valid), &source); err != nil {
			t.Fatalf("valid partial or opaque earlier object rejected: %v", err)
		}
	}
}
