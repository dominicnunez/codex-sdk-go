package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

const (
	pluginHookEventNamePresent uint8 = 1 << iota
	pluginHookKeyPresent
)

// validatePluginHookSummaryRecords checks required HookSummary field presence
// after native decoding has validated value types and enum values. The borrowed
// scan tracks only source presence across duplicate-array slice reuse.
func validatePluginHookSummaryRecords(data []byte) error {
	state := pluginHookSummaryPresence{}
	var validationErr error
	jsonobject.WalkFields(data, true, func(key, raw []byte) {
		if validationErr != nil || !jsonobject.FieldMatchesFolded(key, "hooks") {
			return
		}
		if isNullJSONValue(raw) {
			// A null pointer-to-slice resets native storage, but remains forbidden
			// even if a later duplicate supplies a non-null array.
			state.fields = nil
			state.visible = 0
			validationErr = fmt.Errorf("plugin.hooks must not be null")
			return
		}
		validationErr = state.observeArray(raw)
	})
	if validationErr != nil {
		return validationErr
	}
	for index := 0; index < state.visible; index++ {
		mask := state.fields[index]
		if mask&pluginHookEventNamePresent == 0 {
			return fmt.Errorf("plugin.hooks[%d].eventName is required", index)
		}
		if mask&pluginHookKeyPresent == 0 {
			return fmt.Errorf("plugin.hooks[%d].key is required", index)
		}
	}
	return nil
}

type pluginHookSummaryPresence struct {
	fields  []uint8
	visible int
}

func (state *pluginHookSummaryPresence) observeArray(raw []byte) error {
	if len(raw) == 0 || raw[0] != '[' {
		// PluginDetail's native decoder owns non-array type errors.
		return nil
	}
	count := 0
	err := validateArrayElements(raw, func(index int, item []byte) error {
		count = index + 1
		if isNullJSONValue(item) {
			return fmt.Errorf("plugin.hooks[%d] must not be null", index)
		}
		if len(state.fields) <= index {
			state.fields = append(state.fields, make([]uint8, index+1-len(state.fields))...)
		}
		var fieldErr error
		jsonobject.WalkFields(item, true, func(key, value []byte) {
			if fieldErr != nil {
				return
			}
			switch {
			case jsonobject.FieldMatchesFolded(key, "eventname"):
				state.fields[index] |= pluginHookEventNamePresent
			case jsonobject.FieldMatchesFolded(key, "key"):
				if isNullJSONValue(value) {
					fieldErr = fmt.Errorf("plugin.hooks[%d].key must not be null", index)
					return
				}
				state.fields[index] |= pluginHookKeyPresent
			}
		})
		return fieldErr
	})
	if err != nil {
		return err
	}
	if count == 0 {
		// Native decoding of [] gives the hooks slice fresh empty storage.
		state.fields = nil
	}
	state.visible = count
	return nil
}
