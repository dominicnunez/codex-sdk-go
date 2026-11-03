package protocol_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func FuzzRequiredNullDuplicateHistory(f *testing.F) {
	for i := uint8(0); i < 28; i++ {
		f.Add(i, uint8(1), i%2 == 0)
	}
	f.Fuzz(func(t *testing.T, index, history uint8, escaped bool) {
		records := requiredNullRecords()
		var choices []struct {
			record requiredNullRecord
			field  string
		}
		for _, record := range records {
			for _, field := range record.fields {
				choices = append(choices, struct {
					record requiredNullRecord
					field  string
				}{record, field})
			}
		}
		choice := choices[int(index)%len(choices)]
		body := choice.record.body
		prefix := strings.Repeat(strings.TrimSuffix(body[1:], "}")+",", int(history%6))
		invalid := "{" + prefix + hiddenNullBody(body, choice.field, escaped)[1:]
		if err := json.Unmarshal([]byte(invalid), choice.record.newValue()); !errors.Is(err, p.ErrNullResultField) {
			t.Fatalf("valid history hid required null: %v", err)
		}
		valid := "{" + prefix + body[1:]
		if err := json.Unmarshal([]byte(valid), choice.record.newValue()); err != nil {
			t.Fatalf("valid history rejected: %v", err)
		}
	})
}
