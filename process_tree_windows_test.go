//go:build windows

package codex

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

const (
	testWindowsPID           = 4242
	testWindowsJobHandle     = syscall.Handle(41)
	testWindowsProcessHandle = syscall.Handle(99)
)

func testWindowsProcessTreeAPI() windowsProcessTreeAPI {
	return windowsProcessTreeAPI{
		openProcess: func(uint32, bool, uint32) (syscall.Handle, error) {
			return 0, nil
		},
		closeHandle: func(syscall.Handle) error { return nil },
		createJobObject: func() (syscall.Handle, error) {
			return 0, nil
		},
		assignProcessToJobObject: func(syscall.Handle, syscall.Handle) error { return nil },
		setInformationJobObject: func(syscall.Handle, uint32, uintptr, uint32) error {
			return nil
		},
		terminateJobObject: func(syscall.Handle, uint32) error { return nil },
		killProcess:        func(*os.Process) error { return nil },
		after: func(time.Duration) <-chan time.Time {
			return make(chan time.Time)
		},
	}
}

func TestCreateKillOnCloseJobObjectConfiguresKillOnCloseLimit(t *testing.T) {
	api := testWindowsProcessTreeAPI()

	var gotInfoClass uint32
	var gotInfoLen uint32
	var gotInfo jobObjectExtendedLimitInfo

	api.createJobObject = func() (syscall.Handle, error) {
		return testWindowsJobHandle, nil
	}
	api.setInformationJobObject = func(job syscall.Handle, infoClass uint32, info uintptr, infoLen uint32) error {
		if job != testWindowsJobHandle {
			t.Fatalf("job = %v, want %v", job, testWindowsJobHandle)
		}
		gotInfoClass = infoClass
		gotInfoLen = infoLen
		gotInfo = *(*jobObjectExtendedLimitInfo)(unsafe.Pointer(info))
		return nil
	}

	job, err := createKillOnCloseJobObject(api)
	if err != nil {
		t.Fatalf("createKillOnCloseJobObject() error = %v, want nil", err)
	}
	if job != testWindowsJobHandle {
		t.Fatalf("job = %v, want %v", job, testWindowsJobHandle)
	}
	if gotInfoClass != jobObjectExtendedLimitInformation {
		t.Fatalf("infoClass = %d, want %d", gotInfoClass, jobObjectExtendedLimitInformation)
	}
	if gotInfoLen != uint32(unsafe.Sizeof(jobObjectExtendedLimitInfo{})) {
		t.Fatalf("infoLen = %d, want %d", gotInfoLen, uint32(unsafe.Sizeof(jobObjectExtendedLimitInfo{})))
	}
	if gotInfo.BasicLimitInformation.LimitFlags != jobObjectLimitKillOnJobClose {
		t.Fatalf("limitFlags = %#x, want %#x", gotInfo.BasicLimitInformation.LimitFlags, jobObjectLimitKillOnJobClose)
	}
}

func TestCreateKillOnCloseJobObjectClosesHandleOnConfigureError(t *testing.T) {
	api := testWindowsProcessTreeAPI()
	wantErr := errors.New("configure failed")
	var closed []syscall.Handle

	api.createJobObject = func() (syscall.Handle, error) {
		return testWindowsJobHandle, nil
	}
	api.setInformationJobObject = func(syscall.Handle, uint32, uintptr, uint32) error {
		return wantErr
	}
	api.closeHandle = func(handle syscall.Handle) error {
		closed = append(closed, handle)
		return nil
	}

	job, err := createKillOnCloseJobObject(api)
	if job != 0 {
		t.Fatalf("job = %v, want 0", job)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "configure job object") {
		t.Fatalf("error = %q, want configure job object context", err.Error())
	}
	if !slices.Equal(closed, []syscall.Handle{testWindowsJobHandle}) {
		t.Fatalf("closed handles = %v, want [%v]", closed, testWindowsJobHandle)
	}
}

func TestAttachProcessTreeReturnsZeroStateWithoutProcess(t *testing.T) {
	state, err := attachProcessTreeWithAPI(&exec.Cmd{}, testWindowsProcessTreeAPI())
	if err != nil {
		t.Fatalf("attachProcessTreeWithAPI() error = %v, want nil", err)
	}
	if state.job != 0 {
		t.Fatalf("job = %v, want 0", state.job)
	}
}

func TestAttachProcessTreeAttachesProcessAndClosesProcessHandle(t *testing.T) {
	api := testWindowsProcessTreeAPI()
	cmd := &exec.Cmd{Process: &os.Process{Pid: testWindowsPID}}
	var (
		gotAccess  uint32
		gotInherit bool
		gotPID     uint32
		closed     []syscall.Handle
	)

	api.createJobObject = func() (syscall.Handle, error) {
		return testWindowsJobHandle, nil
	}
	api.setInformationJobObject = func(syscall.Handle, uint32, uintptr, uint32) error {
		return nil
	}
	api.openProcess = func(desiredAccess uint32, inheritHandle bool, processID uint32) (syscall.Handle, error) {
		gotAccess = desiredAccess
		gotInherit = inheritHandle
		gotPID = processID
		return testWindowsProcessHandle, nil
	}
	api.assignProcessToJobObject = func(job, process syscall.Handle) error {
		if job != testWindowsJobHandle {
			t.Fatalf("job = %v, want %v", job, testWindowsJobHandle)
		}
		if process != testWindowsProcessHandle {
			t.Fatalf("process = %v, want %v", process, testWindowsProcessHandle)
		}
		return nil
	}
	api.closeHandle = func(handle syscall.Handle) error {
		closed = append(closed, handle)
		return nil
	}

	state, err := attachProcessTreeWithAPI(cmd, api)
	if err != nil {
		t.Fatalf("attachProcessTreeWithAPI() error = %v, want nil", err)
	}
	if state.job != testWindowsJobHandle {
		t.Fatalf("job = %v, want %v", state.job, testWindowsJobHandle)
	}
	if gotAccess != processSetQuota|syscall.PROCESS_TERMINATE {
		t.Fatalf("desiredAccess = %#x, want %#x", gotAccess, processSetQuota|syscall.PROCESS_TERMINATE)
	}
	if gotInherit {
		t.Fatal("inheritHandle = true, want false")
	}
	if gotPID != testWindowsPID {
		t.Fatalf("processID = %d, want %d", gotPID, testWindowsPID)
	}
	if !slices.Equal(closed, []syscall.Handle{testWindowsProcessHandle}) {
		t.Fatalf("closed handles = %v, want [%v]", closed, testWindowsProcessHandle)
	}
}

func TestAttachProcessTreeClosesJobHandleWhenOpenProcessFails(t *testing.T) {
	api := testWindowsProcessTreeAPI()
	wantErr := errors.New("open failed")
	var closed []syscall.Handle

	api.createJobObject = func() (syscall.Handle, error) {
		return testWindowsJobHandle, nil
	}
	api.setInformationJobObject = func(syscall.Handle, uint32, uintptr, uint32) error {
		return nil
	}
	api.openProcess = func(uint32, bool, uint32) (syscall.Handle, error) {
		return 0, wantErr
	}
	api.closeHandle = func(handle syscall.Handle) error {
		closed = append(closed, handle)
		return nil
	}

	state, err := attachProcessTreeWithAPI(&exec.Cmd{Process: &os.Process{Pid: testWindowsPID}}, api)
	if state.job != 0 {
		t.Fatalf("job = %v, want 0", state.job)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "open process handle") {
		t.Fatalf("error = %q, want open process handle context", err.Error())
	}
	if !slices.Equal(closed, []syscall.Handle{testWindowsJobHandle}) {
		t.Fatalf("closed handles = %v, want [%v]", closed, testWindowsJobHandle)
	}
}

func TestAttachProcessTreeClosesHandlesWhenAssignFails(t *testing.T) {
	api := testWindowsProcessTreeAPI()
	wantErr := errors.New("assign failed")
	var closed []syscall.Handle

	api.createJobObject = func() (syscall.Handle, error) {
		return testWindowsJobHandle, nil
	}
	api.setInformationJobObject = func(syscall.Handle, uint32, uintptr, uint32) error {
		return nil
	}
	api.openProcess = func(uint32, bool, uint32) (syscall.Handle, error) {
		return testWindowsProcessHandle, nil
	}
	api.assignProcessToJobObject = func(syscall.Handle, syscall.Handle) error {
		return wantErr
	}
	api.closeHandle = func(handle syscall.Handle) error {
		closed = append(closed, handle)
		return nil
	}

	state, err := attachProcessTreeWithAPI(&exec.Cmd{Process: &os.Process{Pid: testWindowsPID}}, api)
	if state.job != 0 {
		t.Fatalf("job = %v, want 0", state.job)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "assign process to job") {
		t.Fatalf("error = %q, want assign process to job context", err.Error())
	}
	wantClosed := []syscall.Handle{testWindowsJobHandle, testWindowsProcessHandle}
	if !slices.Equal(closed, wantClosed) {
		t.Fatalf("closed handles = %v, want %v", closed, wantClosed)
	}
}

func TestProcessTreeForceKillUsesJobObjectWhenPresent(t *testing.T) {
	api := testWindowsProcessTreeAPI()
	var (
		gotJob      syscall.Handle
		gotExitCode uint32
	)

	api.terminateJobObject = func(job syscall.Handle, exitCode uint32) error {
		gotJob = job
		gotExitCode = exitCode
		return nil
	}
	api.killProcess = func(*os.Process) error {
		t.Fatal("killProcess should not be called when a job handle is present")
		return nil
	}

	state := processTreeState{job: testWindowsJobHandle, api: api}
	if err := state.forceKill(&os.Process{Pid: testWindowsPID}); err != nil {
		t.Fatalf("forceKill() error = %v, want nil", err)
	}
	if gotJob != testWindowsJobHandle || gotExitCode != 1 {
		t.Fatalf("terminateJobObject(%v, %d), want (%v, 1)", gotJob, gotExitCode, testWindowsJobHandle)
	}
}

func TestProcessTreeForceKillFallsBackToProcessKill(t *testing.T) {
	api := testWindowsProcessTreeAPI()
	var gotPID int

	api.killProcess = func(process *os.Process) error {
		if process == nil {
			t.Fatal("process = nil, want non-nil")
		}
		gotPID = process.Pid
		return nil
	}

	state := processTreeState{api: api}
	if err := state.forceKill(&os.Process{Pid: testWindowsPID}); err != nil {
		t.Fatalf("forceKill() error = %v, want nil", err)
	}
	if gotPID != testWindowsPID {
		t.Fatalf("killProcess pid = %d, want %d", gotPID, testWindowsPID)
	}
}

func TestProcessTreeWaitForExitUsesConfiguredTimer(t *testing.T) {
	api := testWindowsProcessTreeAPI()
	timerFired := make(chan time.Time)
	close(timerFired)
	var gotDuration time.Duration

	api.after = func(d time.Duration) <-chan time.Time {
		gotDuration = d
		return timerFired
	}

	state := processTreeState{api: api}
	if state.waitForExit(make(chan struct{}), nil, 2*time.Second) {
		t.Fatal("waitForExit() = true, want false when grace period expires")
	}
	if gotDuration != 2*time.Second {
		t.Fatalf("gracePeriod = %v, want %v", gotDuration, 2*time.Second)
	}
}

func TestProcessTreeCloseClosesHandleOnce(t *testing.T) {
	api := testWindowsProcessTreeAPI()
	var closed []syscall.Handle

	api.closeHandle = func(handle syscall.Handle) error {
		closed = append(closed, handle)
		return nil
	}

	state := &processTreeState{job: testWindowsJobHandle, api: api}
	if err := state.close(); err != nil {
		t.Fatalf("close() error = %v, want nil", err)
	}
	if state.job != 0 {
		t.Fatalf("job = %v, want 0", state.job)
	}
	if err := state.close(); err != nil {
		t.Fatalf("second close() error = %v, want nil", err)
	}
	if !slices.Equal(closed, []syscall.Handle{testWindowsJobHandle}) {
		t.Fatalf("closed handles = %v, want [%v]", closed, testWindowsJobHandle)
	}
}
