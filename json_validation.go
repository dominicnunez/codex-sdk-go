package codex

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
)

type inboundObjectField struct {
	index []int
}

type objectValidationErrors struct {
	notObject func(error) error
	missing   func(string) error
	null      func(string) error
}

var inboundObjectFieldCache sync.Map

func validateInboundObjectFields(data []byte, requiredFields []string, nonNullFields []string) error {
	return decodeObjectWithValidation(data, nil, requiredFields, nonNullFields, inboundObjectValidationErrors())
}

func unmarshalInboundObject(data []byte, dest interface{}, requiredFields []string, nonNullFields []string) error {
	return decodeObjectWithValidation(data, dest, requiredFields, nonNullFields, inboundObjectValidationErrors())
}

func unmarshalResponseObject(data []byte, dest interface{}, requiredFields []string, nonNullFields []string) error {
	return decodeObjectWithValidation(data, dest, requiredFields, nonNullFields, responseObjectValidationErrors())
}

func decodeObjectWithValidation(
	data []byte,
	dest interface{},
	requiredFields []string,
	nonNullFields []string,
	validation objectValidationErrors,
) error {
	required, nonNull := inboundObjectValidation(requiredFields, nonNullFields)

	destValue, fields, handled, err := resolveInboundObjectDestination(data, dest)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	decoder := newInboundObjectDecoder(data)
	if err := expectInboundObjectStart(decoder); err != nil {
		return validation.notObject(err)
	}

	for decoder.More() {
		if err := decodeInboundObjectField(decoder, required, nonNull, destValue, fields, validation); err != nil {
			return err
		}
	}

	if err := expectInboundObjectEnd(decoder); err != nil {
		return validation.notObject(err)
	}
	if err := validateRequiredInboundObjectFields(required, validation); err != nil {
		return err
	}
	if err := expectNoTrailingInboundObjectData(decoder); err != nil {
		return validation.notObject(err)
	}

	return nil
}

func inboundObjectValidationErrors() objectValidationErrors {
	return objectValidationErrors{
		notObject: func(err error) error {
			return err
		},
		missing: func(field string) error {
			return fmt.Errorf("missing required field %q", field)
		},
		null: func(field string) error {
			return fmt.Errorf("required field %q must not be null", field)
		},
	}
}

func responseObjectValidationErrors() objectValidationErrors {
	return objectValidationErrors{
		notObject: func(err error) error {
			return fmt.Errorf("%w: %w", ErrResultNotObject, err)
		},
		missing: func(field string) error {
			return fmt.Errorf("%w %q", ErrMissingResultField, field)
		},
		null: func(field string) error {
			return fmt.Errorf("%w %q", ErrNullResultField, field)
		},
	}
}

func inboundObjectFields(typ reflect.Type) map[string]inboundObjectField {
	if cached, ok := inboundObjectFieldCache.Load(typ); ok {
		fields, ok := cached.(map[string]inboundObjectField)
		if ok {
			return fields
		}
	}

	fields := make(map[string]inboundObjectField)
	for _, field := range reflect.VisibleFields(typ) {
		if !field.IsExported() {
			continue
		}
		name, ok := inboundObjectFieldName(field)
		if !ok {
			continue
		}
		fields[name] = inboundObjectField{index: field.Index}
	}

	actual, _ := inboundObjectFieldCache.LoadOrStore(typ, fields)
	cachedFields, ok := actual.(map[string]inboundObjectField)
	if ok {
		return cachedFields
	}
	return fields
}

func inboundObjectFieldName(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	name, _, _ := strings.Cut(tag, ",")
	if name != "" {
		return name, true
	}
	if tag != "" {
		return field.Name, true
	}
	return field.Name, true
}

func inboundObjectValidation(
	requiredFields []string,
	nonNullFields []string,
) (map[string]bool, map[string]struct{}) {
	required := make(map[string]bool, len(requiredFields))
	for _, field := range requiredFields {
		required[field] = false
	}

	nonNull := make(map[string]struct{}, len(nonNullFields))
	for _, field := range nonNullFields {
		nonNull[field] = struct{}{}
	}

	return required, nonNull
}

func resolveInboundObjectDestination(
	data []byte,
	dest interface{},
) (reflect.Value, map[string]inboundObjectField, bool, error) {
	if dest == nil {
		return reflect.Value{}, nil, false, nil
	}

	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return reflect.Value{}, nil, false, fmt.Errorf("destination must be a non-nil pointer")
	}

	destValue := value.Elem()
	if destValue.Kind() != reflect.Struct {
		return reflect.Value{}, nil, true, json.Unmarshal(data, dest)
	}

	return destValue, inboundObjectFields(destValue.Type()), false, nil
}

func newInboundObjectDecoder(data []byte) *json.Decoder {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder
}

func expectInboundObjectStart(decoder *json.Decoder) error {
	start, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := start.(json.Delim)
	if !ok || delim != '{' {
		return fmt.Errorf("expected JSON object")
	}
	return nil
}

func decodeInboundObjectField(
	decoder *json.Decoder,
	required map[string]bool,
	nonNull map[string]struct{},
	destValue reflect.Value,
	fields map[string]inboundObjectField,
	validation objectValidationErrors,
) error {
	keyToken, err := decoder.Token()
	if err != nil {
		return validation.notObject(err)
	}
	key, ok := keyToken.(string)
	if !ok {
		return validation.notObject(fmt.Errorf("expected object field name"))
	}

	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return validation.notObject(err)
	}

	if _, ok := required[key]; ok {
		required[key] = true
	}
	if _, mustBeNonNull := nonNull[key]; mustBeNonNull && isNullJSONValue(raw) {
		return validation.null(key)
	}
	if !destValue.IsValid() {
		return nil
	}

	field, ok := fields[key]
	if !ok {
		return nil
	}
	return json.Unmarshal(raw, destValue.FieldByIndex(field.index).Addr().Interface())
}

func expectInboundObjectEnd(decoder *json.Decoder) error {
	end, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := end.(json.Delim)
	if !ok || delim != '}' {
		return fmt.Errorf("expected JSON object end")
	}
	return nil
}

func validateRequiredInboundObjectFields(required map[string]bool, validation objectValidationErrors) error {
	for field, seen := range required {
		if !seen {
			return validation.missing(field)
		}
	}
	return nil
}

func expectNoTrailingInboundObjectData(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("unexpected trailing data")
		}
		return err
	}
	return nil
}
