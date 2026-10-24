package protocol_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// The oracle interprets the checked-in JSON Schema rather than reproducing the
// production alternative table. Generated values have no duplicate members;
// separate deterministic tests exercise the decoder's occurrence semantics.
func FuzzElicitationSchemaContract(f *testing.F) {
	raw, err := os.ReadFile("schema/json/McpServerElicitationRequestParams.json")
	if err != nil {
		f.Fatal(err)
	}
	var contract map[string]any
	if err := json.Unmarshal(raw, &contract); err != nil {
		f.Fatal(err)
	}
	definitions := contract["definitions"].(map[string]any)
	root := definitions["McpElicitationSchema"].(map[string]any)
	for kind := uint8(0); kind < 7; kind++ {
		for mutation := uint8(0); mutation < 10; mutation++ {
			f.Add("a", int64(0), kind, mutation)
		}
	}
	f.Add("\x00日本語", int64(-1), uint8(3), uint8(0))
	f.Fuzz(func(t *testing.T, text string, number int64, kind, mutation uint8) {
		if len(text) > 256 {
			text = text[:256]
		}
		option := map[string]any{"const": text, "title": text}
		var property map[string]any
		switch kind % 7 {
		case 0:
			property = map[string]any{"type": "string", "default": text, "format": "uri", "minLength": number}
		case 1:
			property = map[string]any{"type": "number", "minimum": number, "default": number}
		case 2:
			property = map[string]any{"type": "boolean", "default": true}
		case 3:
			property = map[string]any{"type": "string", "enum": []any{text}, "enumNames": []any{text}}
		case 4:
			property = map[string]any{"type": "string", "oneOf": []any{option}}
		case 5:
			property = map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{text}}, "default": []any{text}}
		case 6:
			property = map[string]any{"type": "array", "items": map[string]any{"anyOf": []any{option}}, "maxItems": number}
		}
		schema := map[string]any{"type": "object", "properties": map[string]any{text: property}}
		switch mutation % 10 {
		case 1:
			property["extra"] = nil
		case 2:
			property["default"] = number
		case 3:
			property["default"] = []any{text, nil}
		case 4:
			property["type"] = text
		case 5:
			delete(property, "type")
		case 6:
			schema["required"] = []any{text, nil}
		case 7:
			property["description"] = text
			property["title"] = nil
		case 8:
			option["title"] = nil
		case 9:
			property["minLength"] = nil
		}
		data, err := json.Marshal(schema)
		if err != nil {
			t.Fatal(err)
		}
		var reference any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&reference); err != nil {
			t.Fatal(err)
		}
		expected := elicitationContractMatches(root, reference, definitions)
		var actual p.McpElicitationSchema
		err = json.Unmarshal(data, &actual)
		if (err == nil) != expected {
			t.Fatalf("schema contract disagreement expected=%v error=%v JSON=%s", expected, err, data)
		}
		if expected {
			encoded, err := json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatalf("accepted typed schema failed round trip: %v", err)
			}
		}
	})
}

func elicitationContractMatches(schema map[string]any, value any, definitions map[string]any) bool {
	if ref, ok := schema["$ref"].(string); ok {
		return elicitationContractMatches(definitions[strings.TrimPrefix(ref, "#/definitions/")].(map[string]any), value, definitions)
	}
	if choices, ok := schema["anyOf"].([]any); ok {
		for _, choice := range choices {
			if elicitationContractMatches(choice.(map[string]any), value, definitions) {
				return true
			}
		}
		return false
	}
	if literal, ok := schema["enum"].([]any); ok {
		found := false
		for _, candidate := range literal {
			found = found || reflect.DeepEqual(candidate, value)
		}
		if !found {
			return false
		}
	}
	var types []any
	switch kind := schema["type"].(type) {
	case string:
		types = []any{kind}
	case []any:
		types = kind
	}
	if len(types) > 0 {
		matched := false
		for _, kind := range types {
			matched = matched || elicitationContractType(kind.(string), value)
		}
		if !matched {
			return false
		}
	}
	if value == nil {
		return true
	}
	if minimum, ok := schema["minimum"].(float64); ok {
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		rat, _ := new(big.Rat).SetString(string(number))
		if rat.Cmp(new(big.Rat).SetFloat64(minimum)) < 0 {
			return false
		}
	}
	if object, ok := value.(map[string]any); ok {
		if required, ok := schema["required"].([]any); ok {
			for _, name := range required {
				if _, exists := object[name.(string)]; !exists {
					return false
				}
			}
		}
		properties, _ := schema["properties"].(map[string]any)
		for key, member := range object {
			if rule, known := properties[key]; known {
				if !elicitationContractMatches(rule.(map[string]any), member, definitions) {
					return false
				}
				continue
			}
			switch extra := schema["additionalProperties"].(type) {
			case bool:
				if !extra {
					return false
				}
			case map[string]any:
				if !elicitationContractMatches(extra, member, definitions) {
					return false
				}
			}
		}
	}
	if values, ok := value.([]any); ok {
		if items, ok := schema["items"].(map[string]any); ok {
			for _, item := range values {
				if !elicitationContractMatches(items, item, definitions) {
					return false
				}
			}
		}
	}
	return true
}

func elicitationContractType(kind string, value any) bool {
	switch kind {
	case "null":
		return value == nil
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "number":
		_, ok := value.(json.Number)
		return ok
	case "integer":
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		rat, ok := new(big.Rat).SetString(string(number))
		return ok && rat.IsInt()
	default:
		panic(fmt.Sprintf("unhandled schema type %s", kind))
	}
}
