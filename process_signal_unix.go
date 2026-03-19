//go:build !windows

package codex

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func defaultProcessShutdownMode() processShutdownMode {
	return processShutdownModeGraceful
}

func requestProcessShutdown(process *os.Process) error {
	if process == nil {
		return nil
	}
	interruptSignal, ok := os.Interrupt.(syscall.Signal)
	if !ok {
		return process.Signal(os.Interrupt)
	}
	return syscall.Kill(-process.Pid, interruptSignal)
}

func isExpectedShutdownWaitError(err error, attempt processShutdownAttempt) bool {
	if err == nil || attempt == processShutdownAttemptNone {
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
	case processShutdownAttemptInterrupt:
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
	case processShutdownAttemptKill:
		return waitStatus.Signaled() && waitStatus.Signal() == syscall.SIGKILL
	}

	return false
}
