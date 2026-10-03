package protocol_test

import (
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestMcpResultMetadataWire(t *testing.T) {
	for _, meta := range []string{`{"ui":{"resourceUri":"ui://audit/widget"}}`, `[]`, `"text"`, `9007199254740993`, `true`, `null`} {
		t.Run(meta, func(t *testing.T) {
			input := `{"content":[],"_meta":` + meta + `}`
			var result codex.McpToolCallResult
			if err := json.Unmarshal([]byte(input), &result); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["_meta"]) != meta {
				t.Fatalf("metadata = %s, want %s", fields["_meta"], meta)
			}
		})
	}
}
