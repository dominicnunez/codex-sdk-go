//go:build !race

package appserver_test

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
)

// This public-path fixture decodes more than 2 GiB of JSON. Race builds
// exercise the long counter workload directly in TestCollectorCommandByteCounterOwner
// and keep the existing smaller public notification/lifecycle cases.
func TestCollectorCommandByteCounter(t *testing.T) {
	proc, mock := mockProcess(t)
	collector := codex.NewStreamCollector()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	stream := proc.RunStreamedWithCollector(ctx, codex.RunOptions{Prompt: "synthetic accounting fixture"}, collector)
	done := make(chan error, 1)
	go func() {
		var result error
		for _, err := range stream.Events() {
			if err != nil {
				result = err
			}
		}
		done <- result
	}()
	waitForRunStreamedReady(t, mock)
	const chunkBytes = 8 << 20
	const retainedBytes = 64 << 10
	// Reuse one frame; the fixture never retains a multi-gigabyte history.
	delta := strings.Repeat("x", chunkBytes-4) + "tail"
	body := json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"command","delta":"` + delta + `"}`)
	for count := 1; count <= 257; count++ {
		mock.InjectServerNotification(ctx, codex.Notification{JSONRPC: "2.0", Method: "item/commandExecution/outputDelta", Params: body})
		if err := ctx.Err(); err != nil {
			t.Fatalf("notification fixture interrupted after %d chunks: %v", count, err)
		}
		if count < 256 {
			continue
		}
		command := collector.Summary().CommandExecutions["command"]
		want := int64(count)*chunkBytes - retainedBytes
		if command.DroppedOutputDeltaBytes != want {
			t.Errorf("%s: after %d chunks dropped %d bytes, want %d", runtime.GOARCH, count, command.DroppedOutputDeltaBytes, want)
		}
		if len(command.OutputDeltas) != 1 || command.OutputDeltas[0] != delta[chunkBytes-retainedBytes:] || command.DroppedOutputDeltas != count-1 {
			t.Fatal("command did not retain the expected suffix and delta count")
		}
	}
	// Wrong run identities must not alter the high counter or create records.
	for _, body := range []json.RawMessage{
		json.RawMessage(`{"threadId":"other","turnId":"turn-1","itemId":"command","delta":"wrong"}`),
		json.RawMessage(`{"threadId":"thread-1","turnId":"other","itemId":"command","delta":"wrong"}`),
	} {
		mock.InjectServerNotification(ctx, codex.Notification{JSONRPC: "2.0", Method: "item/commandExecution/outputDelta", Params: body})
	}
	mock.InjectServerNotification(ctx, codex.Notification{JSONRPC: "2.0", Method: "item/commandExecution/outputDelta", Params: json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"other","delta":"separate"}`)})
	mock.InjectServerNotification(ctx, codex.Notification{JSONRPC: "2.0", Method: "item/completed", Params: json.RawMessage(`{"completedAtMs":1,"threadId":"thread-1","turnId":"turn-1","item":{"type":"commandExecution","id":"command","command":"synthetic","commandActions":[],"cwd":"/tmp","status":"completed","aggregatedOutput":"complete"}}`)})
	mock.InjectServerNotification(ctx, codex.Notification{JSONRPC: "2.0", Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("stream did not complete")
	}
	summary := collector.Summary()
	command := summary.CommandExecutions["command"]
	if !command.Completed || command.AggregatedOutput != "complete" || command.DroppedOutputDeltas != 256 {
		t.Fatal("command completion changed the accumulated accounting or final output")
	}
	if command.DroppedOutputDeltaBytes != int64(257)*chunkBytes-retainedBytes {
		t.Fatal("command completion changed the accumulated byte count")
	}
	other := summary.CommandExecutions["other"]
	if len(summary.CommandExecutions) != 2 || other.DroppedOutputDeltaBytes != 0 || other.DroppedOutputDeltas != 0 || len(other.OutputDeltas) != 1 || other.OutputDeltas[0] != "separate" {
		t.Fatal("command identities did not retain separate accounting")
	}
}
