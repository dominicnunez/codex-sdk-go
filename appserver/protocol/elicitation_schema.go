package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// These rules describe only the checked-in MCP form schema. OpenAI form
// schemas and application-provided answers do not pass through this validator.
type elicitationRule func(json.RawMessage) error

var elicitationSchemaRule = elicitationObjectRule(map[string]elicitationRule{
	"$schema":    elicitationNullable(elicitationString),
	"type":       elicitationLiteral("object"),
	"properties": elicitationPropertyObject,
	"required":   elicitationNullable(elicitationList(elicitationString)),
}, true, "type", "properties")

var elicitationPrimitiveRules = buildElicitationRules()

func buildElicitationRules() []elicitationRule {
	stringList := elicitationList(elicitationString)
	option := elicitationObject(map[string]elicitationRule{"const": elicitationString, "title": elicitationString}, "const", "title")
	options := elicitationList(option)
	untitledItems := elicitationObject(map[string]elicitationRule{"type": elicitationLiteral("string"), "enum": stringList}, "type", "enum")
	titledItems := elicitationObject(map[string]elicitationRule{"anyOf": options}, "anyOf")
	// Each alternative has its own closed member set: sharing a broad wire
	// record would accidentally accept combinations that match no alternative.
	definitions := []struct {
		typeRule elicitationRule
		members  map[string]elicitationRule
		required []string
	}{
		{elicitationLiteral("string"), map[string]elicitationRule{"default": elicitationNullable(elicitationString), "format": elicitationNullable(elicitationLiteral("email", "uri", "date", "date-time")), "minLength": elicitationNullable(elicitationNatural), "maxLength": elicitationNullable(elicitationNatural)}, nil},
		{elicitationLiteral("number", "integer"), map[string]elicitationRule{"default": elicitationNullable(elicitationNumber), "minimum": elicitationNullable(elicitationNumber), "maximum": elicitationNullable(elicitationNumber)}, nil},
		{elicitationLiteral("boolean"), map[string]elicitationRule{"default": elicitationNullable(elicitationBoolean)}, nil},
		{elicitationLiteral("string"), map[string]elicitationRule{"default": elicitationNullable(elicitationString), "enum": stringList, "enumNames": elicitationNullable(stringList)}, []string{"enum"}},
		{elicitationLiteral("string"), map[string]elicitationRule{"default": elicitationNullable(elicitationString), "oneOf": options}, []string{"oneOf"}},
		{elicitationLiteral("array"), map[string]elicitationRule{"default": elicitationNullable(stringList), "items": untitledItems, "minItems": elicitationNullable(elicitationNatural), "maxItems": elicitationNullable(elicitationNatural)}, []string{"items"}},
		{elicitationLiteral("array"), map[string]elicitationRule{"default": elicitationNullable(stringList), "items": titledItems, "minItems": elicitationNullable(elicitationNatural), "maxItems": elicitationNullable(elicitationNatural)}, []string{"items"}},
	}
	rules := make([]elicitationRule, 0, len(definitions))
	for _, definition := range definitions {
		definition.members["type"] = definition.typeRule
		definition.members["title"] = elicitationNullable(elicitationString)
		definition.members["description"] = elicitationNullable(elicitationString)
		rules = append(rules, elicitationObject(definition.members, append([]string{"type"}, definition.required...)...))
	}
	return rules
}

// Validate after the original typed decode so its native and required errors
// retain precedence. Repeated properties dictionaries merge like the public Go
// map, while each property's object members use their existing last-value rule.
func validateElicitationSchema(data []byte) error {
	if err := elicitationSchemaRule(data); err != nil {
		return err
	}
	var wire struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := jsondecode.Unmarshal(data, &wire); err != nil {
		return err
	}
	for name, property := range wire.Properties {
		matched := false
		for _, rule := range elicitationPrimitiveRules {
			if rule(property) == nil {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("invalid MCP elicitation property %s", quotedValueDiagnostic(name))
		}
	}
	return nil
}

func elicitationPropertyObject(raw json.RawMessage) error {
	if len(raw) == 0 || raw[0] != '{' {
		return errors.New("expected properties object")
	}
	return nil
}

func elicitationObject(members map[string]elicitationRule, required ...string) elicitationRule {
	return elicitationObjectRule(members, false, required...)
}

func elicitationObjectRule(members map[string]elicitationRule, occurrences bool, required ...string) elicitationRule {
	return func(raw json.RawMessage) error {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || raw[0] != '{' {
			return errors.New("expected schema object")
		}
		seen := make(map[string]bool, len(required))
		var failure error
		check := func(name string, value []byte) {
			if failure != nil {
				return
			}
			rule, ok := members[name]
			if !ok {
				failure = fmt.Errorf("unsupported schema member %s", quotedValueDiagnostic(name))
				return
			}
			seen[name] = true
			if err := rule(value); err != nil {
				failure = fmt.Errorf("schema %s: %w", name, err)
			}
		}
		if occurrences {
			// The public root has typed fields, whose invalid occurrences cannot be
			// hidden by later duplicates. Its properties map is validated separately.
			jsonobject.WalkFields(raw, true, func(key, value []byte) {
				if failure != nil {
					return
				}
				var name string
				if err := jsondecode.Unmarshal(key, &name); err != nil {
					failure = err
					return
				}
				check(name, value)
			})
		} else {
			// Property definitions and option/item records live inside interface{}
			// map values. Preserve their final-member decoding instead of adding a
			// new occurrence policy at this previously untyped boundary.
			var fields map[string]json.RawMessage
			if err := jsondecode.Unmarshal(raw, &fields); err != nil {
				return err
			}
			for name, value := range fields {
				check(name, value)
			}
		}
		if failure != nil {
			return failure
		}
		for _, name := range required {
			if !seen[name] {
				return fmt.Errorf("missing schema %s", name)
			}
		}
		return nil
	}
}

func elicitationNullable(rule elicitationRule) elicitationRule {
	return func(raw json.RawMessage) error {
		if isNullJSONValue(raw) {
			return nil
		}
		return rule(raw)
	}
}

func elicitationList(item elicitationRule) elicitationRule {
	return func(raw json.RawMessage) error {
		if len(raw) == 0 || raw[0] != '[' {
			return errors.New("expected schema array")
		}
		var values []json.RawMessage
		if err := jsondecode.Unmarshal(raw, &values); err != nil {
			return err
		}
		for _, value := range values {
			if err := item(value); err != nil {
				return err
			}
		}
		return nil
	}
}

func elicitationLiteral(allowed ...string) elicitationRule {
	return func(raw json.RawMessage) error {
		if err := elicitationString(raw); err != nil {
			return err
		}
		maxBytes := 0
		for _, candidate := range allowed {
			if size := 6*len(candidate) + 2; size > maxBytes {
				maxBytes = size
			}
		}
		if len(raw) > maxBytes {
			return errors.New("unsupported schema literal")
		}
		var value string
		if err := jsondecode.Unmarshal(raw, &value); err != nil {
			return err
		}
		for _, candidate := range allowed {
			if value == candidate {
				return nil
			}
		}
		return errors.New("unsupported schema literal")
	}
}

func elicitationString(raw json.RawMessage) error {
	if len(raw) == 0 || raw[0] != '"' {
		return errors.New("expected schema string")
	}
	return nil
}

func elicitationBoolean(raw json.RawMessage) error {
	if !bytes.Equal(raw, []byte("true")) && !bytes.Equal(raw, []byte("false")) {
		return errors.New("expected schema boolean")
	}
	return nil
}

func elicitationNumber(raw json.RawMessage) error {
	if len(raw) == 0 || (raw[0] != '-' && (raw[0] < '0' || raw[0] > '9')) {
		return errors.New("expected schema number")
	}
	return nil
}

func elicitationNatural(raw json.RawMessage) error {
	if err := elicitationNumber(raw); err != nil {
		return err
	}
	// Work on the validated decimal spelling instead of expanding a power of
	// ten. Very small exponents can decode to zero in the public float64 map,
	// but still describe a fractional, schema-invalid length.
	spelling := string(raw)
	mantissa, exponent := spelling, int64(0)
	if index := strings.IndexAny(spelling, "eE"); index >= 0 {
		mantissa = spelling[:index]
		// ParseInt saturates on overflow. Either limit is beyond the number of
		// digits in this input, so its comparison below remains sufficient.
		exponent, _ = strconv.ParseInt(spelling[index+1:], 10, 64)
	}
	nonzero := false
	trailing := int64(0)
	fraction := int64(0)
	dot := false
	for _, digit := range mantissa {
		if digit == '.' {
			dot = true
			continue
		}
		if digit == '-' {
			continue
		}
		if dot {
			fraction++
		}
		if digit != '0' {
			nonzero = true
			trailing = 0
		} else {
			trailing++
		}
	}
	if !nonzero {
		return nil
	}
	if mantissa[0] == '-' || exponent < fraction-trailing {
		return errors.New("expected nonnegative schema integer")
	}
	return nil
}
