//go:build !windows

package codex

import "os"

func defaultProcessShutdownMode() processShutdownMode {
	return processShutdownModeGraceful
}

func requestProcessShutdown(process *os.Process) error {
	return process.Signal(os.Interrupt)
}
