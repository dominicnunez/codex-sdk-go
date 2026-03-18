//go:build windows

package codex

import "os"

func defaultProcessShutdownMode() processShutdownMode {
	return processShutdownModeNoSignal
}

func requestProcessShutdown(_ *os.Process) error {
	return nil
}
