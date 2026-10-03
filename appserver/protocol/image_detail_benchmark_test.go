package protocol_test

import (
	"encoding/json"
	"fmt"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func BenchmarkImageDetailSnapshots(b *testing.B) {
	for _, count := range []int{1, 1000} {
		for _, present := range []bool{false, true} {
			b.Run(fmt.Sprintf("%d/detail=%v", count, present), func(b *testing.B) {
				suffix := ""
				if present {
					suffix = `,"detail":"original"`
				}
				thread := p.Thread{ID: "t", Turns: []p.Turn{{ID: "u", Items: make([]p.ThreadItemWrapper, count)}}}
				for i := range thread.Turns[0].Items {
					var item p.ThreadItemWrapper
					payload := `{"type":"userMessage","id":"i","content":[{"type":"image","fileId":"file"` + suffix + `},{"type":"localImage","path":"relative"` + suffix + `}]}`
					if err := json.Unmarshal([]byte(payload), &item); err != nil {
						b.Fatal(err)
					}
					thread.Turns[0].Items[i] = item
				}
				client := p.NewClient(NewMockTransport())
				defer client.Close()
				client.CacheThreadState(thread)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					snapshot, ok := client.ThreadStateSnapshot("t")
					if !ok || len(snapshot.Turns[0].Items) != count {
						b.Fatal("snapshot lost records")
					}
				}
			})
		}
	}
}
