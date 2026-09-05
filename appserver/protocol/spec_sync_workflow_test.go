package protocol_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpecSyncPrunesOnlySupersededBotPRs(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	for _, tc := range []struct {
		name      string
		apiExit   string
		closeExit string
		wantError bool
	}{
		{name: "eligible PRs only", apiExit: "0", closeExit: "0"},
		{name: "listing failure", apiExit: "1", closeExit: "0", wantError: true},
		{name: "closing failure", apiExit: "0", closeExit: "1", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fixture := filepath.Join(dir, "prs.tsv")
			calls := filepath.Join(dir, "calls.txt")
			// These are the REST fields projected by the workflow's --jq expression.
			rows := "54\tgithub-actions[bot]\tspec-sync/current\ttrue\n" +
				"53\tgithub-actions[bot]\tspec-sync/old\ttrue\n" +
				"52\tcontributor\tspec-sync/manual\ttrue\n" +
				"51\tgithub-actions[bot]\tfix/unrelated\ttrue\n" +
				"50\tgithub-actions[bot]\tspec-sync/fork\tfalse\n" +
				"49\tother[bot]\tspec-sync/other\ttrue\n"
			if err := os.WriteFile(fixture, []byte(rows), 0o600); err != nil {
				t.Fatal(err)
			}
			const harness = `
gh() {
  if [ "$1" = api ]; then
    [ "$2" = --paginate ] || return 2
    [ "$3" = 'repos/owner/repo/pulls?state=open&per_page=100' ] || return 2
    [ "$4" = --jq ] || return 2
    [ "$5" = '.[] | [.number, .user.login, .head.ref, (.head.repo.full_name == .base.repo.full_name)] | @tsv' ] || return 2
    [ "$API_EXIT" = 0 ] || return "$API_EXIT"
    cat "$SPEC_FIXTURE"
  elif [ "$1" = pr ]; then
    printf '%s\n' "$*" >> "$SPEC_CALLS"
    return "$CLOSE_EXIT"
  else
    return 2
  fi
}
source "$1" 54 abc123
`
			cmd := exec.Command(bash, "-c", harness, "test", filepath.ToSlash(repoPath(t, "scripts", "spec-sync-prune.sh")))
			cmd.Env = append(os.Environ(), "GITHUB_REPOSITORY=owner/repo",
				"SPEC_FIXTURE="+filepath.ToSlash(fixture), "SPEC_CALLS="+filepath.ToSlash(calls),
				"API_EXIT="+tc.apiExit, "CLOSE_EXIT="+tc.closeExit)
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantError {
				t.Fatalf("err = %v, output = %s", err, output)
			}
			data, err := os.ReadFile(calls)
			if tc.apiExit != "0" {
				if !os.IsNotExist(err) {
					t.Fatalf("listing failure should not close PRs: %s, %v", data, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := "pr close 53 --delete-branch --comment Superseded by newer spec-sync PR #54 for abc123."
			if strings.TrimSpace(string(data)) != want {
				t.Fatalf("unexpected mutations: %s", data)
			}
		})
	}
}
