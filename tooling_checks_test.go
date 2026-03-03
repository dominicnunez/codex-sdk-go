//go:build tooling
// +build tooling

package codex_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGolangciLint verifies that golangci-lint passes with no issues.
//
// To run golangci-lint manually:
//
//	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
//	golangci-lint run ./...
func TestGolangciLint(t *testing.T) {
	lintBin := "golangci-lint"
	if _, err := exec.LookPath(lintBin); err != nil {
		// Fall back to GOPATH/bin or ~/go/bin
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			gopath = filepath.Join(os.Getenv("HOME"), "go")
		}
		candidate := filepath.Join(gopath, "bin", "golangci-lint")
		if _, err := exec.LookPath(candidate); err != nil {
			t.Skip("golangci-lint not found - install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
		}
		lintBin = candidate
	}

	cmd := exec.Command(lintBin, "run", "./...")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("golangci-lint failed:\n%s", string(output))
	}

	t.Logf("golangci-lint passed successfully")
}

// TestGoBuild verifies the package compiles cleanly with no errors or warnings.
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

// TestGoModTidyDiff verifies module files are tidy without mutating the workspace.
func TestGoModTidyDiff(t *testing.T) {
	cmd := exec.Command("go", "mod", "tidy", "-diff")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy -diff failed: %v\nOutput:\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Fatalf("go mod files are not tidy:\n%s", output)
	}
}
