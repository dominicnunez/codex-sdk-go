package codex_test

import (
	"context"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestTransportInterface verifies the Transport interface contract is satisfied
func TestTransportInterface(t *testing.T) {
	// This test verifies that the Transport interface type exists and has the expected methods
	// by compiling a function that uses all methods of the interface.

	var checkInterface = func(tr codex.Transport) {
		ctx := context.Background()

		// Send method
		req := codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: "1"},
			Method:  "test",
		}
		_, _ = tr.Send(ctx, req)

		// Notify method
		notif := codex.Notification{
			JSONRPC: "2.0",
			Method:  "test",
		}
		_ = tr.Notify(ctx, notif)

		// OnRequest method
		tr.OnRequest(func(ctx context.Context, req codex.Request) (codex.Response, error) {
			return codex.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
			}, nil
		})

		// OnNotify method
		tr.OnNotify(func(ctx context.Context, notif codex.Notification) {})

		// Close method
		_ = tr.Close()
	}

	// This test passes if the code compiles, proving the interface exists with the expected signature
	_ = checkInterface
}

// TestTransportMethodSignatures verifies method signatures match expectations
func TestTransportMethodSignatures(t *testing.T) {
	// Verify RequestHandler type exists and has correct signature
	var _ codex.RequestHandler = func(ctx context.Context, req codex.Request) (codex.Response, error) {
		return codex.Response{}, nil
	}

	// Verify NotificationHandler type exists and has correct signature
	var _ codex.NotificationHandler = func(ctx context.Context, notif codex.Notification) {}
}
