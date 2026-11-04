package protocol_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type commitFixture struct {
	dir, parent, bash, calls string
	env                      []string
}

func newCommitFixture(t *testing.T, files map[string]string, realLint bool) *commitFixture {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("Go toolchain not installed")
	}
	f := &commitFixture{parent: t.TempDir(), bash: bash, env: os.Environ()}
	localEnv, err := exec.Command("git", "rev-parse", "--local-env-vars").CombinedOutput()
	if err != nil {
		t.Fatalf("identify repository-local Git environment: %v: %s", err, localEnv)
	}
	local := make(map[string]bool)
	for _, name := range strings.Fields(string(localEnv)) {
		local[strings.ToUpper(name)] = true
	}
	f.env = nil
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !local[strings.ToUpper(name)] {
			f.env = append(f.env, entry)
		}
	}
	f.dir = filepath.Join(f.parent, "repo")
	f.calls = filepath.Join(f.parent, "lint-calls.txt")
	f.write(t, "go.mod", "module github.com/dominicnunez/codex-sdk-go\n\ngo 1.27.3\n")
	f.write(t, "README.md", "fixture\n")
	paths := []string{".gitattributes", ".githooks/pre-commit", "scripts/hooks/run.sh", ".golangci.yml", "scripts/hooks/check.go", "scripts/hooks/bootstrap.mod"}
	processFiles, err := os.ReadDir(repoPath(t, "internal/process"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range processFiles {
		if strings.HasSuffix(file.Name(), ".go") && !strings.HasSuffix(file.Name(), "_test.go") {
			paths = append(paths, "internal/process/"+file.Name())
		}
	}
	for _, path := range paths {
		data, err := os.ReadFile(repoPath(t, filepath.FromSlash(path)))
		if path == "scripts/hooks/check.go" && os.IsNotExist(err) {
			continue // The original hook has no standalone snapshot checker.
		}
		if err != nil {
			t.Fatal(err)
		}
		f.write(t, path, string(data))
		if strings.HasSuffix(path, ".sh") || path == ".githooks/pre-commit" {
			if err := os.Chmod(filepath.Join(f.dir, filepath.FromSlash(path)), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	for path, data := range files {
		f.write(t, path, data)
	}
	f.git(t, "init", "--quiet", "--initial-branch=main")
	f.git(t, "config", "commit.gpgsign", "false")
	f.git(t, "add", "--all")
	f.git(t, "commit", "--quiet", "-m", "test: fixture baseline")
	if !realLint {
		bin := filepath.Join(f.parent, "bin")
		if err := os.MkdirAll(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		const linter = `#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = version ]; then
  if [ "${LINT_FIXTURE_PHASE:-}" = version ]; then
    (read -r reply < "$LINT_FIXTURE_WAIT") &
    printf '%s\n' "$BASHPID" > "$LINT_FIXTURE_READY"
    wait
  fi
  if [ "${LINT_FIXTURE_PHASE:-}" = install ]; then echo 'golangci-lint has version 2.13.0'; exit 0; fi
  echo 'golangci-lint has version 2.14.0'
  exit 0
fi
printf '%s\n' "$*" >> "$LINT_FIXTURE_CALLS"
if [ "${LINT_FIXTURE_PHASE:-}" = complete ]; then
  (read -r reply < "$LINT_FIXTURE_WAIT") &
  printf '%s\n' "$!" > "$LINT_FIXTURE_READY"
else
  if [ -n "${LINT_FIXTURE_READY:-}" ]; then printf '%s\n' "$PPID" > "$LINT_FIXTURE_READY"; fi
  [[ -z "${LINT_FIXTURE_WAIT:-}" ]] || read -r reply < "$LINT_FIXTURE_WAIT"
fi
if [ "$LINT_FIXTURE_EXIT" != 0 ]; then exit "$LINT_FIXTURE_EXIT"; fi
report_body='{"Issues":[]}'
if [[ -v LINT_FIXTURE_REPORT ]]; then report_body="$LINT_FIXTURE_REPORT"; fi
for arg in "$@"; do
  case "$arg" in
    --output.json.path=*) printf '%s' "$report_body" > "${arg#*=}" ;;
  esac
done
`
		if err := os.WriteFile(filepath.Join(bin, "golangci-lint"), []byte(linter), 0o700); err != nil {
			t.Fatal(err)
		}
		f.env = append(f.env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	f.env = append(f.env, "LINT_FIXTURE_CALLS="+filepath.ToSlash(f.calls), "LINT_FIXTURE_EXIT=0")
	return f
}

func (f *commitFixture) write(t *testing.T, path, data string) {
	t.Helper()
	path = filepath.Join(f.dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *commitFixture) git(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = f.dir, f.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %q: %v: %s", args, err, out)
	}
	return out
}

func (f *commitFixture) hook(extra ...string) ([]byte, error) {
	cmd := exec.Command(f.bash, filepath.ToSlash(filepath.Join(f.dir, ".githooks", "pre-commit")))
	cmd.Dir, cmd.Env = f.dir, append(append([]string(nil), f.env...), extra...)
	return cmd.CombinedOutput()
}

func formattedGo(t *testing.T, source string) string {
	t.Helper()
	cmd := exec.Command("gofmt")
	cmd.Stdin = strings.NewReader(source)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("independent gofmt fixture: %v: %s", err, out)
	}
	return string(out)
}

const fixturePackage = "// Package fixture exercises local hook snapshots.\npackage fixture\n\n"

func TestPreCommitMissingBootstrapModuleBlocks(t *testing.T) {
	f := newCommitFixture(t, map[string]string{"sample.go": fixturePackage + "const Before = 0\n"}, false)
	f.write(t, "sample.go", fixturePackage+"const Selected = 1\n")
	f.git(t, "add", "sample.go")
	before := f.git(t, "show", ":sample.go")
	if err := os.Remove(filepath.Join(f.dir, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if out, err := f.hook(); err == nil {
		t.Fatalf("missing bootstrap module did not block check: %s", out)
	}
	if after := f.git(t, "show", ":sample.go"); !bytes.Equal(before, after) {
		t.Fatal("missing bootstrap prerequisite changed staging")
	}
}

func TestPreCommitFixtureIgnoresCallerGitState(t *testing.T) {
	for _, repositoryEnv := range []bool{false, true} {
		t.Run(fmt.Sprint(repositoryEnv), func(t *testing.T) {
			outer := newCommitFixture(t, map[string]string{"owned.go": fixturePackage + "const Owned = 1\n"}, false)
			outer.write(t, "owned.go", fixturePackage+"const Owned = 2\n")
			outer.git(t, "add", "owned.go")
			index := filepath.Join(outer.dir, ".git", "index")
			before, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("GIT_INDEX_FILE", index)
			if repositoryEnv {
				t.Setenv("GIT_DIR", filepath.Join(outer.dir, ".git"))
				t.Setenv("GIT_WORK_TREE", outer.dir)
			}
			inner := newCommitFixture(t, map[string]string{"foreign.go": fixturePackage + "const Foreign = 3\n"}, false)
			after, err := os.ReadFile(index)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("fixture changed caller index: %v", err)
			}
			root := strings.TrimSpace(string(inner.git(t, "rev-parse", "--show-toplevel")))
			gotRoot, err := os.Stat(filepath.FromSlash(root))
			if err != nil {
				t.Fatal(err)
			}
			wantRoot, err := os.Stat(inner.dir)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(gotRoot, wantRoot) {
				t.Fatalf("fixture commands selected caller repository %q", root)
			}
		})
	}
}

func TestPreCommitOwnsStagedContents(t *testing.T) {
	for _, tc := range []struct {
		name, unstaged, lintExit string
		unformatted              bool
	}{
		{"fully staged", "", "0", false},
		{"partial staging", "const Unstaged=2\n", "0", false},
		{"broken working edit", "func broken(\n", "0", false},
		{"downstream failure", "const Unstaged=2\n", "3", false},
		{"needs formatting", "", "0", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{"sample.go": fixturePackage + "const Before = 0\n"}, false)
			staged := fixturePackage + "const Staged = 1\n"
			if tc.unformatted {
				staged = strings.Replace(staged, "Staged = 1", "Staged=1", 1)
			}
			f.write(t, "sample.go", staged)
			f.git(t, "add", "sample.go")
			working := staged + tc.unstaged
			f.write(t, "sample.go", working)
			before := f.git(t, "show", ":sample.go")
			out, err := f.hook("LINT_FIXTURE_EXIT=" + tc.lintExit)
			if (err != nil) != (tc.lintExit != "0" || tc.unformatted) {
				t.Errorf("hook err=%v; output=%s", err, out)
			}
			want := string(before)
			if got := string(f.git(t, "show", ":sample.go")); got != want {
				t.Errorf("index contains %q, want only selected contents %q", got, want)
			}
			data, err := os.ReadFile(filepath.Join(f.dir, "sample.go"))
			if err != nil || string(data) != working {
				t.Errorf("working file changed: got %q, err=%v; want %q", data, err, working)
			}
		})
	}
}

func TestPreCommitKeepsPathIdentity(t *testing.T) {
	for _, name := range []string{"sample.go", "space name.go", "café.go", "tab\tname.go", "line\nname.go"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{"base.go": fixturePackage + "const Before = 0\n"}, false)
			source := formattedGo(t, fixturePackage+"func Value( ) int{return 7}\n")
			if err := os.WriteFile(filepath.Join(f.dir, name), []byte(source), 0o600); err != nil {
				if strings.ContainsAny(name, "\t\n") {
					t.Skipf("fixture filesystem cannot represent this filename: %v", err)
				}
				t.Fatal(err)
			}
			f.git(t, "config", "core.quotePath", "true")
			f.git(t, "add", "--", name)
			if out, err := f.hook(); err != nil {
				t.Fatalf("hook failed for exact filename %q: %v: %s", name, err, out)
			}
			if got := f.git(t, "show", ":"+name); string(got) != formattedGo(t, source) {
				t.Fatalf("selected filename %q lost contents: %q", name, got)
			}
		})
	}
}

func TestPreCommitIncludesEditedRename(t *testing.T) {
	var body strings.Builder
	body.WriteString(fixturePackage)
	for n := range 12 {
		fmt.Fprintf(&body, "func Function%d() int { return %d }\n", n, n)
	}
	initial := formattedGo(t, body.String())
	for _, tc := range []struct {
		name, source string
		wantError    bool
	}{
		{"pure rename", initial, false},
		{"formatted edit", strings.Replace(initial, "return 5", "return 50", 1), false},
		{"unformatted edit", strings.Replace(initial, "Function5()", "Function5( )", 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{"old.go": initial}, false)
			f.git(t, "config", "diff.renames", "true")
			f.git(t, "mv", "old.go", "new.go")
			f.write(t, "new.go", tc.source)
			f.git(t, "add", "--all")
			if status := string(f.git(t, "diff", "--cached", "--name-status")); !strings.HasPrefix(status, "R") {
				t.Fatalf("fixture did not exercise Git rename classification: %s", status)
			}
			out, err := f.hook()
			if (err != nil) != tc.wantError {
				t.Errorf("rename hook err=%v, wantError=%v: %s", err, tc.wantError, out)
			}
			if got := f.git(t, "show", ":new.go"); string(got) != tc.source {
				t.Fatalf("renamed destination changed: %s", got)
			}
			_, lintErr := os.Stat(f.calls)
			if tc.wantError && !os.IsNotExist(lintErr) || !tc.wantError && lintErr != nil {
				t.Fatalf("rename lint invocation differs: wantError=%v, lintStat=%v", tc.wantError, lintErr)
			}
		})
	}
}

func TestPreCommitPreservesFileModes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stagedLink bool
	}{
		{"staged external link", true},
		{"unstaged link replacement", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{"base.go": fixturePackage + "const Before = 0\n"}, false)
			target := filepath.Join(f.parent, "external.go")
			outside := fixturePackage + "func External( ) int{return 9}\n"
			if err := os.WriteFile(target, []byte(outside), 0o600); err != nil {
				t.Fatal(err)
			}
			selected := filepath.Join(f.dir, "selected.go")
			staged := fixturePackage + "const Selected = 1\n"
			if !tc.stagedLink {
				f.write(t, "selected.go", staged)
				f.git(t, "add", "selected.go")
				if err := os.Remove(selected); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink("../external.go", selected); err != nil {
				t.Skipf("fixture symlink not available: %v", err)
			}
			beforeLink, err := os.Readlink(selected)
			if err != nil {
				t.Fatal(err)
			}
			if tc.stagedLink {
				f.git(t, "add", "selected.go")
			}
			before := f.git(t, "ls-files", "--stage", "selected.go")
			out, err := f.hook()
			if (err != nil) != tc.stagedLink {
				t.Errorf("hook err=%v for stagedLink=%v: %s", err, tc.stagedLink, out)
			}
			if after := f.git(t, "ls-files", "--stage", "selected.go"); !bytes.Equal(after, before) {
				t.Errorf("staged mode/content changed: before=%s after=%s", before, after)
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != outside {
				t.Errorf("untracked external target changed: %q, %v", data, err)
			}
			if link, err := os.Readlink(selected); err != nil || link != beforeLink {
				t.Errorf("working link changed: %q, %v", link, err)
			}
		})
	}
	t.Run("executable mode", func(t *testing.T) {
		f := newCommitFixture(t, map[string]string{"base.go": fixturePackage + "const Before = 0\n"}, false)
		f.write(t, "selected.go", fixturePackage+"const Selected = 1\n")
		f.git(t, "add", "selected.go")
		f.git(t, "update-index", "--chmod=+x", "selected.go")
		before := f.git(t, "ls-files", "--stage", "selected.go")
		if !bytes.HasPrefix(before, []byte("100755 ")) {
			t.Fatalf("executable index fixture not established: %s", before)
		}
		if out, err := f.hook(); err != nil {
			t.Errorf("executable mode rejected: %v: %s", err, out)
		}
		if after := f.git(t, "ls-files", "--stage", "selected.go"); !bytes.Equal(after, before) {
			t.Errorf("executable staged mode changed: %s -> %s", before, after)
		}
	})
}

func TestPreCommitSelectionFailureBlocksCommit(t *testing.T) {
	for _, brokenOrder := range []bool{false, true} {
		t.Run(fmt.Sprint(brokenOrder), func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{"base.go": fixturePackage + "const Before = 0\n"}, false)
			f.write(t, "bad.go", "package fixture\nfunc broken(\n")
			f.git(t, "add", "bad.go")
			head := bytes.TrimSpace(f.git(t, "rev-parse", "HEAD"))
			if brokenOrder {
				// The initial-commit path does not have Git's independent HEAD
				// diff failure to hide a falsely successful pre-commit hook.
				f.git(t, "update-ref", "-d", "HEAD")
				head = nil
				f.git(t, "config", "diff.orderFile", filepath.ToSlash(filepath.Join(f.parent, "missing-order-file")))
			}
			if out, err := f.hook(); err == nil {
				t.Errorf("direct hook accepted malformed/failed selection: %s", out)
			}
			f.git(t, "config", "core.hooksPath", ".githooks")
			cmd := exec.Command("git", "commit", "--quiet", "-m", "test: malformed selection fixture")
			cmd.Dir, cmd.Env = f.dir, f.env
			out, err := cmd.CombinedOutput()
			t.Logf("actual commit err=%v output=%s", err, out)
			if err == nil {
				t.Errorf("actual commit accepted unchecked malformed source: %s", out)
			}
			afterCmd := exec.Command("git", "rev-parse", "--verify", "HEAD")
			afterCmd.Dir, afterCmd.Env = f.dir, f.env
			after, afterErr := afterCmd.Output()
			if len(head) == 0 && afterErr == nil || len(head) != 0 && (afterErr != nil || !bytes.Equal(bytes.TrimSpace(after), head)) {
				t.Errorf("rejected commit changed HEAD from %s to %s, err=%v", head, after, afterErr)
			}
			if _, err := os.Stat(f.calls); !os.IsNotExist(err) {
				t.Fatalf("failed selection/formatting reached lint: %v", err)
			}
		})
	}
}
