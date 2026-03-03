package codex

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMarshalForWireFallback(t *testing.T) {
	type plain struct {
		Name string `json:"name"`
	}
	v := plain{Name: "test"}

	got, err := marshalForWire(v)
	if err != nil {
		t.Fatalf("marshalForWire() error = %v", err)
	}

	want, _ := json.Marshal(v)
	if string(got) != string(want) {
		t.Errorf("marshalForWire() = %s, want %s", got, want)
	}
}

func TestMarshalForWireReturnsErrorForTypedNilWireMarshaler(t *testing.T) {
	var params *ApiKeyLoginAccountParams
	_, err := marshalForWire(params)
	if !errors.Is(err, errNilWireMarshaler) {
		t.Fatalf("marshalForWire() error = %v, want errNilWireMarshaler", err)
	}
}
