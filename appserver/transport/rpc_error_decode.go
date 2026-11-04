package transport

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// Preserve native aliases, sticky type errors and final field selection while
// enforcing the response carrier's required/non-null code and message. Decode
// selected strings/data once, without allocating per overwritten field.
func decodeInboundRPCError(data []byte) (Error, bool) {
	var parsed Error
	var message, extra json.RawMessage
	var hasCode, hasMessage bool
	invalid := false
	ok := walkJSONObjectFields(data, true, func(key, value []byte) {
		if invalid {
			return
		}
		switch {
		case jsonFieldMatchesFolded(key, "code"):
			hasCode = true
			if bytes.Equal(value, []byte("null")) {
				invalid = true
				return
			}
			if len(value) > 20 {
				invalid = true
				return
			}
			code, err := strconv.ParseInt(string(value), 10, 64)
			if err != nil {
				invalid = true
				return
			}
			parsed.Code = code
		case jsonFieldMatchesFolded(key, "message"):
			hasMessage = true
			if value[0] == '"' {
				message = value
			} else {
				invalid = true
			}
		case jsonFieldMatchesFolded(key, "data"):
			extra = value
		}
	})
	if !ok || invalid || !hasCode || !hasMessage {
		return Error{}, false
	}
	if len(message) > 0 {
		if json.Unmarshal(message, &parsed.Message) != nil {
			return Error{}, false
		}
	}
	parsed.Data = append(json.RawMessage(nil), extra...)
	return parsed, true
}
