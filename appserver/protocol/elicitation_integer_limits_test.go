package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestElicitationIntegerLimits(t *testing.T) {
	for _, field := range []string{"minLength", "maxLength", "minItems", "maxItems"} {
		maximum, above := "4294967295", "4294967296"
		if strings.HasSuffix(field, "Items") {
			maximum = "18446744073709551615"
			above = "18446744073709551616"
		}
		for _, tc := range []struct {
			name, value string
			valid       bool
		}{{"decimal-maximum", maximum + ".0", true}, {"exponent-maximum", maximum + "0e-1", true}, {"negative-zero", "-0.0", true}, {"power", "1e3", true}, {"above-decimal", above + ".0", false}, {"zero", "0", true}, {"maximum", maximum, true}, {"above", above, false}, {"negative", "-1", false}, {"fraction", "0.5", false}} {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				mock := NewMockTransport()
				diagnostics, calls := 0, 0
				client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { diagnostics++ }))
				defer client.Close()
				var received []byte
				client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(_ context.Context, params p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
					calls++
					definition, ok := params.RequestedSchema.Properties["value"].(map[string]any)
					if !ok {
						t.Fatal("missing definition")
					}
					var e error
					received, e = json.Marshal(definition[field])
					if e != nil {
						t.Fatal(e)
					}
					return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel}, nil
				}})
				property := `{"type":"string","` + field + `":` + tc.value + `}`
				if strings.HasSuffix(field, "Items") {
					property = `{"type":"array","items":{"type":"string","enum":[]},"` + field + `":` + tc.value + `}`
				}
				schema := `{"type":"object","properties":{"value":` + property + `}}`
				body := strings.Replace(elicitationFormBody(p.McpServerElicitationModeForm, `"_meta":null`), `{"type":"object","properties":{}}`, schema, 1)
				response, e := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "elicitation"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(body)})
				if tc.valid {
					if e != nil || calls != 1 || diagnostics != 0 || len(response.Result) == 0 {
						t.Fatalf("valid limit rejected: %v", e)
					}
					if string(received) != tc.value {
						t.Errorf("handler integer limit changed: got=%s want=%s", received, tc.value)
					}
				} else {
					if !errors.Is(e, p.ErrInvalidParams) || calls != 0 || len(response.Result) != 0 || diagnostics != 1 {
						t.Fatalf("invalid limit reached handler or lost rejection: calls=%d error=%v diagnostics=%d", calls, e, diagnostics)
					}
					recoveredBody := strings.Replace(elicitationFormBody(p.McpServerElicitationModeForm, `"_meta":null`), `{"type":"object","properties":{}}`, elicitationLimitSchema(field, "0"), 1)
					recovered, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "recovered"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(recoveredBody)})
					if err != nil || calls != 1 || diagnostics != 1 || string(received) != "0" || len(recovered.Result) == 0 {
						t.Fatalf("same-client recovery failed: %v", err)
					}
				}
			})
		}
	}
}
