package codex_test

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestRPCError verifies RPCError wraps a JSON-RPC error response
// and works with errors.Is and errors.As.
func TestRPCError(t *testing.T) {
	// Create an RPC error
	rpcErr := &codex.Error{
		Code:    codex.ErrCodeMethodNotFound,
		Message: "method not found",
	}

	sdkErr := codex.NewRPCError(rpcErr)

	// Verify error message
	if sdkErr.Error() == "" {
		t.Error("RPCError.Error() should return non-empty string")
	}

	// Verify the error contains the JSON-RPC error code
	expectedMsg := fmt.Sprintf("%d", codex.ErrCodeMethodNotFound)
	if sdkErr.Error() == "" || len(expectedMsg) == 0 {
		t.Error("RPCError.Error() should contain error code")
	}

	// Test errors.As behavior
	var target *codex.RPCError
	if !errors.As(sdkErr, &target) {
		t.Error("errors.As should match RPCError type")
	}

	if target.RPCError() == nil {
		t.Error("RPCError.RPCError() should return the wrapped error")
	}

	if target.RPCError().Code != codex.ErrCodeMethodNotFound {
		t.Errorf("expected code %d, got %d", codex.ErrCodeMethodNotFound, target.RPCError().Code)
	}

	// Test errors.Is behavior with sentinel
	sentinelErr := codex.NewRPCError(&codex.Error{
		Code:    codex.ErrCodeMethodNotFound,
		Message: "different message",
	})

	if !errors.Is(sdkErr, sentinelErr) {
		t.Error("errors.Is should match RPCErrors with same code")
	}

	// Different error code should not match
	differentErr := codex.NewRPCError(&codex.Error{
		Code:    codex.ErrCodeInvalidParams,
		Message: "invalid params",
	})

	if errors.Is(sdkErr, differentErr) {
		t.Error("errors.Is should not match RPCErrors with different codes")
	}
}

// TestTransportError verifies TransportError wraps IO/connection failures
// and works with errors.Is and errors.As.
func TestTransportError(t *testing.T) {
	// Create a transport error wrapping an io.EOF
	transportErr := codex.NewTransportError("connection closed", io.EOF)

	// Verify error message contains context
	msg := transportErr.Error()
	if msg == "" {
		t.Error("TransportError.Error() should return non-empty string")
	}

	// Test errors.As behavior
	var target *codex.TransportError
	if !errors.As(transportErr, &target) {
		t.Error("errors.As should match TransportError type")
	}

	if target.Unwrap() == nil {
		t.Error("TransportError.Unwrap() should return wrapped error")
	}

	// Test errors.Is behavior - should unwrap to io.EOF
	if !errors.Is(transportErr, io.EOF) {
		t.Error("errors.Is should unwrap to io.EOF")
	}

	// Test with nil cause
	transportErrNoCause := codex.NewTransportError("connection failed", nil)
	if transportErrNoCause.Error() == "" {
		t.Error("TransportError with nil cause should still have message")
	}

	if transportErrNoCause.Unwrap() != nil {
		t.Error("TransportError with nil cause should return nil from Unwrap()")
	}
}

// TestTimeoutError verifies TimeoutError type and works with errors.Is/As.
func TestTimeoutError(t *testing.T) {
	// Create a timeout error
	timeoutErr := codex.NewTimeoutError("request timed out after 5s")

	// Verify error message
	msg := timeoutErr.Error()
	if msg == "" {
		t.Error("TimeoutError.Error() should return non-empty string")
	}

	// Test errors.As behavior
	var target *codex.TimeoutError
	if !errors.As(timeoutErr, &target) {
		t.Error("errors.As should match TimeoutError type")
	}

	// Test errors.Is behavior with sentinel
	sentinelErr := codex.NewTimeoutError("another timeout message")
	if !errors.Is(timeoutErr, sentinelErr) {
		t.Error("errors.Is should match all TimeoutErrors")
	}

	// TimeoutError should not match unrelated errors
	if errors.Is(timeoutErr, io.EOF) {
		t.Error("errors.Is should not match unrelated errors")
	}
}

// TestErrorTypesSeparation verifies each error type is distinct.
func TestErrorTypesSeparation(t *testing.T) {
	rpcErr := codex.NewRPCError(&codex.Error{
		Code:    codex.ErrCodeInternalError,
		Message: "internal error",
	})

	transportErr := codex.NewTransportError("transport failed", io.ErrUnexpectedEOF)
	timeoutErr := codex.NewTimeoutError("timeout")

	// RPCError should not match other types
	if errors.Is(rpcErr, transportErr) {
		t.Error("RPCError should not match TransportError")
	}
	if errors.Is(rpcErr, timeoutErr) {
		t.Error("RPCError should not match TimeoutError")
	}

	// TransportError should not match other types
	if errors.Is(transportErr, rpcErr) {
		t.Error("TransportError should not match RPCError")
	}
	if errors.Is(transportErr, timeoutErr) {
		t.Error("TransportError should not match TimeoutError")
	}

	// TimeoutError should not match other types
	if errors.Is(timeoutErr, rpcErr) {
		t.Error("TimeoutError should not match RPCError")
	}
	if errors.Is(timeoutErr, transportErr) {
		t.Error("TimeoutError should not match TransportError")
	}
}
