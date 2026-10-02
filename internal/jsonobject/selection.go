// Package jsonobject selects fixed ASCII fields from raw JSON without decoding
// ignored properties. Selected slices borrow the caller's storage.
package jsonobject

import (
	"bytes"
	"encoding/json"
	"unicode"
	"unicode/utf8"
)

// SelectFields retains only the final exact named fields. The returned values
// borrow data. Null and non-object inputs follow map unmarshaling semantics.
func SelectFields(data []byte, names ...string) (map[string]json.RawMessage, error) {
	if !json.Valid(data) || bytes.TrimSpace(data)[0] != '{' {
		var fields map[string]json.RawMessage
		err := json.Unmarshal(data, &fields)
		return fields, err
	}
	fields := make(map[string]json.RawMessage, len(names))
	WalkFields(data, true, func(key, value []byte) {
		for _, name := range names {
			if FieldMatches(key, name) {
				fields[name] = value
				break
			}
		}
	})
	return fields, nil
}

// TypeField follows standard type-discriminator string-field matching. Null leaves a
// prior string intact and any wrong-type occurrence fails, even if overwritten.
// Only the final non-null string is decoded; ignored keys remain raw.
func TypeField(data []byte) (string, error) {
	if !json.Valid(data) || bytes.TrimSpace(data)[0] != '{' {
		var value struct {
			Type string `json:"type"`
		}
		err := json.Unmarshal(data, &value)
		return value.Type, err
	}
	var selected []byte
	var invalid []byte
	WalkFields(data, true, func(key, value []byte) {
		if !FieldMatchesFolded(key, "type") {
			return
		}
		if value[0] == '"' {
			selected = value
		} else if !bytes.Equal(value, []byte("null")) && invalid == nil {
			invalid = value
		}
	})
	var value string
	if invalid != nil {
		err := json.Unmarshal(invalid, &value)
		return "", err
	}
	if selected != nil {
		if err := json.Unmarshal(selected, &value); err != nil {
			return "", err
		}
	}
	return value, nil
}

// SelectField returns only the last exact property. Values borrow storage;
// ignored metadata never becomes a map, decoded string, or copied payload.
func SelectField(data []byte, name string) (json.RawMessage, bool, bool) {
	if !json.Valid(data) {
		return nil, false, false
	}
	var selected json.RawMessage
	found := false
	ok := WalkFields(data, true, func(key, value []byte) {
		if FieldMatches(key, name) {
			selected, found = value, true
		}
	})
	return selected, found, ok
}

// WalkFields visits properties in wire order. Recovery can inspect a complete
// property before a malformed suffix. Full
// object selection validates first; prefix recovery validates each property.
func WalkFields(data []byte, validated bool, visit func(key, value []byte)) bool {
	i := SkipWhitespace(data, 0)
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return true
	}
	if i >= len(data) || data[i] != '{' {
		return false
	}
	i = SkipWhitespace(data, i+1)
	if i < len(data) && data[i] == '}' {
		return SkipWhitespace(data, i+1) == len(data)
	}
	for i < len(data) {
		keyStart := i
		keyEnd, ok := StringEnd(data, i)
		if !ok || (!validated && !json.Valid(data[keyStart:keyEnd])) {
			return false
		}
		i = SkipWhitespace(data, keyEnd)
		if i >= len(data) || data[i] != ':' {
			return false
		}
		i = SkipWhitespace(data, i+1)
		valueEnd, ok := ValueEnd(data, i)
		if !ok || (!validated && !json.Valid(data[i:valueEnd])) {
			return false
		}
		visit(data[keyStart:keyEnd], data[i:valueEnd])
		i = SkipWhitespace(data, valueEnd)
		if i >= len(data) {
			return false
		}
		if data[i] == '}' {
			return SkipWhitespace(data, i+1) == len(data)
		}
		if data[i] != ',' {
			return false
		}
		i = SkipWhitespace(data, i+1)
	}
	return false
}

// ValueEnd finds boundaries without decoding strings or allocating a stack.
// Callers must validate the whole object or the returned value before use.
func ValueEnd(data []byte, start int) (int, bool) {
	if start >= len(data) {
		return start, false
	}
	if data[start] == '"' {
		return StringEnd(data, start)
	}
	if data[start] != '{' && data[start] != '[' {
		return scanJSONScalarEnd(data, start)
	}
	depth := 0
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '"':
			end, ok := StringEnd(data, i)
			if !ok {
				return start, false
			}
			i = end - 1
		case '{', '[':
			depth++
			// The surrounding object is one container too. Match the
			// standard validator's 10,000-container limit during recovery.
			if depth >= 10000 {
				return start, false
			}
		case '}', ']':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return start, false
}

func scanJSONScalarEnd(data []byte, start int) (int, bool) {
	for _, literal := range [...]string{"true", "false", "null"} {
		if bytes.HasPrefix(data[start:], []byte(literal)) {
			return start + len(literal), true
		}
	}
	i := start
	if data[i] == '-' {
		i++
	}
	if i >= len(data) {
		return start, false
	}
	if data[i] == '0' {
		i++
	} else {
		for i < len(data) && data[i] >= '0' && data[i] <= '9' {
			i++
		}
	}
	if i < len(data) && data[i] == '.' {
		i++
		for i < len(data) && data[i] >= '0' && data[i] <= '9' {
			i++
		}
	}
	if i < len(data) && (data[i] == 'e' || data[i] == 'E') {
		i++
		if i < len(data) && (data[i] == '-' || data[i] == '+') {
			i++
		}
		for i < len(data) && data[i] >= '0' && data[i] <= '9' {
			i++
		}
	}
	return i, i > start
}

func StringEnd(data []byte, start int) (int, bool) {
	if start >= len(data) || data[start] != '"' {
		return start, false
	}
	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			return i + 1, true
		}
	}
	return start, false
}

// FieldMatches compares the decoded spelling of an ASCII schema property
// without unquoting every peer-controlled key, including Unicode escapes.
func FieldMatches(raw []byte, name string) bool {
	return matchJSONField(raw, name, false)
}

func FieldMatchesFolded(raw []byte, name string) bool {
	return matchJSONField(raw, name, true)
}

func matchJSONField(raw []byte, name string, folded bool) bool {
	j := 0
	for i := 1; i < len(raw)-1; {
		c, next, ok := jsonKeyRune(raw, i)
		if !ok {
			return false
		}
		i = next
		if folded {
			// Match encoding/json's Unicode simple-fold behavior, including
			// the long-s alias for an ASCII 's' in envelope property names.
			if c > 127 {
				for candidate := unicode.SimpleFold(c); candidate != c; candidate = unicode.SimpleFold(candidate) {
					if candidate <= 127 {
						c = candidate
						break
					}
				}
			}
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
		}
		if j >= len(name) || c != rune(name[j]) {
			return false
		}
		j++
	}
	return j == len(name)
}

func jsonKeyRune(raw []byte, start int) (rune, int, bool) {
	if raw[start] != '\\' {
		c, size := utf8.DecodeRune(raw[start:])
		return c, start + size, true
	}
	// Selected property names do not contain characters represented by JSON's
	// short escapes. Unicode escapes may spell any selected ASCII property.
	i := start + 1
	if i >= len(raw)-1 || raw[i] != 'u' || i+4 >= len(raw)-1 {
		return 0, start, false
	}
	var decoded rune
	for range 4 {
		i++
		hex := raw[i]
		switch {
		case hex >= '0' && hex <= '9':
			decoded = decoded*16 + rune(hex-'0')
		case hex >= 'a' && hex <= 'f':
			decoded = decoded*16 + rune(hex-'a'+10)
		case hex >= 'A' && hex <= 'F':
			decoded = decoded*16 + rune(hex-'A'+10)
		default:
			return 0, start, false
		}
	}
	return decoded, i + 1, true
}

func SkipWhitespace(data []byte, start int) int {
	for start < len(data) {
		switch data[start] {
		case ' ', '\t', '\r', '\n':
			start++
		default:
			return start
		}
	}
	return start
}
