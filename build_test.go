package codex_test

import (
	"fmt"
	"runtime"
	"testing"
)

// TestGoVersion verifies the runtime Go version meets the module's minimum (go 1.25).
func TestGoVersion(t *testing.T) {
	const minMajor, minMinor = 1, 25

	var major, minor int
	// runtime.Version() returns e.g. "go1.25.1"
	if _, err := fmt.Sscanf(runtime.Version(), "go%d.%d", &major, &minor); err != nil {
		t.Fatalf("failed to parse Go version %q: %v", runtime.Version(), err)
	}

	if major < minMajor || (major == minMajor && minor < minMinor) {
		t.Fatalf("Go %d.%d required, running %d.%d", minMajor, minMinor, major, minor)
	}

	t.Logf("Go version: %s", runtime.Version())
}
