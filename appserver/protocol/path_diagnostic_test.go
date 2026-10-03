package protocol_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestMarketplacePathDiagnosticBound(t *testing.T) {
	for _, installed := range []bool{false, true} {
		method := "plugin/list"
		if installed {
			method = "plugin/installed"
		}
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			call := func() error {
				if installed {
					_, err := client.Plugin.Installed(context.Background(), codex.PluginInstalledParams{})
					return err
				}
				_, err := client.Plugin.List(context.Background(), codex.PluginListParams{})
				return err
			}
			for _, path := range []string{
				strings.Repeat("relative", 1<<20),
				strings.Repeat("\x00", 1<<18),
				"/tmp/../" + strings.Repeat("x", 1<<20),
				`C:\tmp\..\` + strings.Repeat("x", 1<<20),
				`\\server\share\tmp\..\` + strings.Repeat("x", 1<<20),
				`\\?\UNC\server\share\tmp\..\` + strings.Repeat("x", 1<<20),
			} {
				quoted, err := json.Marshal(path)
				if err != nil {
					t.Fatal(err)
				}
				// The folded last member is the actual wire-selected path.
				body := `{"marketplaces":[],"marketplaceLoadErrors":[{"marketplacePath":"/valid","MARKETPLACEPATH":` + string(quoted) + `,"message":"failed"}]}`
				mock.SetResponse(method, codex.Response{Result: json.RawMessage(body)})
				err = call()
				if err == nil {
					t.Fatal("invalid path admitted")
				}
				if len(err.Error()) > 2048 || !strings.Contains(err.Error(), "marketplacePath") || !strings.Contains(err.Error(), "bytes omitted") {
					t.Fatalf("path diagnostic not bounded with field context: %d bytes", len(err.Error()))
				}
			}
			mock.SetResponse(method, codex.Response{Result: json.RawMessage(`{"marketplaces":[],"marketplaceLoadErrors":[{"marketplacePath":"/valid","message":"failed"}]}`)})
			if err := call(); err != nil {
				t.Fatalf("valid request after rejected paths: %v", err)
			}
		})
	}
}

func BenchmarkMarketplacePathDiagnostic(b *testing.B) {
	for _, size := range []int{32, 8 << 20} {
		name := "small"
		if size > 32 {
			name = "8MiB"
		}
		b.Run(name, func(b *testing.B) {
			mock := NewMockTransport()
			body := `{"marketplaces":[],"marketplaceLoadErrors":[{"marketplacePath":"` + strings.Repeat("x", size) + `","message":"failed"}]}`
			mock.SetResponse("plugin/list", codex.Response{Result: json.RawMessage(body)})
			client := codex.NewClient(mock)
			b.Cleanup(func() { _ = client.Close() })
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_, err := client.Plugin.List(context.Background(), codex.PluginListParams{})
				if err == nil {
					b.Fatal("invalid path admitted")
				}
				mock.mu.Lock()
				mock.SentRequests = mock.SentRequests[:0]
				mock.mu.Unlock()
			}
		})
	}
}
