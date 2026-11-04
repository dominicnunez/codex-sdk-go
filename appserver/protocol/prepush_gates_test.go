package protocol_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrePushRequiredGates(t *testing.T) {
	for _, lane := range []string{"ordinary", "race", "lint", "tidy"} {
		t.Run(lane, func(t *testing.T) {
			f := newPushFixture(t)
			switch lane {
			case "ordinary", "race":
				source := "package fixture\nimport \"testing\"\nfunc TestGate(t *testing.T) { t.Fatal(\"outgoing " + lane + " gate\") }\n"
				if lane == "race" {
					source = "//go:build race\n\n" + source
				}
				f.write(t, "gate_test.go", source)
			case "lint":
				f.env = append(f.env, "LINT_FIXTURE_EXIT=3")
			case "tidy":
				f.write(t, "go.mod", "module github.com/dominicnunez/codex-sdk-go\n\ngo 1.27.3\n\nrequire example.invalid/unused v0.0.0\n\nreplace example.invalid/unused => ./unused\n")
				f.write(t, "unused/go.mod", "module example.invalid/unused\n\ngo 1.27.3\n")
			}
			f.git(t, "add", "--all")
			f.git(t, "commit", "--quiet", "--allow-empty", "-m", "test: outgoing gate failure")
			f.install(t)
			out, err := f.push("main:refs/heads/selected")
			if err == nil {
				t.Fatalf("%s gate failure published tip: %s", lane, out)
			}
			if lane == "ordinary" || lane == "race" {
				if !strings.Contains(string(out), "outgoing "+lane+" gate") {
					t.Fatalf("wrong failure boundary for %s: %s", lane, out)
				}
			}
			if lane == "tidy" && !strings.Contains(string(out), "diff current/go.mod tidy/go.mod") {
				t.Fatalf("tidiness was not the failing consumer: %s", out)
			}
			if _, err := f.remoteCommand("rev-parse", "--verify", "refs/heads/selected"); err == nil {
				t.Fatal("failed gate published remote ref")
			}
		})
	}
}

func TestPrePushRealLint(t *testing.T) {
	requireHookLinter(t)
	for _, state := range []string{"working source fix", "working lint override"} {
		t.Run(state, func(t *testing.T) {
			f := pushFixtureWithLint(t, true)
			bad := fixturePackage + "import \"os\"\n\nfunc Check() { os.Chdir(\".\") }\n"
			f.write(t, "sample.go", formattedGo(t, bad))
			f.git(t, "add", "sample.go")
			f.git(t, "commit", "--quiet", "-m", "test: outgoing errcheck finding")
			if state == "working source fix" {
				f.write(t, "sample.go", formattedGo(t, strings.Replace(bad, "os.Chdir", "_ = os.Chdir", 1)))
			} else {
				f.write(t, ".golangci.yml", "version: \"2\"\nlinters:\n  default: none\n  enable: [asciicheck]\n")
			}
			f.install(t)
			out, err := f.push("main:refs/heads/selected")
			if err == nil || !strings.Contains(string(out), "os.Chdir") {
				t.Fatalf("outgoing lint finding masked by %s: %v: %s", state, err, out)
			}
			if _, err := f.remoteCommand("rev-parse", "--verify", "refs/heads/selected"); err == nil {
				t.Fatal("lint failure published remote ref")
			}
		})
	}
}

func TestPrePushProtocolAndImmutableSelection(t *testing.T) {
	for _, state := range []string{"malformed", "missing object", "immutable old tip", "empty"} {
		t.Run(state, func(t *testing.T) {
			f := newPushFixture(t)
			old := strings.TrimSpace(string(f.git(t, "rev-parse", "HEAD")))
			input := "refs/heads/main " + old + " refs/heads/remote " + strings.Repeat("0", len(old)) + "\n"
			wantError := false
			switch state {
			case "malformed":
				input += "malformed trailing record\n"
				wantError = true
			case "missing object":
				input = strings.Replace(input, old, strings.Repeat("f", len(old)), 1)
				wantError = true
			case "immutable old tip":
				f.write(t, "sample.go", "package fixture\nfunc Broken( {\n")
				f.git(t, "add", "sample.go")
				f.git(t, "commit", "--quiet", "-m", "test: move mutable branch past captured tip")
			case "empty":
				input = ""
			}
			cmd := exec.Command(f.bash, filepath.ToSlash(filepath.Join(f.dir, ".githooks", "pre-push")))
			cmd.Dir, cmd.Env, cmd.Stdin = f.dir, f.env, strings.NewReader(input)
			out, err := cmd.CombinedOutput()
			if (err != nil) != wantError {
				t.Fatalf("%s protocol outcome: %v: %s", state, err, out)
			}
			if state != "immutable old tip" {
				if _, err := os.Stat(f.calls); !os.IsNotExist(err) {
					t.Fatalf("invalid/empty selection ran lint: %v", err)
				}
			}
		})
	}
}
