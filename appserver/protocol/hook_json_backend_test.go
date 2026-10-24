package protocol_test

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestHookRunnerSelectsLegacyJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell-hook fixture")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("fixture git init: %v: %s", err, output)
	}
	for _, experiment := range []string{"", "jsonv2", "nojsonv2", "fieldtrack,jsonv2"} {
		t.Run(experiment, func(t *testing.T) {
			cmd := exec.Command(bash, repoPath(t, "scripts", "hooks", "run.sh"), bash, "-c", `printf '%s' "$GOEXPERIMENT"`)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "IN_NIX_SHELL=1", "GOEXPERIMENT="+experiment)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("hook runner: %v: %s", err, output)
			}
			want := "nojsonv2"
			if experiment != "" {
				want = experiment + ",nojsonv2"
			}
			if strings.TrimSpace(string(output)) != want {
				t.Fatalf("GOEXPERIMENT=%q, want %q", output, want)
			}
		})
	}
}
