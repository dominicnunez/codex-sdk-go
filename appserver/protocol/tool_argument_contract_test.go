package protocol_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func toolArgumentRecord(kind, fields string) string {
	if kind == "request" {
		return `{"tool":"fixture","callId":"c","threadId":"t","turnId":"u",` + fields + `}`
	}
	return `{"type":"` + kind + `","id":"i","tool":"fixture","server":"fixture","status":"completed",` + fields + `}`
}

func decodeToolArguments(raw, kind string) (any, error) {
	if kind == "request" {
		var value p.DynamicToolCallParams
		err := value.UnmarshalJSON([]byte(raw))
		return value.Arguments, err
	}
	var item p.ThreadItemWrapper
	err := json.Unmarshal([]byte(raw), &item)
	if err != nil {
		return nil, err
	}
	switch value := item.Value.(type) {
	case *p.McpToolCallThreadItem:
		return value.Arguments, nil
	case *p.DynamicToolCallThreadItem:
		return value.Arguments, nil
	default:
		return nil, fmt.Errorf("unexpected item %T", value)
	}
}

func TestToolArgumentHistoryAndReceivers(t *testing.T) {
	for _, kind := range []string{"request", "mcpToolCall", "dynamicToolCall"} {
		for _, tc := range []struct{ fields, want string }{
			{`"arguments":{"old":1},"arguments":{"new":9007199254740993}`, `{"new":9007199254740993}`},
			{`"arguments":[1],"arguments":null`, `null`},
			{`"arguments":null,"arguments":[]`, `[]`},
			{`"arguments":null,"argum\u0065nts":18446744073709551615`, `18446744073709551615`},
			{`"arguments":{"ids":[9007199254740993],"ids":[-0,1e400]}`, `{"ids":[-0,1e400]}`},
		} {
			args, err := decodeToolArguments(toolArgumentRecord(kind, tc.fields), kind)
			if err != nil {
				t.Fatalf("%s: %v", kind, err)
			}
			encoded, err := json.Marshal(args)
			if err != nil || string(encoded) != tc.want {
				t.Fatalf("%s changed argument history: %s want %s: %v", kind, encoded, tc.want, err)
			}
		}
		// Request admission uses exact schema keys; item native aliases retain
		// their established matching, while canonical presence is still required.
		raw := toolArgumentRecord(kind, `"arguments":0,"ARGUMENTS":9007199254740993`)
		args, err := decodeToolArguments(raw, kind)
		if err != nil {
			t.Fatal(err)
		}
		want := "9007199254740993"
		if kind == "request" {
			want = "0"
		}
		if number, ok := args.(json.Number); !ok || string(number) != want {
			t.Fatalf("alias contract changed for %s: %#v", kind, args)
		}
		if _, err = decodeToolArguments(toolArgumentRecord(kind, `"ARGUMENTS":0`), kind); err == nil {
			t.Fatalf("missing canonical arguments accepted by %s", kind)
		}
	}
	prior := map[string]any{"n": json.Number("9007199254740993")}
	value := p.DynamicToolCallParams{Tool: "prior", Arguments: prior}
	if err := value.UnmarshalJSON([]byte(`{"arguments":1,"tool":"fixture","callId":null,"threadId":"t","turnId":"u"}`)); err == nil {
		t.Fatal("invalid call ID accepted")
	}
	if value.Tool != "prior" || reflect.ValueOf(value.Arguments).Pointer() != reflect.ValueOf(prior).Pointer() {
		t.Fatal("failed decode replaced receiver references")
	}
	if err := value.UnmarshalJSON([]byte(toolArgumentRecord("request", `"arguments":null`))); err != nil || value.Arguments != nil {
		t.Fatalf("null did not replace prior argument map: %v", err)
	}
	// The two plain item records retain ordinary JSON and embedding behavior.
	for _, value := range []any{new(p.McpToolCallThreadItem), new(p.DynamicToolCallThreadItem)} {
		if err := json.Unmarshal([]byte(`{"arguments":42}`), value); err != nil {
			t.Fatal(err)
		}
		field := reflect.ValueOf(value).Elem().FieldByName("Arguments").Interface()
		if _, ok := field.(float64); !ok {
			t.Fatalf("plain record changed decoder policy: %T", field)
		}
	}
}

func FuzzExactToolArguments(f *testing.F) {
	for _, n := range []int64{0, 9007199254740993, -9007199254740993} {
		for mode := uint8(0); mode < 5; mode++ {
			f.Add(n, int16(400), mode, "text")
		}
	}
	f.Fuzz(func(t *testing.T, n int64, exponent int16, mode uint8, text string) {
		if len(text) > 256 {
			text = text[:256]
		}
		// Valid structure and exact numeric tokens are constructed from the
		// contract, independent of the production decoder or float64 arithmetic.
		integer := json.Number(fmt.Sprint(n))
		exponential := json.Number(fmt.Sprintf("1e%d", exponent))
		quoted, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		var normalized string
		if err = json.Unmarshal(quoted, &normalized); err != nil {
			t.Fatal(err)
		}
		var want any
		switch mode % 5 {
		case 0:
			want = nil
		case 1:
			want = integer
		case 2:
			want = []any{integer, exponential, normalized, true}
		case 3:
			want = map[string]any{"ids": []any{integer, exponential}, "text": normalized}
		case 4:
			want = []any{}
		}
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"request", "mcpToolCall", "dynamicToolCall"} {
			got, err := decodeToolArguments(toolArgumentRecord(kind, `"arguments":`+string(raw)), kind)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("%s altered exact JSON: %v %#v / %#v", kind, err, got, want)
			}
			encoded, err := json.Marshal(got)
			if err != nil || strings.TrimSpace(string(encoded)) != string(raw) {
				t.Fatalf("%s roundtrip changed JSON: %s / %s %v", kind, encoded, raw, err)
			}
		}
	})
}

func TestToolArgumentEnvelopeAndUnrelatedFields(t *testing.T) {
	// Adding an argument policy at union admission must not add a JSON method
	// to plain records or consume an application's anonymous envelope.
	var embedded struct {
		p.McpToolCallThreadItem
		Extra int
	}
	if err := json.Unmarshal([]byte(`{"arguments":42,"Extra":7}`), &embedded); err != nil || embedded.Extra != 7 {
		t.Fatalf("plain envelope changed: %v", err)
	}
	raw := toolArgumentRecord("mcpToolCall", `"arguments":9007199254740993,"result":{"content":[],"structuredContent":{"n":42}}`)
	var item p.ThreadItemWrapper
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatal(err)
	}
	call := item.Value.(*p.McpToolCallThreadItem)
	if _, ok := call.Arguments.(json.Number); !ok {
		t.Fatal("argument policy missing")
	}
	if _, ok := call.Result.StructuredContent.(map[string]any)["n"].(float64); !ok {
		t.Fatal("unrelated result decoder policy changed")
	}
	wrapper := struct {
		Before int
		Params p.DynamicToolCallParams
		After  int
	}{}
	body := `{"Before":"bad","Params":` + toolArgumentRecord("request", `"arguments":9007199254740993`) + `,"After":7}`
	if err := json.Unmarshal([]byte(body), &wrapper); err == nil || wrapper.After != 7 {
		t.Fatalf("enclosing error traversal changed: %v", err)
	}
	if n, ok := wrapper.Params.Arguments.(json.Number); !ok || string(n) != "9007199254740993" {
		t.Fatal("enclosing traversal lost exact argument")
	}
}
