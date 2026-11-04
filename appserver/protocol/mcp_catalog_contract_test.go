package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type catalogTarget struct{ owner, field string }

var catalogTargets = []catalogTarget{{"tool", "inputSchema"}, {"tool", "outputSchema"}, {"tool", "_meta"}, {"tool", "annotations"}, {"tool", "icons"}, {"resource", "_meta"}, {"resource", "annotations"}, {"resource", "icons"}, {"template", "annotations"}}

func decodeCatalogRecord(raw []byte, owner string) (any, error) {
	switch owner {
	case "tool":
		value := new(p.Tool)
		err := json.Unmarshal(raw, value)
		return value, err
	case "resource":
		value := new(p.Resource)
		err := json.Unmarshal(raw, value)
		return value, err
	default:
		value := new(p.ResourceTemplate)
		err := json.Unmarshal(raw, value)
		return value, err
	}
}
func catalogField(record any, field string) any {
	switch v := record.(type) {
	case *p.Tool:
		switch field {
		case "inputSchema":
			return v.InputSchema
		case "outputSchema":
			return v.OutputSchema
		case "_meta":
			return v.Meta
		case "annotations":
			return v.Annotations
		case "icons":
			return v.Icons
		}
	case *p.Resource:
		switch field {
		case "_meta":
			return v.Meta
		case "annotations":
			return v.Annotations
		case "icons":
			return v.Icons
		}
	case *p.ResourceTemplate:
		return v.Annotations
	}
	return nil
}
func catalogBase(owner string) map[string]any {
	switch owner {
	case "tool":
		return map[string]any{"name": "n", "inputSchema": nil}
	case "resource":
		return map[string]any{"name": "n", "uri": "relative/../opaque"}
	default:
		return map[string]any{"name": "n", "uriTemplate": "relative/{id}"}
	}
}

func TestMcpCatalogReceiverAndSelection(t *testing.T) {
	for _, target := range catalogTargets {
		prior := map[string]any{"n": json.Number("9007199254740993")}
		fields := catalogBase(target.owner)
		fields[target.field] = prior
		if target.field == "icons" {
			fields[target.field] = []any{prior}
		}
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		record, err := decodeCatalogRecord(raw, target.owner)
		if err != nil {
			t.Fatal(err)
		}
		before := reflect.ValueOf(record).Elem().Interface()
		ref := reflect.ValueOf(catalogField(record, target.field)).Pointer()
		bad := catalogBase(target.owner)
		bad["name"] = 17
		bad[target.field] = nil
		invalid, e := json.Marshal(bad)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(invalid, record); e == nil {
			t.Fatal("invalid name accepted")
		}
		if !reflect.DeepEqual(before, reflect.ValueOf(record).Elem().Interface()) || reflect.ValueOf(catalogField(record, target.field)).Pointer() != ref {
			t.Fatalf("%s/%s failed decode replaced receiver references", target.owner, target.field)
		}
		clean, e := json.Marshal(catalogBase(target.owner))
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(clean, record); e != nil || catalogField(record, target.field) != nil {
			t.Fatalf("%s/%s successful reuse retained old optional value: %v", target.owner, target.field, e)
		}
	}
	for _, target := range catalogTargets {
		base, err := json.Marshal(catalogBase(target.owner))
		if err != nil {
			t.Fatal(err)
		}
		values := `{"old":1},"` + target.field + `":{"n":9007199254740993}`
		want := any(map[string]any{"n": json.Number("9007199254740993")})
		if target.field == "icons" {
			values = `[1],"ICONS":[18446744073709551615]`
			want = []any{json.Number("18446744073709551615")}
		}
		// Duplicate top-level fields retain native final-interface selection.
		raw := append(append(append([]byte(nil), base[:len(base)-1]...), []byte(`,"`+target.field+`":`+values)...), '}')
		record, err := decodeCatalogRecord(raw, target.owner)
		if err != nil || !reflect.DeepEqual(catalogField(record, target.field), want) {
			t.Fatalf("duplicate selection changed for %s/%s: %v", target.owner, target.field, err)
		}
	}
	for _, raw := range []string{`{"name":"r","uri":"","size":-9223372036854775808,"_meta":1e400}`, `{"name":"r","uri":"","size":9223372036854775807,"_meta":1e400}`} {
		var resource p.Resource
		if err := json.Unmarshal([]byte(raw), &resource); err != nil || resource.Size == nil || resource.Meta != json.Number("1e400") {
			t.Fatalf("typed resource size or opaque URI changed: %v", err)
		}
	}
}

func TestMcpCatalogPublicRecoveryAndOwnership(t *testing.T) {
	for _, owner := range []string{"tool", "resource"} {
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		publish := func(icons json.RawMessage) (p.ListMcpServerStatusResponse, error) {
			record := catalogBase(owner)
			record["icons"] = icons
			server := map[string]any{"name": "s", "authStatus": "unsupported", "tools": map[string]any{}, "resources": []any{}, "resourceTemplates": []any{}}
			if owner == "tool" {
				server["tools"] = map[string]any{"n": record}
			} else {
				server["resources"] = []any{record}
			}
			if err := mock.SetResponseData("mcpServerStatus/list", map[string]any{"data": []any{server}}); err != nil {
				t.Fatal(err)
			}
			return client.Mcp.ListServerStatus(context.Background(), p.ListMcpServerStatusParams{})
		}
		failed, err := publish(json.RawMessage(`1e400`))
		if err == nil || !reflect.DeepEqual(failed, p.ListMcpServerStatusResponse{}) {
			t.Fatal("invalid icon root published")
		}
		first, err := publish(json.RawMessage(`[{"n":9007199254740993}]`))
		if err != nil {
			t.Fatal(err)
		}
		iconValue := func(response p.ListMcpServerStatusResponse) map[string]any {
			var icons any
			if owner == "tool" {
				icons = response.Data[0].Tools["n"].Icons
			} else {
				icons = response.Data[0].Resources[0].Icons
			}
			return icons.([]any)[0].(map[string]any)
		}
		if iconValue(first)["n"] != json.Number("9007199254740993") {
			t.Fatal("recovered icon number changed")
		}
		iconValue(first)["n"] = json.Number("2")
		next, err := client.Mcp.ListServerStatus(context.Background(), p.ListMcpServerStatusParams{})
		if err != nil || iconValue(next)["n"] != json.Number("9007199254740993") {
			t.Fatal("response mutation contaminated subsequent catalog")
		}
	}
}

func FuzzMcpCatalogNumbers(f *testing.F) {
	for kind := uint8(0); kind < 9; kind++ {
		for mode := uint8(0); mode < 6; mode++ {
			f.Add(int64(9007199254740993), int16(400), kind, mode, "text")
		}
	}
	f.Fuzz(func(t *testing.T, n int64, exponent int16, kind, mode uint8, text string) {
		if len(text) > 256 {
			text = text[:256]
		}
		quoted, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		var normalized string
		if err = json.Unmarshal(quoted, &normalized); err != nil {
			t.Fatal(err)
		}
		integer := json.Number(strconv.FormatInt(n, 10))
		exp := json.Number(fmt.Sprintf("1e%d", exponent))
		var want any
		switch mode % 6 {
		case 0:
			want = nil
		case 1:
			want = integer
		case 2:
			want = map[string]any{"const": integer, "nested": []any{exp, nil, true, normalized}}
		case 3:
			want = []any{integer, exp}
		case 4:
			want = normalized
		case 5:
			want = false
		}
		target := catalogTargets[int(kind)%len(catalogTargets)]
		if target.field == "icons" && want != nil {
			want = []any{want}
		}
		fields := catalogBase(target.owner)
		fields[target.field] = want
		if target.owner == "resource" {
			fields["size"] = n
		}
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		record, err := decodeCatalogRecord(raw, target.owner)
		if err != nil || !reflect.DeepEqual(catalogField(record, target.field), want) {
			t.Fatalf("exact catalog value changed at %s/%s: %v", target.owner, target.field, err)
		}
		if resource, ok := record.(*p.Resource); ok && (resource.Size == nil || *resource.Size != n) {
			t.Fatal("typed size changed numeric policy")
		}
		for i := range raw {
			raw[i] = ' '
		}
		if !reflect.DeepEqual(catalogField(record, target.field), want) {
			t.Fatal("catalog retains mutable input")
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		round, err := decodeCatalogRecord(encoded, target.owner)
		if err != nil || !reflect.DeepEqual(record, round) {
			t.Fatalf("catalog roundtrip changed values: %v", err)
		}
	})
}
