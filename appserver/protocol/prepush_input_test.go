package protocol_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrePushOwnsInputBeforeBootstrap(t *testing.T) {
	for _, state := range []string{"bootstrap reads stdin", "capture fails"} {
		t.Run(state, func(t *testing.T) {
			f := newPushFixture(t)
			f.write(t, "sample.go", "package fixture\nfunc Broken( {\n")
			f.git(t, "add", "sample.go")
			f.git(t, "commit", "--quiet", "-m", "test: bad outgoing source")
			f.write(t, "sample.go", fixturePackage+"func Value() int { return 1 }\n")
			realGo, err := exec.LookPath("go")
			if err != nil {
				t.Fatal(err)
			}
			const fakeGo = `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$HOOK_GO_CALLS"
if [ "$1" = -C ]; then cd -- "$2"; shift 2; fi
if [ "$1" = build ]; then cat > "$HOOK_BUILD_INPUT"; fi
exec "$HOOK_REAL_GO" "$@"
`
			if err := os.WriteFile(filepath.Join(f.parent, "bin", "go"), []byte(fakeGo), 0o700); err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(f.parent, "build-input")
			calls := filepath.Join(f.parent, "go-calls")
			f.env = append(f.env, "HOOK_REAL_GO="+filepath.ToSlash(realGo), "HOOK_BUILD_INPUT="+filepath.ToSlash(input), "HOOK_GO_CALLS="+filepath.ToSlash(calls))
			if state == "capture fails" {
				if err := os.WriteFile(filepath.Join(f.parent, "bin", "cat"), []byte("#!/usr/bin/env bash\necho 'capture fixture failed' >&2\nexit 7\n"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			f.install(t)
			out, err := f.push("main:refs/heads/selected")
			if err == nil {
				t.Fatalf("%s allowed bad outgoing source: %s", state, out)
			}
			if state == "capture fails" {
				if !strings.Contains(string(out), "capture fixture failed") {
					t.Fatalf("capture was not the failure owner: %s", out)
				}
				if _, err := os.Stat(calls); !os.IsNotExist(err) {
					t.Fatalf("failed capture reached bootstrap: %v", err)
				}
			} else {
				data, err := os.ReadFile(input)
				if err != nil || len(data) != 0 {
					t.Fatalf("bootstrap could consume ref records: %q, %v", data, err)
				}
				if !strings.Contains(string(out), "syntax error") {
					t.Fatalf("captured outgoing source was not checked: %s", out)
				}
			}
			if _, err := f.remoteCommand("rev-parse", "--verify", "refs/heads/selected"); err == nil {
				t.Fatal("input failure published bad remote ref")
			}
		})
	}
}

func TestPrePushNestedGitEnvironment(t *testing.T) {
	f := newPushFixture(t)
	const source = `package fixture
import (
 "os"
 "os/exec"
 "path/filepath"
 "testing"
)
func TestNestedGit(t *testing.T) {
 for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
  if os.Getenv(name) != "" { t.Fatalf("inherited source repository context: %s", name) }
 }
 root := t.TempDir()
 if out, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil { t.Fatalf("nested Git: %v: %s", err, out) }
 if _, err := os.Stat(filepath.Join(root, ".git", "config")); err != nil { t.Fatal(err) }
}
`
	f.write(t, "nested_test.go", source)
	f.git(t, "add", "nested_test.go")
	f.git(t, "commit", "--quiet", "-m", "test: nested Git context ownership")
	f.env = append(f.env, "GIT_DIR="+filepath.ToSlash(filepath.Join(f.dir, ".git")), "GIT_WORK_TREE="+filepath.ToSlash(f.dir), "GIT_INDEX_FILE="+filepath.ToSlash(filepath.Join(f.dir, ".git", "index")))
	f.install(t)
	if out, err := f.push("main:refs/heads/selected"); err != nil {
		t.Fatalf("source repository context leaked into owned checks: %v: %s", err, out)
	}
}

func TestPrePushExactUnicodeRef(t *testing.T) {
	f := newPushFixture(t)
	ref := "café\u00a0branch"
	f.git(t, "branch", ref)
	f.install(t)
	if out, err := f.push(ref + ":refs/heads/" + ref); err != nil {
		t.Fatalf("valid Unicode ref identity rejected: %v: %s", err, out)
	}
	if _, err := f.remoteCommand("show", "refs/heads/"+ref+":sample.go"); err != nil {
		t.Fatalf("exact Unicode ref was not published: %v", err)
	}
}

func TestPrePushIgnoresReplacementObjects(t *testing.T) {
	f := newPushFixture(t)
	goodTree := strings.TrimSpace(string(f.git(t, "rev-parse", "HEAD^{tree}")))
	f.write(t, "sample.go", "package fixture\nfunc Broken( {\n")
	f.git(t, "add", "sample.go")
	f.git(t, "commit", "--quiet", "-m", "test: bad raw outgoing object")
	bad := strings.TrimSpace(string(f.git(t, "rev-parse", "HEAD")))
	replacement := strings.TrimSpace(string(f.git(t, "commit-tree", goodTree, "-m", "test: independent valid replacement")))
	f.git(t, "replace", bad, replacement)
	f.write(t, "sample.go", fixturePackage+"func Value() int { return 1 }\n")
	f.install(t)
	out, err := f.push("main:refs/heads/selected")
	if err == nil || !strings.Contains(string(out), "syntax error") {
		t.Fatalf("replacement object masked raw outgoing source: %v: %s", err, out)
	}
	if _, err := f.remoteCommand("rev-parse", "--verify", "refs/heads/selected"); err == nil {
		t.Fatal("raw broken object was published")
	}
}
