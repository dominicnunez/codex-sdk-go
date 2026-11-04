package protocol_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type pushFixture struct {
	*commitFixture
	remote string
}

func newPushFixture(t *testing.T) *pushFixture {
	return pushFixtureWithLint(t, false)
}

func pushFixtureWithLint(t *testing.T, realLint bool) *pushFixture {
	t.Helper()
	f := newCommitFixture(t, map[string]string{"sample.go": fixturePackage + "func Value() int { return 1 }\n"}, realLint)
	f.git(t, "config", "tag.gpgSign", "false")
	data, err := os.ReadFile(repoPath(t, ".githooks/pre-push"))
	if err != nil {
		t.Fatal(err)
	}
	f.write(t, ".githooks/pre-push", string(data))
	f.git(t, "add", ".githooks/pre-push")
	f.git(t, "commit", "--quiet", "-m", "test: outgoing hook baseline")
	p := &pushFixture{commitFixture: f, remote: filepath.Join(f.parent, "remote with space.git")}
	f.git(t, "init", "--bare", "--quiet", p.remote)
	f.git(t, "remote", "add", "fixture", filepath.ToSlash(p.remote))
	return p
}

func TestPrePushWorkingInputs(t *testing.T) {
	for _, state := range []string{"untracked Go", "ignored Go", "staged Go", "working module", "working workspace", "working lint config"} {
		t.Run(state, func(t *testing.T) {
			f := newPushFixture(t)
			path := "untracked.go"
			content := "package fixture\nfunc Broken( {\n"
			switch state {
			case "ignored Go":
				f.write(t, ".git/info/exclude", "untracked.go\n")
			case "working module":
				path, content = "go.mod", "invalid working module\n"
			case "working workspace":
				path, content = "go.work", "invalid working workspace\n"
			case "working lint config":
				path, content = ".golangci.yml", "invalid: [\n"
			}
			f.write(t, path, content)
			if state == "staged Go" {
				f.git(t, "add", path)
			}
			before, err := os.ReadFile(filepath.Join(f.dir, ".git", "index"))
			if err != nil {
				t.Fatal(err)
			}
			f.install(t)
			wantError := state == "staged Go" || state == "working module" || state == "working lint config"
			out, err := f.push("main:refs/heads/selected")
			if (err != nil) != wantError {
				t.Fatalf("working %s outcome: %v: %s", state, err, out)
			}
			if wantError && !strings.Contains(string(out), "tracked source differs") {
				t.Fatalf("tracked changes were not the rejection owner: %s", out)
			}
			if after, err := os.ReadFile(filepath.Join(f.dir, ".git", "index")); err != nil || !bytes.Equal(before, after) {
				t.Fatalf("outgoing check changed caller index: %v", err)
			}
			if after, err := os.ReadFile(filepath.Join(f.dir, path)); err != nil || string(after) != content {
				t.Fatalf("outgoing check changed %s: %q, %v", path, after, err)
			}
		})
	}
}

func TestPrePushPreflightsAllUpdates(t *testing.T) {
	for _, state := range []string{"distinct good", "same tree", "same commit", "second bad"} {
		t.Run(state, func(t *testing.T) {
			f := newPushFixture(t)
			f.git(t, "checkout", "--quiet", "-b", "other")
			switch state {
			case "distinct good":
				f.write(t, "sample.go", fixturePackage+"func Value() int { return 2 }\n")
			case "second bad":
				f.write(t, "sample.go", "package fixture\nfunc Broken( {\n")
			}
			if state != "same commit" {
				f.git(t, "add", "sample.go")
				f.git(t, "commit", "--quiet", "--allow-empty", "-m", "test: another outgoing tip")
			}
			f.git(t, "checkout", "--quiet", "main")
			f.install(t)
			out, err := f.push("main:refs/heads/first", "other:refs/heads/second")
			if state != "same commit" {
				if err == nil {
					t.Fatalf("unsupported second tip was published: %s", out)
				}
				for _, ref := range []string{"first", "second"} {
					if id, err := f.remoteCommand("rev-parse", "--verify", "refs/heads/"+ref); err == nil {
						t.Fatalf("failed multi-update published %s: %s", ref, id)
					}
				}
				if _, err := os.Stat(f.calls); !os.IsNotExist(err) {
					t.Fatalf("unsupported update selection ran lint: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("valid multi-update failed: %v: %s", err, out)
			}
			calls, err := os.ReadFile(f.calls)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if got := strings.Count(string(calls), "\n"); got != want {
				t.Fatalf("lint ran %d times for %s, want %d: %s", got, state, want, calls)
			}
			for _, ref := range []string{"first", "second"} {
				if _, err := f.remoteCommand("show", "refs/heads/"+ref+":sample.go"); err != nil {
					t.Fatalf("verified update %s absent: %v", ref, err)
				}
			}
		})
	}
}

func TestPrePushDeletionAndTags(t *testing.T) {
	for _, state := range []string{"deletion only", "delete with bad tip", "annotated commit tag", "blob tag"} {
		t.Run(state, func(t *testing.T) {
			f := newPushFixture(t)
			refs := []string{"main:refs/heads/selected"}
			switch state {
			case "deletion only", "delete with bad tip":
				if out, err := f.push("main:refs/heads/seed"); err != nil {
					t.Fatalf("seed remote: %v: %s", err, out)
				}
				refs = []string{":refs/heads/seed"}
				f.write(t, "sample.go", "package fixture\nfunc Broken( {\n")
				if state == "delete with bad tip" {
					f.git(t, "add", "sample.go")
					f.git(t, "commit", "--quiet", "-m", "test: invalid mixed outgoing tip")
					f.write(t, "sample.go", fixturePackage+"func Value() int { return 1 }\n")
					refs = append(refs, "main:refs/heads/selected")
				}
			case "annotated commit tag":
				f.git(t, "tag", "-a", "release", "-m", "test: commit tag")
				refs = []string{"refs/tags/release"}
			case "blob tag":
				blob := strings.TrimSpace(string(f.git(t, "rev-parse", "HEAD:sample.go")))
				f.git(t, "update-ref", "refs/tags/blob", blob)
				refs = []string{"refs/tags/blob"}
			}
			f.install(t)
			out, err := f.push(refs...)
			wantError := state == "delete with bad tip" || state == "blob tag"
			if (err != nil) != wantError {
				t.Fatalf("%s outcome: %v: %s", state, err, out)
			}
			if state == "deletion only" {
				if _, err := f.remoteCommand("rev-parse", "--verify", "refs/heads/seed"); err == nil {
					t.Fatal("deletion was not published")
				}
				if _, err := os.Stat(f.calls); !os.IsNotExist(err) {
					t.Fatalf("deletion-only push ran source checks: %v", err)
				}
			}
			if state == "delete with bad tip" {
				if _, err := f.remoteCommand("rev-parse", "--verify", "refs/heads/seed"); err != nil {
					t.Fatal("failed mixed push deleted existing remote ref")
				}
				if _, err := f.remoteCommand("rev-parse", "--verify", "refs/heads/selected"); err == nil {
					t.Fatal("failed mixed push published broken tip")
				}
			}
			if state == "annotated commit tag" {
				if _, err := f.remoteCommand("show", "release:sample.go"); err != nil {
					t.Fatalf("verified commit tag missing: %v", err)
				}
			}
			if state == "blob tag" {
				if _, err := f.remoteCommand("rev-parse", "--verify", "refs/tags/blob"); err == nil {
					t.Fatal("unsupported noncommit tag was published")
				}
			}
		})
	}
}

func (f *pushFixture) install(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.dir, ".githooks", "pre-push"))
	if err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(f.parent, "active-hooks")
	if err := os.Mkdir(hooks, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), data, 0o700); err != nil {
		t.Fatal(err)
	}
	f.git(t, "config", "core.hooksPath", filepath.ToSlash(hooks))
}

func (f *pushFixture) push(refs ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"push", "--quiet", "fixture"}, refs...)...)
	cmd.Dir, cmd.Env = f.dir, f.env
	return cmd.CombinedOutput()
}

func (f *pushFixture) remoteCommand(args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"--git-dir=" + f.remote}, args...)...)
	cmd.Dir, cmd.Env = f.dir, f.env
	return cmd.CombinedOutput()
}

func TestPrePushOwnsOutgoingRevision(t *testing.T) {
	const good = fixturePackage + "func Value() int { return 1 }\n"
	const bad = "package fixture\nfunc Broken( {\n"
	for _, kind := range []string{"clean good", "clean bad", "bad HEAD with working fix", "other bad branch", "good HEAD with broken working edit"} {
		t.Run(kind, func(t *testing.T) {
			f := newPushFixture(t)
			branch := "main"
			if kind == "other bad branch" {
				f.git(t, "checkout", "--quiet", "-b", "other")
				branch = "other"
			}
			wantError := kind != "clean good"
			badTip := kind == "clean bad" || kind == "bad HEAD with working fix" || kind == "other bad branch"
			if badTip {
				f.write(t, "sample.go", bad)
				f.git(t, "add", "sample.go")
				f.git(t, "commit", "--quiet", "-m", "test: broken outgoing source")
			}
			switch kind {
			case "bad HEAD with working fix":
				f.write(t, "sample.go", good)
			case "other bad branch":
				f.git(t, "checkout", "--quiet", "main")
			case "good HEAD with broken working edit":
				f.write(t, "sample.go", bad)
			}
			f.install(t)
			index := filepath.Join(f.dir, ".git", "index")
			beforeIndex, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			beforeWork, err := os.ReadFile(filepath.Join(f.dir, "sample.go"))
			if err != nil {
				t.Fatal(err)
			}
			out, err := f.push(branch + ":refs/heads/selected")
			if (err != nil) != wantError {
				remote, remoteErr := f.remoteCommand("show", "refs/heads/selected:sample.go")
				t.Fatalf("push outcome differs for %s: err=%v output=%s remote=%q remoteErr=%v", kind, err, out, remote, remoteErr)
			}
			if wantError {
				if ref, err := f.remoteCommand("rev-parse", "--verify", "refs/heads/selected"); err == nil {
					t.Fatalf("failed verification published remote ref: %s", ref)
				}
			} else if source, err := f.remoteCommand("show", "refs/heads/selected:sample.go"); err != nil || string(source) != good {
				t.Fatalf("successful push did not publish verified source: %q, %v", source, err)
			}
			if after, err := os.ReadFile(index); err != nil || !bytes.Equal(after, beforeIndex) {
				t.Fatalf("push changed caller index: %v", err)
			}
			if after, err := os.ReadFile(filepath.Join(f.dir, "sample.go")); err != nil || !bytes.Equal(after, beforeWork) {
				t.Fatalf("push changed caller working source: %v", err)
			}
		})
	}
}
