package codex

import (
	"encoding/json"
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
