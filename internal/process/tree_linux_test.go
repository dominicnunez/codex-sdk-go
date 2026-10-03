//go:build linux

package process

import (
	"syscall"
	"testing"
	"time"
)

func processCPUTime(t *testing.T) time.Duration {
	t.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		t.Fatal(err)
	}
	return time.Duration(usage.Utime.Sec+usage.Stime.Sec)*time.Second + time.Duration(usage.Utime.Usec+usage.Stime.Usec)*time.Microsecond
}

func TestTreeWaitForExitDoesNotSpinAfterParentExit(t *testing.T) {
	parent, _, parentDone := startTestGroup(t, true)
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-parentDone
	cpuBefore := processCPUTime(t)
	started := time.Now()
	if (Tree{}).WaitForExit(parentDone, parent.Process, time.Second) {
		t.Fatal("live descendant completed")
	}
	cpu := processCPUTime(t) - cpuBefore
	elapsed := time.Since(started)
	t.Logf("shutdown wait: wall=%v CPU=%v", elapsed, cpu)
	// This deliberately broad bound distinguishes a CPU core spinning for the
	// whole grace period from 10ms polling; it is not a latency benchmark.
	if cpu > 250*time.Millisecond {
		t.Fatalf("shutdown wait consumed %v CPU during %v grace wait", cpu, elapsed)
	}
}
