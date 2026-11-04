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

func TestPrePushRetainsTestCleanup(t *testing.T) {
	for _, phase := range []string{"ordinary", "race"} {
		t.Run(phase, func(t *testing.T) { testRetainedPush(t, phase) })
	}
}

func testRetainedPush(t *testing.T, phase string) {
	t.Helper()
	f := newPushFixture(t)
	const source = `package fixture
import (
 "bufio"
 "fmt"
 "os"
 "os/exec"
 "syscall"
 "testing"
 processctl "github.com/dominicnunez/codex-sdk-go/internal/process"
)
func TestOwnedCleanup(t *testing.T) {
 if os.Getenv("HOOK_WAIT_PHASE") != activePhase { return }
 child := exec.Command("bash", "-c", "read -r token < \"$HOOK_CHILD_HELD\"")
 child.Stdout, child.Stderr = os.Stdout, os.Stderr
 processctl.ConfigureCommand(child)
 if err := child.Start(); err != nil { t.Fatal(err) }
 t.Cleanup(func() {
  _ = child.Process.Kill()
  _ = child.Wait()
  if err := os.WriteFile(os.Getenv("HOOK_CLEANUP_DONE"), []byte("cleaned"), 0600); err != nil { t.Error(err) }
 })
 root, err := os.Getwd(); if err != nil { t.Fatal(err) }
 if err := os.WriteFile(os.Getenv("HOOK_ACTIVE_ROOT"), []byte(root), 0600); err != nil { t.Fatal(err) }
 group, err := syscall.Getpgid(0); if err != nil { t.Fatal(err) }
 ready, err := os.OpenFile(os.Getenv("HOOK_READY"), os.O_WRONLY, 0600); if err != nil { t.Fatal(err) }
 _, err = fmt.Fprintf(ready, "%d %d\n", child.Process.Pid, group); _ = ready.Close(); if err != nil { t.Fatal(err) }
 resume, err := os.Open(os.Getenv("HOOK_RESUME")); if err != nil { t.Fatal(err) }
 _, err = bufio.NewReader(resume).ReadString('\n'); _ = resume.Close(); if err != nil { t.Fatal(err) }
 if _, err := os.Stat("sample.go"); err != nil { t.Fatalf("source removed before test cleanup: %v", err) }
}
`
	f.write(t, "owned_test.go", source)
	f.write(t, "phase_plain_test.go", "//go:build !race\n\npackage fixture\nconst activePhase = \"ordinary\"\n")
	f.write(t, "phase_race_test.go", "//go:build race\n\npackage fixture\nconst activePhase = \"race\"\n")
	f.git(t, "add", "--all")
	f.git(t, "commit", "--quiet", "-m", "test: finite test-owned process cleanup")
	oid := strings.TrimSpace(string(f.git(t, "rev-parse", "HEAD")))
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	const goSpy = "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"$HOOK_GO_CALLS\"\nexec \"$HOOK_REAL_GO\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(f.parent, "bin", "go"), []byte(goSpy), 0o700); err != nil {
		t.Fatal(err)
	}
	goCalls := filepath.Join(f.parent, "go-calls")
	f.env = append(f.env, "HOOK_REAL_GO="+filepath.ToSlash(realGo), "HOOK_GO_CALLS="+goCalls)
	ready, resume, held := filepath.Join(f.parent, "ready"), filepath.Join(f.parent, "resume"), filepath.Join(f.parent, "child-held")
	for _, name := range []string{ready, resume, held} {
		if err := syscall.Mkfifo(name, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	readReady, err := os.OpenFile(ready, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer readReady.Close()
	doneFile, rootFile := filepath.Join(f.parent, "cleaned"), filepath.Join(f.parent, "active-root")
	temporary := filepath.Join(f.parent, "temporary")
	if err := os.Mkdir(temporary, 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(f.dir, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.bash, filepath.Join(f.dir, ".githooks", "pre-push"))
	cmd.Dir, cmd.Env = f.dir, append(append([]string(nil), f.env...), "TMPDIR="+temporary, "HOOK_WAIT_PHASE="+phase, "HOOK_READY="+ready, "HOOK_RESUME="+resume, "HOOK_CHILD_HELD="+held, "HOOK_CLEANUP_DONE="+doneFile, "HOOK_ACTIVE_ROOT="+rootFile)
	cmd.Stdin = strings.NewReader("refs/heads/main " + oid + " refs/heads/remote " + strings.Repeat("0", len(oid)) + "\n")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	var output bytes.Buffer
	outputDone := make(chan struct{})
	go func() { _, _ = io.Copy(&output, reader); close(outputDone) }()
	defer func() { _ = reader.Close(); <-outputDone }()
	cmd.Stdout, cmd.Stderr = writer, writer
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	waited, completed := false, false
	childPID, testGroup := 0, 0
	defer func() {
		if !completed {
			if childPID > 1 {
				_ = syscall.Kill(-childPID, syscall.SIGKILL)
			}
			if testGroup > 1 {
				_ = syscall.Kill(-testGroup, syscall.SIGKILL)
			}
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		if !waited {
			<-waitDone
		}
	}()
	readyLine := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(readReady).ReadString('\n'); readyLine <- line }()
	select {
	case line := <-readyLine:
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("invalid test ownership receipt %q", line)
		}
		childPID, err = strconv.Atoi(fields[0])
		if err != nil {
			t.Fatal(err)
		}
		testGroup, err = strconv.Atoi(fields[1])
		if err != nil || childPID <= 1 || testGroup <= 1 || childPID == testGroup {
			t.Fatalf("fixture did not create an independent owned group: %q", line)
		}
	case <-ctx.Done():
		t.Fatal("actual test did not reach cleanup barrier")
	case err := <-waitDone:
		waited = true
		_ = reader.Close()
		<-outputDone
		t.Fatalf("test fixture failed before cleanup barrier: %v: %s", err, output.String())
	}
	for n := 0; n < 2; n++ {
		if err := syscall.Kill(cmd.Process.Pid, syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-waitDone:
			waited = true
			_, cleanupErr := os.Stat(doneFile)
			t.Fatalf("cancelled hook interrupted test-owned cleanup: %v; cleanup=%v, independent child group alive=%v", err, cleanupErr, syscall.Kill(-childPID, 0) == nil)
		case <-time.After(100 * time.Millisecond):
		}
	}
	root, err := os.ReadFile(rootFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(string(root), "sample.go")); err != nil {
		t.Fatalf("active test source removed before completion: %v", err)
	}
	release, err := os.OpenFile(resume, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = release.WriteString("finish test\n")
	_ = release.Close()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-waitDone:
		waited = true
		if err == nil {
			t.Fatal("cancelled push reported success")
		}
	case <-ctx.Done():
		t.Fatal("retained test did not finish")
	}
	select {
	case <-outputDone:
	case <-ctx.Done():
		t.Fatal("test-owned child retained output")
	}
	if data, err := os.ReadFile(doneFile); err != nil || string(data) != "cleaned" {
		t.Fatalf("test cleanup did not finish: %q, %v: %s", data, err, output.String())
	}
	completed = true
	if _, err := os.Stat(f.calls); !os.IsNotExist(err) {
		t.Fatalf("cancellation started a later lint gate: %v", err)
	}
	calls, err := os.ReadFile(goCalls)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "mod tidy") || phase == "ordinary" && strings.Contains(string(calls), "test -race") {
		t.Fatalf("cancellation started a later Go gate: %s", calls)
	}
	if after, err := os.ReadFile(filepath.Join(f.dir, ".git", "index")); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("cancellation changed caller index: %v", err)
	}
	if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
		t.Fatalf("retained check leaked temporary resources: %v, %v", entries, err)
	}
}
