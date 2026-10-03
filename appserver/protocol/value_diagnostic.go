package protocol

import "github.com/dominicnunez/codex-sdk-go/internal/diagnostic"

// Bound the source bytes before quoting: escaping and error wrapping must not
// turn a large rejected value into multiple retained copies of that value.
func quotedValueDiagnostic(value string) string {
	return diagnostic.Quote(value)
}
