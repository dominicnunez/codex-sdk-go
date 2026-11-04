package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func elicitationLimitSchema(field, value string) string {
	property := `{"type":"string","` + field + `":` + value + `}`
	if strings.HasSuffix(field, "Items") {
		property = `{"type":"array","items":{"type":"string","enum":[]},"` + field + `":` + value + `}`
	}
	return `{"type":"object","properties":{"value":` + property + `}}`
}

func TestElicitationLimitSelectionAndTypes(t *testing.T) {
	for _, tc := range []struct{ raw, field, want string }{
		{elicitationLimitSchema("minItems", `18446744073709551616,"minItems":0`), "minItems", "0"},
		{elicitationLimitSchema("minItems", `-1,"minItems":null`), "minItems", "null"},
		{elicitationLimitSchema("minItems", `null,"min\u0049tems":18446744073709551615`), "minItems", "18446744073709551615"},
		{`{"type":"object","properties":{"value":{"type":"array","items":{"type":"string","enum":[]},"minItems":18446744073709551616}},"properties":{"value":{"type":"string","minLength":1}}}`, "minLength", "1"},
		{elicitationLimitSchema("maxLength", `42949672950e-1`), "maxLength", "42949672950e-1"},
		{elicitationLimitSchema("maxItems", `0e999999999999999999999`), "maxItems", "0e999999999999999999999"},
	} {
		var value p.McpElicitationSchema
		if err := json.Unmarshal([]byte(tc.raw), &value); err != nil {
			t.Fatalf("valid final selection rejected: %s %v", tc.raw, err)
		}
		property := value.Properties["value"].(map[string]any)
		encoded, err := json.Marshal(property[tc.field])
		if err != nil || string(encoded) != tc.want {
			t.Fatalf("selected limit changed: %s %v", encoded, err)
		}
		if tc.want != "null" {
			if _, ok := property[tc.field].(json.Number); !ok {
				t.Fatal("integer limit is not exact")
			}
		}
		if tc.field == "minLength" && len(property) != 2 {
			t.Fatalf("old property members survived replacement: %#v", property)
		}
	}
	// Root property dictionaries merge while their property values replace.
	var merged p.McpElicitationSchema
	raw := `{"type":"object","properties":{"a":{"type":"string","minLength":4294967295}},"properties":{"b":{"type":"array","items":{"anyOf":[]},"maxItems":18446744073709551615}}}`
	if err := json.Unmarshal([]byte(raw), &merged); err != nil || len(merged.Properties) != 2 {
		t.Fatalf("properties merge changed: %v", err)
	}
	if merged.Properties["a"].(map[string]any)["minLength"] != json.Number("4294967295") || merged.Properties["b"].(map[string]any)["maxItems"] != json.Number("18446744073709551615") {
		t.Fatal("merged limits lost precision")
	}
	// The schema declares numeric-property values as doubles, including an
	// integer property's fractional default. No new relationship is imposed.
	raw = `{"type":"object","properties":{"n":{"type":"integer","minimum":-2.5,"maximum":4,"default":0.5},"s":{"type":"string","minLength":5,"maxLength":1}}}`
	if err := json.Unmarshal([]byte(raw), &merged); err != nil {
		t.Fatal(err)
	}
	numbers := merged.Properties["n"].(map[string]any)
	for _, name := range []string{"minimum", "maximum", "default"} {
		if _, ok := numbers[name].(float64); !ok {
			t.Fatalf("double %s changed representation", name)
		}
	}
	source := []byte(elicitationLimitSchema("maxItems", "18446744073709551615"))
	if err := merged.UnmarshalJSON(source); err != nil {
		t.Fatal(err)
	}
	for i := range source {
		source[i] = ' '
	}
	if merged.Properties["value"].(map[string]any)["maxItems"] != json.Number("18446744073709551615") {
		t.Fatal("integer token retained mutable input")
	}
}

func TestElicitationLimitReceiverAndOpenAI(t *testing.T) {
	prior := map[string]any{"prior": map[string]any{"type": "string"}}
	value := p.McpElicitationSchema{Type: "prior", Properties: prior}
	for _, raw := range []string{elicitationLimitSchema("minLength", "4294967296"), elicitationLimitSchema("maxItems", "18446744073709551616")} {
		if err := value.UnmarshalJSON([]byte(raw)); err == nil {
			t.Fatal("out-of-range limit accepted")
		}
		if value.Type != "prior" || reflect.ValueOf(value.Properties).Pointer() != reflect.ValueOf(prior).Pointer() || len(prior) != 1 {
			t.Fatal("invalid schema replaced receiver references")
		}
	}
	for _, mode := range []p.McpServerElicitationMode{p.McpServerElicitationModeOpenAIForm, p.McpServerElicitationModeOpenAIFormLegacy} {
		schema := elicitationLimitSchema("minItems", "18446744073709551616")
		body := strings.Replace(elicitationFormBody(mode, `"_meta":null`), `{"type":"object","properties":{}}`, schema, 1)
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		calls := 0
		client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(_ context.Context, params p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
			calls++
			if params.RequestedSchema != nil || string(params.OpenAIRequestedSchema) != schema {
				t.Error("OpenAI raw schema path changed")
			}
			return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel}, nil
		}})
		if _, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "openai"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(body)}); err != nil || calls != 1 {
			t.Fatalf("OpenAI arbitrary schema restricted: %v", err)
		}
	}
}

func FuzzElicitationIntegerWidths(f *testing.F) {
	for _, n := range []uint64{0, 1, 4294967295, 4294967296, 18446744073709551615} {
		for kind := uint8(0); kind < 4; kind++ {
			f.Add(n, int8(0), uint8(0), false, false, kind)
		}
	}
	f.Add(uint64(0), int8(1), uint8(8), false, true, uint8(2))
	f.Add(uint64(1), int8(-1), uint8(1), false, false, uint8(2))
	f.Add(uint64(1), int8(1), uint8(2), true, false, uint8(0))
	f.Fuzz(func(t *testing.T, n uint64, scale int8, tail uint8, fraction, negative bool, kind uint8) {
		mantissa := strconv.FormatUint(n, 10)
		if fraction {
			mantissa += ".5"
		}
		if n != 0 || fraction {
			mantissa += strings.Repeat("0", int(tail%9))
		}
		if negative {
			mantissa = "-" + mantissa
		}
		raw := mantissa + fmt.Sprintf("e%d", int(scale)%33)
		rational, ok := new(big.Rat).SetString(raw)
		if !ok {
			t.Fatal("generator produced invalid rational")
		}
		fields := []string{"minLength", "maxLength", "minItems", "maxItems"}
		field := fields[int(kind)%4]
		width := uint(32)
		if strings.HasSuffix(field, "Items") {
			width = 64
		}
		maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), width), big.NewInt(1))
		valid := rational.IsInt() && rational.Sign() >= 0 && rational.Num().Cmp(maximum) <= 0
		var value p.McpElicitationSchema
		err := json.Unmarshal([]byte(elicitationLimitSchema(field, raw)), &value)
		if !valid {
			if err == nil || value.Properties != nil {
				t.Fatalf("out-of-range/noninteger limit admitted: %s", raw)
			}
			return
		}
		if err != nil {
			t.Fatalf("schema-valid integer rejected: %s %v", raw, err)
		}
		number, ok := value.Properties["value"].(map[string]any)[field].(json.Number)
		if !ok || string(number) != raw {
			t.Fatalf("integer token changed: %s / %#v", raw, number)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var round p.McpElicitationSchema
		if err = json.Unmarshal(encoded, &round); err != nil || !reflect.DeepEqual(value, round) {
			t.Fatalf("integer limit roundtrip changed: %v", err)
		}
	})
}

func TestElicitationLargeDecimalSpellings(t *testing.T) {
	for _, base := range []string{"1", "18446744073709551615", "18446744073709551616"} {
		raw := base + strings.Repeat("0", 20000) + "e-20000"
		var schema p.McpElicitationSchema
		err := json.Unmarshal([]byte(elicitationLimitSchema("maxItems", raw)), &schema)
		if base == "18446744073709551616" {
			if err == nil {
				t.Fatal("large cancelling exponent hid uint64 overflow")
			}
			continue
		}
		if err != nil || schema.Properties["value"].(map[string]any)["maxItems"] != json.Number(raw) {
			t.Fatalf("valid large integer spelling rejected or changed: %v", err)
		}
	}
}
