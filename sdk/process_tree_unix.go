//go:build !windows

package codex

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

type processTreeState struct{}

func configureProcessTree(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func attachProcessTree(_ *exec.Cmd) (processTreeState, error) {
	return processTreeState{}, nil
}

func (processTreeState) requestShutdown(process *os.Process) error {
	return requestProcessShutdown(process)
}

func (processTreeState) forceKill(process *os.Process) error {
	if process == nil {
		return nil
	}
	return syscall.Kill(-process.Pid, syscall.SIGKILL)
}

func (processTreeState) waitForExit(waitDone <-chan struct{}, process *os.Process, gracePeriod time.Duration) bool {
	if process == nil {
		select {
		case <-waitDone:
			return true
		case <-time.After(gracePeriod):
			return false
		}
	}

	deadline := time.Now().Add(gracePeriod)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	parentExited := false
	for {
		if !parentExited {
			select {
			case <-waitDone:
				parentExited = true
			default:
			}
		}

		if parentExited && !processGroupExists(process.Pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}

		select {
		case <-waitDone:
			parentExited = true
		case <-ticker.C:
		}
	}
}

func (processTreeState) close() error {
	return nil
}

func processGroupExists(pid int) bool {
	err := syscall.Kill(-pid, 0)
	return err == nil || err == syscall.EPERM
}
