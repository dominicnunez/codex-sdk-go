package protocol_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type requiredNullRecord struct {
	name     string
	body     string
	fields   []string
	newValue func() any
}

func requiredNullRecords() []requiredNullRecord {
	return []requiredNullRecord{
		{"Thread", `{"cliVersion":"v","createdAt":1,"cwd":"/workspace","ephemeral":true,"id":"thread","modelProvider":"provider","preview":"preview","projectId":null,"sessionId":"session","source":"cli","status":{"type":"notLoaded"},"turns":[],"updatedAt":1,"name":"prior"}`, []string{"cliVersion", "createdAt", "cwd", "ephemeral", "id", "modelProvider", "preview", "source", "status", "turns", "updatedAt"}, func() any { return new(p.Thread) }},
		{"PluginSummary", `{"authPolicy":"ON_INSTALL","enabled":true,"id":"plugin","installPolicy":"NOT_AVAILABLE","installed":true,"name":"plugin","source":{"path":"/workspace/plugin","type":"local"}}`, []string{"enabled", "id", "installed", "name", "source"}, func() any { return new(p.PluginSummary) }},
		{"AppSummary", `{"id":"app","name":"app","category":"category"}`, []string{"id", "name"}, func() any { return new(p.AppSummary) }},
		{"PluginDetail", `{"appTemplates":[],"apps":[],"hooks":[],"marketplaceName":"market","mcpServers":[],"skills":[],"summary":{"authPolicy":"ON_INSTALL","enabled":true,"id":"plugin","installPolicy":"NOT_AVAILABLE","installed":true,"name":"plugin","source":{"path":"/workspace/plugin","type":"local"}}}`, []string{"appTemplates", "apps", "marketplaceName", "skills", "summary"}, func() any { return new(p.PluginDetail) }},
		{"PluginShareContext", `{"remotePluginId":"remote","canPublishToWorkspace":true}`, []string{"remotePluginId"}, func() any { return new(p.PluginShareContext) }},
		{"PluginSharePrincipal", `{"name":"principal","principalId":"principal","principalType":"user","role":"reader"}`, []string{"name", "principalId", "principalType", "role"}, func() any { return new(p.PluginSharePrincipal) }},
	}
}

func hiddenNullBody(body, field string, escaped bool) string {
	key := field
	if escaped {
		key = fmt.Sprintf(`\u%04x%s`, field[0], field[1:])
	}
	return `{"` + key + `":null,` + body[1:]
}

func TestRequiredNullRecordOccurrences(t *testing.T) {
	for _, record := range requiredNullRecords() {
		t.Run(record.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(record.body), record.newValue()); err != nil {
				t.Fatalf("schema-derived valid control: %v", err)
			}
			for _, field := range record.fields {
				for _, escaped := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/escaped=%t", field, escaped), func(t *testing.T) {
						value := record.newValue()
						if err := json.Unmarshal([]byte(record.body), value); err != nil {
							t.Fatal(err)
						}
						before, err := json.Marshal(value)
						if err != nil {
							t.Fatal(err)
						}
						prior := reflect.ValueOf(value).Elem()
						refs := make(map[int]uintptr)
						for i := 0; i < prior.NumField(); i++ {
							f := prior.Field(i)
							if f.Kind() == reflect.Pointer || f.Kind() == reflect.Slice {
								refs[i] = f.Pointer()
							}
						}
						err = json.Unmarshal([]byte(hiddenNullBody(record.body, field, escaped)), value)
						if !errors.Is(err, p.ErrNullResultField) {
							t.Fatalf("required null hidden by later valid occurrence: %v", err)
						}
						after, err := json.Marshal(value)
						if err != nil {
							t.Fatal(err)
						}
						if string(before) != string(after) {
							t.Fatal("new rejection changed prior receiver")
						}
						for i, ptr := range refs {
							if reflect.ValueOf(value).Elem().Field(i).Pointer() != ptr {
								t.Fatal("new rejection replaced prior reference")
							}
						}
					})
				}
			}
			// Duplicate valid values and open alias extras retain their old behavior.
			duplicate := strings.TrimSuffix(record.body, "}") + "," + record.body[1:]
			if err := json.Unmarshal([]byte(duplicate), record.newValue()); err != nil {
				t.Fatalf("valid duplicate rejected: %v", err)
			}
			aliases := "{"
			for _, field := range record.fields {
				aliases += `"` + strings.ToUpper(field) + `":null,`
			}
			aliases += record.body[1:]
			if err := json.Unmarshal([]byte(aliases), record.newValue()); err != nil {
				t.Fatalf("noncanonical null before valid canonical value rejected: %v", err)
			}
		})
	}
}
