package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

// TestHandleMalformedRequestSendsParseError directly tests the defense-in-depth
// path where handleRequest's unmarshal fails and handleMalformedRequest sends
// a parse error response. With current types this path is unreachable via the
// normal readLoop because Request accepts any valid JSON, but the test verifies
// the error response format for future-proofing.
func TestHandleMalformedRequestSendsParseError(t *testing.T) {
	var buf safeBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transport := &StdioTransport{
		reader:        strings.NewReader(""),
		writer:        &buf,
		pendingReqs:   make(map[string]pendingReq),
		readerStopped: make(chan struct{}),
		ctx:           ctx,
		cancelCtx:     cancel,
	}

	// Call handleMalformedRequest with data containing a valid ID
	data := []byte(`{"id":"malformed-req","method":"test"}`)
	transport.handleMalformedRequest(data)

	// Verify the response written to the writer
	output := buf.String()
	if output == "" {
		t.Fatal("expected error response to be written")
	}

	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != ErrCodeParseError {
		t.Errorf("error code = %d; want %d", resp.Error.Code, ErrCodeParseError)
	}
	if resp.ID.Value != "malformed-req" {
		t.Errorf("response ID = %v; want malformed-req", resp.ID.Value)
	}
	if resp.Error.Data != nil {
		t.Errorf("error Data should be nil to avoid leaking internal details, got %s", resp.Error.Data)
	}
}

// safeBuffer is a concurrency-safe bytes.Buffer for testing.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
