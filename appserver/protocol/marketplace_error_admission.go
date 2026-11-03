package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

const (
	marketplaceNamePresent uint8 = 1 << iota
	marketplaceMessagePresent
)

// Track presence across native duplicate-array storage reuse without adding
// JSON methods to the public error record or decoding a second value graph.
func validateMarketplaceErrors(data []byte) error {
	state := marketplaceErrorPresence{}
	var validationErr error
	jsonobject.WalkFields(data, true, func(key, raw []byte) {
		if validationErr != nil || !jsonobject.FieldMatchesFolded(key, "errors") {
			return
		}
		if isNullJSONValue(raw) {
			validationErr = fmt.Errorf("marketplace.upgrade.errors must not be null")
			return
		}
		validationErr = state.observe(raw)
	})
	if validationErr != nil {
		return validationErr
	}
	for index := 0; index < state.visible; index++ {
		if state.fields[index]&marketplaceNamePresent == 0 {
			return fmt.Errorf("marketplace.upgrade.errors[%d].marketplaceName is required", index)
		}
		if state.fields[index]&marketplaceMessagePresent == 0 {
			return fmt.Errorf("marketplace.upgrade.errors[%d].message is required", index)
		}
	}
	return nil
}

type marketplaceErrorPresence struct {
	fields  []uint8
	visible int
}

func (state *marketplaceErrorPresence) observe(raw []byte) error {
	if len(raw) == 0 || raw[0] != '[' {
		// The native response decoder owns non-array and record-kind errors.
		return nil
	}
	count := 0
	err := validateArrayElements(raw, func(index int, item []byte) error {
		count = index + 1
		if isNullJSONValue(item) {
			return fmt.Errorf("marketplace.upgrade.errors[%d] must not be null", index)
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
			case jsonobject.FieldMatchesFolded(key, "marketplacename"):
				bit, name = marketplaceNamePresent, "marketplaceName"
			case jsonobject.FieldMatchesFolded(key, "message"):
				bit, name = marketplaceMessagePresent, "message"
			default:
				return
			}
			if isNullJSONValue(value) {
				fieldErr = fmt.Errorf("marketplace.upgrade.errors[%d].%s must not be null", index, name)
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
		// An empty array resets native storage; nonempty shrink/regrow retains it.
		state.fields = nil
	}
	state.visible = count
	return nil
}
