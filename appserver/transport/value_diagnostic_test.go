package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRejectedValueDiagnosticTransport(t *testing.T) {
	t.Run("duplicate-admission", func(t *testing.T) {
		reader, peer := io.Pipe()
		transport := NewStdioTransport(reader, io.Discard)
		t.Cleanup(func() { _ = peer.Close(); _ = transport.Close() })
		for _, id := range []string{"short-id", strings.Repeat("x", 1<<20)} {
			key, err := normalizePendingRequestID(id)
			if err != nil {
				t.Fatal(err)
			}
			pending := pendingReq{ch: make(chan pendingReqResult, 1), id: RequestID{Value: id}}
			admit := func() {
				transport.mu.Lock()
				transport.pendingReqs[key] = pending
				transport.mu.Unlock()
			}
			admit()
			_, err = transport.Send(context.Background(), Request{ID: RequestID{Value: id}, Method: "test"})
			var typed *TransportError
			if err == nil || !errors.As(err, &typed) || !strings.Contains(err.Error(), "duplicate request ID:") || len(err.Error()) > 1800 {
				t.Error("unbounded duplicate diagnostic or lost transport classification")
			}
			if id == "short-id" && !strings.HasSuffix(err.Error(), "duplicate request ID: short-id") {
				t.Error("short duplicate diagnostic changed")
			}
			transport.mu.Lock()
			retained := transport.pendingReqs[key]
			delete(transport.pendingReqs, key)
			transport.mu.Unlock()
			if retained.ch != pending.ch || retained.id.Value != id {
				t.Fatal("duplicate rejection replaced the admitted request")
			}
		}
	})
	_, _, err := canonicalInt64RequestID(json.Number(strings.Repeat("9", 1<<20)))
	if err == nil || !errors.Is(err, errUnexpectedIDType) || len(err.Error()) > 1800 {
		t.Error("unbounded constructed numeric identifier diagnostic or lost classification")
	}
}
