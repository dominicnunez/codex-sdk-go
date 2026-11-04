package protocol_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSpecSyncPinsSnapshot(t *testing.T) {
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
	const selected = "2222222222222222222222222222222222222222"
	const advanced = "3333333333333333333333333333333333333333"
	cases := []struct{ name, schedule, sha string }{
		{"stable", "", selected},
		{"after selection", "after_selection", selected},
		{"after inventory", "after_inventory", selected},
		{"between files", "after_file", selected},
		{"empty SHA", "", ""},
		{"null SHA", "", "null"},
		{"branch SHA", "", "main"},
		{"short SHA", "", selected[:39]},
		{"nonhex SHA", "", strings.Repeat("z", 40)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(path, content string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			current := filepath.Join(dir, "upstream-head.txt")
			write(current, selected)
			var mu sync.Mutex
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, r.URL.Path)
				mu.Unlock()
				parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
				if len(parts) != 2 {
					http.NotFound(w, r)
					return
				}
				revision, name := parts[0], parts[1]
				if revision == "main" {
					data, err := os.ReadFile(current)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					revision = string(data)
				}
				if revision != selected && revision != advanced || revision == selected && strings.HasSuffix(name, "-b.json") || revision == advanced && strings.HasSuffix(name, "-a.json") {
					http.NotFound(w, r)
					return
				}
				if tc.schedule == "after_file" && name == "root-a.json" {
					if err := os.WriteFile(current, []byte(advanced), 0o600); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}
				_, _ = fmt.Fprintf(w, `{"commit":%q,"file":%q}`, revision, name)
			}))
			defer server.Close()
			snapshot := filepath.Join(dir, "appserver", "protocol", "schema", "json")
			baseline := filepath.Join(snapshot, ".upstream-sha")
			const oldSHA = "1111111111111111111111111111111111111111\n"
			write(baseline, oldSHA)
			write(filepath.Join(snapshot, "old.json"), `{}`)
			proposal := filepath.Join(dir, "proposal.txt")
			write(proposal, "open")
			prune, err := os.ReadFile(repoPath(t, "scripts", "spec-sync-prune.sh"))
			if err != nil {
				t.Fatal(err)
			}
			write(filepath.Join(dir, "scripts", "spec-sync-prune.sh"), string(prune))
			calls, api := filepath.Join(dir, "mutations.txt"), filepath.Join(dir, "api.txt")
			temporary := filepath.Join(dir, "temporary downloads")
			if err := os.MkdirAll(temporary, 0o700); err != nil {
				t.Fatal(err)
			}
			const scheduleFixture = `
gh() {
  case "$1 $2" in
    'api repos/openai/codex/commits'*)
      if [ "$PIN_SCHEDULE" = after_selection ]; then printf '%s' "$PIN_ADVANCED" > "$PIN_CURRENT"; fi
      printf '%s\n' "$PIN_SHA"
      ;;
    'api repos/openai/codex/contents/'*)
      printf '%s\n' "$2" >> "$PIN_API"
      case "${2%%\?*}" in
        */json) stage=root ;;
        */json/v1) stage=v1 ;;
        */json/v2) stage=v2 ;;
        *) return 2 ;;
      esac
      if [[ "$2" == *'?ref='* ]]; then
        revision="${2##*?ref=}"
        url_revision="$revision"
      else
        revision="$(cat "$PIN_CURRENT")"
        url_revision=main
      fi
      side=a
      if [ "$revision" = "$PIN_ADVANCED" ]; then side=b; fi
      if [ "$PIN_SCHEDULE" = after_inventory ] && [ "$stage" = root ]; then printf '%s' "$PIN_ADVANCED" > "$PIN_CURRENT"; fi
      printf '%s/%s/%s-%s.json\n' "$SPEC_HTTP" "$url_revision" "$stage" "$side"
      printf '%s/%s/%s.json\n' "$SPEC_HTTP" "$url_revision" "$stage"
      ;;
    *) spec_fixture_gh "$@" ;;
  esac
}
export -f gh
`
			cmd := exec.Command(bash, "-c", specSyncFixture+scheduleFixture+specSyncWorkflowRun(t))
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "NO_PROXY=127.0.0.1,localhost", "no_proxy=127.0.0.1,localhost",
				"SPEC_HTTP="+server.URL, "SPEC_CURL="+filepath.ToSlash(curl), "SPEC_CALLS="+filepath.ToSlash(calls), "SPEC_PROPOSAL="+filepath.ToSlash(proposal),
				"PIN_CURRENT="+filepath.ToSlash(current), "PIN_API="+filepath.ToSlash(api), "PIN_SHA="+tc.sha, "PIN_ADVANCED="+advanced, "PIN_SCHEDULE="+tc.schedule,
				"TMPDIR="+filepath.ToSlash(temporary), "GITHUB_REPOSITORY=owner/repo", "GITHUB_RUN_ID=123", "GITHUB_RUN_ATTEMPT=1")
			output, runErr := cmd.CombinedOutput()
			marker, err := os.ReadFile(baseline)
			if err != nil {
				t.Fatal(err)
			}
			inventory, err := os.ReadFile(api)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			mutations, err := os.ReadFile(calls)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			leftovers, err := os.ReadDir(temporary)
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("temporary candidate not removed: %v, %v", leftovers, err)
			}
			if tc.sha != selected {
				state, err := os.ReadFile(proposal)
				if err != nil || string(state) != "open" {
					t.Fatalf("invalid commit changed existing proposal: %q, %v", state, err)
				}
				if runErr == nil || len(inventory) != 0 || len(mutations) != 0 || string(marker) != oldSHA {
					t.Fatalf("invalid immutable commit reached inventory/publication: err=%v inventory=%s marker=%q operations=%s", runErr, inventory, marker, mutations)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("workflow failed across upstream advance: %v, %s", runErr, output)
			}
			if string(marker) != selected+"\n" || !strings.Contains(string(mutations), "**Upstream commit:** `"+selected[:12]+"`") || !strings.Contains(string(mutations), "git push -u origin spec-sync/"+selected[:12]+"-") {
				t.Fatalf("snapshot marker and PR metadata differ: marker=%q operations=%s", marker, mutations)
			}
			// Inventories at A include A-only files that disappear at B; B adds
			// separate B-only files. Both inventories also include common files.
			for _, rel := range []string{"root-a.json", "root.json", "v1/v1-a.json", "v1/v1.json", "v2/v2-a.json", "v2/v2.json"} {
				data, err := os.ReadFile(filepath.Join(snapshot, filepath.FromSlash(rel)))
				if err != nil {
					t.Errorf("selected snapshot file %s missing: %v", rel, err)
					continue
				}
				var value struct{ Commit, File string }
				if err := json.Unmarshal(data, &value); err != nil || value.Commit != selected || value.File != filepath.Base(rel) {
					t.Errorf("selected snapshot file %s has content %s, err=%v", rel, data, err)
				}
			}
			lines := strings.Split(strings.TrimSpace(string(inventory)), "\n")
			if len(lines) != 3 {
				t.Fatalf("inventory calls=%q, want all three directories", lines)
			}
			for _, line := range lines {
				if !strings.HasSuffix(line, "?ref="+selected) {
					t.Errorf("inventory is not pinned: %s", line)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if len(requests) != 6 {
				t.Fatalf("download requests=%q, want six selected files", requests)
			}
			for _, path := range requests {
				if !strings.HasPrefix(path, "/"+selected+"/") || strings.HasSuffix(path, "-b.json") {
					t.Errorf("download is not from selected snapshot: %s", path)
				}
			}
		})
	}
}
