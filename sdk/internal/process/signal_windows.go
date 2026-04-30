//go:build windows

package process

import (
	"errors"
	"os"
	"os/exec"
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
	return errors.As(err, &exitErr)
}
