package codex_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestZeroExternalDependencies verifies that the SDK has no external dependencies
// outside the standard library. This is a key design goal: zero deps, stdlib only.
func TestZeroExternalDependencies(t *testing.T) {
	cmd := exec.Command("go", "list", "-m", "all")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run 'go list -m all': %v\nOutput: %s", err, output)
	}

	// Parse module list - should contain only our own module
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Filter out empty lines
	var modules []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			modules = append(modules, strings.TrimSpace(line))
		}
	}

	// Should have exactly 1 line: github.com/dominicnunez/codex-sdk-go
	if len(modules) != 1 {
		t.Errorf("Expected exactly 1 module (our own), got %d:\n%s", len(modules), strings.Join(modules, "\n"))
		return
	}

	expectedModule := "github.com/dominicnunez/codex-sdk-go"
	if modules[0] != expectedModule {
		t.Errorf("Expected module %q, got %q", expectedModule, modules[0])
	}
}

// TestGoModTidy verifies that 'go mod tidy' produces no changes.
// If this test fails, it means go.mod is out of sync with the codebase.
func TestGoModTidy(t *testing.T) {
	// Read go.mod before
	cmd := exec.Command("cat", "go.mod")
	beforeBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to read go.mod before tidy: %v", err)
	}
	before := string(beforeBytes)

	// Run go mod tidy
	cmd = exec.Command("go", "mod", "tidy")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\nOutput: %s", err, output)
	}

	// Read go.mod after
	cmd = exec.Command("cat", "go.mod")
	afterBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to read go.mod after tidy: %v", err)
	}
	after := string(afterBytes)

	// They should be identical
	if before != after {
		t.Errorf("go.mod changed after 'go mod tidy'. This means go.mod is out of sync.\nBefore:\n%s\n\nAfter:\n%s", before, after)
	}
}
