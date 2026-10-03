package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func BenchmarkModerationMetadataDelivery(b *testing.B) {
	for _, tc := range []struct {
		name  string
		kind  string
		count int
	}{
		{"object-small", "object", 1},
		{"object-large", "object", 256},
		{"duplicates-small", "duplicates", 8},
		{"duplicates-large", "duplicates", 256},
		{"overwrite-large", "overwrite", 256},
		{"array-large", "array", 256},
		{"numeric-large", "number", 256},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var fields strings.Builder
			fields.WriteString(`{"threadId":"t","turnId":"u"`)
			text := strings.Repeat("x", 1024)
			switch tc.kind {
			case "duplicates", "overwrite":
				for i := range tc.count {
					key := i
					if tc.kind == "overwrite" {
						key = 0
					}
					fmt.Fprintf(&fields, `,"metadata":{"k%d":"%s"}`, key, text)
				}
			case "number":
				fields.WriteString(`,"metadata":`)
				fields.WriteString(strings.Repeat("9", tc.count*1024))
			default:
				var metadata interface{}
				if tc.kind == "array" {
					values := make([]string, tc.count)
					for i := range values {
						values[i] = text
					}
					metadata = values
				} else {
					values := make(map[string]string, tc.count)
					for i := range tc.count {
						values[fmt.Sprintf("k%d", i)] = text
					}
					metadata = values
				}
				encoded, err := json.Marshal(metadata)
				if err != nil {
					b.Fatal(err)
				}
				fields.WriteString(`,"metadata":`)
				fields.Write(encoded)
			}
			fields.WriteByte('}')
			mock := NewMockTransport()
			client := protocol.NewClient(mock)
			defer client.Close()
			calls := 0
			client.OnTurnModerationMetadataJSON(func(protocol.TurnModerationMetadataJSONNotification) { calls++ })
			remove := client.AddTurnModerationMetadataJSONListener(func(protocol.TurnModerationMetadataJSONNotification) { calls++ })
			defer remove()
			notification := protocol.Notification{Method: "turn/moderationMetadata", Params: json.RawMessage(fields.String())}
			mock.InjectServerNotification(context.Background(), notification)
			if calls != 2 {
				b.Fatal("typed delivery did not reach both recipients")
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(notification.Params)))
			b.ResetTimer()
			for range b.N {
				mock.InjectServerNotification(context.Background(), notification)
			}
			b.StopTimer()
			if calls != 2*(b.N+1) {
				b.Fatal("typed delivery stopped reaching both recipients")
			}
		})
	}
}
