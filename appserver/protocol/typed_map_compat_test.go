package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestTypedMapReceiverContracts(t *testing.T) {
	for _, field := range []string{"labels", "iconAssets", "iconDarkAssets"} {
		prior := map[string]string{"prior": "old"}
		v := p.AppInfo{ID: "prior", Name: "Prior", Labels: prior, IconAssets: prior, IconDarkAssets: prior}
		before := v
		if err := v.UnmarshalJSON([]byte(`{"id":"new","name":"New","` + field + `":{"key":null}}`)); err == nil {
			t.Fatal("invalid app map accepted")
		}
		if !reflect.DeepEqual(v, before) {
			t.Fatal("failed app decode changed prior record")
		}
		appMap(v, field)["identity"] = "kept"
		if prior["identity"] != "kept" {
			t.Fatal("failed app decode replaced map reference")
		}
	}
	prior := map[string]bool{"prior": true}
	response := p.ExperimentalFeatureEnablementSetResponse{Enablement: prior}
	if err := response.UnmarshalJSON([]byte(`{"enablement":{"key":null}}`)); err == nil || !reflect.DeepEqual(response.Enablement, prior) {
		t.Fatal("failed enablement changed prior map")
	}
	response.Enablement["identity"] = true
	if !prior["identity"] {
		t.Fatal("failed enablement replaced map reference")
	}

	// Config's established decoder intentionally updates its reused receiver and
	// backing references in wire order. A new admission failure retains those
	// native partial updates; the service still returns a zero response on error.
	makeConfig := func() *p.Config {
		origins := map[string]p.BrowserUseOriginPolicyConfig{"prior": {Access: p.Ptr(p.AllowDenyRequirementAllow)}}
		return &p.Config{BrowserUse: &p.BrowserUseConfig{Origins: &origins}, Model: p.Ptr("prior")}
	}
	actual := makeConfig()
	browser, origins := actual.BrowserUse, actual.BrowserUse.Origins
	type native p.Config
	expected := native(*makeConfig())
	raw := []byte(`{"browser_use":{"origins":{"prior":null,"new":{}}},"model":"updated"}`)
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	if err := actual.UnmarshalJSON(raw); err == nil {
		t.Fatal("invalid browser origin accepted")
	}
	if !reflect.DeepEqual(*actual, p.Config(expected)) || actual.BrowserUse != browser || actual.BrowserUse.Origins != origins {
		t.Fatal("Config partial-update or reference contract changed")
	}
}

func TestTypedMapExistingErrorPriority(t *testing.T) {
	for _, tc := range []struct {
		body   string
		decode func([]byte) error
	}{
		{`{"id":"a","name":"A","labels":@VALUE@,"pluginDisplayNames":[null]}`, new(p.AppInfo).UnmarshalJSON},
		{`{"id":"a","name":"A","labels":@VALUE@,"isEnabled":null}`, new(p.AppInfo).UnmarshalJSON},
		{`{"id":"a","name":"A","labels":@VALUE@,"description":42}`, new(p.AppInfo).UnmarshalJSON},
		{`{"labels":@VALUE@}`, new(p.AppInfo).UnmarshalJSON},
		{`{"enablement":@VALUE@,"ENABLEMENT":42}`, new(p.ExperimentalFeatureEnablementSetResponse).UnmarshalJSON},
		{`{"browser_use":{"origins":@VALUE@},"sandbox_workspace_write":{"network_access":null}}`, new(p.Config).UnmarshalJSON},
		{`{"browser_use":{"origins":@VALUE@},"model":42}`, new(p.Config).UnmarshalJSON},
	} {
		// Equal-length JSON values keep native offsets and error context
		// comparable between the new map violation and a valid control.
		valid := `{"k":false}`
		bad := `{"k":null }`
		if strings.Contains(tc.body, `"labels"`) {
			valid = `{"k":""   }`
		}
		if strings.Contains(tc.body, `"origins"`) {
			valid = `{"k":{}   }`
		}
		control := tc.decode([]byte(strings.Replace(tc.body, "@VALUE@", valid, 1)))
		mixed := tc.decode([]byte(strings.Replace(tc.body, "@VALUE@", bad, 1)))
		if control == nil || mixed == nil || !reflect.DeepEqual(control, mixed) {
			t.Fatalf("old check changed: %v vs %v", control, mixed)
		}
	}
}

func TestTypedMapPlainRecords(t *testing.T) {
	for _, v := range []any{p.BrowserUseConfig{}, &p.BrowserUseConfig{}, p.BrowserUseOriginPolicyConfig{}, &p.BrowserUseOriginPolicyConfig{}, p.ConfigRequirements{}, &p.ConfigRequirements{}} {
		if _, ok := v.(json.Unmarshaler); ok {
			t.Fatalf("plain record gained decoder: %T", v)
		}
	}
	var browser struct {
		p.BrowserUseConfig
		Marker string
	}
	if err := json.Unmarshal([]byte(`{"origins":{"site":null},"Marker":"kept"}`), &browser); err != nil || browser.Marker != "kept" || browser.Origins == nil {
		t.Fatalf("plain browser composition changed: %v", err)
	}
	var requirements struct {
		p.ConfigRequirements
		Marker string
	}
	if err := json.Unmarshal([]byte(`{"featureRequirements":{"key":null},"Marker":"kept"}`), &requirements); err != nil || requirements.Marker != "kept" || requirements.FeatureRequirements["key"] {
		t.Fatalf("plain requirements composition changed: %v", err)
	}
}

func TestTypedMapRequirementsPriority(t *testing.T) {
	for _, suffix := range []string{`"allowLoginShell":42`, `"models":true`, `"cliAuthCredentialsStore":"bad"`} {
		var errs []error
		for _, value := range []string{`false`, `null `} {
			mock := NewMockTransport()
			c := p.NewClient(mock)
			t.Cleanup(func() { _ = c.Close() })
			mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":{"featureRequirements":{"k":` + value + `},` + suffix + `}}`)})
			got, err := c.Config.ReadRequirements(context.Background())
			errs = append(errs, err)
			if !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
				t.Fatal("partial requirements response")
			}
		}
		if errs[0] == nil || errs[1] == nil || !reflect.DeepEqual(errs[0], errs[1]) {
			t.Fatalf("established requirements checks changed: %v vs %v", errs[0], errs[1])
		}
	}
}

func TestTypedMapBoundedDiagnostics(t *testing.T) {
	key, _ := json.Marshal(strings.Repeat("peer-", 20000))
	for _, field := range []string{"labels", "iconAssets", "iconDarkAssets", "featureRequirements", "origins", "enablement"} {
		mock := NewMockTransport()
		c := p.NewClient(mock)
		t.Cleanup(func() { _ = c.Close() })
		got, err := typedMapRead(t, c, mock, field, `,"`+field+`":{`+string(key)+`:null}`)
		if err == nil || len(err.Error()) > 512 || !reflect.ValueOf(got).IsZero() {
			t.Fatal("unbounded key diagnostic or partial response")
		}
	}
}

func FuzzTypedMapOccurrenceAdmission(f *testing.F) {
	for _, seed := range [][]byte{{}, {0}, {1}, {2, 0}, {0, 2, 1}, {0, 1, 0, 1}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, sequence []byte) {
		if len(sequence) > 32 {
			sequence = sequence[:32]
		}
		key, err := json.Marshal(string(sequence))
		if err != nil {
			t.Fatal(err)
		}
		valid := true
		for _, step := range sequence {
			valid = valid && step%3 != 2
		}
		for _, field := range []string{"labels", "iconAssets", "iconDarkAssets", "featureRequirements", "origins", "enablement"} {
			value := `""`
			if field == "featureRequirements" || field == "enablement" {
				value = `false`
			}
			if field == "origins" {
				value = `{}`
			}
			var entries []string
			for _, step := range sequence {
				entry := value
				if step%3 == 2 {
					entry = `null`
				}
				entries = append(entries, string(key)+`:`+entry)
			}
			canonical := `,"` + field + `":`
			members := canonical + `{` + strings.Join(entries, ",") + `}`
			if field != "enablement" && len(sequence)%2 == 1 {
				members += canonical + `null`
			}
			members += canonical + `{}`
			mock := NewMockTransport()
			c := p.NewClient(mock)
			t.Cleanup(func() { _ = c.Close() })
			got, err := typedMapRead(t, c, mock, field, members)
			if (err == nil) != valid {
				t.Fatalf("field=%s sequence=%v err=%v", field, sequence, err)
			}
			if !valid {
				if !reflect.ValueOf(got).IsZero() {
					t.Fatal("partial result after forbidden map occurrence")
				}
			} else if !reflect.DeepEqual(actualTypedMap(got, field), expectedTypedMap(t, field, members)) {
				t.Fatal("valid native map selection changed")
			}
		}
	})
}
