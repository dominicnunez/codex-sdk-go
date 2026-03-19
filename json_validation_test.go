package codex

import "testing"

func TestUnmarshalInboundObjectPopulatesDestination(t *testing.T) {
	type payload struct {
		Summary string `json:"summary"`
		Count   int    `json:"count"`
	}

	var got payload
	err := unmarshalInboundObject(
		[]byte(`{"summary":"ready","count":2,"ignored":true}`),
		&got,
		[]string{"summary", "count"},
		[]string{"summary", "count"},
	)
	if err != nil {
		t.Fatalf("unmarshalInboundObject() error = %v", err)
	}
	if got.Summary != "ready" || got.Count != 2 {
		t.Fatalf("decoded payload = %+v, want summary=ready count=2", got)
	}
}

func TestUnmarshalInboundObjectRejectsTrailingData(t *testing.T) {
	type payload struct {
		Summary string `json:"summary"`
	}

	var got payload
	if err := unmarshalInboundObject(
		[]byte(`{"summary":"ready"}{"summary":"extra"}`),
		&got,
		[]string{"summary"},
		[]string{"summary"},
	); err == nil {
		t.Fatal("expected trailing data to be rejected")
	}
}

func TestValidateInboundObjectFieldsRejectsMissingRequiredField(t *testing.T) {
	err := validateInboundObjectFields(
		[]byte(`{"count":2}`),
		[]string{"summary", "count"},
		[]string{"summary", "count"},
	)
	if err == nil {
		t.Fatal("expected missing required field error")
	}
}
