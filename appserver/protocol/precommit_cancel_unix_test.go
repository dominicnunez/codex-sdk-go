//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package protocol_test

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPreCommitCancellationPreservesIndex(t *testing.T) {
	f := newCommitFixture(t, map[string]string{"sample.go": fixturePackage + "const Before = 0\n"}, false)
	f.write(t, "sample.go", fixturePackage+"const Selected = 1\n")
	f.git(t, "add", "sample.go")
	index := filepath.Join(f.dir, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	ready, held := filepath.Join(f.parent, "ready"), filepath.Join(f.parent, "held")
	for _, name := range []string{ready, held} {
		if err := syscall.Mkfifo(name, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	reader, err := os.OpenFile(ready, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	temporary := filepath.Join(f.parent, "temporary")
	if err := os.Mkdir(temporary, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.bash, filepath.Join(f.dir, ".githooks", "pre-commit"))
	cmd.WaitDelay = time.Second
	cmd.Dir = f.dir
	cmd.Env = append(append([]string(nil), f.env...), "LINT_FIXTURE_READY="+ready, "LINT_FIXTURE_WAIT="+held, "TMPDIR="+temporary)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
	pid := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(reader).ReadString('\n')
		pid <- strings.TrimSpace(line)
	}()
	select {
	case line := <-pid:
		checker, err := strconv.Atoi(line)
		if err != nil || checker <= 1 {
			t.Fatalf("invalid supervised checker PID %q", line)
		}
		if err := syscall.Kill(checker, syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		_ = cmd.Wait()
		waited = true
		t.Fatalf("checker did not reach supervised lint: %s", output.String())
	}
	waitErr := cmd.Wait()
	waited = true
	if waitErr == nil {
		t.Fatalf("cancelled checker reported success: %s", output.String())
	}
	if after, err := os.ReadFile(index); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("cancellation changed index bytes: %v", err)
	}
	leftovers, err := os.ReadDir(temporary)
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("cancellation leaked temporary files: %v, %v", leftovers, err)
	}
	indexes, err := filepath.Glob(filepath.Join(f.dir, ".git", "hook-index-*"))
	if err != nil || len(indexes) != 0 {
		t.Fatalf("cancellation leaked private index resources: %q, %v", indexes, err)
	}
}
