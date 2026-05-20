//go:build !windows

package process

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// DefaultShutdownMode returns the platform default child-process stop mode.
func DefaultShutdownMode() ShutdownMode {
	return ShutdownModeGraceful
}

func requestShutdown(process *os.Process) error {
	if process == nil {
		return nil
	}
	interruptSignal, ok := os.Interrupt.(syscall.Signal)
	if !ok {
		return process.Signal(os.Interrupt)
	}
	return syscall.Kill(-process.Pid, interruptSignal)
}

// IsExpectedShutdownWaitError reports whether Wait returned because of our shutdown signal.
func IsExpectedShutdownWaitError(err error, attempt ShutdownAttempt) bool {
	if err == nil || attempt == ShutdownAttemptNone {
		return false
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ProcessState == nil {
		return false
	}

	waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}

	switch attempt {
	case ShutdownAttemptInterrupt:
		interruptSignal, ok := os.Interrupt.(syscall.Signal)
		if !ok {
			return false
		}
		if waitStatus.Signaled() {
			return waitStatus.Signal() == interruptSignal
		}
		if waitStatus.Exited() {
			return waitStatus.ExitStatus() == 128+int(interruptSignal)
		}
	case ShutdownAttemptKill:
		return waitStatus.Signaled() && waitStatus.Signal() == syscall.SIGKILL
	}

	return false
}
