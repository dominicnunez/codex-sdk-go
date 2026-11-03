package protocol_test

import (
	"encoding/json"
	"errors"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestSkillRequiredNullOccurrences(t *testing.T) {
	base := `{"description":"description","enabled":true,"name":"skill"}`
	for _, field := range []string{"description", "enabled", "name"} {
		for _, escaped := range []bool{false, true} {
			key := `"` + field + `"`
			if escaped {
				key = `"\u` + map[byte]string{'d': "0064", 'e': "0065", 'n': "006e"}[field[0]] + field[1:] + `"`
			}
			t.Run(field+"/"+key, func(t *testing.T) {
				raw := `{` + key + `:null,` + base[1:]
				prior := p.SkillSummary{Description: "prior", Name: "prior", Path: p.Ptr("/prior")}
				value := prior
				err := json.Unmarshal([]byte(raw), &value)
				if !errors.Is(err, p.ErrNullResultField) {
					t.Fatalf("earlier required null erased by valid duplicate: %v", err)
				}
				if value.Name != prior.Name || value.Path != prior.Path {
					t.Fatal("new rejection changed prior receiver references")
				}
			})
		}
	}
	for _, raw := range []string{
		base,
		`{"description":"first","enabled":false,"name":"first",` + base[1:],
		`{"ENABLED":null,"path":null,` + base[1:],
	} {
		var value p.SkillSummary
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			t.Fatalf("valid duplicate or open extra rejected: %v", err)
		}
	}
}
