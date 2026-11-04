package protocol_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrePushRejectsUninspectedCode(t *testing.T) {
	for _, state := range []string{"unattached tip", "mixed updates", "unstaged patch", "staged patch", "assume unchanged", "sparse file", "stale stat cache"} {
		t.Run(state, func(t *testing.T) {
			f := newPushFixture(t)
			if state == "unattached tip" {
				f.git(t, "checkout", "--quiet", "-b", "other")
			}
			f.write(t, "marker_test.go", "package fixture\nimport \"os\"\nfunc init() { _ = os.WriteFile(os.Getenv(\"HOOK_ATTACK_MARKER\"), []byte(\"executed\"), 0600) }\n")
			if state == "stale stat cache" {
				old := time.Unix(946684800, 0)
				if err := os.Chtimes(filepath.Join(f.dir, "marker_test.go"), old, old); err != nil {
					t.Fatal(err)
				}
			}
			f.git(t, "add", "marker_test.go")
			f.git(t, "commit", "--quiet", "-m", "test: controlled executable marker")
			refs := []string{"main:refs/heads/selected"}
			switch state {
			case "unattached tip":
				f.git(t, "checkout", "--quiet", "main")
				refs = []string{"other:refs/heads/selected"}
			case "mixed updates":
				f.git(t, "checkout", "--quiet", "-b", "other")
				f.git(t, "commit", "--quiet", "--allow-empty", "-m", "test: unsupported update")
				f.git(t, "checkout", "--quiet", "main")
				refs = append(refs, "other:refs/heads/other")
			default:
				f.write(t, "marker_test.go", "package fixture\n")
				switch state {
				case "stale stat cache":
					path := filepath.Join(f.dir, "marker_test.go")
					original := string(f.git(t, "show", "HEAD:marker_test.go"))
					safe := "package fixture\n//" + strings.Repeat("x", len(original)-len("package fixture\n//\n")) + "\n"
					f.write(t, "marker_test.go", safe)
					old := time.Unix(946684800, 0)
					if err := os.Chtimes(path, old, old); err != nil {
						t.Fatal(err)
					}
					f.git(t, "config", "core.trustctime", "false")
					// Control: the caller's cached index misses the changed bytes.
					f.git(t, "-c", "core.fsmonitor=false", "diff", "--quiet", "--no-ext-diff", "--no-textconv", "--")
				case "staged patch":
					f.git(t, "add", "marker_test.go")
				case "assume unchanged":
					f.git(t, "update-index", "--assume-unchanged", "marker_test.go")
				case "sparse file":
					f.git(t, "update-index", "--skip-worktree", "marker_test.go")
				}
			}
			marker := filepath.Join(f.parent, "execution-marker")
			f.env = append(f.env, "HOOK_ATTACK_MARKER="+filepath.ToSlash(marker))
			f.install(t)
			out, err := f.push(refs...)
			data, markerErr := os.ReadFile(marker)
			if err == nil || !os.IsNotExist(markerErr) {
				t.Fatalf("uninspected code executed for %s: push=%v marker=%q markerErr=%v output=%s", state, err, data, markerErr, out)
			}
			if _, err := os.Stat(f.calls); !os.IsNotExist(err) {
				t.Fatalf("unsupported selection reached lint: %v", err)
			}
			for _, ref := range []string{"selected", "other"} {
				if _, err := f.remoteCommand("rev-parse", "--verify", "refs/heads/"+ref); err == nil {
					t.Fatalf("unsupported selection published %s", ref)
				}
			}
		})
	}
}

func TestPrePushCheckoutConversions(t *testing.T) {
	f := newPushFixture(t)
	f.git(t, "config", "core.autocrlf", "true")
	f.write(t, ".gitattributes", "*.txt text eol=crlf\n")
	f.write(t, "checkout space.txt", "checkout\nconversion\n")
	f.git(t, "add", ".gitattributes", "checkout space.txt")
	f.git(t, "commit", "--quiet", "-m", "test: normalized checkout content")
	f.write(t, "checkout space.txt", "checkout\r\nconversion\r\n")
	before, err := os.ReadFile(filepath.Join(f.dir, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	f.install(t)
	out, err := f.push("main:refs/heads/selected")
	if err != nil {
		t.Fatalf("clean normalized checkout rejected: %v: %s", err, out)
	}
	if after, err := os.ReadFile(filepath.Join(f.dir, ".git", "index")); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("caller index changed: %v", err)
	}
	if got := string(f.git(t, "--git-dir="+filepath.ToSlash(f.remote), "show", "refs/heads/selected:checkout space.txt")); got != "checkout\nconversion\n" {
		t.Fatalf("published content = %q", got)
	}
}
