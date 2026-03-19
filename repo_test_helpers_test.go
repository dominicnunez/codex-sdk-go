package codex_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test helper path")
	}

	return filepath.Dir(file)
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

	matches := regexp.MustCompile(`(?m)^go (\d+)\.(\d+)$`).FindStringSubmatch(string(data))
	if len(matches) != 3 {
		t.Fatalf("go.mod is missing a valid go directive")
	}

	major, err = strconv.Atoi(matches[1])
	if err != nil {
		t.Fatalf("parse go.mod major version %q: %v", matches[1], err)
	}
	minor, err = strconv.Atoi(matches[2])
	if err != nil {
		t.Fatalf("parse go.mod minor version %q: %v", matches[2], err)
	}

	return major, minor, matches[1] + "." + matches[2]
}
