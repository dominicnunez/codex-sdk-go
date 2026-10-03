package protocol

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPathDiagnosticContract(t *testing.T) {
	for _, value := range []string{"relative", "\x00\"\\", "日本語", "\xff", strings.Repeat("x", 256)} {
		if got, want := quotedValueDiagnostic(value), fmt.Sprintf("%q", value); got != want {
			t.Fatalf("short path diagnostic changed: %q != %q", got, want)
		}
	}
	for _, value := range []string{strings.Repeat("x", 257), strings.Repeat("\x00", 1<<20), strings.Repeat("\xff", 1<<20), strings.Repeat("日本語", 1<<18)} {
		got := quotedValueDiagnostic(value)
		if len(got) > 1600 || !strings.Contains(got, "bytes omitted") {
			t.Fatalf("large path preview is not bounded: %d bytes", len(got))
		}
	}
	for _, tc := range []struct {
		name   string
		call   func(string) (string, error)
		prefix string
	}{
		{"relative", normalizeAbsolutePath, ""},
		{"malformed-unc", normalizeWindowsUNCPath, `\\`},
		{"malformed-extended", normalizeWindowsExtendedAbsolutePath, `\\?\`},
		{"malformed-extended-unc", normalizeWindowsExtendedUNCPath, `\\?\UNC\`},
		{"inbound-normalization", func(s string) (string, error) { return validateInboundAbsolutePathField("field", s) }, "/tmp/../"},
		{"outbound", func(s string) (string, error) { return normalizeAbsolutePathField("field", s) }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.call(tc.prefix + strings.Repeat("x", 1<<20))
			if err == nil || len(err.Error()) > 1800 || !strings.Contains(err.Error(), "bytes omitted") {
				t.Fatal("shared path owner does not return a bounded error")
			}
			if tc.name == "outbound" && !errors.Is(err, errInvalidParams) {
				t.Fatal("lost invalid-params classification")
			}
		})
	}
	// Diagnostic truncation must never impose a path-length admission limit.
	value := "/" + strings.Repeat("x", 1<<20)
	got, err := validateInboundAbsolutePathField("field", value)
	if err != nil || got != value {
		t.Fatal("valid absolute path changed")
	}
}
