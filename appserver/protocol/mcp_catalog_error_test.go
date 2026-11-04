package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func legacyCatalogError(data []byte, kind string) error {
	switch kind {
	case "tool":
		if err := validateObjectFields(data, []string{"inputSchema", "name"}, []string{"name"}); err != nil {
			return err
		}
		type wire Tool
		var value wire
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		return validateOptionalFields(data, validateMcpCatalogIconArray, "icons")
	case "resource":
		if err := validateRequiredObjectFields(data, "name", "uri"); err != nil {
			return err
		}
		type wire Resource
		var value wire
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		return validateOptionalFields(data, validateMcpCatalogIconArray, "icons")
	default:
		if err := validateRequiredObjectFields(data, "name", "uriTemplate"); err != nil {
			return err
		}
		type wire ResourceTemplate
		var value wire
		return json.Unmarshal(data, &value)
	}
}

func TestMcpCatalogPriorErrors(t *testing.T) {
	for _, kind := range []string{"tool", "resource", "template"} {
		for _, raw := range []string{`[]`, `null`, `{}`, `{"name":null,"inputSchema":null,"uri":"u","uriTemplate":"t"}`, `{"name":17,"inputSchema":null,"uri":"u","uriTemplate":"t"}`, `{"name":"n","inputSchema":null,"uri":"u","uriTemplate":"t","title":17}`, `{"name":"n","inputSchema":null,"uri":"u","uriTemplate":"t","icons":42}`, `{"icons":1e400,"name":"n","inputSchema":null,"uri":"u","uriTemplate":"t"}`, `{"icons":1e400,"name":17,"inputSchema":null,"uri":"u","uriTemplate":"t"}`, `{"ICONS":1e400,"name":"n","inputSchema":null,"uri":"u","uriTemplate":"t","icons":[]}`, `{"name":"n","inputSchema":null,"uri":"u","uriTemplate":"t","size":"bad"}`, `{"name":`} {
			want := legacyCatalogError([]byte(raw), kind)
			if want == nil {
				continue
			} // size/icons are ignored by the template record.
			var got error
			switch kind {
			case "tool":
				var value Tool
				got = value.UnmarshalJSON([]byte(raw))
			case "resource":
				var value Resource
				got = value.UnmarshalJSON([]byte(raw))
			default:
				var value ResourceTemplate
				got = value.UnmarshalJSON([]byte(raw))
			}
			if got == nil || want.Error() != got.Error() {
				t.Fatalf("%s error changed for %s: %v / %v", kind, raw, want, got)
			}
			var a, b *json.UnmarshalTypeError
			if errors.As(want, &a) != errors.As(got, &b) || !reflect.DeepEqual(a, b) {
				t.Fatalf("%s native error fields changed: %#v / %#v", kind, a, b)
			}
		}
	}
}
