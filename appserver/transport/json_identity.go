package transport

import (
	"bytes"
	"encoding/json"
	"unicode"
	"unicode/utf8"
)

// Select only the last exact property. Values borrow the caller's storage;
// ignored metadata never becomes a map, decoded string, or copied payload.
func selectJSONObjectField(data []byte, name string) (json.RawMessage, bool, bool) {
	if !json.Valid(data) {
		return nil, false, false
	}
	var selected json.RawMessage
	found := false
	ok := walkJSONObjectFields(data, true, func(key, value []byte) {
		if jsonFieldMatches(key, name) {
			selected, found = value, true
		}
	})
	return selected, found, ok
}

// Recovery can inspect a complete property before a malformed suffix. Full
// object selection validates first; prefix recovery validates each property.
func walkJSONObjectFields(data []byte, validated bool, visit func(key, value []byte)) bool {
	i := skipJSONWhitespace(data, 0)
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return true
	}
	if i >= len(data) || data[i] != '{' {
		return false
	}
	i = skipJSONWhitespace(data, i+1)
	if i < len(data) && data[i] == '}' {
		return skipJSONWhitespace(data, i+1) == len(data)
	}
	for i < len(data) {
		keyStart := i
		keyEnd, ok := scanJSONStringEnd(data, i)
		if !ok || (!validated && !json.Valid(data[keyStart:keyEnd])) {
			return false
		}
		i = skipJSONWhitespace(data, keyEnd)
		if i >= len(data) || data[i] != ':' {
			return false
		}
		i = skipJSONWhitespace(data, i+1)
		valueEnd, ok := scanJSONValueEnd(data, i)
		if !ok || (!validated && !json.Valid(data[i:valueEnd])) {
			return false
		}
		visit(data[keyStart:keyEnd], data[i:valueEnd])
		i = skipJSONWhitespace(data, valueEnd)
		if i >= len(data) {
			return false
		}
		if data[i] == '}' {
			return skipJSONWhitespace(data, i+1) == len(data)
		}
		if data[i] != ',' {
			return false
		}
		i = skipJSONWhitespace(data, i+1)
	}
	return false
}

// Find boundaries without decoding ignored strings or allocating a stack.
// Callers must validate the whole object or the returned value before use.
func scanJSONValueEnd(data []byte, start int) (int, bool) {
	if start >= len(data) {
		return start, false
	}
	if data[start] == '"' {
		return scanJSONStringEnd(data, start)
	}
	if data[start] != '{' && data[start] != '[' {
		return scanJSONScalarEnd(data, start)
	}
	depth := 0
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '"':
			end, ok := scanJSONStringEnd(data, i)
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

func scanJSONStringEnd(data []byte, start int) (int, bool) {
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

// The selected schema properties are ASCII. Compare their decoded spelling
// without unquoting every peer-controlled key, including Unicode escapes.
func jsonFieldMatches(raw []byte, name string) bool {
	return matchJSONField(raw, name, false)
}

func jsonFieldMatchesFolded(raw []byte, name string) bool {
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
	// Selected property names contain letters only, so all other escapes
	// cannot match. Unicode escapes may spell a selected ASCII property.
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
