//go:build windows

package codex

import "os"

func requestProcessShutdown(_ *os.Process) error {
	return nil
}
