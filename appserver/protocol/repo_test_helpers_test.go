package protocol_test

import (
	"go/version"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test helper path")
	}

	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func repoPath(t *testing.T, parts ...string) string {
	t.Helper()

	pathParts := append([]string{repoRoot(t)}, parts...)
	return filepath.Join(pathParts...)
}

func moduleGoVersion(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(repoPath(t, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		parts := strings.Fields(line)
		if len(parts) < 2 || parts[0] != "go" {
			continue
		}

		if !version.IsValid("go" + parts[1]) {
			t.Fatalf("invalid go.mod version %q", parts[1])
		}
		return parts[1]
	}

	t.Fatalf("go.mod is missing a valid go directive")

	return ""
}
