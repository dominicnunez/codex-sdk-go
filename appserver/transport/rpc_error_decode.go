package transport

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// Preserve the standard Error struct's aliases, sticky type errors and
// null-no-clear scalar semantics without allocating per overwritten field.
func decodeInboundRPCError(data []byte) (Error, bool) {
	var parsed Error
	var message, extra json.RawMessage
	invalid := false
	ok := walkJSONObjectFields(data, true, func(key, value []byte) {
		if invalid {
			return
		}
		switch {
		case jsonFieldMatchesFolded(key, "code"):
			if bytes.Equal(value, []byte("null")) {
				return
			}
			if len(value) > 20 {
				invalid = true
				return
			}
			code, err := strconv.ParseInt(string(value), 10, strconv.IntSize)
			if err != nil {
				invalid = true
				return
			}
			parsed.Code = int(code)
		case jsonFieldMatchesFolded(key, "message"):
			if value[0] == '"' {
				message = value
			} else if !bytes.Equal(value, []byte("null")) {
				invalid = true
			}
		case jsonFieldMatchesFolded(key, "data"):
			extra = value
		}
	})
	if !ok || invalid {
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
