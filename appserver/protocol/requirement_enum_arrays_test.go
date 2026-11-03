package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestInboundRequirementEnumArrays(t *testing.T) {
	for _, field := range []struct {
		name   string
		values []string
	}{
		{"allowedLoginMethods", []string{"chatgpt", "api"}},
		{"allowedWindowsSandboxImplementations", []string{"elevated", "unelevated", "mxc"}},
	} {
		known, _ := json.Marshal(field.values)
		for _, tc := range []struct {
			name, raw string
			valid     bool
		}{
			{"absent", "", true}, {"null", "null", true}, {"empty", "[]", true}, {"all-current", string(known), true},
			{"bogus", `["bogus"]`, false}, {"empty-item", `[""]`, false}, {"null-item", `[null]`, false}, {"number-item", `[17]`, false},
			{"object", `{}`, false}, {"number", `17`, false}, {"boolean", `false`, false}, {"string", `"chatgpt"`, false},
			{"hidden", `["bogus"],"` + field.name + `":` + string(known), false},
			{"reset", `["bogus"],"` + field.name + `":null`, false},
		} {
			t.Run(field.name+"/"+tc.name, func(t *testing.T) {
				body := `{}`
				if tc.raw != "" {
					body = `{"` + field.name + `":` + tc.raw + `}`
				}
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":` + body + `}`)})
				got, err := c.Config.ReadRequirements(context.Background())
				if !tc.valid {
					if err == nil {
						t.Fatalf("invalid requirements array published: %s", body)
					}
					if !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
						t.Fatal("partial response")
					}
					mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":{}}`)})
					if _, err = c.Config.ReadRequirements(context.Background()); err != nil {
						t.Fatalf("recovery: %v", err)
					}
					return
				}
				if err != nil || got.Requirements == nil {
					t.Fatalf("valid control err=%v response=%+v", err, got)
				}
				var raw json.RawMessage
				if field.name == "allowedLoginMethods" {
					raw = got.Requirements.AllowedLoginMethods
				} else {
					raw = got.Requirements.AllowedWindowsSandboxImplementations
				}
				if tc.raw == "" {
					if len(raw) != 0 {
						t.Fatal("absent became present")
					}
					return
				}
				var want, actual any
				if err := json.Unmarshal([]byte(tc.raw), &want); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(raw, &actual); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(want, actual) {
					t.Fatalf("valid array changed: %s -> %s", tc.raw, raw)
				}
			})
		}
	}
}
