package codex_test

import (
	"regexp"
	"runtime"
	"strconv"
	"testing"
)

// TestGoVersion verifies the runtime Go version meets the module's minimum Go directive.
func TestGoVersion(t *testing.T) {
	const goVersionPattern = `go(\d+)\.(\d+)`
	minMajor, minMinor, minVersion := moduleGoVersion(t)

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
		t.Fatalf("Go %s required, running %d.%d", minVersion, major, minor)
	}

	t.Logf("Go version: %s", version)
}
