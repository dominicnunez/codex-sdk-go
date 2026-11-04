//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package protocol_test

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPrePushRejectsCheckoutDrift(t *testing.T) {
	for _, state := range []string{"HEAD", "working source", "staged source"} {
		t.Run(state, func(t *testing.T) {
			f := newPushFixture(t)
			oid := strings.TrimSpace(string(f.git(t, "rev-parse", "HEAD")))
			ready, resume := filepath.Join(f.parent, "ready"), filepath.Join(f.parent, "resume")
			for _, path := range []string{ready, resume} {
				if err := syscall.Mkfifo(path, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			reader, err := os.OpenFile(ready, os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, f.bash, filepath.Join(f.dir, ".githooks", "pre-push"))
			cmd.Dir, cmd.Env = f.dir, append(append([]string(nil), f.env...), "LINT_FIXTURE_READY="+ready, "LINT_FIXTURE_WAIT="+resume)
			cmd.Stdin = strings.NewReader("refs/heads/main " + oid + " refs/heads/remote " + strings.Repeat("0", len(oid)) + "\n")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.WaitDelay = time.Second
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			defer func() {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				if !waited {
					_ = cmd.Wait()
				}
			}()
			arrived := make(chan struct{})
			go func() { _, _ = bufio.NewReader(reader).ReadString('\n'); close(arrived) }()
			select {
			case <-arrived:
			case <-ctx.Done():
				t.Fatal("verification did not reach lint barrier")
			}
			if state == "HEAD" {
				f.git(t, "commit", "--quiet", "--allow-empty", "-m", "test: change HEAD during verification")
			} else {
				f.write(t, "sample.go", fixturePackage+"const NewWorking = 7\n")
				if state == "staged source" {
					f.git(t, "add", "sample.go")
				}
			}
			release, err := os.OpenFile(resume, os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			_, err = release.WriteString("continue\n")
			_ = release.Close()
			if err != nil {
				t.Fatal(err)
			}
			err = cmd.Wait()
			waited = true
			if err == nil || !strings.Contains(output.String(), "changed during verification") && !strings.Contains(output.String(), "tracked source differs") {
				t.Fatalf("checkout drift was accepted or failed elsewhere: %v: %s", err, output.String())
			}
			if state != "HEAD" {
				data, err := os.ReadFile(filepath.Join(f.dir, "sample.go"))
				if err != nil || !strings.Contains(string(data), "NewWorking") {
					t.Fatalf("concurrent user edit was overwritten: %q, %v", data, err)
				}
			}
		})
	}
}
