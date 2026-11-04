package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestMcpCatalogPrecision(t *testing.T) {
	targets := []struct{ owner, field string }{{"tool", "inputSchema"}, {"tool", "outputSchema"}, {"tool", "_meta"}, {"tool", "annotations"}, {"tool", "icons"}, {"resource", "_meta"}, {"resource", "annotations"}, {"resource", "icons"}, {"template", "annotations"}, {"server", "toolListMeta"}, {"server", "serverCapabilities"}, {"info", "icons"}}
	for _, target := range targets {
		for _, number := range []string{"42", "9007199254740993", "18446744073709551615", "-9007199254740993", "1e400", "1e-400", "-0", "0.10"} {
			t.Run(target.owner+"/"+target.field+"/"+number, func(t *testing.T) {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				defer client.Close()
				raw := `{"n":` + number + `}`
				if target.field == "inputSchema" || target.field == "outputSchema" {
					raw = `{"const":` + number + `}`
				}
				if target.field == "icons" {
					raw = `[{"n":` + number + `}]`
				}
				tool := map[string]any{"name": "fixture", "inputSchema": map[string]any{"type": "object"}}
				resource := map[string]any{"name": "r", "uri": "fixture:r"}
				template := map[string]any{"name": "rt", "uriTemplate": "fixture:{id}"}
				info := map[string]any{"name": "s", "version": "1"}
				server := map[string]any{"name": "s", "authStatus": "unsupported", "tools": map[string]any{"fixture": tool}, "resources": []any{resource}, "resourceTemplates": []any{template}, "serverInfo": info}
				objects := map[string]map[string]any{"tool": tool, "resource": resource, "template": template, "info": info, "server": server}
				objects[target.owner][target.field] = json.RawMessage(raw)
				if e := mock.SetResponseData("mcpServerStatus/list", map[string]any{"data": []any{server}}); e != nil {
					t.Fatal(e)
				}
				r, e := client.Mcp.ListServerStatus(context.Background(), p.ListMcpServerStatusParams{})
				if e != nil || len(r.Data) != 1 {
					t.Fatalf("valid catalog rejected: %v", e)
				}
				s := r.Data[0]
				var value any
				switch target.owner {
				case "tool":
					v := s.Tools["fixture"]
					switch target.field {
					case "inputSchema":
						value = v.InputSchema
					case "outputSchema":
						value = v.OutputSchema
					case "_meta":
						value = v.Meta
					case "annotations":
						value = v.Annotations
					case "icons":
						value = v.Icons
					}
				case "resource":
					v := s.Resources[0]
					switch target.field {
					case "_meta":
						value = v.Meta
					case "annotations":
						value = v.Annotations
					case "icons":
						value = v.Icons
					}
				case "template":
					value = s.ResourceTemplates[0].Annotations
				case "info":
					value = s.ServerInfo.Icons
				case "server":
					if target.field == "toolListMeta" {
						value = s.ToolListMeta
					} else {
						value = s.ServerCapabilities
					}
				}
				encoded, e := json.Marshal(value)
				if e != nil {
					t.Fatal(e)
				}
				if string(encoded) != raw {
					t.Errorf("catalog field numeric value changed: got=%s want=%s", encoded, raw)
				}
			})
		}
	}
}
