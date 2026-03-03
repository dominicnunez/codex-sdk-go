package codex

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
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

func TestBuildArgsEmitFlagsAcceptedByCodexCLI(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex binary not available in PATH")
	}

	opts := &ProcessOptions{
		Model:        "o3",
		Sandbox:      SandboxModeReadOnly,
		ApprovalMode: "full-auto",
		Config:       map[string]string{"foo": `"bar"`},
		ExecArgs:     []string{"--help"},
	}

	args, err := opts.buildArgs()
	if err != nil {
		t.Fatalf("buildArgs: %v", err)
	}

	cmd := exec.CommandContext(context.Background(), "codex", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("codex %s failed: %v\noutput:\n%s", strings.Join(args, " "), err, string(out))
	}
}
