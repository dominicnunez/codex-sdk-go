package codex_test

import (
	"regexp"
	"runtime"
	"strconv"
	"testing"
)

// TestGoVersion verifies the runtime Go version meets the module's minimum (go 1.25).
func TestGoVersion(t *testing.T) {
	const minMajor, minMinor = 1, 25
	const goVersionPattern = `go(\d+)\.(\d+)`

	version := runtime.Version()
	matches := regexp.MustCompile(goVersionPattern).FindStringSubmatch(version)
	if len(matches) != 3 {
		t.Fatalf("failed to parse Go version %q", version)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		t.Fatalf("failed to parse major version %q: %v", matches[1], err)
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		t.Fatalf("failed to parse minor version %q: %v", matches[2], err)
	}
	if major < minMajor || (major == minMajor && minor < minMinor) {
		t.Fatalf("Go %d.%d required, running %d.%d", minMajor, minMinor, major, minor)
	}

	t.Logf("Go version: %s", version)
}
