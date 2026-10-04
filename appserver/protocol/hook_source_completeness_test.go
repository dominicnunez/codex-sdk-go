package protocol_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func hookMetadataWithSourceFields(fields string) []byte {
	return []byte(`{"currentHash":"hash","displayOrder":1,"enabled":true,"eventName":"sessionStart","handlerType":"command","command":"hook","isManaged":false,"key":"hook",` + fields + `,"sourcePath":"/tmp/hook","timeoutSec":1,"trustStatus":"trusted"}`)
}

func TestHookMetadataSourceOccurrenceSemantics(t *testing.T) {
	tests := []struct {
		name, fields, wantSource, wantError string
	}{
		{"escaped key and value", `"\u0073ource":"syst\u0065m"`, "system", ""},
		{"escaped duplicate before valid is sticky", `"\u0073ource":"bogus","source":"user"`, "", "invalid hook.source"},
		{"valid duplicates retain final value", `"source":"system","source":"user"`, "user", ""},
		{"invalid earlier duplicate is sticky", `"source":"bogus","source":"user"`, "", "invalid hook.source"},
		{"invalid final duplicate is rejected", `"source":"user","source":"bogus"`, "", "invalid hook.source"},
		{"folded root alias is ignored", `"SOURCE":"bogus","source":"user"`, "user", ""},
		{"null before valid duplicate is rejected", `"source":null,"source":"user"`, "", "must not be null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var metadata codex.HookMetadata
			err := json.Unmarshal(hookMetadataWithSourceFields(tt.fields), &metadata)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("HookMetadata error = %v; want substring %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("HookMetadata decode failed: %v", err)
			}
			if string(metadata.Source) != tt.wantSource {
				t.Fatalf("source = %q; want %q", metadata.Source, tt.wantSource)
			}
		})
	}
}

func TestHookRunSummarySourceRoundTripsAllKnownValuesAndOmission(t *testing.T) {
	sources := []codex.HookSource{
		codex.HookSourceSystem,
		codex.HookSourceUser,
		codex.HookSourceProject,
		codex.HookSourceMDM,
		codex.HookSourceSessionFlags,
		codex.HookSourcePlugin,
		codex.HookSourceCloudRequirements,
		codex.HookSourceCloudManagedConfig,
		codex.HookSourceLegacyManagedConfigFile,
		codex.HookSourceLegacyManagedConfigMDM,
		codex.HookSourceUnknown,
	}
	for _, source := range sources {
		t.Run(string(source), func(t *testing.T) {
			sourceCopy := source
			want := codex.HookRunSummary{
				DisplayOrder:  9,
				Entries:       []codex.HookOutputEntry{},
				EventName:     codex.HookEventName("sessionStart"),
				ExecutionMode: codex.HookExecutionMode("sync"),
				HandlerType:   codex.HookHandlerTypeCommand,
				ID:            "run-roundtrip",
				Scope:         codex.HookScope("thread"),
				Source:        &sourceCopy,
				SourcePath:    `C:\hooks\sample`,
				StartedAt:     123,
				Status:        codex.HookRunStatus("completed"),
			}
			encoded, err := json.Marshal(want)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			var got codex.HookRunSummary
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("roundtrip Unmarshal failed: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("roundtrip = %+v; want %+v", got, want)
			}
		})
	}

	want := codex.HookRunSummary{
		DisplayOrder:  9,
		Entries:       []codex.HookOutputEntry{},
		EventName:     codex.HookEventName("sessionStart"),
		ExecutionMode: codex.HookExecutionMode("sync"),
		HandlerType:   codex.HookHandlerTypeCommand,
		ID:            "run-omitted-source",
		Scope:         codex.HookScope("thread"),
		SourcePath:    `C:\hooks\sample`,
		StartedAt:     123,
		Status:        codex.HookRunStatus("completed"),
	}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		t.Fatal(err)
	}
	if _, present := members["source"]; present {
		t.Fatal("nil optional source was materialized during serialization")
	}
	var got codex.HookRunSummary
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("omitted source roundtrip failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) || got.Source != nil || got.SourcePath != want.SourcePath {
		t.Fatalf("omitted source roundtrip = %+v; want %+v", got, want)
	}
}

type anonymousHookSourceEnvelope struct {
	Before string `json:"before"`
	codex.HookSource
	After string `json:"after"`
}

func TestHookSourceRemainsPlainAndAnonymousEnvelopesRemainNative(t *testing.T) {
	if reflect.TypeOf(codex.HookSource("")).NumMethod() != 0 || reflect.TypeOf((*codex.HookSource)(nil)).NumMethod() != 0 {
		t.Fatal("HookSource acquired public methods")
	}
	var source codex.HookSource
	if err := json.Unmarshal([]byte(`"outsideSchema"`), &source); err != nil || source != codex.HookSource("outsideSchema") {
		t.Fatalf("plain HookSource behavior changed: source=%q err=%v", source, err)
	}
	var envelope anonymousHookSourceEnvelope
	if err := json.Unmarshal([]byte(`{"before":"left","HookSource":"outsideSchema","after":"right"}`), &envelope); err != nil {
		t.Fatalf("anonymous HookSource envelope decode failed: %v", err)
	}
	if envelope.Before != "left" || envelope.HookSource != source || envelope.After != "right" {
		t.Fatalf("anonymous HookSource envelope = %+v", envelope)
	}
}

func TestHookMetadataInvalidSourceDiagnosticLengthIsValueIndependent(t *testing.T) {
	decodeError := func(value string) string {
		sourceJSON, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var metadata codex.HookMetadata
		err = json.Unmarshal(hookMetadataWithSourceFields(`"source":`+string(sourceJSON)), &metadata)
		if err == nil {
			t.Fatal("invalid source was accepted")
		}
		return err.Error()
	}
	short := decodeError("bad")
	large := decodeError(strings.Repeat("x", 1<<20))
	if short != "invalid hook.source" || large != short || len(large) != len(short) {
		t.Fatalf("invalid-source diagnostic differs by peer value: short=%q large length=%d", short, len(large))
	}
}
