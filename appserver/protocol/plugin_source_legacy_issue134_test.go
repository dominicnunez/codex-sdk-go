package protocol_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestPluginSourceIssue134LocalAndRemoteSchemaExtrasAcceptWrongTypes(t *testing.T) {
	services := []string{"read", "list", "installed", "share-list"}
	wrongTypes := []struct {
		name string
		raw  string
	}{
		{name: "number", raw: "42"},
		{name: "boolean", raw: "false"},
		{name: "object", raw: `{"extra":true}`},
		{name: "array", raw: `[` + `"extra"` + `]`},
	}
	fields := []struct {
		branch string
		field  string
	}{
		{branch: "local", field: "refName"},
		{branch: "local", field: "sha"},
		{branch: "local", field: "url"},
		{branch: "remote", field: "path"},
		{branch: "remote", field: "refName"},
		{branch: "remote", field: "sha"},
		{branch: "remote", field: "url"},
	}
	for fieldIndex, target := range fields {
		for serviceIndex, service := range services {
			bad := wrongTypes[(fieldIndex+serviceIndex)%len(wrongTypes)]
			name := fmt.Sprintf("%s/%s/%s/%s", service, target.branch, target.field, bad.name)
			t.Run(name, func(t *testing.T) {
				wire := issue134LegacySourceJSON(target.branch, target.field, bad.raw)
				got, err := issue132PluginSource(t, service, wire)
				if err != nil {
					t.Fatalf("public %s rejected schema-unrestricted %s.%s extra (%s): %v", service, target.branch, target.field, bad.name, err)
				}
				want := issue134ExpectedLegacySource(target.branch, target.field)
				assertIssue134PluginSourceEqual(t, got, want)
			})
		}
	}
}

func TestPluginSourceIssue134LocalAndRemoteStringNullControls(t *testing.T) {
	for _, service := range []string{"read", "list", "installed", "share-list"} {
		for _, branch := range []string{"local", "remote"} {
			for _, state := range []string{"strings", "nulls"} {
				t.Run(fmt.Sprintf("%s/%s/%s", service, branch, state), func(t *testing.T) {
					wire := issue134LegacySourceStringControlsJSON(branch)
					if state == "nulls" {
						wire = issue134LegacySourceNullExtrasJSON(branch)
					}
					got, err := issue132PluginSource(t, service, wire)
					if err != nil {
						t.Fatalf("public %s rejected existing %s %s-extra control: %v", service, branch, state, err)
					}
					want := issue134ExpectedLegacySource(branch, "")
					if state == "strings" {
						want.RefName = stringPointer("")
						want.SHA = stringPointer("legacy-sha")
						want.URL = stringPointer("")
						if branch == "remote" {
							want.Path = stringPointer("")
						}
					} else {
						want.RefName = nil
						want.SHA = nil
						want.URL = nil
						if branch == "remote" {
							want.Path = nil
						}
					}
					assertIssue134PluginSourceEqual(t, got, want)
				})
			}
		}
	}
}

func TestPluginSourceIssue134RequiredAndConstrainedFieldsRemainChecked(t *testing.T) {
	for _, service := range []string{"read", "list", "installed", "share-list"} {
		t.Run("local-path/"+service, func(t *testing.T) {
			for _, wire := range []string{
				`{"type":"local"}`,
				`{"type":"local","path":null}`,
				`{"type":"local","path":"relative"}`,
			} {
				if _, err := issue132PluginSource(t, service, wire); err == nil {
					t.Errorf("public %s accepted invalid local source %s", service, wire)
				}
			}
		})
		t.Run("git-url-and-optional-fields/"+service, func(t *testing.T) {
			for _, wire := range []string{
				`{"type":"git"}`,
				`{"type":"git","url":null}`,
				`{"type":"git","url":17}`,
				`{"type":"git","url":"https://example.invalid/repo","path":17}`,
				`{"type":"git","url":"https://example.invalid/repo","refName":false}`,
				`{"type":"git","url":"https://example.invalid/repo","sha":{}}`,
			} {
				if _, err := issue132PluginSource(t, service, wire); err == nil {
					t.Errorf("public %s accepted invalid constrained git source %s", service, wire)
				}
			}
			for _, valid := range []struct {
				wire string
				want codex.PluginSource
			}{
				{wire: `{"type":"git","url":"https://example.invalid/repo","path":null,"refName":null,"sha":null}`, want: codex.PluginSource{Type: "git", URL: stringPointer("https://example.invalid/repo")}},
				{wire: `{"type":"git","url":"","path":"opaque","refName":"","sha":""}`, want: codex.PluginSource{Type: "git", URL: stringPointer(""), Path: stringPointer("opaque"), RefName: stringPointer(""), SHA: stringPointer("")}},
			} {
				got, err := issue132PluginSource(t, service, valid.wire)
				if err != nil {
					t.Errorf("public %s rejected valid constrained git control %s: %v", service, valid.wire, err)
				} else {
					assertIssue134PluginSourceEqual(t, got, valid.want)
				}
			}
		})
	}
}

func TestPluginSourceIssue134NPMBranchControl(t *testing.T) {
	for _, service := range []string{"read", "list", "installed", "share-list"} {
		t.Run(service, func(t *testing.T) {
			got, err := issue132PluginSource(t, service, `{"type":"npm","package":"pkg","registry":null,"version":""}`)
			if err != nil {
				t.Fatalf("public %s rejected existing npm source control: %v", service, err)
			}
			if got.Type != "npm" || got.Package == nil || *got.Package != "pkg" || !got.Registry.Present || got.Registry.Value != nil || !got.Version.Present || got.Version.Value == nil || *got.Version.Value != "" {
				t.Fatalf("public %s changed existing npm source behavior: %#v", service, got)
			}
		})
	}
}

func TestPluginSourceIssue134LocalRemoteSelectorAndExtraAliases(t *testing.T) {
	for _, tt := range []struct {
		name string
		wire string
		want codex.PluginSource
	}{
		{
			name: "folded local fields retain last string after wrong extra",
			wire: `{"TYPE":"local","PATH":"/tmp/plugin","REFNAME":false,"refName":"final","SHA":"s","URL":"u"}`,
			want: codex.PluginSource{Type: "local", Path: stringPointer("/tmp/plugin"), RefName: stringPointer("final"), SHA: stringPointer("s"), URL: stringPointer("u")},
		},
		{
			name: "escaped and Unicode folded remote extras",
			wire: `{"type":"remote","pa\u0074h":false,"PATH":"path","refName":"ref","ſha":"sha","url":"url"}`,
			want: codex.PluginSource{Type: "remote", Path: stringPointer("path"), RefName: stringPointer("ref"), SHA: stringPointer("sha"), URL: stringPointer("url")},
		},
		{
			name: "final remote discriminator makes earlier path unrestricted",
			wire: `{"type":"local","path":17,"type":"remote","path":false}`,
			want: codex.PluginSource{Type: "remote"},
		},
		{
			name: "final local discriminator keeps required path native",
			wire: `{"type":"remote","path":"/tmp/plugin","type":"local","refName":[],"sha":"s"}`,
			want: codex.PluginSource{Type: "local", Path: stringPointer("/tmp/plugin"), SHA: stringPointer("s")},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := issue132PluginSource(t, "read", tt.wire)
			if err != nil {
				t.Fatalf("decode aliased source: %v", err)
			}
			assertIssue134PluginSourceEqual(t, got, tt.want)
		})
	}
	for _, tt := range []struct{ wire, want string }{
		{wire: `{}`, want: "missing plugin.source.type"},
		{wire: `{"type":null}`, want: "missing plugin.source.type"},
		{wire: `{"type":"unknown"}`, want: `invalid plugin.source.type "unknown"`},
	} {
		if _, err := issue132PluginSource(t, "read", tt.wire); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("fallback source %s error = %v; want %q", tt.wire, err, tt.want)
		}
	}
}

func TestPluginSourceIssue134DuplicateExtrasSkipWrongOccurrencesAndRetainLastSuccess(t *testing.T) {
	sequences := []struct {
		name  string
		raw   []string
		value *string
	}{
		{name: "wrong-only", raw: []string{"42"}},
		{name: "wrong-before-string", raw: []string{"false", `"last"`}, value: stringPointer("last")},
		{name: "string-before-wrong", raw: []string{`"first"`, `{"bad":true}`}, value: stringPointer("first")},
		{name: "string-wrong-null", raw: []string{`"first"`, "[]", "null"}},
		{name: "null-before-wrong", raw: []string{"null", "false"}},
		{name: "empty-string-before-wrong", raw: []string{`""`, `{"bad":true}`}, value: stringPointer("")},
		{name: "wrong-null-string", raw: []string{"true", "null", `"last"`}, value: stringPointer("last")},
	}
	services := []string{"read", "list", "installed", "share-list"}
	fields := []struct{ branch, field string }{
		{branch: "local", field: "refName"}, {branch: "local", field: "sha"}, {branch: "local", field: "url"},
		{branch: "remote", field: "path"}, {branch: "remote", field: "refName"}, {branch: "remote", field: "sha"}, {branch: "remote", field: "url"},
	}
	for _, target := range fields {
		for _, service := range services {
			for _, history := range sequences {
				t.Run(fmt.Sprintf("%s/%s/%s/%s", service, target.branch, target.field, history.name), func(t *testing.T) {
					wire := issue134LegacySourceSequenceJSON(target.branch, target.field, history.raw)
					got, err := issue132PluginSource(t, service, wire)
					if err != nil {
						t.Fatalf("public %s rejected duplicate legacy-extra history %s: %v", service, wire, err)
					}
					want := issue134ExpectedLegacySource(target.branch, target.field)
					switch target.field {
					case "path":
						want.Path = history.value
					case "refName":
						want.RefName = history.value
					case "sha":
						want.SHA = history.value
					case "url":
						want.URL = history.value
					}
					assertIssue134PluginSourceEqual(t, got, want)
				})
			}
		}
	}
	for _, service := range services {
		t.Run("valid-final-occurrences/"+service, func(t *testing.T) {
			wire := `{"type":"remote","path":"first","path":"last","refName":"first","refName":"last","sha":"first","sha":null,"url":"first","url":"last"}`
			got, err := issue132PluginSource(t, service, wire)
			if err != nil {
				t.Fatalf("valid duplicate extras rejected: %v", err)
			}
			want := codex.PluginSource{Type: "remote", Path: stringPointer("last"), RefName: stringPointer("last"), URL: stringPointer("last")}
			assertIssue134PluginSourceEqual(t, got, want)
		})
	}
}

func TestPluginSourceIssue134GeneratedExtraSequencesMatchSchemaValueModel(t *testing.T) {
	// This independent schema model decodes each token as an ordinary JSON
	// value. Strings replace the expected value, null clears it, and other valid
	// JSON kinds are unrestricted extras that leave the last string unchanged.
	tokens := []string{`0`, `true`, `{}`, `[]`, `null`, `""`, `"value"`}
	fields := []struct{ branch, field string }{
		{branch: "local", field: "refName"}, {branch: "local", field: "sha"}, {branch: "local", field: "url"},
		{branch: "remote", field: "path"}, {branch: "remote", field: "refName"}, {branch: "remote", field: "sha"}, {branch: "remote", field: "url"},
	}
	accepted := 0
	for _, target := range fields {
		for _, first := range tokens {
			for secondIndex := -1; secondIndex < len(tokens); secondIndex++ {
				occurrences := []string{first}
				if secondIndex >= 0 {
					occurrences = append(occurrences, tokens[secondIndex])
				}
				var wantValue *string
				for _, raw := range occurrences {
					var decoded any
					if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
						t.Fatalf("reference JSON-value decoder rejected generated token %s: %v", raw, err)
					}
					switch value := decoded.(type) {
					case string:
						wantValue = stringPointer(value)
					case nil:
						wantValue = nil
					}
				}
				wire := issue134LegacySourceSequenceJSON(target.branch, target.field, occurrences)
				got, err := issue132PluginSource(t, "read", wire)
				if err != nil {
					t.Fatalf("generated schema-unrestricted extra history %s rejected: %v", wire, err)
				}
				want := issue134ExpectedLegacySource(target.branch, target.field)
				switch target.field {
				case "path":
					want.Path = wantValue
				case "refName":
					want.RefName = wantValue
				case "sha":
					want.SHA = wantValue
				case "url":
					want.URL = wantValue
				}
				assertIssue134PluginSourceEqual(t, got, want)
				accepted++
			}
		}
	}
	if accepted != len(fields)*len(tokens)*(len(tokens)+1) {
		t.Fatalf("generated model exercised %d accepted histories; want %d", accepted, len(fields)*len(tokens)*(len(tokens)+1))
	}
}

func TestPluginSourceIssue134DoesNotSuppressCoreOrSyntaxErrors(t *testing.T) {
	for _, service := range []string{"read", "list", "installed", "share-list"} {
		for _, wire := range []string{
			`{"type":false,"type":"remote","path":false}`,
			`{"type":"local","path":17,"refName":false}`,
			`{"type":"git","url":"https://example.invalid/repo","path":false}`,
			`{"type":"remote","refName":{`,
		} {
			if _, err := issue132PluginSource(t, service, wire); err == nil {
				t.Errorf("public %s suppressed native core/syntax error in %s", service, wire)
			}
		}
	}
	var source codex.PluginSource
	err := json.Unmarshal([]byte(`{"type":false,"type":"remote","path":false}`), &source)
	var typeError *json.UnmarshalTypeError
	if !errors.As(err, &typeError) || typeError.Field != "type" || typeError.Type == nil || typeError.Type.Kind().String() != "string" || typeError.Offset == 0 {
		t.Fatalf("native Type error lost while selecting remote branch: %#v err=%T %v", typeError, err, err)
	}
}

func TestPluginSourceIssue134SuccessfulReplacementAndFailureOwnership(t *testing.T) {
	oldPackage, oldPath, oldRefName, oldRegistry, oldSHA, oldURL, oldVersion := "old-package", "/old", "old-ref", "old-registry", "old-sha", "old-url", "old-version"
	seed := codex.PluginSource{
		Type: "npm", Package: &oldPackage, Path: &oldPath, RefName: &oldRefName,
		Registry: codex.OptionalNullable[string]{Present: true, Value: &oldRegistry},
		SHA:      &oldSHA, URL: &oldURL,
		Version: codex.OptionalNullable[string]{Present: true, Value: &oldVersion},
	}
	held := []*string{seed.Package, seed.Path, seed.RefName, seed.Registry.Value, seed.SHA, seed.URL, seed.Version.Value}
	heldValues := []string{oldPackage, oldPath, oldRefName, oldRegistry, oldSHA, oldURL, oldVersion}
	if err := json.Unmarshal([]byte(`{"type":"remote","path":false,"path":"new-path","refName":false,"refName":"new-ref","sha":null,"url":{}}`), &seed); err != nil {
		t.Fatal(err)
	}
	want := codex.PluginSource{Type: "remote", Path: stringPointer("new-path"), RefName: stringPointer("new-ref")}
	assertIssue134PluginSourceEqual(t, seed, want)
	for i, ref := range held {
		if ref == nil || *ref != heldValues[i] {
			t.Fatalf("successful replacement mutated previously held field %d: %v", i, ref)
		}
	}

	oldPackage2, oldPath2, oldRefName2, oldRegistry2, oldSHA2, oldURL2, oldVersion2 := "p", "/p", "r", "reg", "s", "u", "v"
	target := codex.PluginSource{
		Type: "npm", Package: &oldPackage2, Path: &oldPath2, RefName: &oldRefName2,
		Registry: codex.OptionalNullable[string]{Present: true, Value: &oldRegistry2},
		SHA:      &oldSHA2, URL: &oldURL2,
		Version: codex.OptionalNullable[string]{Present: true, Value: &oldVersion2},
	}
	failedRefs := []*string{target.Package, target.Path, target.RefName, target.Registry.Value, target.SHA, target.URL, target.Version.Value}
	failedValues := []string{oldPackage2, oldPath2, oldRefName2, oldRegistry2, oldSHA2, oldURL2, oldVersion2}
	before := target
	if err := json.Unmarshal([]byte(`{"type":"local","path":"relative","refName":false}`), &target); err == nil {
		t.Fatal("invalid local core path unexpectedly succeeded")
	}
	if target != before {
		t.Fatalf("failed branch decode changed seeded receiver: got %#v want %#v", target, before)
	}
	newRefs := []*string{target.Package, target.Path, target.RefName, target.Registry.Value, target.SHA, target.URL, target.Version.Value}
	for i := range failedRefs {
		if newRefs[i] != failedRefs[i] {
			t.Fatalf("failed branch decode replaced seeded reference %d", i)
		}
		if failedRefs[i] == nil || *failedRefs[i] != failedValues[i] {
			t.Fatalf("failed branch decode mutated seeded value %d: %v", i, failedRefs[i])
		}
	}
}

func issue134LegacySourceJSON(branch, invalidField, invalidRaw string) string {
	values := map[string]string{
		"path":    `"opaque-path"`,
		"refName": `"feature/ref"`,
		"sha":     `"legacy-sha"`,
		"url":     `"https://example.invalid/repo"`,
	}
	if branch == "local" {
		values["path"] = `"/tmp/plugin"`
	}
	if invalidField != "" {
		values[invalidField] = invalidRaw
	}
	fields := []string{"path", "refName", "sha", "url"}
	var object string
	object = `{"type":` + strconv.Quote(branch)
	for _, field := range fields {
		if branch == "local" && field == "path" {
			object += `,"path":` + values[field]
			continue
		}
		object += `,"` + field + `":` + values[field]
	}
	return object + `}`
}

func issue134LegacySourceSequenceJSON(branch, field string, occurrences []string) string {
	values := map[string]string{
		"path":    `"opaque-path"`,
		"refName": `"feature/ref"`,
		"sha":     `"legacy-sha"`,
		"url":     `"https://example.invalid/repo"`,
	}
	if branch == "local" {
		values["path"] = `"/tmp/plugin"`
	}
	fields := []string{"path", "refName", "sha", "url"}
	object := `{"type":` + strconv.Quote(branch)
	for _, name := range fields {
		if name == field {
			continue
		}
		object += `,"` + name + `":` + values[name]
	}
	for _, occurrence := range occurrences {
		object += `,"` + field + `":` + occurrence
	}
	return object + `}`
}

func issue134LegacySourceNullExtrasJSON(branch string) string {
	if branch == "local" {
		return `{"type":"local","path":"/tmp/plugin","refName":null,"sha":null,"url":null}`
	}
	return `{"type":"remote","path":null,"refName":null,"sha":null,"url":null}`
}

func issue134LegacySourceStringControlsJSON(branch string) string {
	if branch == "local" {
		return `{"type":"local","path":"/tmp/plugin","refName":"","sha":"legacy-sha","url":""}`
	}
	return `{"type":"remote","path":"","refName":"","sha":"legacy-sha","url":""}`
}

func issue134ExpectedLegacySource(branch, invalidField string) codex.PluginSource {
	want := codex.PluginSource{
		Type:    branch,
		Path:    stringPointer("opaque-path"),
		RefName: stringPointer("feature/ref"),
		SHA:     stringPointer("legacy-sha"),
		URL:     stringPointer("https://example.invalid/repo"),
	}
	if branch == "local" {
		want.Path = stringPointer("/tmp/plugin")
	}
	switch invalidField {
	case "path":
		want.Path = nil
	case "refName":
		want.RefName = nil
	case "sha":
		want.SHA = nil
	case "url":
		want.URL = nil
	}
	return want
}

func assertIssue134PluginSourceEqual(t *testing.T, got, want codex.PluginSource) {
	t.Helper()
	if got.Type != want.Type || !issue134SameString(got.Path, want.Path) || !issue134SameString(got.RefName, want.RefName) || !issue134SameString(got.SHA, want.SHA) || !issue134SameString(got.URL, want.URL) || got.Package != nil || got.Registry.Present || got.Registry.Value != nil || got.Version.Present || got.Version.Value != nil {
		t.Fatalf("source = %#v; want complete source %#v", got, want)
	}
}

func issue134SameString(got, want *string) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}
