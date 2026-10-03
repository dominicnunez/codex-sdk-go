//go:build !windows

package process

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// Each command is reaped by the test, including the same-group sibling used to
// represent a descendant that outlives the managed parent.
func startTestGroup(t *testing.T, withDescendant bool) (*exec.Cmd, *exec.Cmd, <-chan struct{}) {
	t.Helper()
	parent := exec.Command("/bin/sh", "-c", "exec sleep 60")
	ConfigureCommand(parent)
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = parent.Wait(); close(done) }()
	t.Cleanup(func() { _ = parent.Process.Kill(); <-done })
	var descendant *exec.Cmd
	if withDescendant {
		descendant = exec.Command("/bin/sh", "-c", "exec sleep 60")
		descendant.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: parent.Process.Pid}
		if err := descendant.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if descendant.ProcessState == nil {
				_ = descendant.Process.Kill()
				_ = descendant.Wait()
			}
		})
	}
	return parent, descendant, done
}

func TestTreeWaitForExitAfterParentCompletion(t *testing.T) {
	parent, descendant, parentDone := startTestGroup(t, true)
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-parentDone
	if (Tree{}).WaitForExit(parentDone, parent.Process, 30*time.Millisecond) {
		t.Fatal("parent completion hid a live descendant")
	}
	if err := descendant.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = descendant.Wait()
	if !(Tree{}).WaitForExit(parentDone, parent.Process, time.Second) {
		t.Fatal("reaped group did not complete")
	}
}

func TestTreeWaitForExitParentCompletesDuringWait(t *testing.T) {
	parent, descendant, parentDone := startTestGroup(t, true)
	done := make(chan bool, 1)
	go func() { done <- (Tree{}).WaitForExit(parentDone, parent.Process, 2*time.Second) }()
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-parentDone
	select {
	case result := <-done:
		t.Fatalf("returned with live descendant: %v", result)
	case <-time.After(2 * processGroupPollInterval):
	}
	if err := descendant.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = descendant.Wait()
	select {
	case result := <-done:
		if !result {
			t.Fatal("descendant exit did not complete wait")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("wait did not finish")
	}
}

func TestTreeWaitForExitRequiresParentAndGroup(t *testing.T) {
	parent, _, parentDone := startTestGroup(t, false)
	if (Tree{}).WaitForExit(nil, parent.Process, 20*time.Millisecond) {
		t.Fatal("live group completed without parent exit")
	}
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-parentDone
	if (Tree{}).WaitForExit(make(chan struct{}), parent.Process, 20*time.Millisecond) {
		t.Fatal("group disappearance replaced parent completion")
	}
	if !(Tree{}).WaitForExit(parentDone, parent.Process, time.Second) {
		t.Fatal("completed parent and group did not return")
	}
}

func TestTreeWaitForExitWithoutProcess(t *testing.T) {
	done := make(chan struct{})
	if (Tree{}).WaitForExit(done, nil, time.Millisecond) {
		t.Fatal("unfinished parent completed")
	}
	close(done)
	if !(Tree{}).WaitForExit(done, nil, time.Second) {
		t.Fatal("completed parent timed out")
	}
}
