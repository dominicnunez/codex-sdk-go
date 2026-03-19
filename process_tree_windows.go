//go:build windows

package codex

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x00002000
	processSetQuota                   = 0x0100
)

var (
	modkernel32            = syscall.NewLazyDLL("kernel32.dll")
	procAssignProcessToJob = modkernel32.NewProc("AssignProcessToJobObject")
	procCreateJobObjectW   = modkernel32.NewProc("CreateJobObjectW")
	procSetInformationJob  = modkernel32.NewProc("SetInformationJobObject")
	procTerminateJobObject = modkernel32.NewProc("TerminateJobObject")
)

type processTreeState struct {
	job syscall.Handle
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobObjectExtendedLimitInfo struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

func configureProcessTree(_ *exec.Cmd) {}

func attachProcessTree(cmd *exec.Cmd) (processTreeState, error) {
	if cmd == nil || cmd.Process == nil {
		return processTreeState{}, nil
	}

	job, err := createKillOnCloseJobObject()
	if err != nil {
		return processTreeState{}, err
	}

	processHandle, err := syscall.OpenProcess(processSetQuota|syscall.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = syscall.CloseHandle(job)
		return processTreeState{}, fmt.Errorf("open process handle: %w", err)
	}
	defer syscall.CloseHandle(processHandle)

	if err := assignProcessToJobObject(job, processHandle); err != nil {
		_ = syscall.CloseHandle(job)
		return processTreeState{}, fmt.Errorf("assign process to job: %w", err)
	}

	return processTreeState{job: job}, nil
}

func (processTreeState) requestShutdown(_ *os.Process) error {
	return nil
}

func (s processTreeState) forceKill(process *os.Process) error {
	if s.job != 0 {
		return terminateJobObject(s.job, 1)
	}
	if process == nil {
		return nil
	}
	return process.Kill()
}

func (processTreeState) waitForExit(waitDone <-chan struct{}, _ *os.Process, gracePeriod time.Duration) bool {
	select {
	case <-waitDone:
		return true
	case <-time.After(gracePeriod):
		return false
	}
}

func (s *processTreeState) close() error {
	if s == nil || s.job == 0 {
		return nil
	}
	job := s.job
	s.job = 0
	return syscall.CloseHandle(job)
}

func createKillOnCloseJobObject() (syscall.Handle, error) {
	job, err := createJobObject()
	if err != nil {
		return 0, fmt.Errorf("create job object: %w", err)
	}

	info := jobObjectExtendedLimitInfo{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	if err := setInformationJobObject(job, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = syscall.CloseHandle(job)
		return 0, fmt.Errorf("configure job object: %w", err)
	}

	return job, nil
}

func createJobObject() (syscall.Handle, error) {
	r1, _, e1 := procCreateJobObjectW.Call(0, 0)
	if r1 != 0 {
		return syscall.Handle(r1), nil
	}
	if e1 != syscall.Errno(0) {
		return 0, error(e1)
	}
	return 0, syscall.EINVAL
}

func assignProcessToJobObject(job, process syscall.Handle) error {
	r1, _, e1 := procAssignProcessToJob.Call(uintptr(job), uintptr(process))
	if r1 != 0 {
		return nil
	}
	if e1 != syscall.Errno(0) {
		return error(e1)
	}
	return syscall.EINVAL
}

func setInformationJobObject(job syscall.Handle, infoClass uint32, info uintptr, infoLen uint32) error {
	r1, _, e1 := procSetInformationJob.Call(uintptr(job), uintptr(infoClass), info, uintptr(infoLen))
	if r1 != 0 {
		return nil
	}
	if e1 != syscall.Errno(0) {
		return error(e1)
	}
	return syscall.EINVAL
}

func terminateJobObject(job syscall.Handle, exitCode uint32) error {
	r1, _, e1 := procTerminateJobObject.Call(uintptr(job), uintptr(exitCode))
	if r1 != 0 {
		return nil
	}
	if e1 != syscall.Errno(0) {
		return error(e1)
	}
	return syscall.EINVAL
}
