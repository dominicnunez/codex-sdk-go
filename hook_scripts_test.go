package codex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrePushHookRunsRaceAndLintChecks(t *testing.T) {
	scriptPath := filepath.Join("scripts", "hooks", "pre-push.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read %s: %v", scriptPath, err)
	}

	script := string(data)
	requiredChecks := []string{
		`"$runner" go test ./...`,
		`"$runner" go test -race ./...`,
		`"$runner" golangci-lint run ./...`,
		`"$runner" go mod tidy -diff`,
	}
	for _, check := range requiredChecks {
		if !strings.Contains(script, check) {
			t.Fatalf("%s is missing required check %q", scriptPath, check)
		}
	}

	if strings.Contains(script, `"$runner" go vet ./...`) {
		t.Fatalf("%s still runs go vet instead of the documented lint lane", scriptPath)
	}
}

func TestHookRunnerPinsGolangciLintV2(t *testing.T) {
	scriptPath := filepath.Join("scripts", "hooks", "run.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read %s: %v", scriptPath, err)
	}

	script := string(data)
	requiredSnippets := []string{
		`required_golangci_lint_version="v2.11.3"`,
		`github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${required_golangci_lint_version}`,
		`if [[ "${1:-}" == "golangci-lint" ]]; then`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(script, snippet) {
			t.Fatalf("%s is missing required snippet %q", scriptPath, snippet)
		}
	}
}
