package codex

import (
	"errors"
	"os/exec"
	"runtime"
	"testing"
)

func TestIsSignalError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal-based exit errors require unix")
	}

	t.Run("nil error", func(t *testing.T) {
		if isSignalError(nil) {
			t.Error("nil error should not be a signal error")
		}
	})

	t.Run("non-ExitError", func(t *testing.T) {
		if isSignalError(errors.New("something broke")) {
			t.Error("plain error should not be a signal error")
		}
	})

	t.Run("signal-killed process", func(t *testing.T) {
		// Spawn a process and kill it with a signal to produce a real ExitError.
		cmd := exec.Command("sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}

		// Kill produces SIGKILL → Wait returns an ExitError with !Exited().
		if err := cmd.Process.Kill(); err != nil {
			t.Fatalf("kill: %v", err)
		}

		err := cmd.Wait()
		if err == nil {
			t.Fatal("expected error from killed process")
		}

		if !isSignalError(err) {
			t.Errorf("signal-killed process error should be detected: %v", err)
		}
	})

	t.Run("normal nonzero exit", func(t *testing.T) {
		cmd := exec.Command("sh", "-c", "exit 1")
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected error from exit 1")
		}

		if isSignalError(err) {
			t.Errorf("normal exit(1) should not be a signal error: %v", err)
		}
	})
}
