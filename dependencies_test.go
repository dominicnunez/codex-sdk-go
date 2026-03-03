package codex_test

import (
	"os"
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

// TestGoModFileReadable verifies go.mod exists and can be read portably.
func TestGoModFileReadable(t *testing.T) {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("go.mod should not be empty")
	}
}
