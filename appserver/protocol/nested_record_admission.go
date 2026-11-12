package protocol

import (
	"fmt"
	"strings"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// recordRule describes only required presence. Native decoding retains control
// of types, values and receiver updates; this scan does not copy semantic data.
type recordRule struct {
	required []string
	children map[string]recordRule
}
type recordPresence struct {
	present  bool
	seen     map[string]bool
	children map[string]*recordPresence
}

// validateNestedRecord follows native duplicate-object merges and null resets.
// Forbidden null occurrences remain errors even when a later member overwrites
// them. Requiredness is checked on the final merged record before publication.
func validateNestedRecord(data []byte, field string, rule recordRule) error {
	state := recordPresence{}
	if err := validateOptionalFields(data, func(_ string, raw []byte) error { return state.observe(raw, rule, field) }, field); err != nil {
		return err
	}
	return state.validate(rule, field)
}

func (s *recordPresence) observe(raw []byte, rule recordRule, path string) error {
	if isNullJSONValue(raw) {
		*s = recordPresence{}
		return nil
	}
	s.present = true
	if s.seen == nil {
		s.seen = make(map[string]bool)
	}
	var err error
	jsonobject.WalkFields(raw, true, func(key, value []byte) {
		if err != nil {
			return
		}
		for _, name := range rule.required {
			if jsonobject.FieldMatchesFolded(key, strings.ToLower(name)) {
				if isNullJSONValue(value) {
					err = fmt.Errorf("%s.%s must not be null", path, name)
					return
				}
				s.seen[name] = true
			}
		}
		for name, childRule := range rule.children {
			if !jsonobject.FieldMatchesFolded(key, strings.ToLower(name)) {
				continue
			}
			if s.children == nil {
				s.children = make(map[string]*recordPresence)
			}
			child := s.children[name]
			if child == nil {
				child = &recordPresence{}
				s.children[name] = child
			}
			err = child.observe(value, childRule, path+"."+name)
		}
	})
	return err
}
func (s *recordPresence) validate(rule recordRule, path string) error {
	if !s.present {
		return nil
	}
	for _, name := range rule.required {
		if !s.seen[name] {
			return fmt.Errorf("%s.%s is required", path, name)
		}
	}
	for name, childRule := range rule.children {
		if child := s.children[name]; child != nil {
			if err := child.validate(childRule, path+"."+name); err != nil {
				return err
			}
		}
	}
	return nil
}
