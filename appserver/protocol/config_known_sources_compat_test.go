package protocol_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func resultConfigSource(t *testing.T, route string, result any) p.ConfigLayerSourceWrapper {
	t.Helper()
	if read, ok := result.(p.ConfigReadResponse); ok {
		if route == "origins" {
			return read.Origins["model"].Name
		}
		return (*read.Layers)[0].Name
	}
	return result.(p.ConfigWriteResponse).OverriddenMetadata.OverridingLayer.Name
}

func TestKnownConfigSourceTypedEncoding(t *testing.T) {
	for _, source := range []p.ConfigLayerSource{
		p.PackagedDefaultsConfigLayerSource{File: "/unopened-fixture/defaults.toml"},
		p.EnterpriseManagedConfigLayerSource{ID: "", Name: ""},
		p.EnterpriseManagedConfigLayerSource{ID: "id", Name: "Policy"},
	} {
		pointer := reflect.New(reflect.TypeOf(source))
		pointer.Elem().Set(reflect.ValueOf(source))
		for _, value := range []p.ConfigLayerSource{source, pointer.Interface().(p.ConfigLayerSource)} {
			raw, err := json.Marshal(p.ConfigLayerSourceWrapper{Value: value})
			if err != nil {
				t.Fatal(err)
			}
			var wrapper p.ConfigLayerSourceWrapper
			if err := json.Unmarshal(raw, &wrapper); err != nil || !reflect.DeepEqual(wrapper.Value, source) {
				t.Fatalf("value/pointer roundtrip: %v %+v", err, wrapper.Value)
			}
			for _, route := range []string{"origins", "layers", "write", "batch"} {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				result, err := configSourceCall(t, client, mock, route, string(raw))
				client.Close()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(resultConfigSource(t, route, result).Value, source) {
					t.Fatal("public typed fields changed")
				}
			}
		}
	}
}

func TestKnownConfigSourceOccurrenceGuards(t *testing.T) {
	for _, raw := range []string{
		`{"type":"packagedDefaults","file":"relative","file":"/defaults.toml"}`,
		`{"type":"packagedDefaults","fi\u006ce":"/tmp/../defaults.toml","file":"/defaults.toml"}`,
		`{"type":"packagedDefaults","file":null,"file":"/defaults.toml"}`,
		`{"type":"packagedDefaults","fi\u006ce":null,"file":"/defaults.toml"}`,
		`{"type":"enterpriseManaged","id":"id","na\u006de":null,"name":"Policy"}`,
		`{"type":"enterpriseManaged","i\u0064":null,"id":"id","name":"Policy"}`,
	} {
		var wrapper p.ConfigLayerSourceWrapper
		if err := json.Unmarshal([]byte(raw), &wrapper); err == nil {
			t.Fatalf("later valid value hid forbidden source occurrence: %s", raw)
		}
		for _, route := range []string{"origins", "layers", "write", "batch"} {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			result, err := configSourceCall(t, client, mock, route, raw)
			client.Close()
			if err == nil || !reflect.ValueOf(result).IsZero() {
				t.Fatalf("hidden occurrence published by%s: %v", route, err)
			}
		}
	}
	for _, raw := range []string{
		`{"type":"packagedDefaults","file":"/first.toml","fi\u006ce":"/last.toml"}`,
		`{"type":"packagedDefaults","FILE":"unconstrained-earlier-extra","file":"/defaults.toml"}`,
		`{"type":"enterpriseManaged","id":"first","id":"","name":"Policy","name":""}`,
		`{"type":"future","type":"enterpriseManaged","id":"","name":""}`,
		`{"type":"enterpriseManaged","type":"future","id":null,"name":null}`,
	} {
		var wrapper p.ConfigLayerSourceWrapper
		if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
			t.Fatalf("valid duplicate/discriminator behavior changed: %v", err)
		}
	}
}

func TestKnownConfigSourceReceiversAndEarlierErrors(t *testing.T) {
	packaged := p.PackagedDefaultsConfigLayerSource{File: "/prior.toml"}
	before := packaged
	if err := json.Unmarshal([]byte(`{"type":"packagedDefaults","file":"/tmp/../bad.toml"}`), &packaged); err == nil || packaged != before {
		t.Fatal("path failure changed prior source")
	}
	enterprise := p.EnterpriseManagedConfigLayerSource{ID: "prior", Name: "prior"}
	prior := enterprise
	if err := json.Unmarshal([]byte(`{"type":"enterpriseManaged","id":null,"id":"new","name":"new"}`), &enterprise); err == nil || enterprise != prior {
		t.Fatal("null failure changed prior source")
	}
	var outer struct {
		Before int                        `json:"before"`
		Source p.ConfigLayerSourceWrapper `json:"source"`
		After  string                     `json:"after"`
	}
	err := json.Unmarshal([]byte(`{"before":"bad","source":{"type":"packagedDefaults","file":"relative"},"after":"visited"}`), &outer)
	var mismatch *json.UnmarshalTypeError
	if !errors.As(err, &mismatch) || mismatch.Field != "before" || outer.After != "visited" {
		t.Fatalf("earlier error or enclosing traversal changed: %v", err)
	}
	err = json.Unmarshal([]byte(`{"type":"packagedDefaults","file":42}`), &packaged)
	if !errors.As(err, &mismatch) || mismatch.Field != "file" {
		t.Fatalf("native file error lost context: %v", err)
	}
	err = json.Unmarshal([]byte(`{"type":false,"file":"relative"}`), &outer.Source)
	if !errors.As(err, &mismatch) {
		t.Fatalf("old discriminator kind error lost priority: %v", err)
	}
	data := []byte(`{"type":"future","file":"relative","id":null,"name":null,"extra":{"preserved":true}}`)
	if err := json.Unmarshal(data, &outer.Source); err != nil {
		t.Fatal(err)
	}
	unknown, ok := outer.Source.Value.(p.UnknownConfigLayerSource)
	if !ok || string(unknown.Raw) != string(data) {
		t.Fatal("future raw source changed")
	}
	data[2] = 'X'
	if unknown.Raw[2] == 'X' {
		t.Fatal("future source borrows input")
	}
	large := `{"type":"packagedDefaults","file":"` + strings.Repeat("relative", 1<<16) + `"}`
	if err := json.Unmarshal([]byte(large), &packaged); err == nil || len(err.Error()) > 1024 {
		t.Fatal("path diagnostic unbounded or missing")
	}
}
