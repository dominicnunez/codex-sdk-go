package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestConfigProfilePublicCarriers(t *testing.T) {
	for _, profile := range []string{`"production"`, `""`, `null`, "absent"} {
		t.Run(profile, func(t *testing.T) {
			source := `{"type":"user","file":"/tmp/config.toml"`
			if profile != "absent" {
				source += `,"profile":` + profile
			}
			source += "}"
			check := func(value p.ConfigLayerSourceWrapper) {
				t.Helper()
				encoded, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				if profile == "absent" || profile == "null" {
					if _, present := fields["profile"]; present {
						t.Fatalf("nil profile emitted: %s", encoded)
					}
				} else if string(fields["profile"]) != profile {
					t.Fatalf("profile lost: %s", encoded)
				}
				if string(fields["file"]) != `"/tmp/config.toml"` || string(fields["type"]) != `"user"` {
					t.Fatalf("source identity changed: %s", encoded)
				}
			}
			var wrapper p.ConfigLayerSourceWrapper
			if err := json.Unmarshal([]byte(source), &wrapper); err != nil {
				t.Fatal(err)
			}
			check(wrapper)
			mock := NewMockTransport()
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			payload := `{"config":{},"layers":[{"config":{},"name":` + source + `,"version":"v"}],"origins":{"model":{"name":` + source + `,"version":"v"}}}`
			mock.SetResponse("config/read", p.Response{Result: json.RawMessage(payload)})
			got, err := client.Config.Read(context.Background(), p.ConfigReadParams{})
			if err != nil {
				t.Fatal(err)
			}
			check((*got.Layers)[0].Name)
			check(got.Origins["model"].Name)
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var roundtrip p.ConfigReadResponse
			if err := json.Unmarshal(encoded, &roundtrip); err != nil {
				t.Fatal(err)
			}
			check((*roundtrip.Layers)[0].Name)
			check(roundtrip.Origins["model"].Name)
			writePayload := `{"filePath":"/tmp/config.toml","status":"okOverridden","version":"v","overriddenMetadata":{"effectiveValue":null,"message":"","overridingLayer":{"name":` + source + `,"version":"v"}}}`
			mock.SetResponse("config/value/write", p.Response{Result: json.RawMessage(writePayload)})
			mock.SetResponse("config/batchWrite", p.Response{Result: json.RawMessage(writePayload)})
			written, err := client.Config.Write(context.Background(), p.ConfigValueWriteParams{KeyPath: "model", MergeStrategy: p.MergeStrategyReplace, Value: json.RawMessage(`null`)})
			if err != nil {
				t.Fatal(err)
			}
			check(written.OverriddenMetadata.OverridingLayer.Name)
			batched, err := client.Config.BatchWrite(context.Background(), p.ConfigBatchWriteParams{Edits: []p.ConfigEdit{}})
			if err != nil {
				t.Fatal(err)
			}
			check(batched.OverriddenMetadata.OverridingLayer.Name)
		})
	}
}

func TestConfigProfileConstructionAndRecovery(t *testing.T) {
	for _, profile := range []*string{nil, strPtr(""), strPtr("production")} {
		source := p.UserConfigLayerSource{File: "/tmp/config.toml", Profile: profile}
		for _, value := range []p.ConfigLayerSource{source, &source} {
			encoded, err := json.Marshal(p.ConfigLayerSourceWrapper{Value: value})
			if err != nil {
				t.Fatal(err)
			}
			var decoded p.ConfigLayerSourceWrapper
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded.Value, source) {
				t.Fatalf("constructed source changed: %s %+v", encoded, decoded)
			}
		}
	}
	var source p.UserConfigLayerSource
	if err := json.Unmarshal([]byte(`{"type":"user","file":"/tmp/old.toml","profile":"production"}`), &source); err != nil {
		t.Fatal(err)
	}
	prior := source.Profile
	for _, payload := range []string{
		`{"type":"user","file":"/tmp/new.toml","profile":3}`,
		`{"type":"user","file":"/tmp/new.toml","profile":true}`,
		`{"type":"user","file":"/tmp/new.toml","profile":[]}`,
		`{"type":"user","file":"/tmp/new.toml","profile":{}}`,
		`{"type":"user","file":"/tmp/new.toml","profile":3,"profile":"production"}`,
		`{"type":"user","file":"relative","profile":"production"}`,
		`{"type":"user","profile":"production"}`,
		`{"type":"user","file":null,"profile":"production"}`,
	} {
		if err := json.Unmarshal([]byte(payload), &source); err == nil {
			t.Fatalf("invalid source admitted: %s", payload)
		}
		if source.File != "/tmp/old.toml" || source.Profile != prior || *prior != "production" {
			t.Fatalf("failed decode mutated receiver: %+v", source)
		}
	}
	if err := json.Unmarshal([]byte(`{"type":"user","file":"/tmp/new.toml"}`), &source); err != nil || source.Profile != nil {
		t.Fatalf("omitted profile did not reset: %+v %v", source, err)
	}
	if *prior != "production" {
		t.Fatal("replacement mutated prior reference")
	}
	for _, tc := range []struct {
		payload string
		want    *string
	}{
		{`{"type":"user","file":"/tmp/new.toml","profile":"first","profile":""}`, strPtr("")},
		{`{"type":"user","file":"/tmp/new.toml","profile":"first","profile":null}`, nil},
	} {
		var wrapper p.ConfigLayerSourceWrapper
		if err := json.Unmarshal([]byte(tc.payload), &wrapper); err != nil {
			t.Fatal(err)
		}
		named := wrapper.Value.(p.UserConfigLayerSource)
		if !reflect.DeepEqual(named.Profile, tc.want) {
			t.Fatalf("last duplicate lost: %+v", named)
		}
	}
	var wrapper p.ConfigLayerSourceWrapper
	if err := json.Unmarshal([]byte(`{"type":"user","file":"/tmp/config.toml","profile":"production"}`), &wrapper); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"type":"user","file":"/tmp/config.toml","profile":3}`), &wrapper); err == nil {
		t.Fatal("bad profile accepted")
	}
	if got := wrapper.Value.(p.UserConfigLayerSource); got.Profile == nil || *got.Profile != "production" {
		t.Fatal("failed wrapper changed branch")
	}
	if err := json.Unmarshal([]byte(`{"type":"system","file":"/tmp/system.toml"}`), &wrapper); err != nil {
		t.Fatal(err)
	}
	if _, ok := wrapper.Value.(p.SystemConfigLayerSource); !ok {
		t.Fatal("wrapper branch replacement failed")
	}
}

func TestConfigProfileMalformedPublicResponses(t *testing.T) {
	for _, source := range []string{
		`{"type":"user","file":"/tmp/config.toml","profile":3}`,
		`{"type":"user","file":"relative","profile":"production"}`,
	} {
		for _, inLayers := range []bool{false, true} {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			payload := `{"config":{},"origins":{"model":{"name":` + source + `,"version":"v"}}}`
			if inLayers {
				payload = `{"config":{},"origins":{},"layers":[{"config":null,"name":` + source + `,"version":"v"}]}`
			}
			mock.SetResponse("config/read", p.Response{Result: json.RawMessage(payload)})
			if _, err := client.Config.Read(context.Background(), p.ConfigReadParams{}); err == nil {
				t.Fatalf("malformed public source accepted: %s", payload)
			}
		}
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		payload := `{"filePath":"/tmp/config.toml","status":"okOverridden","version":"v","overriddenMetadata":{"effectiveValue":null,"message":"","overridingLayer":{"name":` + source + `,"version":"v"}}}`
		mock.SetResponse("config/value/write", p.Response{Result: json.RawMessage(payload)})
		mock.SetResponse("config/batchWrite", p.Response{Result: json.RawMessage(payload)})
		if _, err := client.Config.Write(context.Background(), p.ConfigValueWriteParams{KeyPath: "model", MergeStrategy: p.MergeStrategyReplace, Value: json.RawMessage(`null`)}); err == nil {
			t.Fatal("invalid overriding source admitted by Write")
		}
		if _, err := client.Config.BatchWrite(context.Background(), p.ConfigBatchWriteParams{Edits: []p.ConfigEdit{}}); err == nil {
			t.Fatal("invalid overriding source admitted by BatchWrite")
		}
	}
}

func TestConfigSourceSiblingFieldRetention(t *testing.T) {
	for _, payload := range []string{
		`{"type":"mdm","domain":"domain","key":"key"}`,
		`{"type":"system","file":"/tmp/system.toml"}`,
		`{"type":"project","dotCodexFolder":"/tmp/.codex"}`,
		`{"type":"sessionFlags"}`,
		`{"type":"legacyManagedConfigTomlFromFile","file":"/tmp/managed.toml"}`,
		`{"type":"legacyManagedConfigTomlFromMdm"}`,
		`{"type":"packagedDefaults","file":"/tmp/defaults.toml"}`,
		`{"type":"enterpriseManaged","id":"id","name":"name"}`,
		`{"type":"future","metadata":{"value":null}}`,
	} {
		var source p.ConfigLayerSourceWrapper
		if err := json.Unmarshal([]byte(payload), &source); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(source)
		if err != nil {
			t.Fatal(err)
		}
		var before, after map[string]any
		if err := json.Unmarshal([]byte(payload), &before); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &after); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("sibling fields lost: %s -> %s", payload, encoded)
		}
	}
}
