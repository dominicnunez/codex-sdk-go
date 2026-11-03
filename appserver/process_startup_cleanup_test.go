package appserver

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"syscall"
	"testing"
)

type startupCleanupTree struct {
	t        *testing.T
	cmd      *exec.Cmd
	steps    []string
	killErr  error
	closeErr error
}

func (tree *startupCleanupTree) ForceKill(process *os.Process) error {
	tree.t.Helper()
	if process != tree.cmd.Process || tree.cmd.ProcessState != nil {
		tree.t.Error("tree termination did not precede parent reaping")
	}
	tree.steps = append(tree.steps, "kill-tree")
	return tree.killErr
}

func (tree *startupCleanupTree) Close() error {
	tree.t.Helper()
	if tree.cmd.ProcessState != nil {
		tree.t.Error("tree handle release did not precede parent reaping")
	}
	tree.steps = append(tree.steps, "close-tree")
	return tree.closeErr
}

func TestCanceledStartupAttemptsCompleteCleanup(t *testing.T) {
	killFailure := errors.New("fixture tree kill failure")
	closeFailure := errors.New("fixture tree close failure")
	for _, tc := range []struct {
		name     string
		killErr  error
		closeErr error
		cause    error
	}{
		{name: "ordinary"},
		{name: "deadline", cause: context.DeadlineExceeded},
		{name: "already-stopped", killErr: os.ErrProcessDone},
		{name: "already-gone", killErr: syscall.ESRCH},
		{name: "kill-fails", killErr: killFailure},
		{name: "close-fails", closeErr: closeFailure},
		{name: "both-fail", killErr: killFailure, closeErr: closeFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cause := tc.cause
			if cause == nil {
				cause = context.Canceled
			}
			// The test binary with no selected tests is an inert native child.
			cmd := exec.Command(os.Args[0], "-test.run=^$")
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
			tree := &startupCleanupTree{t: t, cmd: cmd, killErr: tc.killErr, closeErr: tc.closeErr}
			err := stopCanceledCommand(cmd, tree, cause)
			if !errors.Is(err, cause) {
				t.Fatalf("cancellation identity lost: %v", err)
			}
			if !slices.Equal(tree.steps, []string{"kill-tree", "close-tree"}) || cmd.ProcessState == nil {
				t.Fatalf("cleanup omitted a step: steps=%v reaped=%v", tree.steps, cmd.ProcessState != nil)
			}
			materialFailure := false
			for _, failure := range []error{tc.killErr, tc.closeErr} {
				if failure == nil || isExpectedProcessStopError(failure) {
					continue
				}
				materialFailure = true
				if !errors.Is(err, failure) {
					t.Errorf("cleanup failure was lost: wanted%v, got%v", failure, err)
				}
			}
			if !materialFailure && err != cause {
				t.Fatalf("ordinary cleanup changed original cancellation error: %v", err)
			}
		})
	}
}
