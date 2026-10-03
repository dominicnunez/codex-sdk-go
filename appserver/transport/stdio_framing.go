package transport

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
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
	JSONRPC inboundProtocolVersion `json:"jsonrpc"`
	ID      inboundID              `json:"id"`
	Method  string                 `json:"method"`
	Params  json.RawMessage        `json:"params,omitempty"`
	Result  json.RawMessage        `json:"result,omitempty"`
	Error   inboundError           `json:"error,omitempty"`
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

func (f inboundFrame) protocolVersion() string {
	if !f.JSONRPC.present {
		return ""
	}
	return f.JSONRPC.value
}

func (f inboundFrame) hasInvalidProtocolVersion() bool {
	return f.JSONRPC.present && (f.JSONRPC.invalid || f.JSONRPC.value != jsonrpcVersion)
}

type inboundProtocolVersion struct {
	present bool
	value   string
	invalid bool
}

func (v *inboundProtocolVersion) UnmarshalJSON(data []byte) error {
	v.present = true
	var parsed string
	if err := json.Unmarshal(data, &parsed); err != nil {
		v.invalid = true
		return nil //nolint:nilerr // Preserve frame routing; invalid version is handled after classification.
	}
	v.value = parsed
	v.invalid = false
	return nil
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

func (i inboundID) isPresent() bool {
	return i.present
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

	parsed, ok := decodeInboundRPCError(data)
	if !ok {
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
// Wire IDs decode as int64; compatible caller-provided numeric forms normalize
// to the same integer spelling. Pending keys separately prefix the type family.
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
		line = appendLineFragment(line, frag, limit, errors.Is(err, bufio.ErrBufferFull))
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

// Grow geometrically instead of append's smaller growth steps for large slices.
// Reserve no more than limit plus the newline unless the fragment itself is
// already oversized. Each returned line owns its storage; no large buffers are
// pooled or retained between messages.
func appendLineFragment(line, fragment []byte, limit int, more bool) []byte {
	needed := len(line) + len(fragment)
	if needed > cap(line) {
		capacity := needed
		if more {
			capacity = 2 * cap(line)
			if capacity > limit {
				capacity = limit + 1
			}
		}
		if capacity < needed {
			capacity = needed
		}
		grown := make([]byte, len(line), capacity)
		copy(grown, line)
		line = grown
	}
	return append(line, fragment...)
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
	if !json.Valid(data) {
		return frame, errors.New("invalid inbound JSON")
	}
	var version, id, method, params, result, rpcError json.RawMessage
	methodInvalid := false
	if !walkJSONObjectFields(data, true, func(key, value []byte) {
		switch {
		case jsonFieldMatchesFolded(key, "jsonrpc"):
			version = value
		case jsonFieldMatchesFolded(key, "id"):
			id = value
		case jsonFieldMatchesFolded(key, "method"):
			// A null leaves the previous string intact. A wrong type anywhere
			// remains an error, even when a later duplicate has a valid string.
			if value[0] == '"' {
				method = value
			} else if !bytes.Equal(value, []byte("null")) {
				methodInvalid = true
			}
		case jsonFieldMatchesFolded(key, "params"):
			params = value
		case jsonFieldMatchesFolded(key, "result"):
			result = value
		case jsonFieldMatchesFolded(key, "error"):
			rpcError = value
		}
	}) || methodInvalid {
		return frame, errors.New("invalid inbound object")
	}
	// Decode selected fields once. Raw data is copied only after selection,
	// so ignored keys and overwritten duplicate IDs cannot amplify memory.
	if len(version) > 0 {
		_ = frame.JSONRPC.UnmarshalJSON(version)
	}
	if len(id) > 0 {
		_ = frame.ID.UnmarshalJSON(id)
	}
	if len(method) > 0 {
		if err := json.Unmarshal(method, &frame.Method); err != nil {
			return inboundFrame{}, err
		}
	}
	frame.Params = append(json.RawMessage(nil), params...)
	frame.Result = append(json.RawMessage(nil), result...)
	if len(rpcError) > 0 {
		_ = frame.Error.UnmarshalJSON(rpcError)
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
		JSONRPC: f.protocolVersion(),
		Method:  f.Method,
		Params:  f.Params,
	}
}

func extractTopLevelIDAndMethod(data []byte) (RequestID, bool, bool) {
	var id RequestID
	var selected json.RawMessage
	var hasID bool
	var hasMethod bool
	// Keep useful correlation before a malformed suffix, without decoding
	// every token in unrelated metadata. Unsupported later IDs do not erase
	// an earlier scalar ID in this best-effort recovery path.
	walkJSONObjectFields(data, false, func(key, value []byte) {
		switch {
		case jsonFieldMatches(key, "id"):
			if value[0] == '"' || value[0] == '-' || (value[0] >= '0' && value[0] <= '9') {
				selected, hasID = value, true
			}
		case jsonFieldMatches(key, "method"):
			if value[0] == '"' {
				hasMethod = true
			}
		}
	})
	if hasID {
		if selected[0] == '"' {
			var value string
			_ = json.Unmarshal(selected, &value)
			id.Value = value
		} else {
			id.Value = json.Number(string(selected))
		}
	}
	return id, hasID, hasMethod
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

func inspectOversizedFramePrefix(data []byte) (info oversizedFrameInfo) {
	var selectedID json.RawMessage
	defer func() {
		if len(selectedID) > 0 {
			_ = json.Unmarshal(selectedID, &info.id)
		}
	}()

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

		next, ok := scanJSONStringEnd(data, i)
		if !ok || !json.Valid(data[i:next]) {
			return info
		}
		key := ""
		for _, name := range [...]string{"id", "method", "result", "error"} {
			if jsonFieldMatches(data[i:next], name) {
				key = name
				break
			}
		}
		i = skipJSONWhitespace(data, next)
		if i >= len(data) || data[i] != ':' {
			return info
		}
		i = skipJSONWhitespace(data, i+1)
		if i >= len(data) {
			return info
		}

		valueEnd, ok := inspectOversizedFrameField(data, key, i, &info, &selectedID)
		if !ok || info.hasMethod || (info.hasResponseFields && info.hasID) {
			return info
		}
		i = valueEnd
	}

	return info
}

func inspectOversizedFrameField(data []byte, key string, valueStart int, info *oversizedFrameInfo, selectedID *json.RawMessage) (int, bool) {
	switch key {
	case "id":
		valueEnd, ok := consumeJSONValue(data, valueStart)
		if !ok || !validRawRequestID(data[valueStart:valueEnd]) {
			return valueStart, false
		}
		*selectedID = data[valueStart:valueEnd]
		info.hasID = !bytes.Equal(*selectedID, []byte("null"))
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

func validRawRequestID(raw []byte) bool {
	if raw[0] == '"' || bytes.Equal(raw, []byte("null")) {
		return true
	}
	if len(raw) > 20 {
		return false
	}
	_, err := strconv.ParseInt(string(raw), 10, 64)
	return err == nil
}

func consumeJSONValue(data []byte, start int) (int, bool) {
	end, ok := scanJSONValueEnd(data, start)
	if !ok || !json.Valid(data[start:end]) {
		return start, false
	}
	// Oversized scalar inspection requires a delimiter. Malformed Token
	// recovery separately permits useful scalar prefixes such as 7 in 7x.
	if data[start] != '"' && data[start] != '{' && data[start] != '[' && end < len(data) {
		switch data[end] {
		case ',', '}', ']', ' ', '\n', '\r', '\t':
		default:
			return start, false
		}
	}
	return end, true
}

func extractInboundRequestObjectID(data []byte) (RequestID, bool, bool) {
	if !json.Valid(data) {
		return RequestID{}, false, false
	}

	var rawID json.RawMessage
	var hasID, hasMethod bool
	if !walkJSONObjectFields(data, true, func(key, value []byte) {
		if jsonFieldMatches(key, "method") {
			hasMethod = true
		}
		if jsonFieldMatches(key, "id") {
			rawID, hasID = value, true
		}
	}) || !hasMethod {
		return RequestID{}, false, false
	}

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
