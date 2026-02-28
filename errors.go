package codex

import (
	"fmt"
)

// RPCError wraps a JSON-RPC error response.
// It implements error, errors.Is, and errors.As.
type RPCError struct {
	err *Error
}

// NewRPCError creates a new RPCError wrapping a JSON-RPC error.
func NewRPCError(err *Error) *RPCError {
	return &RPCError{err: err}
}

// Error implements the error interface.
// Note: when Data is present, its raw content is included in the string.
// Data is server-controlled and may contain sensitive information (internal
// paths, tokens, PII). Callers who log or display this value should be aware.
func (e *RPCError) Error() string {
	if e.err == nil {
		return "rpc error: <nil>"
	}
	if len(e.err.Data) > 0 {
		return fmt.Sprintf("rpc error: code=%d message=%q data=%s", e.err.Code, e.err.Message, string(e.err.Data))
	}
	return fmt.Sprintf("rpc error: code=%d message=%q", e.err.Code, e.err.Message)
}

// RPCError returns the underlying JSON-RPC error.
func (e *RPCError) RPCError() *Error {
	return e.err
}

// Code returns the JSON-RPC error code.
func (e *RPCError) Code() int {
	if e.err == nil {
		return 0
	}
	return e.err.Code
}

// Message returns the JSON-RPC error message.
func (e *RPCError) Message() string {
	if e.err == nil {
		return ""
	}
	return e.err.Message
}

// Is implements errors.Is by comparing error codes.
// Two RPCErrors match if they have the same error code.
func (e *RPCError) Is(target error) bool {
	t, ok := target.(*RPCError)
	if !ok {
		return false
	}
	if e.err == nil || t.err == nil {
		return e.err == t.err
	}
	return e.err.Code == t.err.Code
}

// TransportError wraps IO/connection failures.
// It implements error, errors.Is (via Unwrap), and errors.As.
type TransportError struct {
	msg   string
	cause error
}

// NewTransportError creates a new TransportError with a message and optional cause.
func NewTransportError(msg string, cause error) *TransportError {
	return &TransportError{msg: msg, cause: cause}
}

// Error implements the error interface.
func (e *TransportError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("transport error: %s: %v", e.msg, e.cause)
	}
	return fmt.Sprintf("transport error: %s", e.msg)
}

// Unwrap returns the underlying cause, enabling errors.Is to traverse the chain.
func (e *TransportError) Unwrap() error {
	return e.cause
}

// TimeoutError represents a request timeout.
// It implements error, errors.Is, errors.As, and Unwrap.
type TimeoutError struct {
	msg   string
	cause error
}

// NewTimeoutError creates a new TimeoutError with the given message and optional cause.
func NewTimeoutError(msg string, cause ...error) *TimeoutError {
	e := &TimeoutError{msg: msg}
	if len(cause) > 0 {
		e.cause = cause[0]
	}
	return e
}

// Error implements the error interface.
func (e *TimeoutError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("timeout error: %s: %v", e.msg, e.cause)
	}
	return fmt.Sprintf("timeout error: %s", e.msg)
}

// Unwrap returns the underlying cause, enabling errors.Is to traverse the chain.
func (e *TimeoutError) Unwrap() error {
	return e.cause
}

// Is implements errors.Is by matching all TimeoutError instances.
// All timeouts are semantically equivalent.
func (e *TimeoutError) Is(target error) bool {
	_, ok := target.(*TimeoutError)
	return ok
}

// CanceledError represents an explicit context cancellation (user-initiated).
// Distinct from TimeoutError which represents deadline-driven cancellation.
type CanceledError struct {
	msg   string
	cause error
}

// NewCanceledError creates a new CanceledError with the given message and optional cause.
func NewCanceledError(msg string, cause ...error) *CanceledError {
	e := &CanceledError{msg: msg}
	if len(cause) > 0 {
		e.cause = cause[0]
	}
	return e
}

// Error implements the error interface.
func (e *CanceledError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("canceled: %s: %v", e.msg, e.cause)
	}
	return fmt.Sprintf("canceled: %s", e.msg)
}

// Unwrap returns the underlying cause, enabling errors.Is to traverse the chain.
func (e *CanceledError) Unwrap() error {
	return e.cause
}

// Is implements errors.Is by matching all CanceledError instances.
func (e *CanceledError) Is(target error) bool {
	_, ok := target.(*CanceledError)
	return ok
}
