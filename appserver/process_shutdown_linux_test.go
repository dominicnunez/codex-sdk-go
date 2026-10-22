//go:build linux

package appserver_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
)

func shutdownCPUTime(t *testing.T) time.Duration {
	t.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		t.Fatal(err)
	}
	return time.Duration(usage.Utime.Sec+usage.Stime.Sec)*time.Second + time.Duration(usage.Utime.Usec+usage.Stime.Usec)*time.Microsecond
}

func shutdownProcessStopped(data []byte, err error) (bool, error) {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	_, tail, ok := strings.Cut(string(data), ") ")
	return ok && strings.HasPrefix(tail, "Z "), nil
}

func TestShutdownProcessStoppedObservations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		data    string
		err     error
		stopped bool
	}{
		{name: "missing", err: &os.PathError{Op: "open", Path: "/proc/123/stat", Err: syscall.ENOENT}, stopped: true},
		{name: "disappeared", err: &os.PathError{Op: "read", Path: "/proc/123/stat", Err: syscall.ESRCH}, stopped: true},
		{name: "permission denied", err: &os.PathError{Op: "open", Path: "/proc/123/stat", Err: syscall.EACCES}},
		{name: "read failure", data: "123 (sleep) Z ", err: &os.PathError{Op: "read", Path: "/proc/123/stat", Err: syscall.EIO}},
		{name: "running", data: "123 (sleep) R 1 2"},
		{name: "sleeping", data: "123 (sleep) S 1 2"},
		{name: "zombie", data: "123 (sleep) Z 1 2", stopped: true},
		{name: "malformed", data: "unrecognized stat"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stopped, err := shutdownProcessStopped([]byte(tc.data), tc.err)
			if stopped != tc.stopped {
				t.Errorf("stopped = %v, want %v", stopped, tc.stopped)
			}
			if tc.err != nil && !tc.stopped {
				if !errors.Is(err, tc.err) {
					t.Errorf("error = %v, want original error %v", err, tc.err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestShutdownProcessStoppedAfterProcfsDisappears(t *testing.T) {
	child := exec.Command("sleep", "60")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	})
	stat, err := os.Open(fmt.Sprintf("/proc/%d/stat", child.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer stat.Close()
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	// A held procfs descriptor outlives its task, reproducing disappearance
	// between the open and read inside os.ReadFile without a timing race.
	data, readErr := io.ReadAll(stat)
	if !errors.Is(readErr, syscall.ESRCH) {
		t.Fatalf("read after reap = %v, want ESRCH", readErr)
	}
	if stopped, err := shutdownProcessStopped(data, readErr); err != nil || !stopped {
		t.Fatalf("reaped process observation = (%v, %v), want (true, nil)", stopped, err)
	}
}

func TestProcessCloseWaitsWithoutSpinningForOwnedDescendant(t *testing.T) {
	for _, alreadyExited := range []bool{true, false} {
		t.Run(fmt.Sprint("parentAlreadyExited=", alreadyExited), func(t *testing.T) {
			dir := t.TempDir()
			pidFile := filepath.Join(dir, "descendant.pid")
			// Ignore INT before spawning so the descendant inherits it without
			// a race between shutdown and installing its signal disposition.
			script := "#!/bin/sh\ntrap '' INT\nsleep 60 &\nprintf '%s' \"$!\" > '" + pidFile + "'\n"
			if !alreadyExited {
				script += "read line\n"
			}
			script += "exit 0\n"
			binary := writeAppServerScriptBinary(t, dir, script)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{BinaryPath: binary})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = proc.Close() })
			pid, err := strconv.Atoi(waitForFileContents(t, pidFile))
			if err != nil {
				t.Fatal(err)
			}
			if alreadyExited {
				if err := proc.Wait(); err != nil {
					t.Fatal(err)
				}
			}
			// Startup context cancellation must not replace explicit Close's
			// ownership of the managed process and group shutdown.
			cancel()
			cpuBefore := shutdownCPUTime(t)
			started := time.Now()
			results := make(chan error, 8)
			for range 4 {
				go func() { results <- proc.Wait() }()
			}
			for range 4 {
				go func() { results <- proc.Close() }()
			}
			for range 8 {
				select {
				case err := <-results:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(8 * time.Second):
					t.Fatal("concurrent Wait/Close did not complete")
				}
			}
			elapsed, cpu := time.Since(started), shutdownCPUTime(t)-cpuBefore
			t.Logf("managed shutdown: wall=%v CPU=%v", elapsed, cpu)
			if elapsed < 3*time.Second {
				t.Fatal("shutdown skipped the live descendant grace period")
			}
			if cpu > 750*time.Millisecond {
				t.Fatalf("shutdown consumed %v CPU during %v grace wait", cpu, elapsed)
			}
			// Orphaned children may remain zombies until the host's reaper runs;
			// they must no longer be executing after group force-kill.
			deadline := time.Now().Add(time.Second)
			for {
				data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
				stopped, err := shutdownProcessStopped(data, err)
				if err != nil {
					t.Fatal(err)
				}
				if stopped {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("descendant still running after Close: pid=%d", pid)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}
