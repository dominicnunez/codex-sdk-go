package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// Public records stay method-free. A single native projection over an
// equal-length view preserves field context, offsets and partial updates while
// hiding only inactive values that their public Go fields cannot represent.
func decodeAgentMessageRecord(data []byte, dest *AgentMessageThreadItem) error {
	type base AgentMessageThreadItem
	if err := jsondecode.Unmarshal(data, (*base)(dest)); err != nil {
		return restoreDisplayTypeError(err)
	}
	return nil
}
func restoreDisplayTypeError(err error) error {
	typed, ok := err.(*json.UnmarshalTypeError) //nolint:errorlint // Transform only directly owned native errors; wrapped errors retain their identity and chain.
	if !ok {
		return err
	}
	if typed.Struct != "base" {
		return err
	}
	adjusted := *typed
	adjusted.Struct = "AgentMessageThreadItem"
	return &adjusted
}

func validateAgentDisplayAdmission(view agentDisplayView, item *AgentMessageThreadItem) error {
	if view.legacy {
		return validateNestedRecord(view.data, "moderation", recordRule{required: []string{"moderated"}, children: map[string]recordRule{"display": {required: []string{"action"}}}})
	}
	if err := validateEstablishedDisplayFields(view); err != nil {
		return err
	}
	if err := validateNestedRecord(view.source, "moderation", recordRule{required: []string{"moderated"}}); err != nil {
		return err
	}
	if _, err := validateDisplayModeHistory(view); err != nil {
		return err
	}
	if item.Moderation == nil || item.Moderation.Display == nil {
		return nil
	}
	d := item.Moderation.Display
	presence := agentDisplayPresence(view.data)
	if d.Mode == "" {
		if !presence.action {
			return fmt.Errorf("moderation.display.action is required")
		}
		return nil
	}
	if d.Version == nil || *d.Version != 1 {
		return fmt.Errorf("display.version must be 1")
	}
	switch d.Mode {
	case AgentMessageDisplayModeShow:
		if d.TrustedContacts == nil || (*d.TrustedContacts != TrustedContactsCheck && *d.TrustedContacts != TrustedContactsSkip) {
			return fmt.Errorf("display.trustedContacts is required and must be check or skip")
		}
		if a := d.Annotation; a != nil && (a.Kind != "wellbeingHelpline" || (a.Subject != HelplineSubjectAnswer && a.Subject != HelplineSubjectPrompt)) {
			return fmt.Errorf("display.annotation requires wellbeingHelpline kind and answer or prompt subject")
		}
	case AgentMessageDisplayModeHide:
		if d.Replacement == nil || d.Replacement.Kind != "wellbeingPromptHelpline" {
			return fmt.Errorf("display.replacement requires wellbeingPromptHelpline kind")
		}
	case AgentMessageDisplayModeBlock:
		if d.Reason == nil {
			return fmt.Errorf("display.reason is required")
		}
		switch d.Reason.Kind {
		case "protection":
			if !presence.protectionType {
				return fmt.Errorf("display.reason.protectionType is required")
			}
		case "cyber":
		default:
			return fmt.Errorf("invalid display.reason.kind %s", quotedValueDiagnostic(d.Reason.Kind))
		}
	default:
		return fmt.Errorf("invalid display.mode %s", quotedValueDiagnostic(string(d.Mode)))
	}
	return nil
}

// Only these required strings admit the zero value. Native pointer and enum
// state establishes every other selected field's presence. Null strings are
// native no-ops; owning pointer resets clear the corresponding presence bits.
type displayPresence struct{ action, protectionType bool }

func agentDisplayPresence(data []byte) displayPresence {
	var state displayPresence
	_ = walkAgentDisplayEvents(data, func(display []byte, _ int, _ string, reset bool) error {
		if reset {
			state = displayPresence{}
			return nil
		}
		return walkDisplayObject(display, 0, func(key, value []byte, _ int) error {
			if jsonobject.FieldMatchesFolded(key, "action") && !isNullJSONValue(value) {
				state.action = true
			}
			if !jsonobject.FieldMatchesFolded(key, "reason") {
				return nil
			}
			if isNullJSONValue(value) {
				state.protectionType = false
				return nil
			}
			return walkDisplayObject(value, 0, func(key, value []byte, _ int) error {
				if jsonobject.FieldMatchesFolded(key, "protectiontype") && !isNullJSONValue(value) {
					state.protectionType = true
				}
				return nil
			})
		})
	})
	return state
}

// A legacy display has no mode discriminator. New field names are then open
// extras. Explicit constraints of a current active segment survive later null
// resets, while missing requiredness is evaluated only on the final record.
type displayHistoryState struct {
	cursor          displayCursor
	current, reason string
	sawMode         bool
}

func validateDisplayModeHistory(view agentDisplayView) (bool, error) {
	state := displayHistoryState{cursor: displayCursor{view: view}}
	err := walkAgentDisplayEvents(view.source, state.visitDisplay)
	return state.sawMode, err
}
func (s *displayHistoryState) visitDisplay(display []byte, offset int, path string, reset bool) error {
	if reset {
		s.cursor.reset()
		s.current, s.reason = "", ""
		return nil
	}
	if s.cursor.start() {
		s.current = s.cursor.context().mode
		s.reason = ""
	}
	if err := walkDisplayObject(display, offset, func(key, value []byte, at int) error { return s.selectMode(key, value, at, path) }); err != nil {
		return err
	}
	return walkDisplayObject(display, offset, func(key, value []byte, at int) error { return s.visitField(key, value, at, path) })
}
func (s *displayHistoryState) selectMode(key, value []byte, at int, path string) error {
	if !jsonobject.FieldMatchesFolded(key, "mode") {
		return nil
	}
	v, err := displayStringAt[AgentMessageDisplayMode](value, at, path+"."+displayKeyName(key), "show", "hide", "block")
	if err != nil {
		return err
	}
	s.current = string(v)
	s.sawMode = true
	return nil
}
func (s *displayHistoryState) visitField(key, value []byte, at int, path string) error {
	field := path + "." + displayKeyName(key)
	if jsonobject.FieldMatchesFolded(key, "reason") {
		s.observeReason(value)
	}
	if s.current == "" {
		if !jsonobject.FieldMatchesFolded(key, "action") {
			return nil
		}
		if isNullJSONValue(value) {
			return fmt.Errorf("%s must not be null", field)
		}
		_, err := decodeDisplayAt[string](value, at, field)
		return err
	}
	if jsonobject.FieldMatchesFolded(key, "version") {
		v, err := decodeDisplayAt[*uint8](value, at, field)
		if err != nil {
			return err
		}
		if v == nil || *v != 1 {
			return fmt.Errorf("display.version must be 1")
		}
		return nil
	}
	switch s.current {
	case "show":
		return s.checkShow(key, value, at, field)
	case "hide":
		return s.checkHide(key, value, at, field)
	case "block":
		return s.checkBlock(key, value, at, field)
	}
	return nil
}
func (s *displayHistoryState) observeReason(value []byte) {
	if isNullJSONValue(value) {
		s.cursor.reason = -1
		s.reason = ""
		return
	}
	if s.cursor.startReason() {
		s.reason = s.cursor.reasonKind()
	}
	s.reason = scanDisplaySelector(value, "kind", s.reason, "protection", "cyber")
}
func (s *displayHistoryState) checkShow(key, value []byte, at int, field string) error {
	if jsonobject.FieldMatchesFolded(key, "trustedcontacts") {
		_, err := displayStringAt[TrustedContacts](value, at, field, "check", "skip")
		return err
	}
	if !jsonobject.FieldMatchesFolded(key, "annotation") || isNullJSONValue(value) {
		return nil
	}
	if _, err := decodeDisplayAt[AnswerAnnotation](value, at, field); err != nil {
		return err
	}
	return walkDisplayObject(value, at, func(key, value []byte, at int) error {
		field := field + "." + displayKeyName(key)
		if jsonobject.FieldMatchesFolded(key, "kind") {
			_, err := displayStringAt[string](value, at, field, "wellbeingHelpline")
			return err
		}
		if jsonobject.FieldMatchesFolded(key, "subject") {
			_, err := displayStringAt[HelplineSubject](value, at, field, "answer", "prompt")
			return err
		}
		return nil
	})
}
func (s *displayHistoryState) checkHide(key, value []byte, at int, field string) error {
	if !jsonobject.FieldMatchesFolded(key, "replacement") {
		return nil
	}
	if isNullJSONValue(value) {
		return fmt.Errorf("%s must not be null", field)
	}
	if _, err := decodeDisplayAt[AnswerReplacement](value, at, field); err != nil {
		return err
	}
	return walkDisplayObject(value, at, func(key, value []byte, at int) error {
		if !jsonobject.FieldMatchesFolded(key, "kind") {
			return nil
		}
		_, err := displayStringAt[string](value, at, field+"."+displayKeyName(key), "wellbeingPromptHelpline")
		return err
	})
}
func (s *displayHistoryState) checkBlock(key, value []byte, at int, field string) error {
	if !jsonobject.FieldMatchesFolded(key, "reason") {
		return nil
	}
	if isNullJSONValue(value) {
		return fmt.Errorf("%s must not be null", field)
	}
	if len(value) > 0 && value[0] != '{' {
		_, err := decodeDisplayAt[BlockReason](value, at, field)
		return err
	}
	if err := walkDisplayObject(value, at, func(key, value []byte, at int) error {
		if !jsonobject.FieldMatchesFolded(key, "kind") {
			return nil
		}
		v, err := displayStringAt[string](value, at, field+"."+displayKeyName(key), "protection", "cyber")
		if err != nil {
			return err
		}
		s.reason = v
		return nil
	}); err != nil {
		return err
	}
	return walkDisplayObject(value, at, func(key, value []byte, at int) error { return s.checkReasonLeaf(key, value, at, field) })
}
func (s *displayHistoryState) checkReasonLeaf(key, value []byte, at int, parent string) error {
	field := parent + "." + displayKeyName(key)
	if s.reason == "protection" && jsonobject.FieldMatchesFolded(key, "protectiontype") {
		if isNullJSONValue(value) {
			return fmt.Errorf("%s must not be null", field)
		}
		_, err := decodeDisplayAt[string](value, at, field)
		return err
	}
	if s.reason == "cyber" && jsonobject.FieldMatchesFolded(key, "trustedaccess") {
		_, err := decodeDisplayAt[*bool](value, at, field)
		return err
	}
	return nil
}

// Established explicit-null failures retain their original wire order before
// final requiredness and the newly introduced display restrictions run.
func validateEstablishedDisplayFields(view agentDisplayView) error {
	cursor := displayCursor{view: view}
	mode := ""
	err := walkDisplayObject(view.source, 0, func(key, raw []byte, offset int) error {
		if !jsonobject.FieldMatchesFolded(key, "moderation") {
			return nil
		}
		if isNullJSONValue(raw) {
			cursor.reset()
			mode = ""
			return nil
		}
		return walkDisplayObject(raw, offset, func(key, display []byte, offset int) error {
			if jsonobject.FieldMatchesFolded(key, "moderated") && isNullJSONValue(display) {
				return fmt.Errorf("moderation.moderated must not be null")
			}
			if !jsonobject.FieldMatchesFolded(key, "display") {
				return nil
			}
			if isNullJSONValue(display) {
				cursor.reset()
				mode = ""
				return nil
			}
			if cursor.start() {
				mode = cursor.context().mode
			}
			mode = scanDisplaySelector(display, "mode", mode, "show", "hide", "block")
			if mode != "" {
				return nil
			}
			return walkDisplayObject(display, offset, func(key, value []byte, _ int) error {
				if jsonobject.FieldMatchesFolded(key, "action") && isNullJSONValue(value) {
					return fmt.Errorf("moderation.display.action must not be null")
				}
				return nil
			})
		})
	})
	if err != nil {
		return fmt.Errorf("moderation: %w", err)
	}
	return nil
}

// The input has already passed native syntax/type admission. Locations are kept
// alongside borrowed slices so native type errors retain original offsets and
// complete field paths even when a later reset erased the bad active record.
func walkDisplayObject(data []byte, base int, visit func([]byte, []byte, int) error) error {
	i := jsonobject.SkipWhitespace(data, 0)
	if i >= len(data) || data[i] != '{' {
		return nil
	}
	i = jsonobject.SkipWhitespace(data, i+1)
	for i < len(data) && data[i] != '}' {
		end, ok := jsonobject.StringEnd(data, i)
		if !ok {
			return nil
		}
		key := data[i:end]
		i = jsonobject.SkipWhitespace(data, end)
		if i >= len(data) || data[i] != ':' {
			return nil
		}
		i = jsonobject.SkipWhitespace(data, i+1)
		end, ok = jsonobject.ValueEnd(data, i)
		if !ok {
			return nil
		}
		if err := visit(key, data[i:end], base+i); err != nil {
			return err
		}
		i = jsonobject.SkipWhitespace(data, end)
		if i >= len(data) || data[i] != ',' {
			return nil
		}
		i = jsonobject.SkipWhitespace(data, i+1)
	}
	return nil
}
func displayKeyName(raw []byte) string { var key string; _ = json.Unmarshal(raw, &key); return key }
func decodeDisplayAt[T any](raw []byte, offset int, field string) (T, error) {
	var value T
	err := jsondecode.Unmarshal(raw, &value)
	if err == nil {
		return value, nil
	}
	err = restoreDisplayTypeError(err)
	if typed, ok := err.(*json.UnmarshalTypeError); ok { //nolint:errorlint // Transform only directly owned native errors; wrapped errors retain their identity and chain.
		adjusted := *typed
		adjusted.Offset += int64(offset)
		adjusted.Struct = "AgentMessageThreadItem"
		if adjusted.Field != "" {
			field += "." + adjusted.Field
		}
		adjusted.Field = field
		return value, &adjusted
	}
	return value, err
}
func displayStringAt[T ~string](raw []byte, offset int, field string, allowed ...string) (T, error) {
	var zero T
	if isNullJSONValue(raw) {
		return zero, fmt.Errorf("%s must not be null", field)
	}
	value, err := decodeDisplayAt[T](raw, offset, field)
	if err != nil {
		return zero, err
	}
	for _, candidate := range allowed {
		if string(value) == candidate {
			return value, nil
		}
	}
	return zero, fmt.Errorf("invalid %s %s", field, quotedValueDiagnostic(string(value)))
}

type displayContext struct {
	mode    string
	reasons []string
}
type agentDisplayView struct {
	source, data []byte
	contexts     []displayContext
	legacy       bool
}
type displayCursor struct {
	view                            agentDisplayView
	index, next, reason, nextReason int
	live                            bool
}

func (c *displayCursor) reset() { c.live = false; c.reason = -1; c.nextReason = 0 }
func (c *displayCursor) start() bool {
	if c.live {
		return false
	}
	c.live = true
	c.index = c.next
	c.next++
	c.reason = -1
	c.nextReason = 0
	return true
}
func (c *displayCursor) context() *displayContext { return &c.view.contexts[c.index] }
func (c *displayCursor) startReason() bool {
	if c.reason >= 0 {
		return false
	}
	c.reason = c.nextReason
	c.nextReason++
	return true
}
func (c *displayCursor) reasonKind() string {
	if c.reason < len(c.context().reasons) {
		return c.context().reasons[c.reason]
	}
	return ""
}

// Events identify live display records without retaining borrowed input spans.
// Only small selector metadata is retained per null-delimited record; both
// traversal and retained metadata grow linearly with the bounded input.
func walkAgentDisplayEvents(data []byte, visit func([]byte, int, string, bool) error) error {
	return walkDisplayObject(data, 0, func(key, raw []byte, offset int) error {
		if !jsonobject.FieldMatchesFolded(key, "moderation") {
			return nil
		}
		if isNullJSONValue(raw) {
			return visit(nil, offset, "", true)
		}
		path := displayKeyName(key)
		return walkDisplayObject(raw, offset, func(key, value []byte, offset int) error {
			if !jsonobject.FieldMatchesFolded(key, "display") {
				return nil
			}
			return visit(value, offset, path+"."+displayKeyName(key), isNullJSONValue(value))
		})
	})
}
func scanDisplaySelector(data []byte, name string, prior string, allowed ...string) string {
	_ = walkDisplayObject(data, 0, func(key, value []byte, _ int) error {
		if !jsonobject.FieldMatchesFolded(key, name) {
			return nil
		}
		var v string
		decoded := jsondecode.Unmarshal(value, &v) == nil
		prior = ""
		if decoded {
			for _, choice := range allowed {
				if v == choice {
					prior = v
					break
				}
			}
		}
		return nil
	})
	return prior
}
func makeAgentDisplayView(data []byte) agentDisplayView {
	view := agentDisplayView{source: data, data: data, legacy: true}
	live := -1
	_ = walkAgentDisplayEvents(data, func(raw []byte, _ int, _ string, reset bool) error {
		if reset {
			live = -1
			return nil
		}
		if live < 0 {
			view.contexts = append(view.contexts, displayContext{})
			live = len(view.contexts) - 1
		}
		if view.contexts[live].mode == "" {
			view.contexts[live].mode = scanDisplaySelector(raw, "mode", "", "show", "hide", "block")
		}
		_ = walkDisplayObject(raw, 0, func(key, _ []byte, _ int) error {
			if jsonobject.FieldMatchesFolded(key, "mode") {
				view.legacy = false
			}
			return nil
		})
		return nil
	})
	cursor := displayCursor{view: view}
	_ = walkAgentDisplayEvents(data, func(raw []byte, _ int, _ string, reset bool) error {
		if reset {
			cursor.reset()
			return nil
		}
		cursor.start()
		return walkDisplayObject(raw, 0, func(key, value []byte, _ int) error {
			if !jsonobject.FieldMatchesFolded(key, "reason") {
				return nil
			}
			if isNullJSONValue(value) {
				cursor.reason = -1
				return nil
			}
			if cursor.startReason() {
				cursor.context().reasons = append(cursor.context().reasons, "")
			}
			if cursor.context().reasons[cursor.reason] == "" {
				cursor.context().reasons[cursor.reason] = scanDisplaySelector(value, "kind", "", "protection", "cyber")
			}
			return nil
		})
	})
	view.data = filterAgentDisplayFields(view)
	return view
}

// Inactive known properties are open extras. Replace only their key bytes in
// native projections so they cannot leave sticky errors across record resets.
// Active values retain original offsets; the single copy is made lazily.
type displayFieldFilter struct {
	view         agentDisplayView
	cursor       displayCursor
	filtered     []byte
	mode, reason string
}

func filterAgentDisplayFields(view agentDisplayView) []byte {
	state := displayFieldFilter{view: view, cursor: displayCursor{view: view}}
	_ = walkAgentDisplayEvents(view.source, state.visitDisplay)
	if state.filtered != nil {
		return state.filtered
	}
	return view.source
}
func (s *displayFieldFilter) suppress(key []byte, offset int) {
	if s.filtered == nil {
		s.filtered = append([]byte(nil), s.view.source...)
	}
	for i := 1; i < len(key)-1; i++ {
		s.filtered[offset+i] = '_'
	}
}
func (s *displayFieldFilter) filterLeaf(key, value []byte, offset int, kind byte) {
	var err error
	switch kind {
	case 's':
		var v string
		err = jsondecode.Unmarshal(value, &v)
	case 'b':
		var v *bool
		err = jsondecode.Unmarshal(value, &v)
	case 'u':
		var v *uint8
		err = jsondecode.Unmarshal(value, &v)
	case 'p':
		var v *string
		err = jsondecode.Unmarshal(value, &v)
	}
	if err != nil {
		s.suppress(key, offset)
	}
}
func (s *displayFieldFilter) visitDisplay(display []byte, offset int, _ string, reset bool) error {
	if reset {
		s.cursor.reset()
		s.mode, s.reason = "", ""
		return nil
	}
	if s.cursor.start() {
		s.mode = s.cursor.context().mode
		s.reason = ""
	}
	s.mode = scanDisplaySelector(display, "mode", s.mode, "show", "hide", "block")
	return walkDisplayKeys(display, offset, s.visitKey)
}
func (s *displayFieldFilter) visitKey(key, value []byte, keyOffset, valueOffset int) error {
	switch {
	case jsonobject.FieldMatchesFolded(key, "version"):
		if s.mode == "" {
			s.filterLeaf(key, value, keyOffset, 'u')
		}
	case jsonobject.FieldMatchesFolded(key, "trustedcontacts"):
		if s.mode != "show" {
			s.filterLeaf(key, value, keyOffset, 'p')
		}
	case jsonobject.FieldMatchesFolded(key, "action"):
		if s.mode != "" {
			s.filterLeaf(key, value, keyOffset, 's')
		}
	case jsonobject.FieldMatchesFolded(key, "blocktype"):
		if s.mode != "" {
			s.filterLeaf(key, value, keyOffset, 'p')
		}
	case jsonobject.FieldMatchesFolded(key, "checktrustedcontacts"), jsonobject.FieldMatchesFolded(key, "cybertrustedaccess"):
		if s.mode != "" {
			s.filterLeaf(key, value, keyOffset, 'b')
		}
	case jsonobject.FieldMatchesFolded(key, "annotation"):
		return s.filterInactiveObject(key, value, keyOffset, valueOffset, s.mode == "show", true)
	case jsonobject.FieldMatchesFolded(key, "replacement"):
		return s.filterInactiveObject(key, value, keyOffset, valueOffset, s.mode == "hide", false)
	case jsonobject.FieldMatchesFolded(key, "reason"):
		return s.filterReason(key, value, keyOffset, valueOffset)
	}
	return nil
}
func (s *displayFieldFilter) filterInactiveObject(key, value []byte, keyOffset, valueOffset int, active, hasSubject bool) error {
	if active || isNullJSONValue(value) {
		return nil
	}
	if len(value) == 0 || value[0] != '{' {
		s.suppress(key, keyOffset)
		return nil
	}
	return walkDisplayKeys(value, valueOffset, func(key, value []byte, at, _ int) error {
		if jsonobject.FieldMatchesFolded(key, "kind") || hasSubject && jsonobject.FieldMatchesFolded(key, "subject") {
			s.filterLeaf(key, value, at, 's')
		}
		return nil
	})
}
func (s *displayFieldFilter) filterReason(key, value []byte, keyOffset, valueOffset int) error {
	if isNullJSONValue(value) {
		s.cursor.reason = -1
		s.reason = ""
		return nil
	}
	if s.cursor.startReason() {
		s.reason = s.cursor.reasonKind()
	}
	s.reason = scanDisplaySelector(value, "kind", s.reason, "protection", "cyber")
	if len(value) == 0 || value[0] != '{' {
		if s.mode != "block" {
			s.suppress(key, keyOffset)
		}
		return nil
	}
	return walkDisplayKeys(value, valueOffset, s.reasonLeaf)
}
func (s *displayFieldFilter) reasonLeaf(key, value []byte, at, _ int) error {
	switch {
	case jsonobject.FieldMatchesFolded(key, "kind"):
		if s.mode != "block" {
			s.filterLeaf(key, value, at, 's')
		}
	case jsonobject.FieldMatchesFolded(key, "protectiontype"):
		if s.mode != "block" || s.reason != "protection" {
			s.filterLeaf(key, value, at, 's')
		}
	case jsonobject.FieldMatchesFolded(key, "trustedaccess"):
		if s.mode != "block" || s.reason != "cyber" {
			s.filterLeaf(key, value, at, 'b')
		}
	}
	return nil
}

func walkDisplayKeys(data []byte, base int, visit func([]byte, []byte, int, int) error) error {
	i := jsonobject.SkipWhitespace(data, 0)
	if i >= len(data) || data[i] != '{' {
		return nil
	}
	i = jsonobject.SkipWhitespace(data, i+1)
	for i < len(data) && data[i] != '}' {
		start := i
		end, ok := jsonobject.StringEnd(data, i)
		if !ok {
			return nil
		}
		key := data[i:end]
		i = jsonobject.SkipWhitespace(data, end)
		if i >= len(data) || data[i] != ':' {
			return nil
		}
		i = jsonobject.SkipWhitespace(data, i+1)
		end, ok = jsonobject.ValueEnd(data, i)
		if !ok {
			return nil
		}
		if err := visit(key, data[i:end], base+start, base+i); err != nil {
			return err
		}
		i = jsonobject.SkipWhitespace(data, end)
		if i >= len(data) || data[i] != ',' {
			return nil
		}
		i = jsonobject.SkipWhitespace(data, i+1)
	}
	return nil
}

// Old-shape records need no selector contexts, filtering or new restrictions.
// Inspect every native-recognized display object, including aliases, escaped
// names and earlier records later cleared by null, before taking this path.
func hasCurrentAgentDisplay(data []byte) bool {
	found := false
	jsonobject.WalkFields(data, true, func(key, moderation []byte) {
		if found || !jsonobject.FieldMatchesFolded(key, "moderation") {
			return
		}
		jsonobject.WalkFields(moderation, true, func(key, display []byte) {
			if found || !jsonobject.FieldMatchesFolded(key, "display") {
				return
			}
			jsonobject.WalkFields(display, true, func(key, _ []byte) {
				for _, name := range [...]string{"mode", "version", "trustedcontacts", "annotation", "replacement", "reason"} {
					if jsonobject.FieldMatchesFolded(key, name) {
						found = true
						return
					}
				}
			})
		})
	})
	return found
}
