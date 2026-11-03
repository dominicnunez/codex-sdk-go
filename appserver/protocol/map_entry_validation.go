package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// Native field aliases match every owning decoder. The owner has already
// decoded map types and owns receiver updates. Inspect
// every borrowed occurrence without retaining or interpreting dynamic keys.
func validateMapEntries(data []byte, nullable bool, names ...string) error {
	return validateOptionalFields(data, func(_ string, raw []byte) error {
		if isNullJSONValue(raw) {
			if nullable {
				return nil
			}
			return ErrNullResultField
		}
		var err error
		jsonobject.WalkFields(raw, true, func(_, value []byte) {
			if err == nil && isNullJSONValue(value) {
				err = fmt.Errorf("map entry: %w", ErrNullResultField)
			}
		})
		return err
	}, names...)
}
