//go:build windows

package process

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

// Both differences must be nonnegative, making any ABI mismatch a compiler
// error even when this test binary is cross-compiled without being executed.
// The expected constants come from the platform ABI, not these Go records.
// Reference: https://go.dev/src/cmd/vendor/golang.org/x/sys/windows/types_windows_386.go
var (
	_ [int(unsafe.Sizeof(jobObjectBasicLimitInformation{})) - jobBasicBytes]byte
	_ [jobBasicBytes - int(unsafe.Sizeof(jobObjectBasicLimitInformation{}))]byte
	_ [int(unsafe.Sizeof(jobObjectExtendedLimitInfo{})) - jobExtendedBytes]byte
	_ [jobExtendedBytes - int(unsafe.Sizeof(jobObjectExtendedLimitInfo{}))]byte
	_ [int(unsafe.Sizeof(ioCounters{})) - 48]byte
	_ [48 - int(unsafe.Sizeof(ioCounters{}))]byte
	_ [int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.PerProcessUserTimeLimit)) - 0]byte
	_ [0 - int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.PerProcessUserTimeLimit))]byte
	_ [int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.PerJobUserTimeLimit)) - 8]byte
	_ [8 - int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.PerJobUserTimeLimit))]byte
	_ [int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.LimitFlags)) - 16]byte
	_ [16 - int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.LimitFlags))]byte
	_ [int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.MinimumWorkingSetSize)) - jobMinimumWorkingSetOffset]byte
	_ [jobMinimumWorkingSetOffset - int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.MinimumWorkingSetSize))]byte
	_ [int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.MaximumWorkingSetSize)) - jobMaximumWorkingSetOffset]byte
	_ [jobMaximumWorkingSetOffset - int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.MaximumWorkingSetSize))]byte
	_ [int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.ActiveProcessLimit)) - jobActiveProcessOffset]byte
	_ [jobActiveProcessOffset - int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.ActiveProcessLimit))]byte
	_ [int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.Affinity)) - jobAffinityOffset]byte
	_ [jobAffinityOffset - int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.Affinity))]byte
	_ [int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.PriorityClass)) - jobPriorityOffset]byte
	_ [jobPriorityOffset - int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.PriorityClass))]byte
	_ [int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.SchedulingClass)) - jobSchedulingOffset]byte
	_ [jobSchedulingOffset - int(unsafe.Offsetof(jobObjectBasicLimitInformation{}.SchedulingClass))]byte
	_ [int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.BasicLimitInformation)) - 0]byte
	_ [0 - int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.BasicLimitInformation))]byte
	_ [int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.IoInfo)) - jobIOOffset]byte
	_ [jobIOOffset - int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.IoInfo))]byte
	_ [int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.ProcessMemoryLimit)) - jobProcessMemoryOffset]byte
	_ [jobProcessMemoryOffset - int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.ProcessMemoryLimit))]byte
	_ [int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.JobMemoryLimit)) - jobMemoryOffset]byte
	_ [jobMemoryOffset - int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.JobMemoryLimit))]byte
	_ [int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.PeakProcessMemoryUsed)) - jobPeakProcessMemoryOffset]byte
	_ [jobPeakProcessMemoryOffset - int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.PeakProcessMemoryUsed))]byte
	_ [int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.PeakJobMemoryUsed)) - jobPeakMemoryOffset]byte
	_ [jobPeakMemoryOffset - int(unsafe.Offsetof(jobObjectExtendedLimitInfo{}.PeakJobMemoryUsed))]byte
)

func TestWindowsJobLifecycle(t *testing.T) {
	t.Logf("architecture=%s basic=%d extended=%d ioOffset=%d processMemoryOffset=%d",
		runtime.GOARCH, unsafe.Sizeof(jobObjectBasicLimitInformation{}), unsafe.Sizeof(jobObjectExtendedLimitInfo{}),
		unsafe.Offsetof(jobObjectExtendedLimitInfo{}.IoInfo), unsafe.Offsetof(jobObjectExtendedLimitInfo{}.ProcessMemoryLimit))
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Start only this package's inert helper, which waits for stdin EOF.
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestWindowsJobChild$")
	cmd.Env = append(os.Environ(), "CODEX_TEST_JOB_CHILD=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(output).ReadString('\n')
		if err == nil && line != "ready\n" {
			err = errors.New("unexpected helper readiness marker")
		}
		ready <- err
	}()
	waited := false
	defer func() {
		cancel()
		if !waited {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("test child did not exit after cancellation")
			}
		}
	}()
	// Exercise the real create/configure/open/assign API boundary, not the
	// injected unit-test API. A bad ABI length must fail this attachment.
	tree, err := AttachTree(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("owned child did not reach its blocking input before the deadline")
	}
	select {
	case err := <-done:
		waited = true
		t.Fatalf("owned child exited before job close: %v", err)
	default:
	}
	if err := tree.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		waited = true
		var exit *exec.ExitError
		// Closing the job guarantees termination, not a particular exit status.
		// The held stdin, readiness marker and live-child check above establish
		// that this helper cannot finish normally before the job is closed.
		if ctx.Err() != nil || (err != nil && !errors.As(err, &exit)) {
			t.Fatalf("kill-on-close did not terminate the owned child: %v (context: %v)", err, ctx.Err())
		}
	case <-ctx.Done():
		t.Fatal("kill-on-close did not terminate the owned child before the deadline")
	}
}

func TestWindowsJobChild(t *testing.T) {
	if os.Getenv("CODEX_TEST_JOB_CHILD") != "1" {
		return
	}
	if _, err := io.WriteString(os.Stdout, "ready\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}
