package protocol_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreCommitActualCommitOwnsSnapshot(t *testing.T) {
	f := newCommitFixture(t, map[string]string{"sample.go": fixturePackage + "const Before = 0\n"}, false)
	selected := fixturePackage + "const Selected = 1\n"
	f.write(t, "sample.go", selected)
	f.git(t, "add", "sample.go")
	working := selected + "func broken(\n"
	f.write(t, "sample.go", working)
	f.git(t, "config", "core.hooksPath", ".githooks")
	f.git(t, "commit", "--quiet", "-m", "test: commit selected source")
	if got := string(f.git(t, "show", "HEAD:sample.go")); got != selected {
		t.Fatalf("actual commit contains unintended source: %q", got)
	}
	if got, err := os.ReadFile(filepath.Join(f.dir, "sample.go")); err != nil || string(got) != working {
		t.Fatalf("actual commit changed working source: %q, %v", got, err)
	}
}

func TestPreCommitOnlyCommitUsesGitIndex(t *testing.T) {
	f := newCommitFixture(t, map[string]string{
		"sample.go": fixturePackage + "const Before = 0\n",
		"other.go":  fixturePackage + "const Other = 0\n",
	}, false)
	selected := fixturePackage + "const Selected = 1\n"
	f.write(t, "sample.go", selected)
	other := "package fixture\nfunc broken(\n"
	f.write(t, "other.go", other)
	f.git(t, "add", "other.go")
	f.git(t, "config", "core.hooksPath", ".githooks")
	f.git(t, "commit", "--quiet", "--only", "-m", "test: selected path commit", "--", "sample.go")
	if got := string(f.git(t, "show", "HEAD:sample.go")); got != selected {
		t.Fatalf("--only commit lost selected working source: %q", got)
	}
	if got := string(f.git(t, "show", ":other.go")); got != other {
		t.Fatalf("unselected staged file changed: %q", got)
	}
	if got := string(f.git(t, "show", "HEAD:other.go")); got != fixturePackage+"const Other = 0\n" {
		t.Fatalf("other staged changes entered --only commit: %q", got)
	}
}

func TestPreCommitEffectiveIndex(t *testing.T) {
	for _, kind := range []string{"alternate index", "linked worktree", "split index"} {
		t.Run(kind, func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{"sample.go": fixturePackage + "const Before = 0\n"}, false)
			originalDir := f.dir
			switch kind {
			case "alternate index":
				index := filepath.Join(f.parent, "alternate index")
				f.env = append(f.env, "GIT_INDEX_FILE="+filepath.ToSlash(index))
				f.git(t, "read-tree", "HEAD")
			case "linked worktree":
				linked := filepath.Join(f.parent, "linked")
				f.git(t, "worktree", "add", "--quiet", "-b", "linked", linked, "HEAD")
				f.dir = linked
			case "split index":
				f.git(t, "update-index", "--split-index")
			}
			selected := fixturePackage + "const Selected = 1\n"
			f.write(t, "sample.go", selected)
			f.git(t, "add", "sample.go")
			working := selected + "func broken(\n"
			f.write(t, "sample.go", working)
			before := f.git(t, "ls-files", "--stage", "-z")
			if out, err := f.hook(); err != nil {
				t.Fatalf("effective %s was not validated: %v: %s", kind, err, out)
			}
			if after := f.git(t, "ls-files", "--stage", "-z"); !bytes.Equal(after, before) {
				t.Fatal("hook modified effective staged entries")
			}
			if got, err := os.ReadFile(filepath.Join(f.dir, "sample.go")); err != nil || string(got) != working {
				t.Fatalf("hook changed effective working file: %q, %v", got, err)
			}
			if kind != "split index" {
				cmd := exec.Command("git", "show", ":sample.go")
				cmd.Dir = originalDir
				out, err := cmd.Output()
				if err != nil || string(out) != fixturePackage+"const Before = 0\n" {
					t.Fatalf("other worktree/index changed: %q, %v", out, err)
				}
			}
		})
	}
}

func TestPreCommitEmptyAndDeletion(t *testing.T) {
	for _, change := range []string{"none", "documentation", "delete Go"} {
		t.Run(change, func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{"base.go": fixturePackage + "const Before = 0\n", "delete.go": fixturePackage + "const Deleted = 1\n"}, false)
			switch change {
			case "documentation":
				f.write(t, "README.md", "selected documentation\n")
				f.git(t, "add", "README.md")
			case "delete Go":
				f.git(t, "rm", "delete.go")
			}
			before := f.git(t, "ls-files", "--stage", "-z")
			if out, err := f.hook(); err != nil {
				t.Fatalf("%s rejected: %v: %s", change, err, out)
			}
			if after := f.git(t, "ls-files", "--stage", "-z"); !bytes.Equal(after, before) {
				t.Fatal("hook changed staged entries")
			}
			_, err := os.Stat(f.calls)
			if change == "delete Go" && err != nil || change != "delete Go" && !os.IsNotExist(err) {
				t.Fatalf("incorrect lint admission for %s: %v", change, err)
			}
		})
	}
}

func TestPreCommitUnbornIndex(t *testing.T) {
	f := newCommitFixture(t, map[string]string{"sample.go": fixturePackage + "const Before = 0\n"}, false)
	f.git(t, "update-ref", "-d", "HEAD")
	f.git(t, "config", "core.hooksPath", ".githooks")
	f.git(t, "commit", "--quiet", "-m", "test: initial staged snapshot")
	if got := string(f.git(t, "show", "HEAD:sample.go")); !strings.Contains(got, "const Before = 0") {
		t.Fatalf("initial commit lost selected source: %q", got)
	}
}
