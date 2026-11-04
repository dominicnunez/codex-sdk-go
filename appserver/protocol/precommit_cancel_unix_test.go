//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package protocol_test

import (
	"bufio"
	"bytes"
	"context"
	"io"
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
	for _, phase := range []string{"analysis", "version", "install", "bootstrap", "complete"} {
		t.Run(phase, func(t *testing.T) { testCommitCancellation(t, phase) })
	}
}

func testCommitCancellation(t *testing.T, phase string) {
	t.Helper()
	f := newCommitFixture(t, map[string]string{"sample.go": fixturePackage + "const Before = 0\n"}, false)
	if phase == "install" || phase == "bootstrap" {
		realGo, err := exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		const fakeGo = `#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = install ] || { [ "$LINT_FIXTURE_PHASE" = bootstrap ] && [ "$1" = build ]; }; then
  (read -r reply < "$LINT_FIXTURE_WAIT") &
  printf '%s\n' "$BASHPID" > "$LINT_FIXTURE_READY"
  wait
  [[ "$1" != install ]] || exit 1
fi
exec "$LINT_FIXTURE_REAL_GO" "$@"
`
		if err := os.WriteFile(filepath.Join(f.parent, "bin", "go"), []byte(fakeGo), 0o700); err != nil {
			t.Fatal(err)
		}
		f.env = append(f.env, "LINT_FIXTURE_REAL_GO="+realGo)
	}
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
	cmd.Dir = f.dir
	cmd.Env = append(append([]string(nil), f.env...), "LINT_FIXTURE_READY="+ready, "LINT_FIXTURE_WAIT="+held, "LINT_FIXTURE_PHASE="+phase, "TMPDIR="+temporary)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	outputReader, outputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outputReader.Close()
	defer outputWriter.Close()
	outputDone := make(chan struct{})
	go func() { _, _ = io.Copy(&output, outputReader); close(outputDone) }()
	defer func() { _ = outputReader.Close(); <-outputDone }()
	cmd.Stdout, cmd.Stderr = outputWriter, outputWriter
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = outputWriter.Close()
	waited := false
	var pendingWait <-chan error
	defer func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if pendingWait != nil {
			<-pendingWait
		} else if !waited {
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
		readyPID, err := strconv.Atoi(line)
		if err != nil || readyPID <= 1 {
			t.Fatalf("invalid supervised checker PID %q", line)
		}
		if phase != "complete" {
			if err := syscall.Kill(cmd.Process.Pid, syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
		}
	case <-ctx.Done():
		_ = cmd.Wait()
		waited = true
		_ = outputReader.Close()
		<-outputDone
		t.Fatalf("checker did not reach supervised lint: %s", output.String())
	}
	waitDone := make(chan error, 1)
	pendingWait = waitDone
	go func() { waitDone <- cmd.Wait() }()
	if phase == "bootstrap" {
		select {
		case err := <-waitDone:
			pendingWait = nil
			waited = true
			t.Fatalf("bootstrap cancellation abandoned active build: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		release, err := os.OpenFile(held, os.O_RDWR, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = release.WriteString("finish bootstrap\n")
		_ = release.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	var waitErr error
	select {
	case waitErr = <-waitDone:
		pendingWait = nil
	case <-ctx.Done():
		t.Fatal("cancelled hook did not finish owned cleanup")
	}
	waited = true
	select {
	case <-outputDone:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled hook descendants retained output pipes")
	}
	if (waitErr == nil) != (phase == "complete") {
		t.Fatalf("checker completion differs for %s: %v: %s", phase, waitErr, output.String())
	}
	if phase == "bootstrap" {
		if !strings.Contains(output.String(), "waiting for bootstrap build") {
			t.Fatalf("bootstrap cancellation did not retain build ownership: %s", output.String())
		}
		if _, err := os.Stat(f.calls); !os.IsNotExist(err) {
			t.Fatalf("cancelled bootstrap launched lint: %v", err)
		}
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
