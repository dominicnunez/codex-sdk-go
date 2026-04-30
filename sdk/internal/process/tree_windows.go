//go:build windows

package process

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

type windowsTreeAPI struct {
	openProcess              func(desiredAccess uint32, inheritHandle bool, processID uint32) (syscall.Handle, error)
	closeHandle              func(handle syscall.Handle) error
	createJobObject          func() (syscall.Handle, error)
	assignProcessToJobObject func(job, process syscall.Handle) error
	setInformationJobObject  func(job syscall.Handle, infoClass uint32, info uintptr, infoLen uint32) error
	terminateJobObject       func(job syscall.Handle, exitCode uint32) error
	killProcess              func(process *os.Process) error
	after                    func(d time.Duration) <-chan time.Time
}

var defaultWindowsTreeAPI = windowsTreeAPI{
	openProcess:              syscall.OpenProcess,
	closeHandle:              syscall.CloseHandle,
	createJobObject:          createJobObject,
	assignProcessToJobObject: assignProcessToJobObject,
	setInformationJobObject:  setInformationJobObject,
	terminateJobObject:       terminateJobObject,
	killProcess: func(process *os.Process) error {
		if process == nil {
			return nil
		}
		return process.Kill()
	},
	after: time.After,
}

// Tree tracks process-tree state for a managed child process.
type Tree struct {
	job syscall.Handle
	api windowsTreeAPI
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

// ConfigureCommand prepares cmd for process-tree management.
func ConfigureCommand(_ *exec.Cmd) {}

// AttachTree attaches process-tree state to a started command.
func AttachTree(cmd *exec.Cmd) (Tree, error) {
	return attachTreeWithAPI(cmd, defaultWindowsTreeAPI)
}

func attachTreeWithAPI(cmd *exec.Cmd, api windowsTreeAPI) (Tree, error) {
	api = (&Tree{api: api}).apiOrDefault()

	if cmd == nil || cmd.Process == nil {
		return Tree{}, nil
	}

	job, err := createKillOnCloseJobObject(api)
	if err != nil {
		return Tree{}, err
	}

	processHandle, err := api.openProcess(processSetQuota|syscall.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = api.closeHandle(job)
		return Tree{}, fmt.Errorf("open process handle: %w", err)
	}
	defer api.closeHandle(processHandle)

	if err := api.assignProcessToJobObject(job, processHandle); err != nil {
		_ = api.closeHandle(job)
		return Tree{}, fmt.Errorf("assign process to job: %w", err)
	}

	return Tree{job: job, api: api}, nil
}

// RequestShutdown asks the process tree to terminate gracefully.
func (Tree) RequestShutdown(_ *os.Process) error {
	return nil
}

// ForceKill kills the process tree.
func (t Tree) ForceKill(process *os.Process) error {
	if t.job != 0 {
		return t.apiOrDefault().terminateJobObject(t.job, 1)
	}
	return t.apiOrDefault().killProcess(process)
}

// WaitForExit waits for the parent process to exit or for gracePeriod to expire.
func (t Tree) WaitForExit(waitDone <-chan struct{}, _ *os.Process, gracePeriod time.Duration) bool {
	select {
	case <-waitDone:
		return true
	case <-t.apiOrDefault().after(gracePeriod):
		return false
	}
}

// Close releases process-tree state.
func (t *Tree) Close() error {
	if t == nil || t.job == 0 {
		return nil
	}
	job := t.job
	t.job = 0
	return t.apiOrDefault().closeHandle(job)
}

func createKillOnCloseJobObject(api windowsTreeAPI) (syscall.Handle, error) {
	api = (&Tree{api: api}).apiOrDefault()

	job, err := api.createJobObject()
	if err != nil {
		return 0, fmt.Errorf("create job object: %w", err)
	}

	info := jobObjectExtendedLimitInfo{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	if err := api.setInformationJobObject(job, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = api.closeHandle(job)
		return 0, fmt.Errorf("configure job object: %w", err)
	}

	return job, nil
}

func (t *Tree) apiOrDefault() windowsTreeAPI {
	if t == nil {
		return defaultWindowsTreeAPI
	}
	api := t.api
	if api.openProcess == nil {
		api.openProcess = defaultWindowsTreeAPI.openProcess
	}
	if api.closeHandle == nil {
		api.closeHandle = defaultWindowsTreeAPI.closeHandle
	}
	if api.createJobObject == nil {
		api.createJobObject = defaultWindowsTreeAPI.createJobObject
	}
	if api.assignProcessToJobObject == nil {
		api.assignProcessToJobObject = defaultWindowsTreeAPI.assignProcessToJobObject
	}
	if api.setInformationJobObject == nil {
		api.setInformationJobObject = defaultWindowsTreeAPI.setInformationJobObject
	}
	if api.terminateJobObject == nil {
		api.terminateJobObject = defaultWindowsTreeAPI.terminateJobObject
	}
	if api.killProcess == nil {
		api.killProcess = defaultWindowsTreeAPI.killProcess
	}
	if api.after == nil {
		api.after = defaultWindowsTreeAPI.after
	}
	return api
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
