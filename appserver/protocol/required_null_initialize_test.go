package protocol_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestInitializeRequiredNullOccurrences(t *testing.T) {
	base := map[string]any{"codexHome": "/tmp/response-home", "platformFamily": "test-family", "platformOs": "test-target", "userAgent": "fixture"}
	for _, field := range []string{"codexHome", "platformFamily", "platformOs", "userAgent"} {
		for _, kind := range []string{"valid", "missing", "null", "hidden-null", "hidden-null-escaped"} {
			t.Run(field+"/"+kind, func(t *testing.T) {
				body := map[string]any{}
				for k, v := range base {
					body[k] = v
				}
				if kind == "missing" {
					delete(body, field)
				}
				if kind == "null" {
					body[field] = nil
				}
				raw, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "hidden-null" || kind == "hidden-null-escaped" {
					final, _ := json.Marshal(base[field])
					if kind == "hidden-null-escaped" {
						raw = []byte(hiddenNullBody(string(raw), field, true))
					} else {
						raw = []byte(strings.TrimSuffix(string(raw), "}") + `,"` + field + `":null,"` + field + `":` + string(final) + `}`)
					}
				}
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				mock.SetResponse("initialize", p.Response{Result: raw})
				_, err = c.Initialize(context.Background(), p.InitializeParams{ClientInfo: p.ClientInfo{Name: "fixture", Version: "1"}})
				if kind == "valid" {
					if err != nil {
						t.Fatal("valid control:", err)
					}
					return
				}
				_, latched := c.InitializedParams()
				if err == nil || latched {
					t.Errorf("invalid occurrence accepted: kind=%s field=%s cached_session=%v", kind, field, latched)
				}
				if err := mock.SetResponseData("initialize", base); err != nil {
					t.Fatal(err)
				}
				if _, err := c.Initialize(context.Background(), p.InitializeParams{ClientInfo: p.ClientInfo{Name: "fixture", Version: "1"}}); err != nil {
					t.Fatal("valid retry failed:", err)
				}
				if len(mock.SentRequests) != 2 {
					t.Errorf("retry reused invalid handshake: requests=%d want=2", len(mock.SentRequests))
				}
			})
		}
	}
}
