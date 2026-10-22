package protocol_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHookRunnerRequiresPinnedVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell-hook fixtures")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	cp, err := exec.LookPath("cp")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, pathVersion, installedVersion, newVersion                         string
		goPath, multiGoPath, nix, noNix, installFails, wantInstall, wantFailure bool
	}{
		{name: "pinned PATH", pathVersion: "2.14.0"},
		{name: "pinned PATH with v", pathVersion: "v2.14.0"},
		{name: "pinned PATH without Nix", pathVersion: "2.14.0", noNix: true},
		{name: "old PATH", pathVersion: "2.11.3", newVersion: "2.14.0", wantInstall: true},
		{name: "newer PATH", pathVersion: "2.15.0", newVersion: "2.14.0", wantInstall: true},
		{name: "prerelease PATH", pathVersion: "2.14.0-dev", newVersion: "2.14.0", wantInstall: true},
		{name: "pinned installed", installedVersion: "2.14.0"},
		{name: "old GOBIN", installedVersion: "2.11.3", newVersion: "2.14.0", wantInstall: true},
		{name: "old GOPATH", installedVersion: "2.11.3", newVersion: "2.14.0", goPath: true, wantInstall: true},
		{name: "pinned multi GOPATH", installedVersion: "2.14.0", goPath: true, multiGoPath: true},
		{name: "install multi GOPATH", newVersion: "2.14.0", goPath: true, multiGoPath: true, wantInstall: true},
		{name: "old Nix PATH", pathVersion: "2.11.3", newVersion: "2.14.0", nix: true, wantInstall: true},
		{name: "wrong installation", newVersion: "2.11.3", wantInstall: true, wantFailure: true},
		{name: "failed installation", installFails: true, wantInstall: true, wantFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			bin, installed := filepath.Join(root, "path"), filepath.Join(root, "gopath", "bin")
			for _, dir := range []string{bin, installed} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(git, filepath.Join(bin, "git")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(bash, filepath.Join(bin, "bash")); err != nil {
				t.Fatal(err)
			}
			write := func(path, body string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			lint := func(path, version, label string) {
				t.Helper()
				write(path, fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = version ]; then printf '%%s\\n' 'golangci-lint has version %s built with go1.27.0'; else printf '%%s\\n' '%s' > \"$TEST_RUN_LOG\"; fi\n", version, label))
			}
			if tc.pathVersion != "" {
				lint(filepath.Join(bin, "golangci-lint"), tc.pathVersion, "PATH")
			}
			if tc.installedVersion != "" {
				lint(filepath.Join(installed, "golangci-lint"), tc.installedVersion, "installed")
			}
			fresh := filepath.Join(root, "fresh-lint")
			lint(fresh, tc.newVersion, "fresh")
			if tc.nix {
				write(filepath.Join(bin, "nix"), `#!/bin/sh
if [ "$1" != develop ] || [ "$3" != --command ]; then exit 99; fi
shift 3
IN_NIX_SHELL=1
export IN_NIX_SHELL
exec "$@"
`)
			}
			write(filepath.Join(bin, "go"), `#!/bin/sh
case "$*" in
  'env GOBIN') if [ "$TEST_EMPTY_GOBIN" != 1 ]; then printf '%s\n' "$TEST_GOBIN"; fi ;;
  'env GOPATH') printf '%s\n' "$TEST_GOPATH" ;;
  'env GOHOSTOS') printf '%s\n' linux ;;
  'install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0')
    printf '%s\n' "$*" > "$TEST_INSTALL_LOG"
    if [ "$TEST_INSTALL_FAIL" = 1 ]; then exit 1; fi
    "$TEST_COPY" "$TEST_FRESH_LINT" "$TEST_GOBIN/golangci-lint" ;;
  *) printf 'unexpected go invocation: %s\n' "$*" >&2; exit 99 ;;
esac
`)
			runLog, installLog := filepath.Join(root, "run.log"), filepath.Join(root, "install.log")
			cmd := exec.Command(bash, repoPath(t, "scripts", "hooks", "run.sh"), "golangci-lint", "run", "./...")
			cmd.Dir = repoRoot(t)
			nixShell := "1"
			if tc.nix || tc.noNix {
				nixShell = ""
			}
			goPath := filepath.Dir(installed)
			if tc.multiGoPath {
				goPath += string(os.PathListSeparator) + filepath.Join(root, "second-gopath")
			}
			cmd.Env = append(os.Environ(), "PATH="+bin, "IN_NIX_SHELL="+nixShell, "TEST_RUN_LOG="+runLog, "TEST_INSTALL_LOG="+installLog, "TEST_GOBIN="+installed, "TEST_GOPATH="+goPath, "TEST_FRESH_LINT="+fresh, "TEST_COPY="+cp, fmt.Sprintf("TEST_EMPTY_GOBIN=%d", boolInt(tc.goPath)), fmt.Sprintf("TEST_INSTALL_FAIL=%d", boolInt(tc.installFails)))
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantFailure {
				t.Fatalf("error=%v output=%s", err, output)
			}
			_, installErr := os.Stat(installLog)
			if (installErr == nil) != tc.wantInstall {
				t.Fatalf("installation=%v want=%v output=%s", installErr == nil, tc.wantInstall, output)
			}
			gotRun, runErr := os.ReadFile(runLog)
			if tc.wantFailure {
				if runErr == nil {
					t.Fatalf("executed rejected binary: %s", gotRun)
				}
				return
			}
			want := "installed"
			if tc.wantInstall {
				want = "fresh"
			} else if tc.pathVersion != "" {
				want = "PATH"
			}
			if runErr != nil || strings.TrimSpace(string(gotRun)) != want {
				t.Fatalf("executed=%q error=%v want=%s", gotRun, runErr, want)
			}
		})
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
