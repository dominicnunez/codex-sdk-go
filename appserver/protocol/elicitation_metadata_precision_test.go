package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestElicitationMetadataPrecision(t *testing.T) {
	for _, mode := range elicitationMetadataModes {
		for _, tc := range []struct{ name, raw string }{
			{"small-object", `{"n":42}`},
			{"nested-large", `{"ids":[9007199254740993,18446744073709551615]}`},
			{"large-scalar", "9007199254740993"},
			{"extreme-exponent", "1e400"},
			{"signed-zero", "-0"},
			{"decimal", "0.12345678901234567890123456789"},
			{"nested-exponents", `{"a":[1e-400,-1e400,{"b":2.000000000000000001}]}`},
			{"null", "null"},
			{"empty-array", "[]"},
			{"numeric-string", `"9007199254740993"`},
			{"explicit-response", "null"},
		} {
			t.Run(string(mode)+"/"+tc.name, func(t *testing.T) {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				defer client.Close()
				calls := 0
				var observed []byte
				schema := json.RawMessage(`{"type":"object","properties":{}}`)
				if mode == p.McpServerElicitationModeOpenAIForm || mode == p.McpServerElicitationModeOpenAIFormLegacy {
					schema = json.RawMessage(`{"const":9007199254740993}`)
				}
				client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(_ context.Context, params p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
					calls++
					var e error
					observed, e = json.Marshal(params.Meta)
					if e != nil {
						t.Fatal(e)
					}
					if mode == p.McpServerElicitationModeOpenAIForm || mode == p.McpServerElicitationModeOpenAIFormLegacy {
						if string(params.OpenAIRequestedSchema) != string(schema) {
							t.Fatal("raw OpenAI schema control changed")
						}
					}
					meta := params.Meta
					if tc.name == "explicit-response" {
						meta = map[string]any{"n": json.Number("9007199254740993")}
					}
					return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel, Meta: meta}, nil
				}})
				body, e := json.Marshal(map[string]any{"serverName": "s", "threadId": "t", "message": "fixture", "mode": mode, "requestedSchema": schema, "url": "https://example.test", "elicitationId": "e", "_meta": json.RawMessage(tc.raw)})
				if e != nil {
					t.Fatal(e)
				}
				response, e := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "e"}, Method: "mcpServer/elicitation/request", Params: body})
				if e != nil || calls != 1 || response.Error != nil || len(response.Result) == 0 {
					t.Fatalf("valid request rejected: %v", e)
				}
				if string(observed) != tc.raw {
					t.Errorf("handler metadata changed: got=%s want=%s", observed, tc.raw)
				}
				var fields map[string]json.RawMessage
				if e := json.Unmarshal(response.Result, &fields); e != nil {
					t.Fatal(e)
				}
				expected := tc.raw
				if tc.name == "explicit-response" {
					expected = `{"n":9007199254740993}`
				}
				if expected != "null" && string(fields["_meta"]) != expected {
					t.Errorf("returned metadata changed: got=%s want=%s", fields["_meta"], expected)
				}
			})
		}
	}
}
