package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestMcpAppUIValidation(t *testing.T) {
	for _, tc := range []struct {
		metadata string
		valid    bool
	}{
		{`{"preferredModelDisplayMode":"inline","resourceUri":"ui://app"}`, true},
		{`{"preferredModelDisplayMode":"fullscreen","resourceUri":""}`, true},
		{`null`, true},
		{`{}`, false},
		{`{"preferredModelDisplayMode":"inline"}`, false},
		{`{"resourceUri":"ui://app"}`, false},
		{`{"preferredModelDisplayMode":"unknown","resourceUri":"ui://app"}`, false},
		{`{"preferredModelDisplayMode":null,"resourceUri":"ui://app"}`, false},
		{`{"preferredModelDisplayMode":"inline","resourceUri":null}`, false},
	} {
		t.Run(tc.metadata, func(t *testing.T) {
			var item codex.ThreadItemWrapper
			payload := `{"type":"mcpToolCall","id":"m","server":"s","tool":"t","status":"completed","arguments":{},"mcpAppUi":` + tc.metadata + `}`
			err := json.Unmarshal([]byte(payload), &item)
			if (err == nil) != tc.valid {
				t.Fatalf("decode valid = %v, want %v: %v", err == nil, tc.valid, err)
			}
		})
	}
}

func TestModelAccessProgramsValidation(t *testing.T) {
	for _, tc := range []struct {
		metadata string
		valid    bool
	}{
		{`{"cyber":[]}`, true},
		{`{"cyber":["standard","daybreakBlue","daybreakRed"]}`, true},
		{`null`, true},
		{`{}`, false},
		{`{"cyber":null}`, false},
		{`{"cyber":["unknown"]}`, false},
		{`{"cyber":[null]}`, false},
	} {
		t.Run(tc.metadata, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			t.Cleanup(func() { _ = client.Close() })
			payload := `{"data":[{"id":"m","model":"m","displayName":"Model","description":"Test","hidden":false,"isDefault":true,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[],"availableAccessPrograms":` + tc.metadata + `}]}`
			transport.SetResponse("model/list", codex.Response{Result: json.RawMessage(payload)})
			_, err := client.Model.List(context.Background(), codex.ModelListParams{})
			if (err == nil) != tc.valid {
				t.Fatalf("decode valid = %v, want %v: %v", err == nil, tc.valid, err)
			}
		})
	}
}
