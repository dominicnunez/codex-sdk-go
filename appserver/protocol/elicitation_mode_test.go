package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func elicitationModeBody(members string) string {
	return `{"serverName":"server","threadId":"thread","message":"input","requestedSchema":{"type":"object","properties":{}},"elicitationId":"e","url":"https://example.test",` + members + `}`
}

func TestElicitationHiddenMode(t *testing.T) {
	for _, mode := range []p.McpServerElicitationMode{
		p.McpServerElicitationModeForm, p.McpServerElicitationModeURL,
		p.McpServerElicitationModeOpenAIForm, p.McpServerElicitationModeOpenAIFormLegacy,
	} {
		t.Run(string(mode), func(t *testing.T) {
			raw := elicitationModeBody(fmt.Sprintf(`"mode":"future","mode":%q`, mode))
			var params p.McpServerElicitationRequestParams
			if err := json.Unmarshal([]byte(raw), &params); err == nil {
				t.Error("direct decode accepted hidden invalid mode")
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			calls := 0
			client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(context.Context, p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
				calls++
				return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel}, nil
			}})
			_, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "request"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(raw)})
			if err == nil || calls != 0 {
				t.Fatalf("invalid request reached callback: error=%v, calls=%d", err, calls)
			}
		})
	}
}

func TestElicitationModeAdmission(t *testing.T) {
	for _, mode := range []p.McpServerElicitationMode{
		p.McpServerElicitationModeForm, p.McpServerElicitationModeURL,
		p.McpServerElicitationModeOpenAIForm, p.McpServerElicitationModeOpenAIFormLegacy,
	} {
		t.Run(string(mode), func(t *testing.T) {
			final := fmt.Sprintf(`"mode":%q`, mode)
			for _, tc := range []struct {
				name, members string
				valid         bool
			}{
				{"single", final, true},
				{"supported transition", `"mode":"url",` + final, true},
				{"escaped supported", `"m\u006fde":"form",` + final, true},
				{"unknown", `"mode":"future",` + final, false},
				{"escaped unknown", `"m\u006fde":"future",` + final, false},
				{"empty", `"mode":"",` + final, false},
				{"null", `"mode":null,` + final, false},
				{"type", `"mode":1,` + final, false},
				{"invalid final", final + `,"mode":"future"`, false},
				{"folded extra", `"MODE":"future",` + final, true},
				{"nested extra", `"extra":{"mode":"future"},` + final, true},
				{"metadata", `"_meta":{"mode":"future"},` + final, true},
				{"long", fmt.Sprintf(`"mode":%q,%s`, strings.Repeat("x", 16000), final), false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					mock := NewMockTransport()
					client := p.NewClient(mock)
					calls := 0
					client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(_ context.Context, params p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
						calls++
						if params.Mode != mode {
							t.Fatalf("mode=%q, want %q", params.Mode, mode)
						}
						switch mode {
						case p.McpServerElicitationModeForm:
							if params.RequestedSchema == nil || params.RequestedSchema.Type != "object" || params.OpenAIRequestedSchema != nil {
								t.Fatal("typed form schema changed")
							}
						case p.McpServerElicitationModeURL:
							if params.URL == nil || *params.URL != "https://example.test" || params.ElicitationID == nil || *params.ElicitationID != "e" {
								t.Fatal("URL branch fields changed")
							}
						default:
							if params.RequestedSchema != nil || string(params.OpenAIRequestedSchema) != `{"type":"object","properties":{}}` {
								t.Fatal("arbitrary OpenAI schema changed")
							}
						}
						return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel}, nil
					}})
					request := p.Request{ID: p.RequestID{Value: "request"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(elicitationModeBody(tc.members))}
					resp, err := mock.InjectServerRequest(context.Background(), request)
					if tc.valid {
						if err != nil || resp.Error != nil || calls != 1 {
							t.Fatalf("valid request: error=%v, response error=%v, calls=%d", err, resp.Error, calls)
						}
						return
					}
					if !errors.Is(err, p.ErrInvalidParams) || calls != 0 || !reflect.DeepEqual(resp, p.Response{}) {
						t.Fatalf("invalid request published: error=%v, calls=%d, response=%+v", err, calls, resp)
					}
					if len(err.Error()) > 4096 {
						t.Fatal("unbounded mode diagnostic")
					}
					request.Params = json.RawMessage(elicitationModeBody(final))
					if _, err := mock.InjectServerRequest(context.Background(), request); err != nil || calls != 1 {
						t.Fatalf("recovery failed: error=%v, calls=%d", err, calls)
					}
				})
			}
		})
	}
}

func TestElicitationModeReceiver(t *testing.T) {
	for _, mode := range []p.McpServerElicitationMode{
		p.McpServerElicitationModeForm, p.McpServerElicitationModeURL,
		p.McpServerElicitationModeOpenAIForm, p.McpServerElicitationModeOpenAIFormLegacy,
	} {
		t.Run(string(mode), func(t *testing.T) {
			var params p.McpServerElicitationRequestParams
			seed := elicitationModeBody(fmt.Sprintf(`"mode":%q,"turnId":"turn","_meta":{"retained":[1]}`, mode))
			if err := json.Unmarshal([]byte(seed), &params); err != nil {
				t.Fatal(err)
			}
			before := params
			raw := elicitationModeBody(fmt.Sprintf(`"mode":"future","mode":%q`, mode))
			if err := json.Unmarshal([]byte(raw), &params); err == nil {
				t.Fatal("hidden mode accepted")
			}
			if !reflect.DeepEqual(params, before) || params.URL != before.URL || params.TurnID != before.TurnID || params.RequestedSchema != before.RequestedSchema {
				t.Fatal("new rejection changed receiver or retained pointers")
			}
			if len(params.OpenAIRequestedSchema) > 0 && &params.OpenAIRequestedSchema[0] != &before.OpenAIRequestedSchema[0] {
				t.Fatal("new rejection changed raw schema backing storage")
			}
			params.Meta.(map[string]any)["probe"] = "same map"
			if before.Meta.(map[string]any)["probe"] != "same map" {
				t.Fatal("new rejection replaced retained metadata map")
			}
			if err := json.Unmarshal([]byte(seed), &params); err != nil {
				t.Fatalf("receiver recovery: %v", err)
			}
		})
	}
}

func TestElicitationModePrecedence(t *testing.T) {
	raw := elicitationModeBody(`"mode":"future","mode":"form"`)
	for _, tc := range []struct{ name, body, want string }{
		{"missing identity", strings.Replace(raw, `"serverName":"server",`, "", 1), "serverName"},
		{"native", strings.Replace(raw, `"threadId":"thread"`, `"threadId":1`, 1), "cannot unmarshal number"},
		{"missing variant field", strings.Replace(raw, `"message":"input",`, "", 1), "message"},
		{"typed schema", strings.Replace(raw, `"properties":{}`, `"properties":1`, 1), "cannot unmarshal number"},
		{"final invalid", elicitationModeBody(`"mode":"future","mode":"later"`), `"later"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var params p.McpServerElicitationRequestParams
			err := json.Unmarshal([]byte(tc.body), &params)
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), `"future"`) {
				t.Fatalf("established error replaced: %v", err)
			}
		})
	}
}

func TestElicitationModeEnvelope(t *testing.T) {
	var envelope struct {
		Params p.McpServerElicitationRequestParams `json:"params"`
		After  string                              `json:"after"`
	}
	seed := elicitationModeBody(`"mode":"form"`)
	if err := json.Unmarshal([]byte(`{"params":`+seed+`}`), &envelope); err != nil {
		t.Fatal(err)
	}
	before := envelope.Params
	bad := elicitationModeBody(`"mode":"future","mode":"form"`)
	if err := json.Unmarshal([]byte(`{"params":`+bad+`,"after":"visited"}`), &envelope); err == nil {
		t.Fatal("envelope accepted hidden unsupported mode")
	}
	if !reflect.DeepEqual(envelope.Params, before) || envelope.After != "visited" {
		t.Fatal("request receiver restoration or enclosing traversal changed")
	}
}

func TestElicitationModeScalar(t *testing.T) {
	var mode p.McpServerElicitationMode
	if err := json.Unmarshal([]byte(`"future"`), &mode); err != nil || mode != "future" {
		t.Fatalf("ordinary scalar decoding changed: %q, %v", mode, err)
	}
	encoded, err := json.Marshal(mode)
	if err != nil || string(encoded) != `"future"` {
		t.Fatalf("ordinary scalar encoding changed: %s, %v", encoded, err)
	}
}
