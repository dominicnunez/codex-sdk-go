// Package process contains internal child-process control helpers.
package process

import (
	"errors"
	"os"
	"syscall"
)

// ShutdownMode controls how a managed child process is stopped.
type ShutdownMode uint8

const (
	ShutdownModeUnset ShutdownMode = iota
	ShutdownModeGraceful
	ShutdownModeNoSignal
)

// ShutdownAttempt records the strongest stop signal sent to a child process.
type ShutdownAttempt uint8

const (
	ShutdownAttemptNone ShutdownAttempt = iota
	ShutdownAttemptInterrupt
	ShutdownAttemptKill
)

// IsExpectedStopError reports whether err means the process was already gone.
func IsExpectedStopError(err error) bool {
	return errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH)
}
