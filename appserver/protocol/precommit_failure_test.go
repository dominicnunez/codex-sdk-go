package protocol_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreCommitRejectsGoFragments(t *testing.T) {
	f := newCommitFixture(t, map[string]string{"base.go": fixturePackage + "const Before = 0\n"}, false)
	f.write(t, "_ignored.go", "var Fragment int\n")
	f.git(t, "add", "_ignored.go")
	if out, err := exec.Command("gofmt", filepath.Join(f.dir, "_ignored.go")).CombinedOutput(); err == nil {
		t.Fatalf("fixture must fail full-file gofmt parsing: %s", out)
	}
	if out, err := f.hook(); err == nil {
		t.Errorf("Go declaration fragment accepted as a source file: %s", out)
	}
	if _, err := os.Stat(f.calls); !os.IsNotExist(err) {
		t.Fatalf("invalid staged Go file reached lint: %v", err)
	}
}

func TestPreCommitRejectsFailedReports(t *testing.T) {
	for _, tc := range []struct {
		name, report string
		wantError    bool
	}{
		{"empty report", "", true},
		{"malformed report", "{", true},
		{"missing Issues", `{}`, true},
		{"invalid Issues", `{"Issues":{}}`, true},
		{"typecheck outside patch", `{"Issues":[{"FromLinter":"typecheck","Text":"fixture cannot compile"}]}`, true},
		{"unknown source", `{"Issues":[{"FromLinter":"errcheck","Text":"fixture","Pos":{"Filename":"/outside.go","Line":1}}]}`, true},
		{"empty Issues", `{"Issues":[]}`, false},
		{"null Issues", `{"Issues":null}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{"sample.go": fixturePackage + "const Before = 0\n"}, false)
			f.write(t, "sample.go", fixturePackage+"const Selected = 1\n")
			f.git(t, "add", "sample.go")
			index := strings.TrimSpace(string(f.git(t, "rev-parse", "--git-path", "index")))
			if !filepath.IsAbs(index) {
				index = filepath.Join(f.dir, index)
			}
			before, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			out, hookErr := f.hook("LINT_FIXTURE_REPORT=" + tc.report)
			if (hookErr != nil) != tc.wantError {
				t.Errorf("report admission err=%v, wantError=%v: %s", hookErr, tc.wantError, out)
			}
			if after, err := os.ReadFile(index); err != nil || !bytes.Equal(before, after) {
				t.Fatalf("report check modified index bytes: %v", err)
			}
			matches, err := filepath.Glob(filepath.Join(filepath.Dir(index), "hook-index-*"))
			if err != nil || len(matches) != 0 {
				t.Fatalf("private index resources leaked: %q, %v", matches, err)
			}
		})
	}
}
