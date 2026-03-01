package codex

import "encoding/json"

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

// RequestID is a union type that can be a string, int64, or null.
// JSON-RPC 2.0 spec allows id to be string | number | null.
//
// Because Go's encoding/json decodes JSON numbers as float64, the concrete
// type of Value may differ depending on whether the ID was constructed
// locally (uint64 from nextRequestID) or decoded from the wire (float64).
// Use [RequestID.Equal] instead of == to compare IDs across type boundaries.
type RequestID struct {
	Value interface{} // string | int64 | float64 | nil
}

// Equal reports whether r and other represent the same logical request ID,
// normalizing across numeric types. For example, uint64(1) and float64(1)
// are considered equal. String IDs and numeric IDs are never equal, even
// if they have the same textual representation.
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
	switch v := v.(type) {
	case float64:
		return normalizeID(v), true
	case int64:
		return normalizeID(v), true
	case int:
		return normalizeID(v), true
	case uint64:
		return normalizeID(v), true
	default:
		return "", false
	}
}

// MarshalJSON implements json.Marshaler for RequestID.
func (r RequestID) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Value)
}

// UnmarshalJSON implements json.Unmarshaler for RequestID.
func (r *RequestID) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as-is to preserve type
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	r.Value = v
	return nil
}
