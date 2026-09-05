package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// BenchmarkInboundPipeline measures one message in flight from pipe write to
// typed callback completion. It excludes server/model time and user callback work.
func BenchmarkInboundPipeline(b *testing.B) {
	for _, size := range []int{64 << 10, 1 << 20, 5 << 20, 9 << 20} {
		b.Run(fmt.Sprintf("TurnDiff/%dKiB", size>>10), func(b *testing.B) {
			unit := "diff\n+\t\"path\\file\"\n-old\n"
			encoded, err := json.Marshal(unit)
			if err != nil {
				b.Fatal(err)
			}
			diff := strings.Repeat(unit, size/(len(encoded)-2))
			payload, err := json.Marshal(protocol.TurnDiffUpdatedNotification{ThreadID: "thread-1", TurnID: "turn-1", Diff: diff})
			if err != nil {
				b.Fatal(err)
			}
			frame := append(append([]byte(`{"method":"turn/diff/updated","params":`), payload...), '}', '\n')
			if len(frame)-1 >= maxInboundMessageSizeBytes {
				b.Fatal("fixture exceeds inbound limit")
			}
			reader, writer := io.Pipe()
			transport := NewStdioTransport(reader, io.Discard)
			client := protocol.NewClient(transport)
			b.Cleanup(func() { _ = writer.Close(); _ = client.Close() })
			done := make(chan int, 1)
			client.OnTurnDiffUpdated(func(n protocol.TurnDiffUpdatedNotification) { done <- len(n.Diff) })
			// A watchdog also bounds a blocked pipe write if dispatch breaks.
			timer := time.AfterFunc(10*time.Minute, func() { _ = reader.CloseWithError(fmt.Errorf("benchmark watchdog expired")) })
			b.Cleanup(func() { timer.Stop() })
			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			for b.Loop() {
				if _, err := writer.Write(frame); err != nil {
					b.Fatal(err)
				}
				select {
				case length := <-done:
					if length != len(diff) {
						b.Fatal("incorrect decoded diff length")
					}
				case <-time.After(10 * time.Second):
					b.Fatal("typed handler did not receive notification")
				}
			}
		})
	}
}
