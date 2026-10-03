package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// Include response admission and, for Thread.Read, complete cache ownership.
// One warm call initializes existing immutable decoding plans before timing.
func BenchmarkOptionalArrayServices(b *testing.B) {
	for _, count := range []int{1, 128} {
		record := `{"marketplacePath":"/tmp/plugins","message":"unavailable"}`
		payload := `{"marketplaces":[],"marketplaceLoadErrors":[` + strings.TrimSuffix(strings.Repeat(record+",", count), ",") + `]}`
		for _, method := range []string{"plugin/list", "plugin/installed"} {
			b.Run(fmt.Sprintf("%s/%d", method, count), func(b *testing.B) {
				mock := NewMockTransport()
				mock.SetResponse(method, codex.Response{Result: json.RawMessage(payload)})
				client := codex.NewClient(mock)
				defer client.Close()
				call := func() error {
					if method == "plugin/list" {
						_, err := client.Plugin.List(context.Background(), codex.PluginListParams{})
						return err
					}
					_, err := client.Plugin.Installed(context.Background(), codex.PluginInstalledParams{})
					return err
				}
				if err := call(); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.SetBytes(int64(len(payload)))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					mock.SentRequests = mock.SentRequests[:0]
					if err := call(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
	for _, count := range []int{1, 128} {
		app := `{"id":"app","name":"App","pluginDisplayNames":["Calendar","Docs"]}`
		payload := `{"data":[` + strings.TrimSuffix(strings.Repeat(app+",", count), ",") + `]}`
		b.Run(fmt.Sprintf("apps/%d", count), func(b *testing.B) {
			mock := NewMockTransport()
			mock.SetResponse("app/list", codex.Response{Result: json.RawMessage(payload)})
			client := codex.NewClient(mock)
			defer client.Close()
			if _, err := client.Apps.List(context.Background(), codex.AppsListParams{}); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				mock.SentRequests = mock.SentRequests[:0]
				if _, err := client.Apps.List(context.Background(), codex.AppsListParams{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	for _, count := range []int{1, 1000} {
		thread := validProcessThreadPayload("array-benchmark")
		items := make([]any, count)
		for i := range items {
			items[i] = map[string]any{"type": "reasoning", "id": fmt.Sprint(i), "summary": []string{"first", "second"}, "content": []string{"one", "two", "three"}}
		}
		thread["turns"] = []any{map[string]any{"id": "turn", "status": "completed", "items": items}}
		payload, err := json.Marshal(map[string]any{"thread": thread})
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("thread/%d", count), func(b *testing.B) {
			mock := NewMockTransport()
			mock.SetResponse("thread/read", codex.Response{Result: payload})
			client := codex.NewClient(mock)
			defer client.Close()
			if _, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "array-benchmark"}); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				mock.SentRequests = mock.SentRequests[:0]
				if _, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "array-benchmark"}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
