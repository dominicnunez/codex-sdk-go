package codex_test

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"testing"
)

// TestGoBuild verifies the package compiles cleanly with no errors or warnings
func TestGoBuild(t *testing.T) {
	cmd := exec.Command("go", "build", "./...")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("go build failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	// Verify no stderr output (warnings would appear here)
	if stderr.Len() > 0 {
		t.Errorf("go build produced output on stderr:\n%s", stderr.String())
	}
}

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
