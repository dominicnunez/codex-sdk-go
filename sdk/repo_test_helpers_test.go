package codex_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test helper path")
	}

	return filepath.Dir(filepath.Dir(file))
}

func repoPath(t *testing.T, parts ...string) string {
	t.Helper()

	pathParts := append([]string{repoRoot(t)}, parts...)
	return filepath.Join(pathParts...)
}

func moduleGoVersion(t *testing.T) (major int, minor int, raw string) {
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

		versionParts := strings.SplitN(parts[1], ".", 2)
		if len(versionParts) != 2 {
			continue
		}

		major, err = strconv.Atoi(versionParts[0])
		if err != nil {
			t.Fatalf("parse go.mod major version %q: %v", versionParts[0], err)
		}
		minor, err = strconv.Atoi(versionParts[1])
		if err != nil {
			t.Fatalf("parse go.mod minor version %q: %v", versionParts[1], err)
		}

		return major, minor, parts[1]
	}

	t.Fatalf("go.mod is missing a valid go directive")

	return 0, 0, ""
}
