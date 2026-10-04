package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

type mcpResourceReadSpan struct {
	start int
	end   int
}

type mcpResourceContentPresence struct {
	uri  bool
	text bool
	blob bool
}

// validateMcpResourceReadContents checks required URI presence and usable
// nullable branch state after native decoding. Its small presence masks follow
// native duplicate-array slot reuse, shrink/reextend and empty-array reset.
func validateMcpResourceReadContents(data []byte) error {
	state := mcpResourceReadAdmission{}
	// The caller has already run validateRequiredObjectFields, which validates
	// the complete JSON input before invoking this borrowed scanner.
	if !state.scan(data) {
		return fmt.Errorf("invalid resource read response contents")
	}
	return state.validationError()
}

// recoverMcpResourceReadExtras retries a failed native decode only when every
// type error is an incompatible opposite-branch extra allowed by the schema.
// The original value and errors remain owned by the native decoder on failure.
func recoverMcpResourceReadExtras(data []byte) (McpResourceReadResponse, bool) {
	state := mcpResourceReadAdmission{projectExtras: true}
	if !state.scan(data) || state.commonInvalid || state.unclassifiedExtra || len(state.extras) == 0 {
		return McpResourceReadResponse{}, false
	}
	if err := state.validationError(); err != nil {
		return McpResourceReadResponse{}, false
	}
	projected := projectMcpResourceReadExtras(data, state.extras)
	type wire McpResourceReadResponse
	var decoded wire
	if err := jsondecode.Unmarshal(projected, &decoded); err != nil {
		return McpResourceReadResponse{}, false
	}
	return McpResourceReadResponse(decoded), true
}

func projectMcpResourceReadExtras(data []byte, spans []mcpResourceReadSpan) []byte {
	var projected []byte
	last := 0
	for _, span := range spans {
		if span.start < last || span.end < span.start || span.end > len(data) {
			return data
		}
		projected = append(projected, data[last:span.start]...)
		projected = append(projected, "null"...)
		last = span.end
	}
	projected = append(projected, data[last:]...)
	return projected
}

type mcpResourceReadAdmission struct {
	// Keep historical slots through duplicate-array shrink/reextend, as the
	// native decoder does. An explicit [] resets the backing storage.
	fields  []mcpResourceContentPresence
	visible int

	projectExtras     bool
	extras            []mcpResourceReadSpan
	commonInvalid     bool
	unclassifiedExtra bool
	validationErr     error
}

func (state *mcpResourceReadAdmission) scan(data []byte) bool {
	valid := walkMcpResourceReadObject(data, func(key, raw []byte, start, end int) {
		if !jsonobject.FieldMatchesFolded(key, "contents") {
			if jsonobject.FieldMatchesFolded(key, "origincallid") && !isStringOrNullJSONValue(raw) {
				state.commonInvalid = true
			}
			return
		}
		if isNullJSONValue(raw) {
			state.commonInvalid = true
			if state.validationErr == nil {
				state.validationErr = responseObjectValidationErrors().null("contents")
			}
			return
		}
		if len(raw) == 0 || raw[0] != '[' {
			state.commonInvalid = true
			return
		}
		state.scanContentsArray(raw, start)
	})
	return valid
}

func (state *mcpResourceReadAdmission) scanContentsArray(raw []byte, base int) {
	count := 0
	if err := walkMcpResourceReadArray(raw, func(index int, item []byte, start, _ int) error {
		count = index + 1
		if isNullJSONValue(item) {
			if state.validationErr == nil {
				state.validationErr = fmt.Errorf("resource.contents[%d] must not be null", index)
			}
			return nil
		}
		if len(item) == 0 || item[0] != '{' {
			state.commonInvalid = true
			return nil
		}
		state.ensureResourceContentSlot(index)
		state.scanResourceContentObject(index, item, base+start)
		return nil
	}); err != nil {
		state.commonInvalid = true
	}
	if count == 0 {
		// encoding/json gives a duplicate empty array a fresh empty slice.
		state.fields = nil
		state.visible = 0
		return
	}
	state.visible = count
}

func (state *mcpResourceReadAdmission) ensureResourceContentSlot(index int) {
	if index < len(state.fields) {
		return
	}
	for len(state.fields) <= index {
		state.fields = append(state.fields, mcpResourceContentPresence{})
	}
}

func (state *mcpResourceReadAdmission) scanResourceContentObject(index int, item []byte, base int) {
	field := &state.fields[index]
	badText, badBlob := false, false
	valid := walkMcpResourceReadObject(item, func(key, raw []byte, start, end int) {
		switch {
		case jsonobject.FieldMatchesFolded(key, "uri"):
			if isNullJSONValue(raw) {
				state.commonInvalid = true
				if state.validationErr == nil {
					state.validationErr = fmt.Errorf("resource.contents[%d].uri must not be null", index)
				}
			} else if isJSONString(raw) {
				field.uri = true
			} else {
				state.commonInvalid = true
			}
		case jsonobject.FieldMatchesFolded(key, "mimetype"):
			if !isStringOrNullJSONValue(raw) {
				state.commonInvalid = true
			}
		case jsonobject.FieldMatchesFolded(key, "text"):
			if isJSONString(raw) {
				field.text = true
			} else if isNullJSONValue(raw) {
				field.text = false
			} else {
				field.text = false
				badText = true
				if state.projectExtras {
					state.extras = append(state.extras, mcpResourceReadSpan{start: base + start, end: base + end})
				}
			}
		case jsonobject.FieldMatchesFolded(key, "blob"):
			if isJSONString(raw) {
				field.blob = true
			} else if isNullJSONValue(raw) {
				field.blob = false
			} else {
				field.blob = false
				badBlob = true
				if state.projectExtras {
					state.extras = append(state.extras, mcpResourceReadSpan{start: base + start, end: base + end})
				}
			}
		}
	})
	if !valid {
		state.commonInvalid = true
	}
	// Classification is deliberately source-object-local. Inherited values
	// count, same-object repairs count, and a later object cannot erase the
	// original native error for an unclassified earlier occurrence.
	if badText && !field.blob || badBlob && !field.text {
		state.unclassifiedExtra = true
	}
}

func (state *mcpResourceReadAdmission) validationError() error {
	if state.validationErr != nil {
		return state.validationErr
	}
	for index := 0; index < state.visible; index++ {
		field := state.fields[index]
		if !field.uri {
			return fmt.Errorf("resource.contents[%d].uri is required", index)
		}
		if !field.text && !field.blob {
			return fmt.Errorf("resource.contents[%d] requires string text or blob", index)
		}
	}
	return nil
}

func isJSONString(raw []byte) bool {
	return len(raw) > 0 && raw[0] == '"'
}

func isStringOrNullJSONValue(raw []byte) bool {
	return isJSONString(raw) || isNullJSONValue(raw)
}

func walkMcpResourceReadArray(data []byte, visit func(index int, value []byte, start, end int) error) error {
	i := jsonobject.SkipWhitespace(data, 0)
	if i >= len(data) || data[i] != '[' {
		return fmt.Errorf("expected array")
	}
	i = jsonobject.SkipWhitespace(data, i+1)
	for index := 0; i < len(data) && data[i] != ']'; index++ {
		end, ok := jsonobject.ValueEnd(data, i)
		if !ok {
			return fmt.Errorf("invalid array value")
		}
		if err := visit(index, data[i:end], i, end); err != nil {
			return err
		}
		i = jsonobject.SkipWhitespace(data, end)
		if i < len(data) && data[i] == ',' {
			i = jsonobject.SkipWhitespace(data, i+1)
		}
	}
	return nil
}

func walkMcpResourceReadObject(data []byte, visit func(key, value []byte, start, end int)) bool {
	i := jsonobject.SkipWhitespace(data, 0)
	if i >= len(data) || data[i] != '{' {
		return false
	}
	i = jsonobject.SkipWhitespace(data, i+1)
	if i < len(data) && data[i] == '}' {
		return jsonobject.SkipWhitespace(data, i+1) == len(data)
	}
	for i < len(data) {
		keyStart := i
		keyEnd, ok := jsonobject.StringEnd(data, keyStart)
		if !ok {
			return false
		}
		i = jsonobject.SkipWhitespace(data, keyEnd)
		if i >= len(data) || data[i] != ':' {
			return false
		}
		start := jsonobject.SkipWhitespace(data, i+1)
		end, ok := jsonobject.ValueEnd(data, start)
		if !ok {
			return false
		}
		visit(data[keyStart:keyEnd], data[start:end], start, end)
		i = jsonobject.SkipWhitespace(data, end)
		if i >= len(data) {
			return false
		}
		if data[i] == '}' {
			return jsonobject.SkipWhitespace(data, i+1) == len(data)
		}
		if data[i] != ',' {
			return false
		}
		i = jsonobject.SkipWhitespace(data, i+1)
	}
	return false
}
