package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

type inboundObjectField struct {
	index []int
}

type inboundRequiredFields struct {
	order []string
	seen  map[string]bool
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

	if !json.Valid(data) {
		return decodeMalformedObject(data, destValue, fields, required, nonNull, validation)
	}
	if bytes.TrimSpace(data)[0] != '{' {
		return validation.notObject(fmt.Errorf("expected JSON object"))
	}
	state := inboundObjectDecodeState{dest: destValue, fields: fields, required: required, nonNull: nonNull, validation: validation}
	jsonobject.WalkFields(data, true, state.visit)
	state.flushStrings()
	if state.err != nil {
		return state.err
	}
	if err := validateRequiredInboundObjectFields(required, validation); err != nil {
		return err
	}
	return nil
}

type inboundObjectDecodeState struct {
	dest            reflect.Value
	fields          map[string]inboundObjectField
	required        inboundRequiredFields
	nonNull         map[string]struct{}
	validation      objectValidationErrors
	selectedStrings map[string][]byte
	err             error
}

// Visit recognized occurrences in wire order. Invalid known values remain
// visible even when a later duplicate would otherwise overwrite them.
func (s *inboundObjectDecodeState) visit(key, raw []byte) {
	if s.err != nil {
		return
	}
	name, recognized := inboundObjectKey(key, s.fields, s.required, s.nonNull)
	if !recognized {
		return
	}
	deferred, err := s.deferString(name, raw)
	if deferred {
		s.err = err
		return
	}
	s.err = decodeInboundObjectField(name, raw, s.required, s.nonNull, s.dest, s.fields, s.validation)
}

func (s *inboundObjectDecodeState) deferString(name string, raw []byte) (bool, error) {
	field, exists := s.fields[name]
	if !exists || !s.dest.IsValid() {
		return false, nil
	}
	value := s.dest.FieldByIndex(field.index)
	plain := value.Type() == reflect.TypeFor[string]()
	pointer := value.Type() == reflect.TypeFor[*string]()
	if !plain && !pointer {
		return false, nil
	}
	// Existing pointers may be aliased by direct helper callers. Preserve
	// observable mutations of that prior storage in original wire order.
	if pointer && !value.IsNil() {
		return false, nil
	}
	if _, exists := s.required.seen[name]; exists {
		s.required.seen[name] = true
	}
	if isNullJSONValue(raw) {
		if _, mustBeNonNull := s.nonNull[name]; mustBeNonNull {
			return true, s.validation.null(name)
		}
		if plain {
			return true, nil
		}
	} else if raw[0] != '"' {
		// Decode the prefix before reporting this occurrence's type failure.
		if previous, exists := s.selectedStrings[name]; exists {
			_ = json.Unmarshal(previous, value.Addr().Interface())
			delete(s.selectedStrings, name)
		}
		return true, json.Unmarshal(raw, value.Addr().Interface())
	}
	if s.selectedStrings == nil {
		s.selectedStrings = make(map[string][]byte)
	}
	s.selectedStrings[name] = raw
	return true, nil
}

func (s *inboundObjectDecodeState) flushStrings() {
	for name, raw := range s.selectedStrings {
		field := s.fields[name]
		if err := json.Unmarshal(raw, s.dest.FieldByIndex(field.index).Addr().Interface()); err != nil && s.err == nil {
			s.err = err
		}
	}
}

// Direct helper/receiver callers can pass syntactically invalid JSON. Preserve
// the original prefix mutation and error precedence there. Wire unmarshaling
// validates syntax before invoking receivers and uses the raw walk above.
func decodeMalformedObject(data []byte, dest reflect.Value, fields map[string]inboundObjectField, required inboundRequiredFields, nonNull map[string]struct{}, validation objectValidationErrors) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	start, err := decoder.Token()
	if err != nil {
		return validation.notObject(err)
	}
	if start != json.Delim('{') {
		return validation.notObject(fmt.Errorf("expected JSON object"))
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return validation.notObject(err)
		}
		name, ok := key.(string)
		if !ok {
			return validation.notObject(fmt.Errorf("expected object field name"))
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return validation.notObject(err)
		}
		if err := decodeInboundObjectField(name, raw, required, nonNull, dest, fields, validation); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil {
		return validation.notObject(err)
	}
	if end != json.Delim('}') {
		return validation.notObject(fmt.Errorf("expected JSON object end"))
	}
	if err := validateRequiredInboundObjectFields(required, validation); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return validation.notObject(fmt.Errorf("unexpected trailing data"))
		}
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
) (inboundRequiredFields, map[string]struct{}) {
	required := inboundRequiredFields{
		order: make([]string, 0, len(requiredFields)),
		seen:  make(map[string]bool, len(requiredFields)),
	}
	for _, field := range requiredFields {
		if _, ok := required.seen[field]; ok {
			continue
		}
		required.order = append(required.order, field)
		required.seen[field] = false
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

func inboundObjectKey(raw []byte, fields map[string]inboundObjectField, required inboundRequiredFields, nonNull map[string]struct{}) (string, bool) {
	for name := range fields {
		if jsonobject.FieldMatches(raw, name) {
			return name, true
		}
	}
	for name := range required.seen {
		if jsonobject.FieldMatches(raw, name) {
			return name, true
		}
	}
	for name := range nonNull {
		if jsonobject.FieldMatches(raw, name) {
			return name, true
		}
	}
	return "", false
}

func decodeInboundObjectField(
	key string,
	raw []byte,
	required inboundRequiredFields,
	nonNull map[string]struct{},
	destValue reflect.Value,
	fields map[string]inboundObjectField,
	validation objectValidationErrors,
) error {
	if _, ok := required.seen[key]; ok {
		required.seen[key] = true
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

func validateRequiredInboundObjectFields(required inboundRequiredFields, validation objectValidationErrors) error {
	for _, field := range required.order {
		seen := required.seen[field]
		if !seen {
			return validation.missing(field)
		}
	}
	return nil
}
