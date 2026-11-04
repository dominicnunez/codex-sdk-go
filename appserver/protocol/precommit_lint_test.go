package protocol_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func requireHookLinter(t *testing.T) {
	t.Helper()
	binary, err := exec.LookPath("golangci-lint")
	if err == nil {
		out, versionErr := exec.Command(binary, "version").CombinedOutput()
		if versionErr == nil {
			fields := strings.Fields(string(out))
			for n, field := range fields {
				if field == "version" && n+1 < len(fields) && strings.TrimPrefix(fields[n+1], "v") == "2.14.0" {
					return
				}
			}
		}
	}
	if os.Getenv("REQUIRE_PINNED_HOOK_LINT") == "1" {
		t.Fatal("execution-level hook coverage requires golangci-lint 2.14.0 on PATH")
	}
	t.Skip("golangci-lint 2.14.0 not installed")
}

func TestPreCommitRealLintScope(t *testing.T) {
	requireHookLinter(t)
	const good = fixturePackage + "import \"os\"\n\nfunc Check() { _ = os.Chdir(\".\") }\n"
	const bad = fixturePackage + "import \"os\"\n\nfunc Check() { os.Chdir(\".\") }\n"
	for _, tc := range []struct{ name, path, unrelated string }{
		{"clean control", "sample.go", ""},
		{"tracked documentation", "sample.go", "readme"},
		{"untracked note", "sample.go", "note"},
		{"space filename", "space name.go", "note"},
		{"Unicode filename", "café.go", "note"},
		{"forced Git colors", "sample.go", "color"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{tc.path: formattedGo(t, good)}, true)
			f.write(t, "README.md", "second baseline commit\n")
			f.git(t, "add", "README.md")
			f.git(t, "commit", "--quiet", "-m", "test: second lint baseline")
			selected := formattedGo(t, bad)
			f.write(t, tc.path, selected)
			f.git(t, "add", "--", tc.path)
			switch tc.unrelated {
			case "readme":
				f.write(t, "README.md", "unrelated working documentation\n")
			case "note":
				f.write(t, "notes.txt", "untracked note\n")
			case "color":
				f.git(t, "config", "color.ui", "always")
			}
			out, err := f.hook()
			if err == nil || !strings.Contains(string(out), "os.Chdir") {
				t.Fatalf("staged errcheck finding was missed: err=%v output=%s", err, out)
			}
			if got := string(f.git(t, "show", ":"+tc.path)); got != selected {
				t.Fatalf("lint failure changed selected source: %q", got)
			}
		})
	}
	t.Run("exclude previous commit finding", func(t *testing.T) {
		f := newCommitFixture(t, map[string]string{"sample.go": formattedGo(t, good)}, true)
		f.write(t, "sample.go", formattedGo(t, bad))
		f.git(t, "add", "sample.go")
		f.git(t, "commit", "--quiet", "-m", "test: previous finding baseline")
		f.write(t, "added.go", fixturePackage+"func Added() int { return 7 }\n")
		f.git(t, "add", "added.go")
		if out, err := f.hook(); err != nil {
			t.Fatalf("already committed finding entered staged scope: %v: %s", err, out)
		}
	})
	t.Run("temporary directory alias", func(t *testing.T) {
		f := newCommitFixture(t, map[string]string{"sample.go": formattedGo(t, bad)}, true)
		physical := filepath.Join(f.parent, "physical")
		alias := filepath.Join(f.parent, "alias")
		if err := os.Mkdir(physical, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(physical, alias); err != nil {
			t.Skipf("temporary directory alias unavailable: %v", err)
		}
		f.write(t, "added.go", fixturePackage+"const Added = 1\n")
		f.git(t, "add", "added.go")
		extra := []string{"TMPDIR=" + alias, "TMP=" + alias, "TEMP=" + alias}
		if out, err := f.hook(extra...); err != nil {
			t.Fatalf("temporary directory alias included unchanged finding: %v: %s", err, out)
		}
		f.write(t, "added.go", formattedGo(t, strings.Replace(bad, "Check()", "Added()", 1)))
		f.git(t, "add", "added.go")
		if out, err := f.hook(extra...); err == nil || !strings.Contains(string(out), "os.Chdir") {
			t.Fatalf("temporary directory alias hid selected finding: %v: %s", err, out)
		}
	})
	t.Run("Git diff environment cannot widen scope", func(t *testing.T) {
		f := newCommitFixture(t, map[string]string{"sample.go": formattedGo(t, bad) + "\nconst Value = 1\n"}, true)
		f.write(t, "sample.go", formattedGo(t, bad)+"\nconst Value = 2\n")
		f.git(t, "add", "sample.go")
		if out, err := f.hook("GIT_DIFF_OPTS=--unified=100"); err != nil {
			t.Fatalf("Git environment included unchanged finding: %v: %s", err, out)
		}
	})
	for _, persisted := range []bool{false, true} {
		t.Run("external Go module flags persisted="+strconv.FormatBool(persisted), func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{"sample.go": formattedGo(t, good)}, true)
			f.write(t, "added.go", fixturePackage+"const Added = 1\n")
			f.git(t, "add", "added.go")
			external := filepath.Join(f.parent, "external.mod")
			if err := os.WriteFile(external, []byte("invalid external module\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			extra := []string{"GOFLAGS=-modfile=" + filepath.ToSlash(external)}
			if persisted {
				config := filepath.Join(f.parent, "go-env")
				if err := os.WriteFile(config, []byte(extra[0]+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				extra = []string{"GOENV=" + config, "GOFLAGS="}
			}
			if out, err := f.hook(extra...); err != nil {
				t.Fatalf("external Go flags replaced staged module: %v: %s", err, out)
			}
		})
	}
	t.Run("Go overlay cannot replace checker", func(t *testing.T) {
		f := newCommitFixture(t, map[string]string{"sample.go": formattedGo(t, good)}, true)
		f.write(t, "sample.go", formattedGo(t, bad))
		f.git(t, "add", "sample.go")
		replacement := filepath.Join(f.parent, "replacement.go")
		if err := os.WriteFile(replacement, []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{filepath.Join(f.dir, "scripts", "hooks", "check.go"): replacement}})
		if err != nil {
			t.Fatal(err)
		}
		overlayPath := filepath.Join(f.parent, "overlay.json")
		if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := f.hook("GOFLAGS=-overlay=" + filepath.ToSlash(overlayPath))
		if err == nil || !strings.Contains(string(out), "os.Chdir") {
			t.Fatalf("overlay hid selected finding: %v: %s", err, out)
		}
	})
	t.Run("old findings cannot consume report quota", func(t *testing.T) {
		f := newCommitFixture(t, map[string]string{"a_old.go": formattedGo(t, good)}, true)
		var old strings.Builder
		old.WriteString(fixturePackage + "import \"os\"\n\n")
		for _, name := range []string{"One", "Two", "Three", "Four"} {
			old.WriteString("func " + name + "() { os.Chdir(\".\") }\n")
		}
		f.write(t, "a_old.go", formattedGo(t, old.String()))
		f.git(t, "add", "a_old.go")
		f.git(t, "commit", "--quiet", "-m", "test: repeated previous findings")
		f.write(t, "z_new.go", formattedGo(t, strings.Replace(bad, "Check()", "NewCheck()", 1)))
		f.git(t, "add", "z_new.go")
		out, err := f.hook()
		if err == nil || !strings.Contains(string(out), "z_new.go") || strings.Contains(string(out), "a_old.go") {
			t.Fatalf("report was not scoped to the staged finding: err=%v output=%s", err, out)
		}
	})
	for _, state := range []string{"untracked Go", "working Go", "working module", "working lint config"} {
		t.Run(state, func(t *testing.T) {
			f := newCommitFixture(t, map[string]string{"sample.go": formattedGo(t, good)}, true)
			f.write(t, "README.md", "second baseline commit\n")
			f.git(t, "add", "README.md")
			f.git(t, "commit", "--quiet", "-m", "test: clean staged baseline")
			selected := fixturePackage + "func Added() int { return 7 }\n"
			f.write(t, "added.go", selected)
			f.git(t, "add", "added.go")
			switch state {
			case "untracked Go":
				f.write(t, "untracked.go", "package fixture\nfunc broken(\n")
			case "working Go":
				f.write(t, "sample.go", "package fixture\nfunc broken(\n")
			case "working module":
				f.write(t, "go.mod", "not a valid module file\n")
			case "working lint config":
				f.write(t, ".golangci.yml", "invalid: [\n")
			}
			before, err := os.ReadFile(filepath.Join(f.dir, "added.go"))
			if err != nil {
				t.Fatal(err)
			}
			if out, err := f.hook(); err != nil {
				t.Fatalf("unselected %s changed the staged check: %v: %s", state, err, out)
			}
			if got := f.git(t, "show", ":added.go"); string(got) != string(before) {
				t.Fatalf("selected addition changed: %s", got)
			}
		})
	}
}
