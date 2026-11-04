package protocol_test

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookRunnerOwnsExplicitRoot(t *testing.T) {
	f := newCommitFixture(t, map[string]string{"sample.go": fixturePackage + "const Before = 0\n"}, false)
	root := filepath.Join(f.parent, "captured #% café")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "hooks"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"go.mod":               "module captured\n\ngo 1.27.3\n",
		"flake.nix":            "{}\n",
		"scripts/hooks/run.sh": "#!/usr/bin/env bash\necho 'wrong untrusted runner' >&2\nexit 9\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	const nix = `#!/usr/bin/env bash
set -euo pipefail
printf '%s\0' "$@" > "$HOOK_NIX_ARGUMENTS"
[[ "$1" = develop && "$3" = --command ]]
[[ "$4" -ef "$HOOK_EXPECTED_RUNNER" ]]
shift 3
IN_NIX_SHELL=1 exec "$@"
`
	if err := os.WriteFile(filepath.Join(f.parent, "bin", "nix"), []byte(nix), 0o700); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(f.parent, "nix-arguments")
	runner := filepath.Join(f.dir, "scripts", "hooks", "run.sh")
	cmd := exec.Command(f.bash, filepath.ToSlash(runner), "--repo-root", filepath.ToSlash(root), "go", "env", "GOMOD")
	cmd.Dir, cmd.Env = root, append(append([]string(nil), f.env...), "IN_NIX_SHELL=", "HOOK_NIX_ARGUMENTS="+filepath.ToSlash(log), "HOOK_EXPECTED_RUNNER="+filepath.ToSlash(runner), "GOENV=off", "GOWORK=off", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("explicit source root failed without Git metadata: %v: %s", err, out)
	}
	module, err := os.Stat(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.Stat(filepath.Join(root, "go.mod"))
	if err != nil || !os.SameFile(module, want) {
		t.Fatalf("runner used a different module: %s, %v", out, err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	if len(args) != 9 || args[0] != "develop" || args[2] != "--command" || args[4] != "--repo-root" || args[5] != filepath.ToSlash(root) {
		t.Fatalf("explicit root delegation lost arguments: %q", args)
	}
	flake, err := url.Parse(args[1])
	if err != nil || flake.Scheme != "path" || flake.RawQuery != "" || flake.Fragment != "" {
		t.Fatalf("source root was not an explicit path flake: %q, %v", args[1], err)
	}
	decoded, err := url.PathUnescape(strings.TrimPrefix(args[1], "path:"))
	if err != nil || decoded != filepath.ToSlash(root) {
		t.Fatalf("path flake lost source identity: %q, %v", decoded, err)
	}
}
