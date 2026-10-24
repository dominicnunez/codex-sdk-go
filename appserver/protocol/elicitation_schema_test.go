package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestElicitationTypedSchemaAdmission(t *testing.T) {
	for _, schema := range []string{
		`{"type":"array","properties":{}}`,
		`{"type":"object","properties":{"answer":{"type":"future"}}}`,
		`{"type":"object","properties":{"answer":{"type":"boolean","default":"yes"}}}`,
		`{"type":"object","properties":{"answer":{"type":"string","format":"future"}}}`,
		`{"type":"object","properties":{"answer":{"type":"array","items":{"type":"string","enum":[null]}}}}`,
	} {
		t.Run(schema, func(t *testing.T) {
			raw := strings.Replace(elicitationFormBody(p.McpServerElicitationModeForm, `"_meta":null`), `{"type":"object","properties":{}}`, schema, 1)
			var target p.McpServerElicitationRequestParams
			if err := json.Unmarshal([]byte(raw), &target); err == nil {
				t.Error("invalid typed form schema accepted")
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			calls := 0
			client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(context.Context, p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
				calls++
				return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel}, nil
			}})
			resp, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "schema"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(raw)})
			if !errors.Is(err, p.ErrInvalidParams) || calls != 0 || !reflect.DeepEqual(resp, p.Response{}) {
				t.Errorf("invalid schema published: err=%v calls=%d response=%+v", err, calls, resp)
			}
		})
	}
}

func TestElicitationSchemaAlternatives(t *testing.T) {
	valid := []string{
		`{"type":"string","default":"s","type":"number","default":1}`,
		`{"type":"number","default":1,"type":"string","default":"s"}`,
		`{"type":"future","type":"string"}`,
		`{"type":"string","enum":[null],"enum":["a"]}`,

		`{"type":"string","type":"number","default":1}`,
		`{"type":"number","type":"string","default":"s"}`,
		`{"type":"string"}`,
		`{"type":"string","default":null,"title":null,"description":null,"format":"email","minLength":0,"maxLength":1e3}`,
		`{"type":"string","format":"uri"}`, `{"type":"string","format":"date"}`, `{"type":"string","format":"date-time"}`,
		`{"type":"number","minimum":-2.5,"maximum":4,"default":1.5}`,
		`{"type":"integer","minimum":null,"maximum":null,"default":0.5}`,
		`{"type":"boolean","default":false}`,
		`{"type":"string","enum":[]}`, `{"type":"string","enum":["a"],"enumNames":["A"],"default":"b"}`,
		`{"type":"string","enum":["a","b"],"enumNames":null}`,
		`{"type":"string","oneOf":[{"const":"a","title":"A"}],"default":"a"}`,
		`{"type":"array","items":{"type":"string","enum":[]},"default":[],"minItems":-0,"maxItems":18446744073709551616}`,
		`{"type":"array","items":{"anyOf":[{"const":"a","title":"A"}]},"default":["a"],"minItems":1.0,"maxItems":100e-2}`,
		`{"type":"array","items":{"anyOf":[]},"default":null,"minItems":null,"maxItems":null}`,
	}
	for _, property := range valid {
		t.Run(property, func(t *testing.T) {
			schema := `{"$schema":null,"type":"object","properties":{"answer":` + property + `},"required":["answer","not-a-property"]}`
			checkElicitationSchema(t, schema, true)
		})
	}
	for _, schema := range []string{
		`{"type":"object","properties":{},"required":null}`,
		`{"type":"object","properties":{"x":{"type":"future"}},"properties":{"x":{"type":"boolean"}}}`,
		`{"type":"object","properties":{}}`,
		`{"type":"obj\u0065ct","properties":{"x":{"type":"str\u0069ng","format":"date\u002dtime"}}}`,
		`{"type":"object","type":"object","properties":{"x":{"type":"string","default":"first","default":null}}}`,
		`{"type":"object","properties":{"x":{"type":"string"}},"properties":{"y":{"type":"boolean"}}}`,
	} {
		checkElicitationSchema(t, schema, true)
	}
}

func TestElicitationSchemaRejectsMalformedMembers(t *testing.T) {
	invalidProperties := []string{
		`null`, `[]`, `"string"`, `{}`, `{"type":null}`, `{"type":"object"}`, `{"TYPE":"string"}`,
		`{"type":"string","format":"uuid"}`, `{"type":"string","format":true}`,
		`{"type":"string","default":1}`, `{"type":"string","title":[]}`, `{"type":"string","description":false}`,
		`{"type":"string","minLength":-1}`, `{"type":"string","maxLength":1.1}`, `{"type":"string","minLength":1e-999999999}`,
		`{"type":"number","default":"1"}`, `{"type":"integer","maximum":true}`, `{"type":"number","minimum":{}}`,
		`{"type":"boolean","default":0}`, `{"type":"boolean","format":"uri"}`,
		`{"type":"string","enum":null}`, `{"type":"string","enum":[null]}`, `{"type":"string","enumNames":[]}`,
		`{"type":"string","enum":[],"enumNames":[false]}`, `{"type":"string","enum":[],"format":"uri"}`,
		`{"type":"string","enum":[],"oneOf":[]}`, `{"type":"string","oneOf":null}`,
		`{"type":"string","oneOf":[{}]}`, `{"type":"string","oneOf":[{"const":"a","title":null}]}`,
		`{"type":"string","oneOf":[{"const":null,"title":"A"}]}`, `{"type":"string","oneOf":[{"const":"a","title":"A","extra":true}]}`,
		`{"type":"array"}`, `{"type":"array","items":null}`, `{"type":"array","items":{"type":"string"}}`,
		`{"type":"array","items":{"type":"number","enum":[1]}}`,
		`{"type":"array","items":{"type":"string","enum":[],"anyOf":[]}}`,
		`{"type":"array","items":{"anyOf":null}}`, `{"type":"array","items":{"anyOf":[null]}}`,
		`{"type":"array","items":{"anyOf":[]},"default":[null]}`,
		`{"type":"array","items":{"anyOf":[]},"minItems":-0.1}`,
		`{"type":"array","items":{"anyOf":[]},"maxItems":1e-999999999}`,
		`{"type":"string","extra":true}`,
	}
	for _, property := range invalidProperties {
		t.Run(property, func(t *testing.T) {
			checkElicitationSchema(t, `{"type":"object","properties":{"answer":`+property+`}}`, false)
		})
	}
	for _, schema := range []string{
		`{"type":"object","properties":{},"extra":true}`,
		`{"type":"object","properties":{},"required":[null]}`,
		`{"type":"array","type":"object","properties":{}}`,
		`{"type":"object","properties":{"x":null},"properties":{}}`,
		`{"type":"object","properties":{},"$schema":false}`,
	} {
		checkElicitationSchema(t, schema, false)
	}
}

func checkElicitationSchema(t *testing.T, schema string, valid bool) {
	t.Helper()
	raw := strings.Replace(elicitationFormBody(p.McpServerElicitationModeForm, `"_meta":null`), `{"type":"object","properties":{}}`, schema, 1)
	var typed p.McpElicitationSchema
	if err := json.Unmarshal([]byte(schema), &typed); (err == nil) != valid {
		t.Fatalf("direct typed schema valid=%v error=%v", valid, err)
	}
	var params p.McpServerElicitationRequestParams
	if err := json.Unmarshal([]byte(raw), &params); (err == nil) != valid {
		t.Fatalf("params schema valid=%v error=%v", valid, err)
	}
	mock := NewMockTransport()
	client := p.NewClient(mock)
	calls := 0
	client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(context.Context, p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
		calls++
		return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel}, nil
	}})
	req := p.Request{ID: p.RequestID{Value: "schema"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(raw)}
	resp, err := mock.InjectServerRequest(context.Background(), req)
	if valid {
		if err != nil || calls != 1 || resp.Error != nil {
			t.Fatalf("valid schema rejected: %v calls=%d", err, calls)
		}
	} else {
		if !errors.Is(err, p.ErrInvalidParams) || calls != 0 || !reflect.DeepEqual(resp, p.Response{}) {
			t.Fatalf("invalid schema published: %v calls=%d response=%+v", err, calls, resp)
		}
		req.Params = json.RawMessage(elicitationFormBody(p.McpServerElicitationModeForm, `"_meta":null`))
		if _, err := mock.InjectServerRequest(context.Background(), req); err != nil || calls != 1 {
			t.Fatalf("recovery failed: %v calls=%d", err, calls)
		}
	}
	// OpenAI forms are arbitrary JSON, including values outside the typed grammar.
	for _, mode := range []p.McpServerElicitationMode{p.McpServerElicitationModeOpenAIForm, p.McpServerElicitationModeOpenAIFormLegacy} {
		openAI := strings.Replace(raw, `"mode":"form"`, `"mode":"`+string(mode)+`"`, 1)
		if err := json.Unmarshal([]byte(openAI), &params); err != nil || params.RequestedSchema != nil || !json.Valid(params.OpenAIRequestedSchema) {
			t.Fatalf("arbitrary OpenAI schema restricted: %v", err)
		}
	}
}

func TestElicitationSchemaReceiverAndPrecedence(t *testing.T) {
	var params p.McpServerElicitationRequestParams
	good := elicitationFormBody(p.McpServerElicitationModeForm, `"turnId":"turn","_meta":{"old":[1]}`)
	if err := json.Unmarshal([]byte(good), &params); err != nil {
		t.Fatal(err)
	}
	before := params
	bad := strings.Replace(good, `"properties":{}`, `"properties":{"x":{"type":"future"}}`, 1)
	if err := json.Unmarshal([]byte(bad), &params); err == nil {
		t.Fatal("invalid property accepted")
	}
	if !reflect.DeepEqual(before, params) || before.RequestedSchema != params.RequestedSchema || before.TurnID != params.TurnID {
		t.Fatal("rejected schema replaced receiver")
	}
	params.RequestedSchema.Properties["probe"] = "same map"
	if before.RequestedSchema.Properties["probe"] != "same map" {
		t.Fatal("retained map replaced")
	}
	var envelope struct {
		Params p.McpServerElicitationRequestParams `json:"params"`
		After  string                              `json:"after"`
	}
	envelope.Params = before
	if err := json.Unmarshal([]byte(`{"params":`+bad+`,"after":"visited"}`), &envelope); err == nil || envelope.After != "visited" || envelope.Params.RequestedSchema != before.RequestedSchema {
		t.Fatal("outer traversal/receiver changed")
	}
	native := `{"type":"future","properties":1}`
	var schema p.McpElicitationSchema
	var typeError *json.UnmarshalTypeError
	if err := json.Unmarshal([]byte(native), &schema); !errors.As(err, &typeError) {
		t.Fatalf("native error precedence changed: %v", err)
	}
	if err := json.Unmarshal([]byte(good), &params); err != nil || len(params.RequestedSchema.Properties) != 0 {
		t.Fatalf("valid reuse retained prior map: %v", err)
	}
}

func TestElicitationSchemaMapState(t *testing.T) {
	var schema p.McpElicitationSchema
	seed := `{"type":"object","properties":{"old":{"type":"string"}},"required":["old"]}`
	if err := json.Unmarshal([]byte(seed), &schema); err != nil {
		t.Fatal(err)
	}
	before := schema
	if err := json.Unmarshal([]byte(`{"type":"object","properties":{"new":{"type":"future"}}}`), &schema); err == nil || !reflect.DeepEqual(schema, before) {
		t.Fatal("direct receiver changed on failure")
	}
	schema.Properties["probe"] = "retained"
	if before.Properties["probe"] != "retained" {
		t.Fatal("direct map identity changed")
	}
	merged := `{"type":"object","properties":{"x":{"type":"future"},"retained":{"type":"boolean"}},"properties":{"x":{"type":"number","default":"overwritten","default":2}},"required":null}`
	if err := json.Unmarshal([]byte(merged), &schema); err != nil {
		t.Fatal(err)
	}
	if len(schema.Properties) != 2 || schema.Properties["retained"].(map[string]any)["type"] != "boolean" || schema.Properties["x"].(map[string]any)["default"] != float64(2) || schema.Required != nil {
		t.Fatal("native map merge, member replacement or null reset changed")
	}
	if len(before.Properties) != 2 {
		t.Fatal("success mutated the earlier map")
	}
}

func TestElicitationSchemaDiagnosticBounds(t *testing.T) {
	long := strings.Repeat("\\\"\x00", 4096)
	key, _ := json.Marshal(long)
	for _, schema := range []string{
		`{"type":"object","properties":{},` + string(key) + `:true}`,
		`{"type":"object","properties":{` + string(key) + `:{"type":"future"}}}`,
		`{"type":"object","properties":{"x":{"type":"string","format":` + string(key) + `}}}`,
	} {
		var target p.McpElicitationSchema
		err := json.Unmarshal([]byte(schema), &target)
		if err == nil || len(err.Error()) > 1024 {
			t.Fatalf("unbounded or missing schema diagnostic: %v", err)
		}
	}
}
