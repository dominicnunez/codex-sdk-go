package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"github.com/dominicnunez/codex-sdk-go/internal/deepcopy"
)

func TestPluginSourceNPMDirectDecodeAndOrdinaryEncoding(t *testing.T) {
	tests := []struct {
		name string
		wire string
		want map[string]string
	}{
		{
			name: "optional metadata absent",
			wire: `{"type":"npm","package":"pkg"}`,
			want: map[string]string{"type": `"npm"`, "package": `"pkg"`},
		},
		{
			name: "optional metadata null",
			wire: `{"type":"npm","package":"","registry":null,"version":null}`,
			want: map[string]string{"type": `"npm"`, "package": `""`, "registry": `null`, "version": `null`},
		},
		{
			name: "optional metadata empty and populated",
			wire: `{"type":"npm","package":"pkg","registry":"","version":"1.2.3"}`,
			want: map[string]string{"type": `"npm"`, "package": `"pkg"`, "registry": `""`, "version": `"1.2.3"`},
		},
		{
			name: "npm ignores every old field and unknown extras",
			wire: `{"type":"npm","package":"pkg","path":123,"refName":{},"sha":false,"url":[],"future":{"n":1}}`,
			want: map[string]string{"type": `"npm"`, "package": `"pkg"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var source codex.PluginSource
			if err := json.Unmarshal([]byte(tt.wire), &source); err != nil {
				t.Fatalf("decode %s: %v", tt.wire, err)
			}
			assertPluginSourceJSONFields(t, source, tt.want)
		})
	}
}

func TestPluginSourceNPMConstructedOptionalMetadataEncoding(t *testing.T) {
	packageName, empty, registryValue, versionValue := "", "", "registry", "1.0"
	for _, tt := range []struct {
		name   string
		source codex.PluginSource
		want   map[string]string
	}{
		{name: "absent", source: codex.PluginSource{Type: "npm", Package: &packageName}, want: map[string]string{"type": `"npm"`, "package": `""`}},
		{name: "null", source: codex.PluginSource{Type: "npm", Package: &packageName, Registry: codex.OptionalNullable[string]{Present: true}, Version: codex.OptionalNullable[string]{Present: true}}, want: map[string]string{"type": `"npm"`, "package": `""`, "registry": `null`, "version": `null`}},
		{name: "empty", source: codex.PluginSource{Type: "npm", Package: &packageName, Registry: codex.OptionalNullable[string]{Present: true, Value: &empty}, Version: codex.OptionalNullable[string]{Present: true, Value: &empty}}, want: map[string]string{"type": `"npm"`, "package": `""`, "registry": `""`, "version": `""`}},
		{name: "populated", source: codex.PluginSource{Type: "npm", Package: &packageName, Registry: codex.OptionalNullable[string]{Present: true, Value: &registryValue}, Version: codex.OptionalNullable[string]{Present: true, Value: &versionValue}}, want: map[string]string{"type": `"npm"`, "package": `""`, "registry": `"registry"`, "version": `"1.0"`}},
	} {
		t.Run(tt.name, func(t *testing.T) { assertPluginSourceJSONFields(t, tt.source, tt.want) })
	}
	legacy := codex.PluginSource{Type: "remote"}
	assertPluginSourceJSONFields(t, legacy, map[string]string{"type": `"remote"`})
}

func TestPluginSourceNPMRequiredPackageAndFailureOwnership(t *testing.T) {
	oldPackage, oldPath, oldRef, oldRegistry, oldSHA, oldURL, oldVersion := "old-package", "/tmp/old", "old-ref", "old-registry", "old-sha", "old-url", "old-version"
	seed := func() codex.PluginSource {
		return codex.PluginSource{
			Package: &oldPackage, Path: &oldPath, RefName: &oldRef,
			Registry: codex.OptionalNullable[string]{Present: true, Value: &oldRegistry},
			SHA:      &oldSHA, Type: "remote", URL: &oldURL,
			Version: codex.OptionalNullable[string]{Present: true, Value: &oldVersion},
		}
	}
	for _, wire := range []string{
		`{"type":"npm"}`,
		`{"type":"npm","package":null}`,
		`{"type":"npm","package":17}`,
		`{"type":"npm","package":"pkg","registry":{}}`,
	} {
		t.Run(wire, func(t *testing.T) {
			target := seed()
			before := target
			if err := json.Unmarshal([]byte(wire), &target); err == nil {
				t.Fatalf("decode %s unexpectedly succeeded", wire)
			}
			if target != before {
				t.Fatalf("failed decode changed receiver: got %#v want %#v", target, before)
			}
			if oldPackage != "old-package" || oldPath != "/tmp/old" || oldRef != "old-ref" || oldRegistry != "old-registry" || oldSHA != "old-sha" || oldURL != "old-url" || oldVersion != "old-version" {
				t.Fatal("failed decode changed a value held through the seeded receiver")
			}
		})
	}
}

func TestPluginSourceNPMAndLegacyBranchReuse(t *testing.T) {
	var source codex.PluginSource
	if err := json.Unmarshal([]byte(`{"type":"remote","path":"ignored","refName":"ignored","sha":"ignored","url":"ignored"}`), &source); err != nil {
		t.Fatal(err)
	}
	oldPath, oldRef, oldSHA, oldURL := source.Path, source.RefName, source.SHA, source.URL
	if err := json.Unmarshal([]byte(`{"type":"npm","package":"pkg","registry":null,"version":"2"}`), &source); err != nil {
		t.Fatal(err)
	}
	if source.Package == nil || *source.Package != "pkg" || !source.Registry.Present || source.Registry.Value != nil || !source.Version.Present || source.Version.Value == nil || *source.Version.Value != "2" {
		t.Fatalf("npm source = %#v", source)
	}
	if source.Path != nil || source.RefName != nil || source.SHA != nil || source.URL != nil {
		t.Fatalf("npm success did not clear legacy fields: %#v", source)
	}
	if oldPath == nil || oldRef == nil || oldSHA == nil || oldURL == nil || *oldPath != "ignored" || *oldRef != "ignored" || *oldSHA != "ignored" || *oldURL != "ignored" {
		t.Fatal("npm success mutated pointers held from the previous legacy value")
	}
	if err := json.Unmarshal([]byte(`{"type":"remote","package":false,"registry":{},"version":17}`), &source); err != nil {
		t.Fatalf("legacy source rejected schema-ignored npm extras: %v", err)
	}
	if source.Type != "remote" || source.Package != nil || source.Registry.Present || source.Version.Present {
		t.Fatalf("legacy success did not replace/reset npm state: %#v", source)
	}
	if err := json.Unmarshal([]byte(`{"type":"npm","package":"","registry":"registry","version":null}`), &source); err != nil {
		t.Fatalf("second npm source: %v", err)
	}
	if source.Type != "npm" || source.Package == nil || *source.Package != "" || source.Path != nil || source.RefName != nil || source.SHA != nil || source.URL != nil || !source.Registry.Present || source.Registry.Value == nil || *source.Registry.Value != "registry" || !source.Version.Present || source.Version.Value != nil {
		t.Fatalf("npm success did not replace/reset legacy state: %#v", source)
	}
}

func TestPluginSourceNPMDiscriminatorUsesNativeDuplicateAndAliasSemantics(t *testing.T) {
	tests := []struct {
		name string
		wire string
		want map[string]string
	}{
		{
			name: "final duplicate selects npm",
			wire: `{"type":"remote","url":"ignored","TYPE":"npm","Package":"pkg"}`,
			want: map[string]string{"type": `"npm"`, "package": `"pkg"`},
		},
		{
			name: "final duplicate selects legacy",
			wire: `{"type":"npm","package":"ignored","Type":"remote"}`,
			want: map[string]string{"type": `"remote"`},
		},
		{
			name: "last package occurrence wins",
			wire: `{"type":"npm","package":"first","PACKAGE":""}`,
			want: map[string]string{"type": `"npm"`, "package": `""`},
		},
		{
			name: "escaped and Unicode simple-fold aliases",
			wire: `{"t\u0079pe":"npm","pacKage":"pkg","regiſtry":"r","version":"v"}`,
			want: map[string]string{"type": `"npm"`, "package": `"pkg"`, "registry": `"r"`, "version": `"v"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var source codex.PluginSource
			if err := json.Unmarshal([]byte(tt.wire), &source); err != nil {
				t.Fatalf("decode %s: %v", tt.wire, err)
			}
			assertPluginSourceJSONFields(t, source, tt.want)
		})
	}
}

func TestPluginSourceLegacyIgnoresNPMExtrasAndRetainsValidation(t *testing.T) {
	tests := []struct {
		name     string
		wire     string
		wantType string
	}{
		{name: "local", wire: `{"type":"local","path":"/tmp/plugin","package":17,"registry":{},"version":false}`, wantType: "local"},
		{name: "git", wire: `{"type":"git","url":"https://example.invalid/repo","package":17,"registry":{},"version":false}`, wantType: "git"},
		{name: "remote", wire: `{"type":"remote","package":17,"registry":{},"version":false}`, wantType: "remote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var source codex.PluginSource
			if err := json.Unmarshal([]byte(tt.wire), &source); err != nil {
				t.Fatalf("legacy branch rejected new-name extras: %v", err)
			}
			if source.Type != tt.wantType || source.Package != nil || source.Registry.Present || source.Version.Present {
				t.Fatalf("legacy result = %#v", source)
			}
		})
	}
	var target codex.PluginSource
	err := json.Unmarshal([]byte(`{"type":"not-a-source"}`), &target)
	if err == nil || !strings.Contains(err.Error(), `invalid plugin.source.type "not-a-source"`) {
		t.Fatalf("unknown legacy discriminator error = %v", err)
	}
}

func TestPluginSourceLegacyRetainsNativeRequiredAndTypeErrors(t *testing.T) {
	for _, tt := range []struct {
		wire string
		want string
	}{
		{wire: `{}`, want: "missing plugin.source.type"},
		{wire: `{"type":null}`, want: "missing plugin.source.type"},
		{wire: `{"type":"git"}`, want: "missing plugin.source.url"},
		{wire: `{"type":"local"}`, want: "missing plugin.source.path"},
		{wire: `{"type":"local","path":"relative"}`, want: "plugin.source.path"},
		{wire: `{"type":"unknown"}`, want: `invalid plugin.source.type "unknown"`},
	} {
		var source codex.PluginSource
		err := json.Unmarshal([]byte(tt.wire), &source)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("decode %s error = %v; want substring %q", tt.wire, err, tt.want)
		}
	}
	var alias codex.PluginSource
	if err := json.Unmarshal([]byte(`{"TYPE":"remote"}`), &alias); err != nil || alias.Type != "remote" {
		t.Fatalf("legacy folded Type alias: source=%#v err=%v", alias, err)
	}
}

func TestPluginSourceNPMHasNoMarshalMethodAndEmbedsNormally(t *testing.T) {
	marshalerType := reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	if reflect.TypeOf(codex.PluginSource{}).Implements(marshalerType) || reflect.TypeOf((*codex.PluginSource)(nil)).Implements(marshalerType) {
		t.Fatal("PluginSource unexpectedly implements json.Marshaler")
	}
	if _, ok := any((*codex.PluginSource)(nil)).(json.Unmarshaler); !ok {
		t.Fatal("PluginSource lost its existing pointer UnmarshalJSON method")
	}
	packageName := "pkg"
	registry := ""
	envelope := struct {
		codex.PluginSource
		Label string `json:"label"`
	}{
		PluginSource: codex.PluginSource{Type: "npm", Package: &packageName, Registry: codex.OptionalNullable[string]{Present: true, Value: &registry}},
		Label:        "outside",
	}
	got, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"package":"pkg","registry":"","type":"npm","label":"outside"}`
	if string(got) != want {
		t.Fatalf("anonymous envelope JSON = %s; want %s", got, want)
	}
	var decoded struct {
		codex.PluginSource
		Label string `json:"label"`
	}
	if err := json.Unmarshal([]byte(`{"type":"remote","label":"outside"}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != "remote" || decoded.Label != "" {
		t.Fatalf("embedded pointer UnmarshalJSON behavior changed: %#v", decoded)
	}
}

func TestPluginSourceNPMGenericDeepCopyOwnsNewReferences(t *testing.T) {
	packageName, registry, version := "pkg", "registry", "1.2.3"
	original := codex.PluginSource{
		Type:     "npm",
		Package:  &packageName,
		Registry: codex.OptionalNullable[string]{Present: true, Value: &registry},
		Version:  codex.OptionalNullable[string]{Present: true, Value: &version},
	}
	cloned := deepcopy.Value(original)
	if cloned.Package == original.Package || cloned.Registry.Value == original.Registry.Value || cloned.Version.Value == original.Version.Value {
		t.Fatal("generic deep copy retained a new npm string pointer")
	}
	*cloned.Package = "clone-package"
	*cloned.Registry.Value = "clone-registry"
	*cloned.Version.Value = "clone-version"
	if *original.Package != "pkg" || *original.Registry.Value != "registry" || *original.Version.Value != "1.2.3" {
		t.Fatal("mutating the clone changed source npm metadata")
	}
	*original.Package = "source-package"
	*original.Registry.Value = "source-registry"
	*original.Version.Value = "source-version"
	if *cloned.Package != "clone-package" || *cloned.Registry.Value != "clone-registry" || *cloned.Version.Value != "clone-version" {
		t.Fatal("mutating source npm metadata changed the clone")
	}
	original.Registry.Present = false
	original.Version.Present = false
	if !cloned.Registry.Present || !cloned.Version.Present {
		t.Fatal("mutating OptionalNullable.Present changed the clone")
	}
}

func TestPluginSourceNPMNativeErrorRepairAndNullableOrdering(t *testing.T) {
	for _, wire := range []string{
		`{"type":"npm","package":null}`,
		`{"type":"npm","package":null,"package":"pkg"}`,
	} {
		var source codex.PluginSource
		err := json.Unmarshal([]byte(wire), &source)
		if wire == `{"type":"npm","package":null}` {
			if err == nil || !strings.Contains(err.Error(), "missing plugin.source.package") {
				t.Errorf("final-null package error = %v", err)
			}
		} else if err != nil || source.Package == nil || *source.Package != "pkg" {
			t.Errorf("null package repaired by a later string: source=%#v err=%v", source, err)
		}
	}
	var source codex.PluginSource
	if err := json.Unmarshal([]byte(`{"type":"npm","package":"pkg","registry":null,"registry":"r1","version":"v1","version":null}`), &source); err != nil {
		t.Fatal(err)
	}
	if !source.Registry.Present || source.Registry.Value == nil || *source.Registry.Value != "r1" || !source.Version.Present || source.Version.Value != nil {
		t.Fatalf("nullable duplicate final state = %#v", source)
	}
	var mixed codex.PluginSource
	err := json.Unmarshal([]byte(`{"type":"npm","package":false,"registry":{}}`), &mixed)
	var metadataTypeError *json.UnmarshalTypeError
	if !errors.As(err, &metadataTypeError) || metadataTypeError.Field != "registry" || metadataTypeError.Type == nil || metadataTypeError.Type.Kind() != reflect.String {
		t.Fatalf("OptionalNullable decoder error precedence/metadata changed: %#v (%T %v)", metadataTypeError, err, err)
	}

	var receiver codex.PluginSource
	for _, tt := range []struct {
		wire      string
		wantField string
	}{
		{wire: `{"type":17,"type":"npm","package":"pkg"}`, wantField: "type"},
		{wire: `{"type":"npm","package":false,"package":"pkg"}`, wantField: "package"},
	} {
		err := json.Unmarshal([]byte(tt.wire), &receiver)
		var typed *json.UnmarshalTypeError
		if err == nil || !errors.As(err, &typed) {
			t.Errorf("expected preserved native UnmarshalTypeError for %s, got %T %v", tt.wire, err, err)
			continue
		}
		if typed.Field != tt.wantField || typed.Type == nil || typed.Offset == 0 {
			t.Errorf("native type error metadata for %s = %#v; want field %q and offset", tt.wire, typed, tt.wantField)
		}
	}
	if receiver.Type != "" || receiver.Package != nil {
		t.Fatalf("failed native duplicates changed receiver: %#v", receiver)
	}
}

func TestPluginSourceNPMGeneratedNativeFieldStateReference(t *testing.T) {
	type nativeNPMReference struct {
		Type     *string                        `json:"type"`
		Package  *string                        `json:"package"`
		Registry codex.OptionalNullable[string] `json:"registry"`
		Version  codex.OptionalNullable[string] `json:"version"`
	}
	type fieldState struct {
		name string
		raw  string
	}
	packageStates := []fieldState{{name: "absent"}, {name: "null", raw: "null"}, {name: "empty", raw: `""`}, {name: "value", raw: `"pkg"`}, {name: "wrong", raw: "17"}}
	metadataStates := []fieldState{{name: "absent"}, {name: "null", raw: "null"}, {name: "empty", raw: `""`}, {name: "value", raw: `"x"`}, {name: "wrong", raw: "17"}}
	accepted, rejected := 0, 0
	for _, pkg := range packageStates {
		for _, registry := range metadataStates {
			for _, version := range metadataStates {
				var wire strings.Builder
				wire.WriteString(`{"type":"npm"`)
				if pkg.raw != "" || pkg.name == "null" || pkg.name == "empty" || pkg.name == "value" || pkg.name == "wrong" {
					wire.WriteString(`,"package":`)
					wire.WriteString(pkg.raw)
				}
				if registry.name != "absent" {
					wire.WriteString(`,"registry":`)
					wire.WriteString(registry.raw)
				}
				if version.name != "absent" {
					wire.WriteString(`,"version":`)
					wire.WriteString(version.raw)
				}
				wire.WriteByte('}')
				input := []byte(wire.String())

				var reference nativeNPMReference
				referenceErr := json.Unmarshal(input, &reference)
				wantAccepted := referenceErr == nil && reference.Type != nil && *reference.Type == "npm" && reference.Package != nil
				var actual codex.PluginSource
				actualErr := json.Unmarshal(input, &actual)
				if wantAccepted {
					accepted++
					if actualErr != nil {
						t.Fatalf("reference accepted generated state package=%s registry=%s version=%s (%s), production rejected: %v", pkg.name, registry.name, version.name, input, actualErr)
					}
					if actual.Type != *reference.Type || actual.Package == nil || *actual.Package != *reference.Package || actual.Registry.Present != reference.Registry.Present || actual.Version.Present != reference.Version.Present || !sameOptionalNPMString(actual.Registry, reference.Registry) || !sameOptionalNPMString(actual.Version, reference.Version) {
						t.Fatalf("reference mismatch for %s: production=%#v reference=%#v", input, actual, reference)
					}
				} else {
					rejected++
					if actualErr == nil {
						t.Fatalf("native reference rejected generated state package=%s registry=%s version=%s (%s), production accepted %#v", pkg.name, registry.name, version.name, input, actual)
					}
				}
			}
		}
	}
	if accepted != 32 || rejected != 93 {
		t.Fatalf("generated npm state counts accepted=%d rejected=%d; want 32/93", accepted, rejected)
	}
}

func sameOptionalNPMString(left, right codex.OptionalNullable[string]) bool {
	if left.Present != right.Present {
		return false
	}
	if left.Value == nil || right.Value == nil {
		return left.Value == nil && right.Value == nil
	}
	return *left.Value == *right.Value
}

func TestPluginSourceNPMRepeatedSummarySourcesDoNotCrossMerge(t *testing.T) {
	wire := `{"authPolicy":"ON_USE","enabled":true,"id":"plugin","installPolicy":"AVAILABLE","installed":true,"name":"calendar","source":{"type":"npm"},"source":{"type":"npm","package":"pkg"}}`
	var summary codex.PluginSummary
	err := json.Unmarshal([]byte(wire), &summary)
	if err == nil || err.Error() != "missing plugin.source.package" {
		t.Fatalf("incomplete earlier source merged with later duplicate: summary=%#v err=%v", summary, err)
	}
	valid := `{"authPolicy":"ON_USE","enabled":true,"id":"plugin","installPolicy":"AVAILABLE","installed":true,"name":"calendar","source":{"type":"npm","package":"pkg"}}`
	if err := json.Unmarshal([]byte(valid), &summary); err != nil || summary.Source.Package == nil || *summary.Source.Package != "pkg" {
		t.Fatalf("complete independent source control: summary=%#v err=%v", summary, err)
	}
}

func TestPluginSourceNPMNewlyExposedReadValidationStillReturnsZero(t *testing.T) {
	result := json.RawMessage(`{"plugin":{"appTemplates":[],"apps":[],"hooks":[],"marketplaceName":"official","mcpServers":[],"skills":[],"summary":{"authPolicy":"ON_USE","enabled":true,"id":"","installPolicy":"AVAILABLE","installed":true,"name":"calendar","source":{"type":"npm","package":"pkg"}}}}`)
	transport := NewMockTransport()
	transport.SetResponse("plugin/read", codex.Response{JSONRPC: "2.0", Result: result})
	client := codex.NewClient(transport)
	response, err := client.Plugin.Read(context.Background(), codex.PluginReadParams{PluginName: "calendar", RemoteMarketplaceName: issue132StringPointer("official")})
	if err == nil || !strings.Contains(err.Error(), "missing plugin.summary.id") {
		t.Fatalf("post-source response validation error = %v; want missing summary id", err)
	}
	if !reflect.DeepEqual(response, codex.PluginReadResponse{}) {
		t.Fatalf("public service exposed partially decoded response on failure: %#v", response)
	}
}

func TestPluginSourceNPMDoesNotChangeDetailPathPartialUpdateBoundary(t *testing.T) {
	wire := `{"appTemplates":[],"apps":[],"description":"new","hooks":[],"marketplaceName":"new-name","marketplacePath":"relative","mcpServers":[],"skills":[],"summary":{"authPolicy":"ON_USE","enabled":true,"id":"plugin","installPolicy":"AVAILABLE","installed":true,"name":"calendar","source":{"type":"npm","package":"new-pkg"}}}`
	oldURL := "old-url"
	target := codex.PluginDetail{MarketplaceName: "old-name", Summary: codex.PluginSummary{Source: codex.PluginSource{Type: "remote", URL: &oldURL}}}
	err := json.Unmarshal([]byte(wire), &target)
	if err == nil || !strings.Contains(err.Error(), "plugin.marketplacePath") {
		t.Fatalf("detail path error = %v", err)
	}
	if target.MarketplaceName != "new-name" || target.Description == nil || *target.Description != "new" {
		t.Fatalf("established path-error partial fields not retained: %#v", target)
	}
	if target.Summary.Source.Type != "remote" || target.Summary.Source.URL != &oldURL {
		t.Fatalf("source assignment crossed the existing path-error boundary: %#v", target.Summary.Source)
	}
}

func TestPluginSourceNPMDoesNotChangeSummaryPartialUpdateBoundaries(t *testing.T) {
	const validNPM = `{"type":"npm","package":"pkg","registry":null,"version":""}`
	for _, tt := range []struct {
		name      string
		mutate    string
		wantNPM   bool
		wantError string
	}{
		{name: "invalid auth policy retains established partial assignment", mutate: `"authPolicy":"INVALID"`, wantNPM: true, wantError: "invalid plugin.summary.authPolicy"},
		{name: "invalid keywords restores prior receiver", mutate: `"keywords":[null]`, wantNPM: false, wantError: "keywords"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wire := `{"authPolicy":"ON_USE","enabled":true,"id":"plugin","installPolicy":"AVAILABLE","installed":true,"name":"calendar","source":` + validNPM + `,` + tt.mutate + `}`
			old := "prior"
			target := codex.PluginSummary{Name: "prior-name", Source: codex.PluginSource{Type: "remote", URL: &old}}
			err := json.Unmarshal([]byte(wire), &target)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v; want substring %q", err, tt.wantError)
			}
			if tt.wantNPM {
				if target.Source.Type != "npm" || target.Source.Package == nil || *target.Source.Package != "pkg" {
					t.Fatalf("source was not retained at existing partial-update boundary: %#v", target.Source)
				}
			} else if target.Name != "prior-name" || target.Source.Type != "remote" || target.Source.URL != &old {
				t.Fatalf("transactional validation failure changed receiver: %#v", target)
			}
		})
	}
}

func assertPluginSourceJSONFields(t *testing.T, source codex.PluginSource, want map[string]string) {
	t.Helper()
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("marshal source: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode marshaled source %s: %v", encoded, err)
	}
	if len(got) != len(want) {
		t.Fatalf("source fields = %v; want %v", got, want)
	}
	for field, expected := range want {
		if string(got[field]) != expected {
			t.Errorf("source field %s = %s; want %s (whole JSON %s)", field, got[field], expected, encoded)
		}
	}
}
