package protocol_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func elicitationFormBody(mode p.McpServerElicitationMode, extra string) string {
	return fmt.Sprintf(`{"serverName":"server","threadId":"thread","message":"input","mode":%q,"requestedSchema":{"type":"object","properties":{}},%s}`, mode, extra)
}

func TestElicitationFormExtras(t *testing.T) {
	for _, mode := range []p.McpServerElicitationMode{p.McpServerElicitationModeForm, p.McpServerElicitationModeOpenAIForm, p.McpServerElicitationModeOpenAIFormLegacy} {
		for _, field := range []string{"url", "elicitationId"} {
			t.Run(string(mode)+"/"+field, func(t *testing.T) {
				raw := elicitationFormBody(mode, fmt.Sprintf(`%q:{"extra":true}`, field))
				var params p.McpServerElicitationRequestParams
				if err := json.Unmarshal([]byte(raw), &params); err != nil {
					t.Errorf("valid form rejected unrestricted extra: %v", err)
				}
				mock := NewMockTransport()
				client := p.NewClient(mock)
				calls := 0
				client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(context.Context, p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
					calls++
					return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel}, nil
				}})
				resp, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "request"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(raw)})
				if err != nil || resp.Error != nil || calls != 1 {
					t.Fatalf("valid form failed admission: error=%v, response=%+v, calls=%d", err, resp, calls)
				}
			})
		}
	}
}

func TestElicitationExtraCompatibility(t *testing.T) {
	for _, mode := range []p.McpServerElicitationMode{p.McpServerElicitationModeForm, p.McpServerElicitationModeOpenAIForm, p.McpServerElicitationModeOpenAIFormLegacy} {
		for _, field := range []string{"url", "elicitationId"} {
			t.Run(string(mode)+"/"+field, func(t *testing.T) {
				key := fmt.Sprintf("%q", field)
				for _, tc := range []struct {
					name, members string
					want          *string
				}{
					{"string", key + `:"legacy"`, p.Ptr("legacy")},
					{"empty", key + `:""`, p.Ptr("")},
					{"null", key + `:null`, nil},
					{"number", key + `:7`, nil},
					{"boolean", key + `:true`, nil},
					{"array", key + `:[{},null]`, nil},
					{"object", key + `:{"ignored":[1]}`, nil},
					{"wrong after string", key + `:"first",` + key + `:{}`, p.Ptr("first")},
					{"wrong before string", key + `:{},` + key + `:"last"`, p.Ptr("last")},
					{"null reset", key + `:"first",` + key + `:null,` + key + `:{}`, nil},
					{"null then string", key + `:null,` + key + `:"last"`, p.Ptr("last")},
					{"escaped", fmt.Sprintf(`"\u%04x%s":"legacy"`, field[0], field[1:]), p.Ptr("legacy")},
					{"folded ignored", fmt.Sprintf(`%q:"legacy"`, strings.ToUpper(field)), nil},
				} {
					t.Run(tc.name, func(t *testing.T) {
						mock := NewMockTransport()
						client := p.NewClient(mock)
						calls := 0
						check := func(params p.McpServerElicitationRequestParams) {
							t.Helper()
							got := params.URL
							if field == "elicitationId" {
								got = params.ElicitationID
							}
							if !reflect.DeepEqual(got, tc.want) || params.Mode != mode || params.Message != "input" {
								t.Fatalf("extra=%v, want %v; mode=%q", got, tc.want, params.Mode)
							}
						}
						client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(_ context.Context, params p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
							calls++
							check(params)
							return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel}, nil
						}})
						raw := elicitationFormBody(mode, tc.members)
						var params p.McpServerElicitationRequestParams
						if err := json.Unmarshal([]byte(raw), &params); err != nil {
							t.Fatal(err)
						}
						check(params)
						encoded, err := json.Marshal(params)
						if err != nil {
							t.Fatal(err)
						}
						if err := json.Unmarshal(encoded, &params); err != nil {
							t.Fatal(err)
						}
						check(params)
						resp, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "r"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(raw)})
						if err != nil || resp.Error != nil || calls != 1 {
							t.Fatalf("admission failed: %v, response=%+v, calls=%d", err, resp, calls)
						}
					})
				}
			})
		}
	}
}

func TestElicitationURLStillStrict(t *testing.T) {
	for _, field := range []string{"url", "elicitationId"} {
		for _, invalid := range []string{`{}`, `[]`, `1`, `false`, `null`} {
			t.Run(field+"/"+invalid, func(t *testing.T) {
				raw := elicitationModeBody(fmt.Sprintf(`%q:%s,"mode":"url"`, field, invalid))
				mock := NewMockTransport()
				client := p.NewClient(mock)
				calls := 0
				client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(context.Context, p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
					calls++
					return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel}, nil
				}})
				request := p.Request{ID: p.RequestID{Value: "r"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(raw)}
				resp, err := mock.InjectServerRequest(context.Background(), request)
				if !errors.Is(err, p.ErrInvalidParams) || calls != 0 || !reflect.DeepEqual(resp, p.Response{}) {
					t.Fatalf("URL mode restriction lost: error=%v, calls=%d, response=%+v", err, calls, resp)
				}
				request.Params = json.RawMessage(elicitationModeBody(`"mode":"url"`))
				if _, err := mock.InjectServerRequest(context.Background(), request); err != nil || calls != 1 {
					t.Fatalf("recovery failed: %v, calls=%d", err, calls)
				}
			})
		}
	}
}

func TestElicitationExtraBranchSelection(t *testing.T) {
	for _, tc := range []struct {
		name, members string
		valid         bool
	}{
		{"escaped mode key", `"m\u006fde":"form","url":{}`, true},
		{"escaped mode value", `"mode":"f\u006frm","url":{}`, true},
		{"URL to form", `"mode":"url","mode":"form","url":{}`, true},
		{"form to URL", `"mode":"form","mode":"url","url":{}`, false},
		{"folded extra mode", `"MODE":"url","mode":"form","url":{}`, true},
		{"no exact mode", `"MODE":"form","url":{}`, false},
		{"hidden unsupported mode", `"mode":"future","mode":"form","url":{}`, false},
		{"final unsupported mode", `"mode":"form","mode":"future","url":{}`, false},
		{"null final mode", `"mode":"form","mode":null,"url":{}`, false},
		{"null prior mode", `"mode":null,"mode":"form","url":{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// This supplies all declared fields for either branch; the extra
			// mode/url occurrences above determine the selected contract.
			raw := elicitationModeBody(tc.members)
			var params p.McpServerElicitationRequestParams
			err := json.Unmarshal([]byte(raw), &params)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
			if tc.valid && (params.Mode != p.McpServerElicitationModeForm || params.URL == nil || *params.URL != "https://example.test") {
				t.Fatal("selected form or earlier string value changed")
			}
		})
	}
}

func TestElicitationExtraReceiver(t *testing.T) {
	for _, mode := range []p.McpServerElicitationMode{p.McpServerElicitationModeForm, p.McpServerElicitationModeOpenAIForm, p.McpServerElicitationModeOpenAIFormLegacy} {
		t.Run(string(mode), func(t *testing.T) {
			seed := elicitationFormBody(mode, `"url":"old","elicitationId":"old-id","turnId":"turn","_meta":{"retained":[1]}`)
			var params p.McpServerElicitationRequestParams
			if err := json.Unmarshal([]byte(seed), &params); err != nil {
				t.Fatal(err)
			}
			before := params
			bad := elicitationFormBody(mode, `"url":{},"message":null`)
			if err := json.Unmarshal([]byte(bad), &params); err == nil {
				t.Fatal("invalid shared field accepted")
			}
			if !reflect.DeepEqual(params, before) || params.URL != before.URL || params.TurnID != before.TurnID || params.RequestedSchema != before.RequestedSchema {
				t.Fatal("failure changed receiver or references")
			}
			params.Meta.(map[string]any)["probe"] = "same map"
			if before.Meta.(map[string]any)["probe"] != "same map" {
				t.Fatal("failure replaced retained metadata map")
			}
			if len(params.OpenAIRequestedSchema) > 0 && &params.OpenAIRequestedSchema[0] != &before.OpenAIRequestedSchema[0] {
				t.Fatal("failure replaced raw schema backing storage")
			}
			valid := elicitationFormBody(mode, `"url":{},"elicitationId":[1]`)
			if err := json.Unmarshal([]byte(valid), &params); err != nil {
				t.Fatal(err)
			}
			if params.URL != nil || params.ElicitationID != nil || params.TurnID != nil || params.Meta != nil {
				t.Fatal("reused receiver retained old fields")
			}
			if *before.URL != "old" || *before.ElicitationID != "old-id" {
				t.Fatal("successful replacement changed earlier references")
			}
		})
	}
}

func TestElicitationExtraConstrainedFields(t *testing.T) {
	for _, tc := range []struct{ name, raw, want string }{
		{"missing identity", strings.Replace(elicitationFormBody(p.McpServerElicitationModeForm, `"url":{}`), `"serverName":"server",`, "", 1), "serverName"},
		{"native shared", strings.Replace(elicitationFormBody(p.McpServerElicitationModeForm, `"url":{}`), `"threadId":"thread"`, `"threadId":1`, 1), "cannot unmarshal number"},
		{"typed schema", strings.Replace(elicitationFormBody(p.McpServerElicitationModeForm, `"url":{}`), `"properties":{}`, `"properties":1`, 1), "cannot unmarshal number"},
		{"missing URL", `{"serverName":"s","threadId":"t","message":"m","mode":"url","elicitationId":"e"}`, "url"},
		{"missing elicitation ID", `{"serverName":"s","threadId":"t","message":"m","mode":"url","url":"u"}`, "elicitationId"},
		{"invalid fallback", elicitationFormBody("future", `"url":{}`), "cannot unmarshal object"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var params p.McpServerElicitationRequestParams
			err := json.Unmarshal([]byte(tc.raw), &params)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("constrained failure changed: %v", err)
			}
		})
	}
}

func TestElicitationEscapedFormMode(t *testing.T) {
	for _, tc := range []struct {
		mode p.McpServerElicitationMode
		raw  string
	}{
		{p.McpServerElicitationModeForm, `"f\u006frm"`},
		{p.McpServerElicitationModeOpenAIForm, `"openai\u0046orm"`},
		{p.McpServerElicitationModeOpenAIFormLegacy, `"openai\/form"`},
	} {
		raw := strings.Replace(elicitationFormBody(tc.mode, `"url":{}`), fmt.Sprintf(`"mode":%q`, tc.mode), `"mode":`+tc.raw, 1)
		var params p.McpServerElicitationRequestParams
		if err := json.Unmarshal([]byte(raw), &params); err != nil || params.Mode != tc.mode {
			t.Fatalf("escaped form mode changed: mode=%q error=%v", params.Mode, err)
		}
	}
}

func FuzzElicitationFormExtras(f *testing.F) {
	for kind := uint8(0); kind < 6; kind++ {
		f.Add("", int64(0), kind)
		f.Add("quote\"\n\x00日本語", int64(-1), kind)
		f.Add("slash", int64(0), kind|128)
	}
	f.Fuzz(func(t *testing.T, text string, number int64, kind uint8) {
		if len(text) > 4096 {
			text = text[:4096]
		}
		// Construct valid structured JSON directly. The generic reference
		// decoder models JSON kinds independently of the typed extra decoder.
		var value any
		switch kind % 6 {
		case 0:
			value = nil
		case 1:
			value = text
		case 2:
			value = number
		case 3:
			value = number%2 == 0
		case 4:
			value = []any{text, number, nil}
		case 5:
			value = map[string]any{"nested": []any{map[string]any{"mode": text}, number}}
		}
		rawValue, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var reference any
		decoder := json.NewDecoder(bytes.NewReader(rawValue))
		decoder.UseNumber()
		if err := decoder.Decode(&reference); err != nil {
			t.Fatal(err)
		}
		want, isString := reference.(string)
		for _, mode := range []p.McpServerElicitationMode{p.McpServerElicitationModeForm, p.McpServerElicitationModeOpenAIForm, p.McpServerElicitationModeOpenAIFormLegacy} {
			for _, field := range []string{"url", "elicitationId"} {
				members := fmt.Sprintf(`%q:%s`, field, rawValue)
				var params p.McpServerElicitationRequestParams
				var encodedMode strings.Builder
				encodedMode.WriteByte('"')
				for index, c := range string(mode) {
					if c == '/' && kind&128 != 0 {
						encodedMode.WriteString(`\/`)
					} else if uint64(number)&(1<<uint(index)) != 0 {
						fmt.Fprintf(&encodedMode, `\u%04x`, c)
					} else {
						encodedMode.WriteRune(c)
					}
				}
				encodedMode.WriteByte('"')
				rawBody := strings.Replace(elicitationFormBody(mode, members), fmt.Sprintf(`"mode":%q`, mode), `"mode":`+encodedMode.String(), 1)
				if err := json.Unmarshal([]byte(rawBody), &params); err != nil {
					t.Fatalf("valid unrestricted extra rejected: mode=%q field=%s kind=%d: %v", mode, field, kind%6, err)
				}
				got := params.URL
				if field == "elicitationId" {
					got = params.ElicitationID
				}
				if (got != nil) != isString || (isString && *got != want) || params.Mode != mode {
					t.Fatalf("extra kind/value or branch changed: mode=%q field=%s kind=%d", mode, field, kind%6)
				}
			}
		}
	})
}
