package protocol_test

import (
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestThreadRequiredIdentityFields(t *testing.T) {
	for _, field := range []string{"projectId", "sessionId"} {
		t.Run("missing_"+field, func(t *testing.T) {
			payload := validProcessThreadPayload("thread-1")
			delete(payload, field)
			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			var thread codex.Thread
			if err := json.Unmarshal(data, &thread); err == nil {
				t.Fatalf("accepted missing %s", field)
			}
		})
	}
	for _, session := range []interface{}{nil, "", "session-1"} {
		payload := validProcessThreadPayload("thread-1")
		payload["sessionId"] = session
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		var thread codex.Thread
		err = json.Unmarshal(data, &thread)
		if session == nil {
			if err == nil {
				t.Fatal("accepted null sessionId")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(thread)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		if string(fields["projectId"]) != "null" {
			t.Fatalf("required nullable projectId lost: %s", encoded)
		}
	}
}

func TestMcpServerInfoRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		info  string
		valid bool
	}{
		{`{}`, false},
		{`{"name":"server"}`, false},
		{`{"version":"1"}`, false},
		{`{"name":null,"version":"1"}`, false},
		{`{"name":"server","version":null}`, false},
		{`{"name":"","version":""}`, true},
		{`{"name":"server","version":"1"}`, true},
		{`null`, true},
	} {
		t.Run(tc.info, func(t *testing.T) {
			data := []byte(`{"name":"server","authStatus":"unsupported","resourceTemplates":[],"resources":[],"tools":{},"serverInfo":` + tc.info + `}`)
			var status codex.McpServerStatus
			err := json.Unmarshal(data, &status)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}

func TestThreadSectionRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		section string
		valid   bool
	}{
		{`{}`, false},
		{`{"id":"section"}`, false},
		{`{"name":"name"}`, false},
		{`{"id":null,"name":"name"}`, false},
		{`{"id":"section","name":null}`, false},
		{`{"id":"section","name":"name"}`, true},
		{`null`, true},
	} {
		t.Run(tc.section, func(t *testing.T) {
			payload := validProcessThreadPayload("thread-1")
			payload["section"] = json.RawMessage(tc.section)
			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			var thread codex.Thread
			err = json.Unmarshal(data, &thread)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
