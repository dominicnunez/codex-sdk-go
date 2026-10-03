package protocol_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestPluginSummaryDecodeCompatibility(t *testing.T) {
	for _, key := range []string{"enabled", "ENABLED", "Enabled"} {
		data := strings.ReplaceAll(issue74PluginSummary, `"enabled"`, `"`+key+`"`)
		var summary codex.PluginSummary
		if err := json.Unmarshal([]byte(data), &summary); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if !summary.Enabled || summary.ID != "plugin-74" {
			t.Fatalf("%s: aliases lost", key)
		}
	}
	for _, extra := range []string{
		`,"keywords":[],"keywords":["last"]`,
		`,"availability":null,"availability":"AVAILABLE"`,
	} {
		body := strings.TrimSuffix(issue74PluginSummary, "}") + extra + "}"
		var summary codex.PluginSummary
		if err := json.Unmarshal([]byte(body), &summary); err != nil {
			t.Fatalf("duplicate reset: %v", err)
		}
		if strings.Contains(extra, "keywords") && !reflect.DeepEqual(summary.Keywords, []string{"last"}) {
			t.Fatal("duplicate keywords changed")
		}
		if strings.Contains(extra, "availability") && (summary.Availability == nil || *summary.Availability != codex.PluginAvailabilityAvailable) {
			t.Fatal("duplicate availability changed")
		}
	}
	// Use an independent stdlib wire decoder to establish the error's field,
	// struct and whole-object offset for direct summary decoding.
	type pluginSummaryWire struct {
		Enabled *bool `json:"enabled"`
	}
	for _, body := range []string{
		strings.ReplaceAll(issue74PluginSummary, `"enabled":true`, `"enabled":"yes"`),
		strings.ReplaceAll(issue74PluginSummary, `"enabled":true`, `"enabled":true,"ENABLED":"yes"`),
	} {
		var reference pluginSummaryWire
		var actual codex.PluginSummary
		wantErr := json.Unmarshal([]byte(body), &reference)
		gotErr := json.Unmarshal([]byte(body), &actual)
		var want, got *json.UnmarshalTypeError
		if !errors.As(wantErr, &want) || !errors.As(gotErr, &got) {
			t.Fatalf("got=%v, want=%v", gotErr, wantErr)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("type error = %+v, want %+v", got, want)
		}
	}
}
