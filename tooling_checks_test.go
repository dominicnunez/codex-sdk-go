//go:build tooling
// +build tooling

package codex_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestGolangciLint verifies that golangci-lint passes with no issues.
//
// To run golangci-lint manually:
//
//	scripts/hooks/run.sh golangci-lint run ./...
func TestGolangciLint(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}

	cmd := exec.Command(
		bash,
		repoPath(t, "scripts", "hooks", "run.sh"),
		"golangci-lint",
		"run",
		"./...",
	)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "IN_NIX_SHELL=1")
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
