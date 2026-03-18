//go:build !windows

package codex

import "os"

func requestProcessShutdown(process *os.Process) error {
	return process.Signal(os.Interrupt)
}
