package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func BenchmarkNotificationPayloadFanout(b *testing.B) {
	for _, size := range []int{256, 64 * 1024} {
		for _, listeners := range []int{1, 8} {
			b.Run(fmt.Sprintf("bytes=%d/listeners=%d", size, listeners), func(b *testing.B) {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				b.Cleanup(func() { _ = client.Close() })
				var observed byte
				for range listeners {
					client.AddNotificationListener("extension/changed", func(_ context.Context, n p.Notification) {
						observed ^= n.Params[0]
					})
				}
				notification := p.Notification{
					Method: "extension/changed",
					Params: json.RawMessage(`{"value":"` + strings.Repeat("x", size) + `"}`),
				}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					mock.InjectServerNotification(context.Background(), notification)
				}
				b.StopTimer()
				_ = observed
			})
		}
	}
}

func BenchmarkTypedNotificationPayload(b *testing.B) {
	for _, size := range []int{256, 64 * 1024} {
		for _, mode := range []string{"replacement", "additive"} {
			b.Run(fmt.Sprintf("bytes=%d/%s", size, mode), func(b *testing.B) {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				b.Cleanup(func() { _ = client.Close() })
				var observed string
				handler := func(n p.ThreadNameUpdatedNotification) {
					observed = *n.ThreadName
				}
				if mode == "replacement" {
					client.OnThreadNameUpdated(handler)
				} else {
					client.AddThreadNameUpdatedListener(handler)
				}
				notification := p.Notification{
					Method: "thread/name/updated",
					Params: json.RawMessage(`{"threadId":"thread","threadName":"current","future":"` + strings.Repeat("x", size) + `"}`),
				}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					mock.InjectServerNotification(context.Background(), notification)
				}
				b.StopTimer()
				if observed != "current" {
					b.Fatal("typed notification not delivered")
				}
			})
		}
	}
}
