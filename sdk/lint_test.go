package codex

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestTypedNotificationListenersDispatchSpecMethods(t *testing.T) {
	var handlerErrors []error
	client := NewClient(&mockInternalTransport{}, WithHandlerErrorCallback(func(_ string, err error) {
		handlerErrors = append(handlerErrors, err)
	}))
	methodByType := loadServerNotificationMethodsByType(t)

	methods := reflect.TypeOf(client)

	for i := 0; i < methods.NumMethod(); i++ {
		method := methods.Method(i)
		if !isTypedNotificationMethod(method) {
			continue
		}

		handlerType := method.Type.In(1)
		notificationType := handlerType.In(0)
		notificationTypeName := notificationType.Name()
		notificationMethod, ok := methodByType[notificationTypeName]
		if !ok {
			t.Fatalf("%s handler type %s has no server notification spec method", method.Name, notificationTypeName)
		}

		called := 0
		handler := reflect.MakeFunc(handlerType, func(_ []reflect.Value) []reflect.Value {
			called++
			return nil
		})
		method.Func.Call([]reflect.Value{reflect.ValueOf(client), handler})

		params := sampleServerNotificationParams(t, notificationTypeName)
		handlerErrors = nil
		client.handleNotification(context.Background(), Notification{
			Method: notificationMethod,
			Params: params,
		})

		if called != 1 {
			if len(handlerErrors) > 0 {
				t.Fatalf("%s failed to decode %s sample params for %s: %v", method.Name, notificationMethod, notificationTypeName, handlerErrors[0])
			}
			t.Fatalf("%s did not dispatch %s notification to %s handler", method.Name, notificationMethod, notificationTypeName)
		}
	}
}

func isTypedNotificationMethod(method reflect.Method) bool {
	if !strings.HasPrefix(method.Name, "On") {
		return false
	}
	if method.Name == "OnNotification" || strings.HasPrefix(method.Name, "OnCollabToolCall") {
		return false
	}
	if method.Type.NumIn() != 2 || method.Type.NumOut() != 0 {
		return false
	}

	handlerType := method.Type.In(1)
	return handlerType.Kind() == reflect.Func && handlerType.NumIn() == 1 && handlerType.NumOut() == 0
}

type serverNotificationSpec struct {
	Definitions map[string]json.RawMessage `json:"definitions"`
	OneOf       []struct {
		Properties struct {
			Method struct {
				Enum []string `json:"enum"`
			} `json:"method"`
			Params struct {
				Ref string `json:"$ref"`
			} `json:"params"`
		} `json:"properties"`
	} `json:"oneOf"`
}

func loadServerNotificationSpec(t *testing.T) serverNotificationSpec {
	t.Helper()
	data, err := os.ReadFile("../specs/ServerNotification.json")
	if err != nil {
		t.Fatalf("read ServerNotification spec: %v", err)
	}
	var spec serverNotificationSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("unmarshal ServerNotification spec: %v", err)
	}
	return spec
}

func loadServerNotificationMethodsByType(t *testing.T) map[string]string {
	t.Helper()
	spec := loadServerNotificationSpec(t)
	methods := make(map[string]string, len(spec.OneOf))
	for _, variant := range spec.OneOf {
		if len(variant.Properties.Method.Enum) == 0 || variant.Properties.Params.Ref == "" {
			continue
		}
		methods[refName(variant.Properties.Params.Ref)] = variant.Properties.Method.Enum[0]
	}
	return methods
}

func sampleServerNotificationParams(t *testing.T, typeName string) json.RawMessage {
	t.Helper()
	spec := loadServerNotificationSpec(t)
	value := sampleSchemaValue(t, spec, typeName, map[string]bool{})
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal sample %s params: %v", typeName, err)
	}
	return data
}

func sampleSchemaValue(t *testing.T, spec serverNotificationSpec, typeName string, stack map[string]bool) interface{} {
	t.Helper()
	if stack[typeName] {
		return map[string]interface{}{}
	}
	raw, ok := spec.Definitions[typeName]
	if !ok {
		t.Fatalf("missing schema definition %s", typeName)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema definition %s: %v", typeName, err)
	}
	stack[typeName] = true
	value := sampleSchemaNode(t, spec, schema, stack)
	delete(stack, typeName)
	return value
}

func sampleSchemaNode(t *testing.T, spec serverNotificationSpec, schema interface{}, stack map[string]bool) interface{} {
	t.Helper()
	node, ok := schema.(map[string]interface{})
	if !ok {
		return nil
	}
	if ref, ok := node["$ref"].(string); ok {
		return sampleSchemaValue(t, spec, refName(ref), stack)
	}
	if enumValues, ok := node["enum"].([]interface{}); ok && len(enumValues) > 0 {
		return enumValues[0]
	}
	if variants, ok := node["anyOf"].([]interface{}); ok {
		return sampleFirstNonNullVariant(t, spec, variants, stack)
	}
	if variants, ok := node["oneOf"].([]interface{}); ok {
		return sampleFirstNonNullVariant(t, spec, variants, stack)
	}
	if variants, ok := node["allOf"].([]interface{}); ok {
		merged := make(map[string]interface{})
		for _, variant := range variants {
			value := sampleSchemaNode(t, spec, variant, stack)
			if value, ok := value.(map[string]interface{}); ok {
				for key, field := range value {
					merged[key] = field
				}
				continue
			}
			return value
		}
		return merged
	}

	switch schemaType(node["type"]) {
	case "object":
		return sampleObjectNode(t, spec, node, stack)
	case "array":
		return []interface{}{}
	case "boolean":
		return true
	case "integer", "number":
		return 1
	case "string":
		return "value"
	default:
		if _, ok := node["properties"]; ok {
			return sampleObjectNode(t, spec, node, stack)
		}
		return nil
	}
}

func sampleFirstNonNullVariant(t *testing.T, spec serverNotificationSpec, variants []interface{}, stack map[string]bool) interface{} {
	t.Helper()
	for _, variant := range variants {
		if node, ok := variant.(map[string]interface{}); ok && schemaType(node["type"]) == "null" {
			continue
		}
		return sampleSchemaNode(t, spec, variant, stack)
	}
	return nil
}

func sampleObjectNode(t *testing.T, spec serverNotificationSpec, node map[string]interface{}, stack map[string]bool) map[string]interface{} {
	t.Helper()
	result := make(map[string]interface{})
	properties, _ := node["properties"].(map[string]interface{})
	for _, field := range requiredSchemaFields(node) {
		prop, ok := properties[field]
		if !ok {
			result[field] = nil
			continue
		}
		if strings.HasSuffix(field, "Base64") {
			result[field] = ""
			continue
		}
		if field == "cwd" || strings.HasSuffix(field, "Path") || strings.HasSuffix(field, "path") {
			result[field] = "/tmp/value"
			continue
		}
		result[field] = sampleSchemaNode(t, spec, prop, stack)
	}
	return result
}

func requiredSchemaFields(node map[string]interface{}) []string {
	rawRequired, ok := node["required"].([]interface{})
	if !ok {
		return nil
	}
	required := make([]string, 0, len(rawRequired))
	for _, value := range rawRequired {
		if field, ok := value.(string); ok {
			required = append(required, field)
		}
	}
	return required
}

func schemaType(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []interface{}:
		for _, candidate := range typed {
			if asString, ok := candidate.(string); ok && asString != "null" {
				return asString
			}
		}
	}
	return ""
}

func refName(ref string) string {
	const prefix = "#/definitions/"
	return strings.TrimPrefix(ref, prefix)
}
