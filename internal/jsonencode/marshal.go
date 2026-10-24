// Package jsonencode bounds native number-literal diagnostics before SDK
// serialization owners wrap or report them.
package jsonencode

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/dominicnunez/codex-sdk-go/internal/diagnostic"
)

// Marshal preserves standard serialization and application codec errors. Go
// wraps codec failures in MarshalerError. Only the standard-library Number
// codec is normalized here; application wrappers and error chains retain their
// identity. SDK codecs use this helper before enclosing JSON codecs wrap errors.
func Marshal(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err == nil {
		return data, nil
	}
	custom, ok := err.(*json.MarshalerError) //nolint:errorlint // Only the direct wrapper establishes the codec owner.
	if !ok || (custom.Type != reflect.TypeFor[json.Number]() && custom.Type != reflect.TypeFor[*json.Number]()) {
		return data, err
	}
	// Number is a standard-library streaming marshaler. Application codec
	// failures have their own outer MarshalerError and must remain untouched.
	const prefix = "cannot parse "
	const suffix = " as JSON number: invalid syntax"
	message := custom.Err.Error()
	if len(message) <= 2048 || !strings.HasPrefix(message, prefix) || !strings.HasSuffix(message, suffix) {
		return data, err
	}
	literal := message[len(prefix) : len(message)-len(suffix)]
	bounded := *custom
	bounded.Err = fmt.Errorf("cannot parse %s as JSON number: %w", diagnostic.Display(literal), strconv.ErrSyntax)
	return data, &bounded
}
