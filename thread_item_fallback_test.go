package codex_test

import (
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func assertUnknownThreadItemFallback(t *testing.T, item *codex.UnknownThreadItem, wantRaw string) {
	t.Helper()

	if got := string(item.Raw); got != wantRaw {
		t.Fatalf("UnknownThreadItem.Raw = %s, want %s", got, wantRaw)
	}

	marshaled, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}
	if !json.Valid(marshaled) {
		t.Fatalf("MarshalJSON() produced invalid JSON: %s", marshaled)
	}

	var payload struct {
		Type string          `json:"type"`
		Raw  json.RawMessage `json:"raw"`
	}
	if err := json.Unmarshal(marshaled, &payload); err != nil {
		t.Fatalf("MarshalJSON() payload unmarshal error: %v", err)
	}
	if payload.Type != codex.UnmarshalErrorItemType {
		t.Fatalf("MarshalJSON() type = %q, want %q", payload.Type, codex.UnmarshalErrorItemType)
	}
	if got := string(payload.Raw); got != wantRaw {
		t.Fatalf("MarshalJSON() raw = %s, want %s", got, wantRaw)
	}

	var wrapper codex.ThreadItemWrapper
	if err := json.Unmarshal(marshaled, &wrapper); err != nil {
		t.Fatalf("round-trip unmarshal error: %v", err)
	}
	roundTripped, ok := wrapper.Value.(*codex.UnknownThreadItem)
	if !ok {
		t.Fatalf("round-trip item type = %T, want *UnknownThreadItem", wrapper.Value)
	}
	if roundTripped.Type != codex.UnmarshalErrorItemType {
		t.Fatalf("round-trip type = %q, want %q", roundTripped.Type, codex.UnmarshalErrorItemType)
	}
	if got := string(roundTripped.Raw); got != string(marshaled) {
		t.Fatalf("round-trip raw = %s, want %s", got, marshaled)
	}
}

func TestUnknownThreadItemMarshalWrapsMalformedFallbackPayload(t *testing.T) {
	item := &codex.UnknownThreadItem{
		Type: codex.UnmarshalErrorItemType,
		Raw:  json.RawMessage(`"bad-item"`),
	}

	assertUnknownThreadItemFallback(t, item, `"bad-item"`)
}
