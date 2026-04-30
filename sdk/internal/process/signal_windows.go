//go:build windows

package process

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// DefaultShutdownMode returns the platform default child-process stop mode.
func DefaultShutdownMode() ShutdownMode {
	return ShutdownModeNoSignal
}

func requestShutdown(_ *os.Process) error {
	return nil
}

// IsExpectedShutdownWaitError reports whether Wait returned because of our shutdown signal.
func IsExpectedShutdownWaitError(err error, attempt ShutdownAttempt) bool {
	if attempt != ShutdownAttemptKill {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ProcessState == nil {
		return false
	}
	waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && isExpectedJobTerminationExitCode(waitStatus.ExitStatus())
}

func isExpectedJobTerminationExitCode(exitCode int) bool {
	return exitCode == terminateJobObjectExitCode
}
