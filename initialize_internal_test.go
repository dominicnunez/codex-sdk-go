package codex

import (
	"slices"
	"testing"
)

func TestNormalizeInitializeParamsCanonicalizesOptOutNotificationMethods(t *testing.T) {
	original := InitializeParams{
		ClientInfo: ClientInfo{Name: "test-client", Version: "1.0.0"},
		Capabilities: &InitializeCapabilities{
			OptOutNotificationMethods: []string{
				"thread/started",
				"thread/closed",
				"thread/started",
			},
		},
	}

	normalized := normalizeInitializeParams(original)
	if normalized.Capabilities == nil {
		t.Fatal("normalized capabilities = nil, want non-nil")
	}

	want := []string{"thread/closed", "thread/started"}
	if !slices.Equal(normalized.Capabilities.OptOutNotificationMethods, want) {
		t.Fatalf(
			"normalized opt-out methods = %v, want %v",
			normalized.Capabilities.OptOutNotificationMethods,
			want,
		)
	}

	if !slices.Equal(
		original.Capabilities.OptOutNotificationMethods,
		[]string{"thread/started", "thread/closed", "thread/started"},
	) {
		t.Fatalf("normalizeInitializeParams mutated caller slice: %v", original.Capabilities.OptOutNotificationMethods)
	}
}
