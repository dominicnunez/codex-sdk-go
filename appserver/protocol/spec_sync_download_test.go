package protocol_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func specSyncWorkflowRun(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(repoPath(t, ".github", "workflows", "spec-sync.yml"))
	if err != nil {
		t.Fatal(err)
	}
	_, run, ok := strings.Cut(string(data), "        run: |\n")
	if !ok {
		t.Fatal("spec-sync run block missing")
	}
	var script strings.Builder
	for _, line := range strings.Split(run, "\n") {
		if strings.TrimSpace(line) == "" {
			script.WriteByte('\n')
			continue
		}
		line, ok = strings.CutPrefix(line, "          ")
		if !ok {
			break
		}
		script.WriteString(line + "\n")
	}
	return script.String()
}

func TestSpecSyncRejectsFailedDownloads(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	curl, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl not installed")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	const schema = `{"title":"fixture","type":"object"}`
	type testCase struct {
		name, stage, failure string
	}
	cases := []testCase{{name: "valid"}, {name: "redirect", stage: "v1", failure: "redirect"}}
	for _, stage := range []string{"root", "v1", "v2"} {
		for _, failure := range []string{"302", "404", "503", "malformed", "truncated", "inventory"} {
			cases = append(cases, testCase{name: stage + "/" + failure, stage: stage, failure: failure})
		}
	}
	for _, failure := range []string{"empty", "multiple", "nonfinite"} {
		cases = append(cases, testCase{name: failure, stage: "v2", failure: failure})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/"+tc.stage+".json" {
					switch tc.failure {
					case "302":
						w.WriteHeader(http.StatusFound)
					case "404":
						http.Error(w, "<html>not found</html>", http.StatusNotFound)
						return
					case "503":
						http.Error(w, "<html>unavailable</html>", http.StatusServiceUnavailable)
						return
					case "malformed":
						_, _ = fmt.Fprint(w, `{"title":`)
						return
					case "empty":
						return
					case "multiple":
						_, _ = fmt.Fprint(w, schema+schema)
						return
					case "nonfinite":
						_, _ = fmt.Fprint(w, `{"number":NaN}`)
						return
					case "redirect":
						http.Redirect(w, r, "/redirected.json", http.StatusFound)
						return
					case "truncated":
						w.Header().Set("Content-Length", "1000")
					}
				}
				_, _ = fmt.Fprint(w, schema)
			}))
			defer server.Close()
			dir := t.TempDir()
			snapshot := filepath.Join(dir, "appserver", "protocol", "schema", "json")
			write := func(path, content string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			baseline := filepath.Join(snapshot, ".upstream-sha")
			const oldSHA = "1111111111111111111111111111111111111111\n"
			write(baseline, oldSHA)
			write(filepath.Join(snapshot, "old.json"), schema)
			proposal := filepath.Join(dir, "proposal.txt")
			write(proposal, "open")
			prune, err := os.ReadFile(repoPath(t, "scripts", "spec-sync-prune.sh"))
			if err != nil {
				t.Fatal(err)
			}
			write(filepath.Join(dir, "scripts", "spec-sync-prune.sh"), string(prune))
			calls := filepath.Join(dir, "mutations.txt")
			temporary := filepath.Join(dir, "temporary downloads")
			if err := os.MkdirAll(temporary, 0o700); err != nil {
				t.Fatal(err)
			}
			// All external publication operations are inert fixtures. The actual
			// workflow and real curl run against a loopback-only HTTP server.
			const harness = `
gh() {
  case "$1 $2" in
    'api repos/openai/codex/commits'*) printf '%s\n' 2222222222222222222222222222222222222222 ;;
    'api repos/openai/codex/contents/'*)
      case "${2%%\?*}" in
        */json) stage=root ;;
        */json/v1) stage=v1 ;;
        */json/v2) stage=v2 ;;
        *) return 2 ;;
      esac
      if [ "$SPEC_FAILURE" = inventory ] && [ "$stage" = "$SPEC_STAGE" ]; then return 1; fi
      printf '%s/%s.json\n' "$SPEC_HTTP" "$stage"
      ;;
    'api --paginate') printf '53\tgithub-actions[bot]\tspec-sync/old\ttrue\n' ;;
    'label create') printf '%s\n' "$*" >> "$SPEC_CALLS" ;;
    'pr create') printf '%s\n' "$*" >> "$SPEC_CALLS"; printf '%s\n' https://example.invalid/pull/54 ;;
    'pr view') printf '54\n' ;;
    'pr close') printf '%s\n' "$*" >> "$SPEC_CALLS"; printf closed > "$SPEC_PROPOSAL" ;;
    *) return 2 ;;
  esac
}
git() {
  printf 'git %s\n' "$*" >> "$SPEC_CALLS"
  if [ "$1 $2" = 'diff --quiet' ]; then return 1; fi
}
rsync() {
  printf 'rsync %s\n' "$*" >> "$SPEC_CALLS"
  local source="${@: -2:1}" dest="${@: -1}"
  mkdir -p "$dest"
  cp "$source"*.json "$dest"
}
curl() { "$SPEC_CURL" "$@"; }
export -f gh git rsync curl
`
			cmd := exec.Command(bash, "-c", harness+specSyncWorkflowRun(t))
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "NO_PROXY=127.0.0.1,localhost", "no_proxy=127.0.0.1,localhost", "SPEC_HTTP="+server.URL, "SPEC_FAILURE="+tc.failure, "SPEC_STAGE="+tc.stage,
				"SPEC_CALLS="+filepath.ToSlash(calls), "SPEC_PROPOSAL="+filepath.ToSlash(proposal), "SPEC_CURL="+filepath.ToSlash(curl),
				"TMPDIR="+filepath.ToSlash(temporary), "GITHUB_REPOSITORY=owner/repo", "GITHUB_RUN_ID=123", "GITHUB_RUN_ATTEMPT=1")
			output, runErr := cmd.CombinedOutput()
			wantError := tc.failure != "" && tc.failure != "redirect"
			if (runErr != nil) != wantError {
				t.Errorf("workflow err=%v, wantError=%v; output=%s", runErr, wantError, output)
			}
			leftovers, err := os.ReadDir(temporary)
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("temporary candidate not removed: %v, %v", leftovers, err)
			}
			mutations, err := os.ReadFile(calls)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			marker, err := os.ReadFile(baseline)
			if err != nil {
				t.Fatal(err)
			}
			state, err := os.ReadFile(proposal)
			if err != nil {
				t.Fatal(err)
			}
			if wantError {
				oldSchema, err := os.ReadFile(filepath.Join(snapshot, "old.json"))
				if err != nil || string(oldSchema) != schema {
					t.Fatalf("failed download changed existing schema: %q, %v", oldSchema, err)
				}
				if len(mutations) != 0 || string(marker) != oldSHA || string(state) != "open" {
					t.Fatalf("failed download reached publication: marker=%q proposal=%q operations=%s", marker, state, mutations)
				}
				files, err := os.ReadDir(snapshot)
				if err != nil || len(files) != 2 {
					t.Fatalf("failed download changed snapshot files: %v, %v", files, err)
				}
			} else {
				if string(marker) != "2222222222222222222222222222222222222222\n" || string(state) != "closed" || !strings.Contains(string(mutations), "git push") || !strings.Contains(string(mutations), "pr create") {
					t.Fatalf("valid download did not reach publication: marker=%q proposal=%q operations=%s output=%s", marker, state, mutations, output)
				}
				for _, rel := range []string{"root.json", "v1/v1.json", "v2/v2.json"} {
					data, err := os.ReadFile(filepath.Join(snapshot, filepath.FromSlash(rel)))
					if err != nil || string(data) != schema {
						t.Fatalf("candidate %s = %q, %v", rel, data, err)
					}
				}
			}
		})
	}
}
