package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	requestIDKeyPrefixNumber = "n:"
	requestIDKeyPrefixString = "s:"
)

// errUnexpectedIDType is returned when normalizeID encounters an ID value
// that is not a supported JSON-RPC ID type (string, number).
var errUnexpectedIDType = errors.New("unexpected ID type")

// errNullID is returned when normalizeID encounters a nil (JSON null) ID.
// JSON-RPC 2.0 responses with "id": null indicate the server could not
// parse the request ID.
var errNullID = errors.New("null request ID")

type inboundFrame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      inboundID       `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   inboundError    `json:"error,omitempty"`
}

func (f inboundFrame) hasResultField() bool {
	return len(f.Result) > 0
}

func (f inboundFrame) hasResponseFields() bool {
	return f.hasResultField() || f.Error.present
}

func (f inboundFrame) hasMalformedResponseShape() bool {
	hasResult := f.hasResultField()
	hasError := f.Error.present
	if !hasResult && !hasError {
		return false
	}
	if hasResult && hasError {
		return true
	}
	if hasError {
		return f.Error.invalid || f.Error.isNull || f.Error.value == nil
	}
	return false
}

type inboundID struct {
	present bool
	isNull  bool
	value   RequestID
	invalid bool
}

func (i *inboundID) UnmarshalJSON(data []byte) error {
	i.present = true
	i.isNull = bytes.Equal(data, []byte("null"))
	if i.isNull {
		i.value = RequestID{}
		i.invalid = false
		return nil
	}

	var parsed RequestID
	if json.Unmarshal(data, &parsed) != nil {
		i.invalid = true
		return nil //nolint:nilerr // Preserve frame routing; invalid ID is handled after frame classification.
	}
	i.value = parsed
	i.invalid = false
	return nil
}

func (i inboundID) hasValue() bool {
	return i.present && !i.isNull
}

func (i inboundID) requestID() (RequestID, bool) {
	if !i.hasValue() || i.invalid {
		return RequestID{}, false
	}
	return i.value, true
}

type inboundError struct {
	present bool
	isNull  bool
	value   *Error
	invalid bool
}

func (e *inboundError) UnmarshalJSON(data []byte) error {
	e.present = true
	e.isNull = bytes.Equal(data, []byte("null"))
	if e.isNull {
		e.value = nil
		e.invalid = false
		return nil
	}

	var parsed Error
	if json.Unmarshal(data, &parsed) != nil {
		e.invalid = true
		//nolint:nilerr // Preserve frame routing; invalid error payload is handled as malformed response.
		return nil
	}
	e.value = &parsed
	e.invalid = false
	return nil
}

type oversizedFrameInfo struct {
	id                RequestID
	hasID             bool
	hasMethod         bool
	hasResponseFields bool
}

// normalizeID normalizes request IDs to a string key for map matching.
// JSON unmarshals all numbers as float64, so we format integer-valued
// floats without decimals for consistent lookups.
func normalizeID(id interface{}) (string, error) {
	normalizedID, _, err := normalizeRequestID(id)
	return normalizedID, err
}

func normalizePendingRequestID(id interface{}) (string, error) {
	normalizedID, familyPrefix, err := normalizeRequestID(id)
	if err != nil {
		return "", err
	}
	return familyPrefix + normalizedID, nil
}

func normalizeRequestID(id interface{}) (string, string, error) {
	switch v := id.(type) {
	case nil:
		return "", "", errNullID
	case string:
		return v, requestIDKeyPrefixString, nil
	}

	normalizedID, isNumeric, err := normalizeNumericID(id)
	if err != nil {
		return "", "", err
	}
	if !isNumeric {
		return "", "", fmt.Errorf("%w: %T", errUnexpectedIDType, id)
	}
	return normalizedID, requestIDKeyPrefixNumber, nil
}

func normalizeNumericID(id interface{}) (string, bool, error) {
	return canonicalNumericRequestIDString(id)
}

// readLimitedLine reads one newline-delimited frame and enforces an upper size
// bound. If a frame exceeds max bytes, it returns the oversized frame prefix so
// callers can best-effort route a matching response before terminating the
// transport.
func readLimitedLine(r *bufio.Reader, limit int) ([]byte, *oversizedFrameInfo, error) {
	var line []byte
	for {
		frag, err := r.ReadSlice('\n')
		line = append(line, frag...)
		if lineExceedsLimit(line, limit) {
			return handleOversizedLine(r, err, line)
		}
		switch {
		case err == nil:
			return bytes.TrimSuffix(line, []byte{'\n'}), nil, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if len(line) == 0 {
				return nil, nil, io.EOF
			}
			return line, nil, nil
		default:
			return nil, nil, err
		}
	}
}

func lineExceedsLimit(line []byte, limit int) bool {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		return len(line)-1 > limit
	}
	return len(line) > limit
}

func handleOversizedLine(reader *bufio.Reader, readErr error, line []byte) ([]byte, *oversizedFrameInfo, error) {
	info := extractOversizedFrameInfo(line, reader)
	switch {
	case readErr == nil:
		return nil, &info, nil
	case errors.Is(readErr, io.EOF):
		return nil, &info, io.EOF
	case !errors.Is(readErr, bufio.ErrBufferFull):
		return nil, &info, readErr
	}
	return nil, &info, nil
}

func decodeInboundFrame(data []byte) (inboundFrame, error) {
	var frame inboundFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		return inboundFrame{}, err
	}
	return frame, nil
}

func parseRequestID(data json.RawMessage) (RequestID, error) {
	if len(data) == 0 {
		return RequestID{}, errors.New("missing id")
	}
	var id RequestID
	if err := json.Unmarshal(data, &id); err != nil {
		return RequestID{}, err
	}
	return id, nil
}

func (f inboundFrame) toNotification() Notification {
	return Notification{
		JSONRPC: f.JSONRPC,
		Method:  f.Method,
		Params:  f.Params,
	}
}

func extractTopLevelIDAndMethod(data []byte) (RequestID, bool, bool) {
	id, hasID, hasMethod, _ := extractTopLevelIDAndMethodFromReader(bytes.NewReader(data))
	return id, hasID, hasMethod
}

func extractTopLevelIDAndMethodFromReader(reader io.Reader) (RequestID, bool, bool, error) {
	var id RequestID
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()

	start, err := decoder.Token()
	if err != nil {
		return id, false, false, err
	}
	delim, ok := start.(json.Delim)
	if !ok || delim != '{' {
		return id, false, false, nil
	}

	var hasID bool
	var hasMethod bool
	for decoder.More() {
		keyTok, err := decoder.Token()
		if err != nil {
			return id, hasID, hasMethod, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return id, hasID, hasMethod, nil
		}

		valueTok, err := decoder.Token()
		if err != nil {
			return id, hasID, hasMethod, err
		}

		switch key {
		case "id":
			switch v := valueTok.(type) {
			case string:
				id = RequestID{Value: v}
				hasID = true
			case json.Number:
				id = RequestID{Value: v}
				hasID = true
			case float64:
				id = RequestID{Value: v}
				hasID = true
			}
		case "method":
			if _, ok := valueTok.(string); ok {
				hasMethod = true
			}
		}

		if valueDelim, ok := valueTok.(json.Delim); ok && (valueDelim == '{' || valueDelim == '[') {
			if err := consumeNestedJSONValue(decoder); err != nil {
				return id, hasID, hasMethod, err
			}
		}
	}

	return id, hasID, hasMethod, nil
}

func extractOversizedFrameInfo(prefix []byte, reader *bufio.Reader) oversizedFrameInfo {
	info := inspectOversizedFramePrefix(prefix)
	if info.hasMethod || (info.hasResponseFields && info.hasID) {
		return info
	}
	if reader == nil || bytes.HasSuffix(prefix, []byte{'\n'}) {
		return info
	}

	buffered := reader.Buffered()
	if buffered == 0 {
		return info
	}
	bufferedBytes, err := reader.Peek(buffered)
	if err != nil {
		return info
	}

	inspectionBytes := make([]byte, 0, len(prefix)+len(bufferedBytes))
	inspectionBytes = append(inspectionBytes, prefix...)
	inspectionBytes = append(inspectionBytes, bufferedBytes...)
	return inspectOversizedFramePrefix(inspectionBytes)
}

func inspectOversizedFramePrefix(data []byte) oversizedFrameInfo {
	var info oversizedFrameInfo

	i := skipJSONWhitespace(data, 0)
	if i >= len(data) || data[i] != '{' {
		return info
	}
	i++

	for i < len(data) {
		i = skipJSONWhitespace(data, i)
		if i >= len(data) {
			return info
		}
		switch data[i] {
		case ',':
			i++
			continue
		case '}':
			return info
		default:
			if data[i] != '"' {
				return info
			}
		}

		key, next, ok := consumeJSONString(data, i)
		if !ok {
			return info
		}
		i = skipJSONWhitespace(data, next)
		if i >= len(data) || data[i] != ':' {
			return info
		}
		i = skipJSONWhitespace(data, i+1)
		if i >= len(data) {
			return info
		}

		valueEnd, ok := inspectOversizedFrameField(data, key, i, &info)
		if !ok || info.hasMethod || (info.hasResponseFields && info.hasID) {
			return info
		}
		i = valueEnd
	}

	return info
}

func inspectOversizedFrameField(data []byte, key string, valueStart int, info *oversizedFrameInfo) (int, bool) {
	switch key {
	case "id":
		id, valueEnd, ok := consumeRequestIDValue(data, valueStart)
		if !ok {
			return valueStart, false
		}
		info.id = id
		info.hasID = id.Value != nil
		return valueEnd, true
	case "method":
		valueEnd, ok := consumeJSONValue(data, valueStart)
		if !ok {
			return valueStart, false
		}
		info.hasMethod = true
		return valueEnd, true
	case "result", "error":
		info.hasResponseFields = true
		valueEnd, ok := consumeJSONValue(data, valueStart)
		if !ok {
			return valueStart, false
		}
		return valueEnd, true
	}
	return consumeJSONValue(data, valueStart)
}

func skipJSONWhitespace(data []byte, start int) int {
	for start < len(data) {
		switch data[start] {
		case ' ', '\n', '\r', '\t':
			start++
		default:
			return start
		}
	}
	return start
}

func consumeJSONString(data []byte, start int) (string, int, bool) {
	if start >= len(data) || data[start] != '"' {
		return "", start, false
	}

	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			raw := data[start : i+1]
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				return "", start, false
			}
			return value, i + 1, true
		}
	}

	return "", start, false
}

func consumeRequestIDValue(data []byte, start int) (RequestID, int, bool) {
	if start >= len(data) {
		return RequestID{}, start, false
	}

	switch data[start] {
	case '"':
		_, end, ok := consumeJSONString(data, start)
		if !ok {
			return RequestID{}, start, false
		}
		var id RequestID
		if err := json.Unmarshal(data[start:end], &id); err != nil {
			return RequestID{}, start, false
		}
		return id, end, true
	case '{', '[':
		return RequestID{}, start, false
	default:
		end, ok := consumeJSONScalar(data, start)
		if !ok {
			return RequestID{}, start, false
		}
		var id RequestID
		if err := json.Unmarshal(data[start:end], &id); err != nil {
			return RequestID{}, start, false
		}
		return id, end, true
	}
}

func consumeJSONValue(data []byte, start int) (int, bool) {
	if start >= len(data) {
		return start, false
	}

	switch data[start] {
	case '"':
		_, end, ok := consumeJSONString(data, start)
		return end, ok
	case '{', '[':
		return consumeCompositeJSONValue(data, start)
	default:
		return consumeJSONScalar(data, start)
	}
}

func consumeCompositeJSONValue(data []byte, start int) (int, bool) {
	var stack []byte
	i := start

	for i < len(data) {
		switch data[i] {
		case '"':
			_, next, ok := consumeJSONString(data, i)
			if !ok {
				return start, false
			}
			i = next
			continue
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) == 0 || data[i] != stack[len(stack)-1] {
				return start, false
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1, true
			}
		}
		i++
	}

	return start, false
}

func consumeJSONScalar(data []byte, start int) (int, bool) {
	i := start
	for i < len(data) {
		switch data[i] {
		case ',', '}', ']', ' ', '\n', '\r', '\t':
			end := i
			i = skipJSONWhitespace(data, i)
			if end > start && json.Valid(data[start:end]) {
				return i, true
			}
			return start, false
		default:
			i++
		}
	}

	if json.Valid(data[start:i]) {
		return i, true
	}
	return start, false
}

func consumeNestedJSONValue(decoder *json.Decoder) error {
	depth := 1
	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return err
		}
		d, ok := tok.(json.Delim)
		if !ok {
			continue
		}
		switch d {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
	}
	return nil
}

func extractInboundRequestObjectID(data []byte) (RequestID, bool, bool) {
	var topLevel map[string]json.RawMessage
	if json.Unmarshal(data, &topLevel) != nil {
		return RequestID{}, false, false
	}

	if _, hasMethod := topLevel["method"]; !hasMethod {
		return RequestID{}, false, false
	}

	rawID, hasID := topLevel["id"]
	if !hasID {
		return RequestID{}, false, true
	}

	id, err := parseRequestID(rawID)
	if err != nil {
		return RequestID{}, false, true
	}
	if _, err := normalizeID(id.Value); err != nil {
		return RequestID{}, false, true
	}
	return id, true, true
}
