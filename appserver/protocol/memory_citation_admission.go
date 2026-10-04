package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

const (
	citationEntriesPresent uint8 = 1 << iota
	citationThreadIDsPresent
)

const (
	citationLineEndPresent uint8 = 1 << iota
	citationLineStartPresent
	citationNotePresent
	citationPathPresent
)

// validateMemoryCitationAdmission tracks required-field presence across the
// duplicate-object merges performed by encoding/json, without decoding or
// retaining another semantic copy of the input.
func validateMemoryCitationAdmission(data []byte, item *AgentMessageThreadItem) error {
	state := memoryCitationPresence{}
	var validationErr error
	jsonobject.WalkFields(data, true, func(key, raw []byte) {
		if validationErr != nil || !jsonobject.FieldMatchesFolded(key, "memorycitation") {
			return
		}
		if isNullJSONValue(raw) {
			state.reset()
			return
		}
		state.present = true
		jsonobject.WalkFields(raw, true, func(field, value []byte) {
			if validationErr != nil {
				return
			}
			switch {
			case jsonobject.FieldMatchesFolded(field, "entries"):
				state.fieldsSeen |= citationEntriesPresent
				validationErr = state.observeEntries(value)
			case jsonobject.FieldMatchesFolded(field, "threadids"):
				state.fieldsSeen |= citationThreadIDsPresent
			}
		})
	})
	if validationErr != nil {
		return validationErr
	}
	if !state.present {
		return nil
	}
	if item.MemoryCitation == nil {
		return nil
	}
	if state.fieldsSeen&citationEntriesPresent == 0 {
		return fmt.Errorf("memoryCitation.entries is required")
	}
	if state.fieldsSeen&citationThreadIDsPresent == 0 {
		return fmt.Errorf("memoryCitation.threadIds is required")
	}
	for index := 0; index < state.visibleEntries; index++ {
		mask := state.entryFields[index]
		for _, required := range []struct {
			name string
			bit  uint8
		}{
			{"lineEnd", citationLineEndPresent},
			{"lineStart", citationLineStartPresent},
			{"note", citationNotePresent},
			{"path", citationPathPresent},
		} {
			if mask&required.bit == 0 {
				return fmt.Errorf("memoryCitation.entries[%d].%s is required", index, required.name)
			}
		}
	}
	return nil
}

type memoryCitationPresence struct {
	present        bool
	fieldsSeen     uint8
	entryFields    []uint8
	visibleEntries int
}

func (state *memoryCitationPresence) reset() {
	state.present = false
	state.fieldsSeen = 0
	state.entryFields = nil
	state.visibleEntries = 0
}

func (state *memoryCitationPresence) observeEntries(raw []byte) error {
	if isNullJSONValue(raw) {
		return fmt.Errorf("memoryCitation.entries must not be null")
	}
	if len(raw) == 0 || raw[0] != '[' {
		// Native decoding already reports non-array values with its established
		// type error. Keep this check defensive without replacing that contract.
		return nil
	}
	state.fieldsSeen |= citationEntriesPresent
	indexCount := 0
	err := validateArrayElements(raw, func(index int, entry []byte) error {
		indexCount = index + 1
		if isNullJSONValue(entry) {
			return fmt.Errorf("memoryCitation.entries[%d] must not be null", index)
		}
		if len(state.entryFields) <= index {
			state.entryFields = append(state.entryFields, make([]uint8, index+1-len(state.entryFields))...)
		}
		var fieldErr error
		jsonobject.WalkFields(entry, true, func(key, value []byte) {
			if fieldErr != nil {
				return
			}
			var bit uint8
			var fieldName string
			switch {
			case jsonobject.FieldMatchesFolded(key, "lineend"):
				bit = citationLineEndPresent
				fieldName = "lineEnd"
			case jsonobject.FieldMatchesFolded(key, "linestart"):
				bit = citationLineStartPresent
				fieldName = "lineStart"
			case jsonobject.FieldMatchesFolded(key, "note"):
				bit = citationNotePresent
				fieldName = "note"
			case jsonobject.FieldMatchesFolded(key, "path"):
				bit = citationPathPresent
				fieldName = "path"
			default:
				return
			}
			if isNullJSONValue(value) {
				fieldErr = fmt.Errorf("memoryCitation.entries[%d].%s must not be null", index, fieldName)
				return
			}
			state.entryFields[index] |= bit
		})
		return fieldErr
	})
	if err != nil {
		return err
	}
	if indexCount == 0 {
		state.entryFields = nil
	}
	state.visibleEntries = indexCount
	return nil
}
