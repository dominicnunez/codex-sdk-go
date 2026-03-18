//go:build tooling
// +build tooling

package codex_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const golangciLintVersion = "v2.11.3"

// TestGolangciLint verifies that golangci-lint passes with no issues.
//
// To run golangci-lint manually:
//
//	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.3
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
			t.Skip("golangci-lint not found - install with: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + golangciLintVersion)
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\noutput: %s", err, output)
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
