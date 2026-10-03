//go:build linux

package appserver_test

import (
	"context"
	"errors"
	"fmt"
	"os"
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
				if errors.Is(err, os.ErrNotExist) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				_, tail, ok := strings.Cut(string(data), ") ")
				if ok && strings.HasPrefix(tail, "Z ") {
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
