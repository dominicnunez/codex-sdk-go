// Package diagnostic bounds source previews before formatting rejected values.
package diagnostic

import (
	"fmt"
	"strconv"
)

const previewBytes = 256

// Quote preserves ordinary string quotation for short values and reports the
// omitted byte count after quoting a bounded prefix of larger values.
func Quote(value string) string {
	if len(value) <= previewBytes {
		return strconv.Quote(value)
	}
	return fmt.Sprintf("%s... (%d bytes omitted)", strconv.Quote(value[:previewBytes]), len(value)-previewBytes)
}

// Display retains unquoted short messages at existing display boundaries.
// Larger previews are quoted so escaping cannot amplify their full source.
func Display(value string) string {
	if len(value) <= previewBytes {
		return value
	}
	return Quote(value)
}
