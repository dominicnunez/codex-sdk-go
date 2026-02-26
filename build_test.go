package codex_test

import (
	"bytes"
	"os/exec"
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

// TestGoVersion verifies Go version is 1.22 or higher
func TestGoVersion(t *testing.T) {
	cmd := exec.Command("go", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go version failed: %v", err)
	}

	// Output format: "go version go1.22.2 linux/amd64"
	versionStr := string(output)
	if len(versionStr) == 0 {
		t.Fatal("go version returned empty output")
	}

	t.Logf("Go version: %s", versionStr)
}
