package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// The pre-fix decoder is an independent reference for unchanged admission errors.
type priorElicitationParams McpServerElicitationRequestParams

func (p *priorElicitationParams) UnmarshalJSON(data []byte) error {
	type wire McpServerElicitationRequestParams
	var decoded wire
	var schema json.RawMessage
	type elicitationWire struct {
		*wire
		RequestedSchema *json.RawMessage `json:"requestedSchema"`
	}
	w := elicitationWire{wire: &decoded, RequestedSchema: &schema}
	required := []string{"serverName", "threadId"}
	// Select only a supported form branch. The actual wire decoder retains
	// ownership of selector errors and exact field matching.
	rawMode, _, object := jsonobject.SelectField(data, "mode")
	form := object && isElicitationFormMode(rawMode)
	if form {
		extra := struct {
			*elicitationWire
			ElicitationID stringExtra `json:"elicitationId"`
			URL           stringExtra `json:"url"`
		}{elicitationWire: &w}
		if err := unmarshalInboundObject(data, &extra, required, required); err != nil {
			return err
		}
		decoded.ElicitationID = extra.ElicitationID.Value
		decoded.URL = extra.URL.Value
	} else if err := unmarshalInboundObject(data, &w, required, required); err != nil {
		return err
	}
	if err := validateNonEmptyStringField("serverName", decoded.ServerName); err != nil {
		return err
	}
	if err := validateNonEmptyStringField("threadId", decoded.ThreadID); err != nil {
		return err
	}
	if err := validateNonEmptyStringPointerField("turnId", decoded.TurnID); err != nil {
		return err
	}
	if err := validateMcpServerElicitationVariant(data, decoded.Mode); err != nil {
		return err
	}
	switch decoded.Mode {
	case McpServerElicitationModeForm:
		if err := jsondecode.Unmarshal(schema, &decoded.RequestedSchema); err != nil {
			return err
		}
	case McpServerElicitationModeOpenAIForm, McpServerElicitationModeOpenAIFormLegacy:
		// An explicitly null schema is valid arbitrary JSON.
		if w.RequestedSchema == nil {
			schema = json.RawMessage("null")
		}
		decoded.OpenAIRequestedSchema = schema
	}
	if err := validateEnumOccurrences(data, "mode", false, validateElicitationMode); err != nil {
		return err
	}
	*p = priorElicitationParams(decoded)
	return nil
}

func TestElicitationMetadataPriorErrors(t *testing.T) {
	for _, mode := range []string{"form", "url", "openaiForm", "openai/form"} {
		base := fmt.Sprintf(`"serverName":"s","threadId":"t","mode":%q,"message":"m","requestedSchema":{"type":"object","properties":{}},"elicitationId":"e","url":"opaque"`, mode)
		for _, raw := range []string{
			`null`, `[]`, `{}`, `{"serverName":`,
			`{` + base + `,"serverName":17,"_meta":{"n":42}}`,
			`{` + base + `,"turnId":[],"_meta":42}`,
			`{` + base + `,"message":17,"mode":"future"}`,
			`{` + base + `,"threadId":"","mode":"future"}`,
			`{` + base + `,"mode":null}`,
			`{` + base + `,"mode":"future","mode":` + fmt.Sprintf("%q", mode) + `}`,
			`{` + base + `,"requestedSchema":[]}`,
			`{` + base + `,"url":42,"elicitationId":[]}`,
		} {
			var prior priorElicitationParams
			want := prior.UnmarshalJSON([]byte(raw))
			if want == nil {
				continue
			} // Inactive variant extras remain unrestricted.
			var current McpServerElicitationRequestParams
			got := current.UnmarshalJSON([]byte(raw))
			if got == nil || got.Error() != want.Error() {
				t.Fatalf("%s %s error changed: %v / %v", mode, raw, got, want)
			}
			var a, b *json.UnmarshalTypeError
			if errors.As(got, &a) != errors.As(want, &b) || !reflect.DeepEqual(a, b) {
				t.Fatalf("native error context changed: %#v / %#v", a, b)
			}
		}
	}
}
