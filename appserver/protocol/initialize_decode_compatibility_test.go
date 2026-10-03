package protocol_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// These plain models intentionally have no custom decoder. They define standard
// receiver effects from the schema without sharing the SDK's decode helpers.
type initializeCapsReference struct {
	ExperimentalAPI                bool                       `json:"experimentalApi"`
	ExplicitGatewayOAuth           bool                       `json:"explicitGatewayOauth,omitempty"`
	Extensions                     map[string]json.RawMessage `json:"extensions,omitempty"`
	McpServerOpenaiFormElicitation bool                       `json:"mcpServerOpenaiFormElicitation,omitempty"`
	RequestAttestation             bool                       `json:"requestAttestation,omitempty"`
	OptOutNotificationMethods      []string                   `json:"optOutNotificationMethods,omitempty"`
}

type initializeParamsReference struct {
	ClientInfo   codex.ClientInfo         `json:"clientInfo"`
	Capabilities *initializeCapsReference `json:"capabilities,omitempty"`
}

type initializeWrapper struct {
	Before       int                           `json:"before"`
	Capabilities *codex.InitializeCapabilities `json:"capabilities,omitempty"`
	Params       *codex.InitializeParams       `json:"params,omitempty"`
	After        string                        `json:"after"`
}

type initializeWrapperReference struct {
	Before       int                        `json:"before"`
	Capabilities *initializeCapsReference   `json:"capabilities,omitempty"`
	Params       *initializeParamsReference `json:"params,omitempty"`
	After        string                     `json:"after"`
}

func compareInitializeJSON(t *testing.T, label string, actual, expected interface{}) {
	t.Helper()
	got, gotErr := json.Marshal(actual)
	want, wantErr := json.Marshal(expected)
	if gotErr != nil || wantErr != nil || string(got) != string(want) {
		t.Fatalf("%s=%s want=%s errors=%v,%v", label, got, want, gotErr, wantErr)
	}
}

func compareInitializeDecodeError(t *testing.T, actual, expected error) {
	t.Helper()
	if (actual == nil) != (expected == nil) {
		t.Fatalf("error=%v reference=%v", actual, expected)
	}
	if expected == nil {
		return
	}
	var gotType, wantType *json.UnmarshalTypeError
	var gotSyntax, wantSyntax *json.SyntaxError
	if errors.As(expected, &wantType) {
		if !errors.As(actual, &gotType) {
			t.Fatalf("error=%v reference=%v", actual, expected)
		}
		names := map[string]string{"initializeCapsReference": "InitializeCapabilities", "initializeParamsReference": "InitializeParams", "initializeWrapperReference": "initializeWrapper"}
		wantStruct := wantType.Struct
		if name, ok := names[wantStruct]; ok {
			wantStruct = name
		}
		wantTarget := wantType.Type
		switch wantTarget {
		case reflect.TypeOf(initializeCapsReference{}):
			wantTarget = reflect.TypeOf(codex.InitializeCapabilities{})
		case reflect.TypeOf(initializeParamsReference{}):
			wantTarget = reflect.TypeOf(codex.InitializeParams{})
		case reflect.TypeOf(initializeWrapperReference{}):
			wantTarget = reflect.TypeOf(initializeWrapper{})
		}
		if gotType.Value != wantType.Value || gotType.Offset != wantType.Offset || gotType.Field != wantType.Field || gotType.Struct != wantStruct || gotType.Type != wantTarget {
			t.Fatalf("type error=%+v reference=%+v (expected struct=%s type=%v)", *gotType, *wantType, wantStruct, wantTarget)
		}
	} else if errors.As(expected, &wantSyntax) {
		if !errors.As(actual, &gotSyntax) || gotSyntax.Offset != wantSyntax.Offset || actual.Error() != expected.Error() {
			t.Fatalf("syntax error=%v reference=%v", actual, expected)
		}
	} else {
		t.Fatalf("unexpected reference error=%v", expected)
	}
}

func TestInitializeNestedDecodeCompatibility(t *testing.T) {
	for _, nilCaps := range []bool{false, true} {
		for _, declaration := range []string{
			`{"capabilities":{"experimentalApi":"bad"},"clientInfo":{"name":"updated","version":"2","title":"new"}}`,
			`{"clientInfo":{"name":"updated","version":1},"capabilities":{"experimentalApi":"bad","requestAttestation":true}}`,
			`{"capabilities":{"experimentalApi":"bad","optOutNotificationMethods":["changed"]},"clientInfo":{"name":"updated","version":1}}`,
			`{"capabilities":[],"clientInfo":{"name":"updated","version":"2"}}`,
			`{"capabilities":{"extensions":{"b":2}},"capabilities":null,"capabilities":{"requestAttestation":true}}`,
			`{"clientInfo":{"title":"new"},"capabilities":{"extensions":[],"extensions":{"b":2}}}`,
			`{"clientInfo":{"name":"updated"}} {}`, `null`, `[]`,
		} {
			t.Run(declaration+"/nil="+strconv.FormatBool(nilCaps), func(t *testing.T) {
				const initial = `{"clientInfo":{"name":"old","version":"1","title":"old"},"capabilities":{"experimentalApi":true,"extensions":{"saved":{"n":1}},"optOutNotificationMethods":["old","values","here"]}}`
				var actual codex.InitializeParams
				var expected initializeParamsReference
				if err := json.Unmarshal([]byte(initial), &actual); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(initial), &expected); err != nil {
					t.Fatal(err)
				}
				if nilCaps {
					actual.Capabilities = nil
					expected.Capabilities = nil
				}
				retained, wantRetained := actual.Capabilities, expected.Capabilities
				title, wantTitle := actual.ClientInfo.Title, expected.ClientInfo.Title
				err := json.Unmarshal([]byte(declaration), &actual)
				wantErr := json.Unmarshal([]byte(declaration), &expected)
				compareInitializeJSON(t, "receiver", actual, expected)
				compareInitializeJSON(t, "retained capabilities", retained, wantRetained)
				compareInitializeJSON(t, "retained title", title, wantTitle)
				compareInitializeDecodeError(t, err, wantErr)
			})
		}
	}
}

func TestInitializeUserWrapperDecodeCompatibility(t *testing.T) {
	for _, declaration := range []string{
		`{"capabilities":{"experimentalApi":"bad"},"after":"updated"}`,
		`{"before":"bad","capabilities":{"experimentalApi":"bad"},"after":"updated"}`,
		`{"capabilities":{"experimentalApi":"bad"},"before":"bad","after":"updated"}`,
		`{"params":{"capabilities":{"experimentalApi":"bad"},"clientInfo":{"name":"updated","version":"2"}},"after":"updated"}`,
		`{"before":"bad","params":{"clientInfo":{"version":1},"capabilities":{"requestAttestation":true}},"after":"updated"}`,
		`{"params":{"capabilities":[],"clientInfo":{"title":"new"}},"after":"updated"}`,
		`{"capabilities":{"extensions":{"b":2}},"params":{"capabilities":{"extensions":{"c":3}}},"after":"updated"}`,
		`{"after":"updated"} {}`, `null`, `[]`,
	} {
		t.Run(declaration, func(t *testing.T) {
			const initial = `{"before":1,"after":"old","capabilities":{"experimentalApi":true,"optOutNotificationMethods":["old"]},"params":{"clientInfo":{"name":"old","version":"1","title":"old"},"capabilities":{"experimentalApi":true,"optOutNotificationMethods":["old"]}}}`
			var actual initializeWrapper
			var expected initializeWrapperReference
			if err := json.Unmarshal([]byte(initial), &actual); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(initial), &expected); err != nil {
				t.Fatal(err)
			}
			retained, wantRetained := actual.Capabilities, expected.Capabilities
			params, wantParams := actual.Params, expected.Params
			err := json.Unmarshal([]byte(declaration), &actual)
			wantErr := json.Unmarshal([]byte(declaration), &expected)
			compareInitializeJSON(t, "receiver", actual, expected)
			compareInitializeJSON(t, "retained capabilities", retained, wantRetained)
			compareInitializeJSON(t, "retained params", params, wantParams)
			compareInitializeDecodeError(t, err, wantErr)
		})
	}
}
