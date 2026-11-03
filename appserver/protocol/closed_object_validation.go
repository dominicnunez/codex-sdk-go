package protocol

import (
	"errors"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// Run after the owner's existing decoding and validation. Match schema property
// names exactly, including escaped names, without decoding unknown values or
// retaining untrusted property names in diagnostics.
func validateClosedObjectFields(data []byte, allowed ...string) error {
	unknown := false
	jsonobject.WalkFields(data, true, func(key, _ []byte) {
		for _, name := range allowed {
			if jsonobject.FieldMatches(key, name) {
				return
			}
		}
		unknown = true
	})
	if unknown {
		return errors.New("unexpected field in closed object")
	}
	return nil
}
