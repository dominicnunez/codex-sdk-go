package protocol_test

import (
	"os"
	"strings"
	"testing"
)

func TestHookRunnerPinsGolangciLintV2(t *testing.T) {
	scriptPath := repoPath(t, "scripts", "hooks", "run.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read %s: %v", scriptPath, err)
	}

	script := string(data)
	requiredSnippets := []string{
		`required_golangci_lint_version="v2.14.0"`,
		`github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${required_golangci_lint_version}`,
		`if [[ "${1:-}" == "golangci-lint" ]]; then`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(script, snippet) {
			t.Fatalf("%s is missing required snippet %q", scriptPath, snippet)
		}
	}
}
