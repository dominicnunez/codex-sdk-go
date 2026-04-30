//go:build windows

package codex

import (
	"errors"
	"os"
	"os/exec"
)

func defaultProcessShutdownMode() processShutdownMode {
	return processShutdownModeNoSignal
}

func requestProcessShutdown(_ *os.Process) error {
	return nil
}

func isExpectedShutdownWaitError(err error, attempt processShutdownAttempt) bool {
	if attempt != processShutdownAttemptKill {
		return false
	}
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}
