//go:build !windows

package process

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Tree tracks process-tree state for a managed child process.
type Tree struct{}

// ConfigureCommand prepares cmd so child processes share a process group.
func ConfigureCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// AttachTree attaches process-tree state to a started command.
func AttachTree(_ *exec.Cmd) (Tree, error) {
	return Tree{}, nil
}

// RequestShutdown asks the process group to terminate gracefully.
func (Tree) RequestShutdown(process *os.Process) error {
	return requestShutdown(process)
}

// ForceKill kills the process group.
func (Tree) ForceKill(process *os.Process) error {
	if process == nil {
		return nil
	}
	return syscall.Kill(-process.Pid, syscall.SIGKILL)
}

// WaitForExit waits until the parent exits and its process group disappears.
func (Tree) WaitForExit(waitDone <-chan struct{}, process *os.Process, gracePeriod time.Duration) bool {
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

// Close releases process-tree state.
func (Tree) Close() error {
	return nil
}

func processGroupExists(pid int) bool {
	err := syscall.Kill(-pid, 0)
	return err == nil || err == syscall.EPERM
}
