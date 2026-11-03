//go:build !windows

package appserver_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	sdk "github.com/dominicnunez/codex-sdk-go/appserver"
)

type startupTreeContext struct {
	context.Context
	calls         atomic.Int32
	afterAttached func()
}

func (c *startupTreeContext) Err() error {
	// StartProcess checks twice before launch, then immediately after tree
	// attachment. This barrier selects that real post-attachment interleaving.
	if c.calls.Add(1) == 3 {
		c.afterAttached()
	}
	return c.Context.Err()
}

func TestStartupCancellationStopsTrackedDescendants(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "startup-tree")
	build := exec.Command("go", "build", "-o", binary, "./testdata/startup-tree")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build startup fixture: %v\n%s", err, output)
	}
	for _, mode := range []string{"pre-cancelled", "ordinary-close", "post-attachment-cancelled"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			opts := &sdk.ProcessOptions{BinaryPath: binary, Env: append(os.Environ(), "CODEX_TEST_PROCESS_ROOT="+root)}
			if mode == "pre-cancelled" {
				cancel()
				process, err := sdk.StartProcess(base, opts)
				if process != nil || err != context.Canceled {
					t.Fatalf("pre-cancelled startup accepted: process=%v error=%v", process != nil, err)
				}
				if _, err := os.Stat(filepath.Join(root, "pids")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("pre-cancelled startup launched a fixture")
				}
				return
			}
			var parentPID int
			var child *os.Process
			awaitChild := func() {
				values := strings.Fields(waitForFileContents(t, filepath.Join(root, "pids")))
				if len(values) != 2 {
					t.Fatal("invalid fixture PID record")
				}
				var err error
				parentPID, err = strconv.Atoi(values[0])
				if err != nil {
					t.Fatal(err)
				}
				childPID, err := strconv.Atoi(values[1])
				if err != nil {
					t.Fatal(err)
				}
				child, err = os.FindProcess(childPID)
				if err != nil {
					t.Fatal(err)
				}
				waitForFileContents(t, filepath.Join(root, "heartbeat"))
				if group, err := syscall.Getpgid(childPID); err != nil || group != parentPID {
					t.Fatalf("fixture child is outside tracked group: group=%d parent=%d error=%v", group, parentPID, err)
				}
			}
			defer func() {
				_ = os.WriteFile(filepath.Join(root, "stop"), []byte("stop"), 0o600)
				if child != nil {
					if err := child.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
						t.Errorf("fixture cleanup: %v", err)
					}
					_ = child.Release()
				}
			}()
			ctx := base
			if mode == "post-attachment-cancelled" {
				ctx = &startupTreeContext{Context: base, afterAttached: func() { awaitChild(); cancel() }}
			}
			process, err := sdk.StartProcess(ctx, opts)
			if process != nil {
				defer process.Close()
			}
			if mode == "ordinary-close" {
				if err != nil || process == nil {
					t.Fatalf("ordinary startup failed: %v", err)
				}
				awaitChild()
				if err := process.Close(); err != nil {
					t.Fatal(err)
				}
			} else if process != nil || err != context.Canceled {
				t.Fatalf("cancellation outcome changed: process=%v error=%v", process != nil, err)
			}
			if err := syscall.Kill(parentPID, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("managed parent was not reaped: %v", err)
			}
			before, err := strconv.Atoi(waitForFileContents(t, filepath.Join(root, "heartbeat")))
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(200 * time.Millisecond)
			for time.Now().Before(deadline) {
				after, err := strconv.Atoi(waitForFileContents(t, filepath.Join(root, "heartbeat")))
				if err != nil {
					t.Fatal(err)
				}
				if after >= before+2 {
					t.Fatalf("tracked child kept executing after startup cleanup: heartbeat%d ->%d", before, after)
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}
