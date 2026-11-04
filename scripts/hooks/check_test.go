package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestCapturedLinkBoundaries(t *testing.T) {
	for _, target := range []string{"/payload", string(filepath.Separator) + "payload", "../payload"} {
		if err := capturedLink("link", map[string]string{"link": target}, map[string]bool{"payload": true}, map[string]bool{".": true}); err == nil {
			t.Errorf("rooted/escaping target %q accepted as captured source", target)
		}
	}
	for _, tc := range []struct {
		name      string
		links     map[string]string
		wantError bool
	}{
		{"captured regular", map[string]string{"link": "payload"}, false},
		{"missing target", map[string]string{"link": "missing"}, true},
		{"cycle", map[string]string{"link": "other", "other": "link"}, true},
		{"parent after link expansion", map[string]string{"link": "root/../payload", "root": "."}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := capturedLink("link", tc.links, map[string]bool{"payload": true}, map[string]bool{".": true})
			if (err != nil) != tc.wantError {
				t.Fatalf("link error=%v, wantError=%v", err, tc.wantError)
			}
		})
	}
}

func TestAddedLines(t *testing.T) {
	for _, tc := range []struct {
		name, diff string
		want       []lineRange
		wantError  bool
	}{
		{"one line", "@@ -2 +3 @@\n-old\n+new\n", []lineRange{{3, 3}}, false},
		{"separate additions", "@@ -1,0 +2,2 @@\n+a\n+b\n@@ -5 +7 @@\n-x\n+y\n", []lineRange{{2, 3}, {7, 7}}, false},
		{"deletion", "@@ -1,4 +0,0 @@\n-a\n-b\n-c\n-d\n", nil, false},
		{"empty", "", nil, false},
		{"invalid", "@@ invalid @@\n", nil, true},
		{"overflow", "@@ -1 +" + strconv.Itoa(int(^uint(0)>>1)) + ",2 @@\n", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := addedLines([]byte(tc.diff))
			if (err != nil) != tc.wantError || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ranges=%v err=%v, want %v error=%v", got, err, tc.want, tc.wantError)
			}
		})
	}
}

func FuzzAddedLineOwnership(f *testing.F) {
	for _, data := range [][]byte{{}, {0}, {1}, {2}, {3}, {0, 3, 2, 1, 0, 3}, bytes.Repeat([]byte{2}, 40)} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if _, err := exec.LookPath("git"); err != nil {
			t.Skip("git not installed")
		}
		if len(data) > 128 {
			data = data[:128]
		}
		var before, after []string
		want := make(map[int]bool)
		for n, choice := range data {
			old := fmt.Sprintf("old-%03d", n)
			before = append(before, old)
			if choice&1 == 0 {
				after = append(after, old)
			}
			if choice&2 != 0 {
				after = append(after, fmt.Sprintf("new-%03d", n))
				want[len(after)] = true
			}
		}
		dir := t.TempDir()
		for name, lines := range map[string][]string{"old": before, "new": after} {
			body := strings.Join(lines, "\n")
			if len(lines) != 0 {
				body += "\n"
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command("git", "diff", "--no-index", "--no-color", "--no-ext-diff", "--no-textconv", "--no-indent-heuristic", "--diff-algorithm=myers", "--text", "--unified=0", "--inter-hunk-context=0", "--", "old", "new")
		cmd.Dir = dir
		cmd.Env = replaceEnv(os.Environ(), "GIT_DIFF_OPTS", "")
		diff, err := cmd.Output()
		if err != nil && !exitCode(err, 1) {
			t.Fatal(err)
		}
		ranges, err := addedLines(diff)
		if err != nil {
			t.Fatalf("actual Git diff rejected: %v\n%s", err, diff)
		}
		got := make(map[int]bool)
		for _, interval := range ranges {
			for n := interval.first; n <= interval.last; n++ {
				got[n] = true
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("added-line ownership=%v, want inserted labels at %v\n%s", got, want, diff)
		}
	})
}
