package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

const (
	hookErrorMessagePresent uint8 = 1 << iota
	hookErrorPathPresent
)

// validateHookErrorInfoAdmission checks required HookErrorInfo field presence
// without decoding a second copy of the peer's records. The ordinary decoder
// owns value types and preserves its existing errors; this pass tracks only
// which fields reached each visible array slot across duplicate arrays.
func validateHookErrorInfoAdmission(data []byte) error {
	state := hookErrorInfoPresence{}
	var validationErr error
	jsonobject.WalkFields(data, true, func(key, raw []byte) {
		if validationErr != nil || !jsonobject.FieldMatches(key, "errors") {
			return
		}
		validationErr = state.observeArray(raw)
	})
	if validationErr != nil {
		return validationErr
	}
	for index := 0; index < state.visible; index++ {
		mask := state.fields[index]
		if mask&hookErrorMessagePresent == 0 {
			return fmt.Errorf("hook.errors[%d].message is required", index)
		}
		if mask&hookErrorPathPresent == 0 {
			return fmt.Errorf("hook.errors[%d].path is required", index)
		}
	}
	return nil
}

type hookErrorInfoPresence struct {
	fields  []uint8
	visible int
}

func (state *hookErrorInfoPresence) observeArray(raw []byte) error {
	if len(raw) == 0 || raw[0] != '[' {
		// HooksListEntry's ordinary decoder owns non-array type errors.
		return nil
	}
	count := 0
	err := validateArrayElements(raw, func(index int, item []byte) error {
		count = index + 1
		if isNullJSONValue(item) {
			return fmt.Errorf("hook.errors[%d] must not be null", index)
		}
		if len(state.fields) <= index {
			state.fields = append(state.fields, make([]uint8, index+1-len(state.fields))...)
		}
		var fieldErr error
		jsonobject.WalkFields(item, true, func(key, value []byte) {
			if fieldErr != nil {
				return
			}
			var bit uint8
			var name string
			switch {
			case jsonobject.FieldMatchesFolded(key, "message"):
				bit, name = hookErrorMessagePresent, "message"
			case jsonobject.FieldMatchesFolded(key, "path"):
				bit, name = hookErrorPathPresent, "path"
			default:
				return
			}
			if isNullJSONValue(value) {
				fieldErr = fmt.Errorf("hook.errors[%d].%s must not be null", index, name)
				return
			}
			state.fields[index] |= bit
		})
		return fieldErr
	})
	if err != nil {
		return err
	}
	if count == 0 {
		// encoding/json replaces the old slice storage when it decodes [];
		// subsequent duplicate arrays therefore cannot inherit old fields.
		state.fields = nil
	}
	state.visible = count
	return nil
}
