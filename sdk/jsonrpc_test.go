package codex_test

import (
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/sdk"
)

func TestRequestMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		req  codex.Request
	}{
		{
			name: "string id",
			req: codex.Request{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: "req-123"},
				Method:  "initialize",
				Params:  json.RawMessage(`{"clientInfo":{"name":"test"}}`),
			},
		},
		{
			name: "int64 id",
			req: codex.Request{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: int64(42)},
				Method:  "thread/start",
				Params:  json.RawMessage(`{"prompt":"hello"}`),
			},
		},
		{
			name: "nil params",
			req: codex.Request{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: "req-456"},
				Method:  "model/list",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded codex.Request
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Compare fields
			if decoded.JSONRPC != tt.req.JSONRPC {
				t.Errorf("JSONRPC mismatch: got %q, want %q", decoded.JSONRPC, tt.req.JSONRPC)
			}
			if decoded.Method != tt.req.Method {
				t.Errorf("Method mismatch: got %q, want %q", decoded.Method, tt.req.Method)
			}
			if !requestIDEqual(decoded.ID, tt.req.ID) {
				t.Errorf("ID mismatch: got %v, want %v", decoded.ID, tt.req.ID)
			}
		})
	}
}

func TestResponseMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		resp codex.Response
	}{
		{
			name: "success with result",
			resp: codex.Response{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: "resp-123"},
				Result:  json.RawMessage(`{"status":"ok"}`),
			},
		},
		{
			name: "error response",
			resp: codex.Response{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: int64(42)},
				Error: &codex.Error{
					Code:    codex.ErrCodeInvalidParams,
					Message: "Invalid parameters",
					Data:    json.RawMessage(`{"field":"prompt"}`),
				},
			},
		},
		{
			name: "null id",
			resp: codex.Response{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: nil},
				Error: &codex.Error{
					Code:    codex.ErrCodeParseError,
					Message: "Parse error",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded codex.Response
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if decoded.JSONRPC != tt.resp.JSONRPC {
				t.Errorf("JSONRPC mismatch: got %q, want %q", decoded.JSONRPC, tt.resp.JSONRPC)
			}
			if !requestIDEqual(decoded.ID, tt.resp.ID) {
				t.Errorf("ID mismatch: got %v, want %v", decoded.ID, tt.resp.ID)
			}

			// Check error equality
			if (decoded.Error == nil) != (tt.resp.Error == nil) {
				t.Errorf("Error presence mismatch")
			}
			if decoded.Error != nil && tt.resp.Error != nil {
				if decoded.Error.Code != tt.resp.Error.Code {
					t.Errorf("Error code mismatch: got %d, want %d", decoded.Error.Code, tt.resp.Error.Code)
				}
				if decoded.Error.Message != tt.resp.Error.Message {
					t.Errorf("Error message mismatch: got %q, want %q", decoded.Error.Message, tt.resp.Error.Message)
				}
			}
		})
	}
}

func TestNotificationMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name  string
		notif codex.Notification
	}{
		{
			name: "with params",
			notif: codex.Notification{
				JSONRPC: "2.0",
				Method:  "thread/started",
				Params:  json.RawMessage(`{"threadId":"thread-123"}`),
			},
		},
		{
			name: "nil params",
			notif: codex.Notification{
				JSONRPC: "2.0",
				Method:  "thread/closed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.notif)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded codex.Notification
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if decoded.JSONRPC != tt.notif.JSONRPC {
				t.Errorf("JSONRPC mismatch: got %q, want %q", decoded.JSONRPC, tt.notif.JSONRPC)
			}
			if decoded.Method != tt.notif.Method {
				t.Errorf("Method mismatch: got %q, want %q", decoded.Method, tt.notif.Method)
			}
		})
	}
}

func TestErrorSerialization(t *testing.T) {
	tests := []struct {
		name string
		err  codex.Error
	}{
		{
			name: "with data",
			err: codex.Error{
				Code:    codex.ErrCodeInvalidRequest,
				Message: "Invalid request",
				Data:    json.RawMessage(`{"detail":"missing field"}`),
			},
		},
		{
			name: "without data",
			err: codex.Error{
				Code:    codex.ErrCodeMethodNotFound,
				Message: "Method not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.err)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded codex.Error
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if decoded.Code != tt.err.Code {
				t.Errorf("Code mismatch: got %d, want %d", decoded.Code, tt.err.Code)
			}
			if decoded.Message != tt.err.Message {
				t.Errorf("Message mismatch: got %q, want %q", decoded.Message, tt.err.Message)
			}
		})
	}
}

func TestErrorCodeConstants(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"parse error", codex.ErrCodeParseError, -32700},
		{"invalid request", codex.ErrCodeInvalidRequest, -32600},
		{"method not found", codex.ErrCodeMethodNotFound, -32601},
		{"invalid params", codex.ErrCodeInvalidParams, -32602},
		{"internal error", codex.ErrCodeInternalError, -32603},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("Error code %s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}

func TestRequestIDStringInt64Union(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantType string
		wantVal  string
	}{
		{
			name:     "string id",
			json:     `{"jsonrpc":"2.0","id":"req-123","method":"test"}`,
			wantType: "string",
			wantVal:  "req-123",
		},
		{
			name:     "numeric id",
			json:     `{"jsonrpc":"2.0","id":42,"method":"test"}`,
			wantType: "int64",
			wantVal:  "42",
		},
		{
			name:     "null id",
			json:     `{"jsonrpc":"2.0","id":null,"method":"test"}`,
			wantType: "nil",
			wantVal:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req codex.Request
			if err := json.Unmarshal([]byte(tt.json), &req); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			switch tt.wantType {
			case "string":
				if s, ok := req.ID.Value.(string); !ok || s != tt.wantVal {
					t.Errorf("Expected string ID %q, got %v (type %T)", tt.wantVal, req.ID.Value, req.ID.Value)
				}
			case "int64":
				if n, ok := req.ID.Value.(int64); !ok || n != 42 {
					t.Errorf("Expected int64 ID %q, got %v (type %T)", tt.wantVal, req.ID.Value, req.ID.Value)
				}
			case "nil":
				if req.ID.Value != nil {
					t.Errorf("Expected nil ID, got %v", req.ID.Value)
				}
			}
		})
	}
}

func TestRequestIDUnmarshalPreservesLargeNumericPrecision(t *testing.T) {
	var req codex.Request
	raw := []byte(`{"jsonrpc":"2.0","id":9007199254740993,"method":"test"}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	id, ok := req.ID.Value.(int64)
	if !ok {
		t.Fatalf("ID type = %T, want int64", req.ID.Value)
	}
	if id != 9007199254740993 {
		t.Fatalf("ID value = %d, want 9007199254740993", id)
	}
}

func TestRequestIDUnmarshalRejectsInvalidTypes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "object id", raw: `{"jsonrpc":"2.0","id":{},"method":"test"}`},
		{name: "array id", raw: `{"jsonrpc":"2.0","id":[],"method":"test"}`},
		{name: "boolean id", raw: `{"jsonrpc":"2.0","id":true,"method":"test"}`},
		{name: "decimal number id", raw: `{"jsonrpc":"2.0","id":1.0,"method":"test"}`},
		{name: "fractional number id", raw: `{"jsonrpc":"2.0","id":2.5,"method":"test"}`},
		{name: "scientific number id", raw: `{"jsonrpc":"2.0","id":1e3,"method":"test"}`},
		{name: "out of range number id", raw: `{"jsonrpc":"2.0","id":9223372036854775808,"method":"test"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req codex.Request
			if err := json.Unmarshal([]byte(tt.raw), &req); err == nil {
				t.Fatalf("expected unmarshal error for invalid id type")
			}
		})
	}
}

// Helper to compare RequestIDs (handles string, int64, float64, and nil)
func requestIDEqual(a, b codex.RequestID) bool {
	if a.Value == nil && b.Value == nil {
		return true
	}
	if a.Value == nil || b.Value == nil {
		return false
	}

	// Handle numeric types (JSON unmarshal gives float64, but we construct with int64)
	aNum, aIsNum := toInt64(a.Value)
	bNum, bIsNum := toInt64(b.Value)
	if aIsNum && bIsNum {
		return aNum == bNum
	}

	return a.Value == b.Value
}

func toInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case json.Number:
		i, err := val.Int64()
		if err != nil {
			return 0, false
		}
		return i, true
	case float64:
		return int64(val), true
	case int:
		return int64(val), true
	default:
		return 0, false
	}
}

func TestRequestIDEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b codex.RequestID
		want bool
	}{
		{
			name: "uint64 vs float64 same value",
			a:    codex.RequestID{Value: uint64(99)},
			b:    codex.RequestID{Value: float64(99)},
			want: true,
		},
		{
			name: "int64 vs float64 same value",
			a:    codex.RequestID{Value: int64(42)},
			b:    codex.RequestID{Value: float64(42)},
			want: true,
		},
		{
			name: "string vs string equal",
			a:    codex.RequestID{Value: "req-1"},
			b:    codex.RequestID{Value: "req-1"},
			want: true,
		},
		{
			name: "string vs string differ",
			a:    codex.RequestID{Value: "req-1"},
			b:    codex.RequestID{Value: "req-2"},
			want: false,
		},
		{
			name: "uint64 vs float64 differ",
			a:    codex.RequestID{Value: uint64(1)},
			b:    codex.RequestID{Value: float64(2)},
			want: false,
		},
		{
			name: "nil vs nil",
			a:    codex.RequestID{Value: nil},
			b:    codex.RequestID{Value: nil},
			want: true,
		},
		{
			name: "nil vs non-nil",
			a:    codex.RequestID{Value: nil},
			b:    codex.RequestID{Value: uint64(1)},
			want: false,
		},
		{
			name: "string vs int",
			a:    codex.RequestID{Value: "42"},
			b:    codex.RequestID{Value: uint64(42)},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Errorf("RequestID(%v).Equal(%v) = %v, want %v", tt.a.Value, tt.b.Value, got, tt.want)
			}
		})
	}
}
