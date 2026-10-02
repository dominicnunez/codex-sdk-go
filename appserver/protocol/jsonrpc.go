package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
)

// jsonrpcVersion is the protocol version string for JSON-RPC 2.0.
const jsonrpcVersion = "2.0"

// JSON-RPC 2.0 error codes
const (
	ErrCodeParseError     = -32700 // Invalid JSON was received
	ErrCodeInvalidRequest = -32600 // The JSON sent is not a valid Request object
	ErrCodeMethodNotFound = -32601 // The method does not exist / is not available
	ErrCodeInvalidParams  = -32602 // Invalid method parameter(s)
	ErrCodeInternalError  = -32603 // Internal error
)

// Request represents a JSON-RPC 2.0 request sent from client to server.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      RequestID       `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response from server to client.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      RequestID       `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Notification represents a JSON-RPC 2.0 notification (request without id).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// RequestID is a protocol request ID.
//
// Protocol requests and responses use string IDs or 64-bit signed integer IDs.
// A nil value is reserved for JSON-RPC parse/invalid-request error handling.
// Use [RequestID.Equal] instead of == to compare IDs across type boundaries.
type RequestID struct {
	Value interface{} // string | int64 | compatible Go integer forms | nil
}

// Equal reports whether r and other represent the same logical request ID,
// normalizing compatible integer-valued Go numeric types to int64. String IDs
// and numeric IDs are never equal, even if they have the same textual
// representation.
func (r RequestID) Equal(other RequestID) bool {
	if r.Value == nil && other.Value == nil {
		return true
	}
	if r.Value == nil || other.Value == nil {
		return false
	}
	rNum, rIsNum := isNumericID(r.Value)
	oNum, oIsNum := isNumericID(other.Value)
	if rIsNum && oIsNum {
		return rNum == oNum
	}
	// Both must be strings (same type family) to match.
	rStr, rIsStr := r.Value.(string)
	oStr, oIsStr := other.Value.(string)
	if rIsStr && oIsStr {
		return rStr == oStr
	}
	return false
}

// isNumericID normalizes a numeric ID value to a comparable string.
// Returns the normalized form and true if the value is numeric, or ("", false) otherwise.
func isNumericID(v interface{}) (string, bool) {
	s, isNumeric, err := normalizeNumericID(v)
	if err != nil || !isNumeric {
		return "", false
	}
	return s, true
}

// MarshalJSON implements json.Marshaler for RequestID.
func (r RequestID) MarshalJSON() ([]byte, error) {
	value, err := canonicalRequestIDValue(r.Value, true)
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

// UnmarshalJSON implements json.Unmarshaler for RequestID.
func (r *RequestID) UnmarshalJSON(data []byte) error {
	if !json.Valid(data) {
		return invalidRequestIDSyntax(data)
	}
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		r.Value = nil
		return nil
	}

	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		r.Value = value
		return nil
	}
	// Reject composite IDs before they can materialize a peer-controlled graph.
	// An int64 spelling is at most 20 bytes; reject longer input before an
	// error formatter or numeric parser can copy the entire rejected value.
	if len(data) > 20 || (data[0] != '-' && (data[0] < '0' || data[0] > '9')) {
		return errUnexpectedIDType
	}
	value, err := parseJSONRequestID(string(data))
	if err != nil {
		return err
	}
	r.Value = value
	return nil
}

// Preserve the original Decoder diagnostics and trailing-input error without
// constructing a value graph. Decoder validates before invoking this sink.
type discardedRequestIDValue struct{}

func (*discardedRequestIDValue) UnmarshalJSON([]byte) error { return nil }

func invalidRequestIDSyntax(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var ignored discardedRequestIDValue
	if err := decoder.Decode(&ignored); err != nil {
		return err
	}
	// Whole-input validation already failed. If the first value decoded, the
	// remaining input is exactly the old trailing-input error case.
	return errors.New("invalid request id")
}
