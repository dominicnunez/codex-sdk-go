// Package jsonencode bounds native number-literal diagnostics before SDK
// serialization owners wrap or report them.
package jsonencode

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/dominicnunez/codex-sdk-go/internal/diagnostic"
)

// Marshal preserves standard serialization and application codec errors. Go
// wraps value JSON/Text codec failures in MarshalerError; do not traverse that
// chain or format it here. SDK codecs use this helper inside their own boundary
// so native failures are bounded before Go adds an enclosing MarshalerError.
func Marshal(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err == nil {
		return data, nil
	}
	if _, custom := err.(*json.MarshalerError); custom { //nolint:errorlint // Direct ownership, not a search through application error chains.
		return data, err
	}
	const prefix = "json: invalid number literal "
	message := err.Error()
	if len(message) > 2048 && strings.HasPrefix(message, prefix) {
		// This direct formatter is owned by encoding/json, with no structured
		// fields or underlying cause. Keep no reference to its full diagnostic.
		return data, errors.New(prefix + diagnostic.Display(message[len(prefix):]))
	}
	return data, err
}
