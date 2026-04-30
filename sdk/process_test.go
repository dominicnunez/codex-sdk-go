package codex_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/sdk"
)

func writeEnvDumpBinary(t *testing.T, dir, envFile string) string {
	t.Helper()

	var (
		binaryPath string
		script     string
	)

	if runtime.GOOS == "windows" {
		binaryPath = filepath.Join(dir, "fake-codex.cmd")
		script = "@echo off\r\nset > \"" + envFile + "\"\r\nexit /b 0\r\n"
	} else {
		binaryPath = filepath.Join(dir, "fake-codex")
		script = "#!/bin/sh\nenv > \"" + envFile + "\"\nexit 0\n"
	}

	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	return binaryPath
}

func readEnvFile(t *testing.T, envFile string) map[string]string {
	t.Helper()

	file, err := os.Open(envFile)
	if err != nil {
		t.Fatalf("open env file: %v", err)
	}
	defer func() { _ = file.Close() }()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan env file: %v", err)
	}
	return values
}

func requiredMinimalEnvForRuntime(t *testing.T) map[string]string {
	t.Helper()

	required := map[string]string{
		"PATH": os.Getenv("PATH"),
	}

	if runtime.GOOS == "windows" {
		required["USERPROFILE"] = filepath.Join(t.TempDir(), "profile")
		required["APPDATA"] = filepath.Join(t.TempDir(), "appdata")
		required["LOCALAPPDATA"] = filepath.Join(t.TempDir(), "localappdata")
		return required
	}

	required["HOME"] = filepath.Join(t.TempDir(), "home")
	required["TMPDIR"] = filepath.Join(t.TempDir(), "tmp")
	return required
}

func writeProcessScriptBinary(t *testing.T, dir, script string) string {
	t.Helper()

	binaryPath := filepath.Join(dir, "fake-codex")
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return binaryPath
}

func waitForRealtimeErrorMessage(t *testing.T, received <-chan codex.ThreadRealtimeErrorNotification, wantMessage string) {
	t.Helper()

	select {
	case notif := <-received:
		if notif.Message != wantMessage {
			t.Fatalf("notification message = %q, want %q", notif.Message, wantMessage)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for shutdown notification %q", wantMessage)
	}
}

func waitForFileContents(t *testing.T, path string) string {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			text := strings.TrimSpace(string(data))
			if text != "" {
				return text
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for file contents: %s", path)
	return ""
}

func pidIsAlive(pid int) bool {
	return exec.Command("sh", "-c", "kill -0 \"$1\"", "sh", strconv.Itoa(pid)).Run() == nil
}

func waitForPIDExit(t *testing.T, pid int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !pidIsAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("process %d still alive after timeout", pid)
}

// TestStartProcess verifies that StartProcess spawns a child process,
// creates a working Client, and Close cleans up properly.
func TestStartProcess(t *testing.T) {
	// Create a fake "codex" binary that reads stdin and writes a JSON-RPC response.
	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")

	// The script ignores args, reads one line from stdin, and writes a schema-valid
	// JSON-RPC initialize response back.
	script := `#!/bin/sh
read line
echo '{"jsonrpc":"2.0","id":1,"result":{"codexHome":"/tmp/codex-home","platformFamily":"unix","platformOs":"linux","userAgent":"fake-codex/0.0.1"}}'
# Keep running until killed
while true; do sleep 1; done
`
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	if err := os.WriteFile(fakeBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath:    fakeBinary,
		ClientOptions: []codex.ClientOption{codex.WithRequestTimeout(2 * time.Second)},
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	// Verify the client is usable by sending an initialize request.
	_, err = proc.Client.Initialize(ctx, codex.InitializeParams{
		ClientInfo: codex.ClientInfo{Name: "test", Version: "0.0.1"},
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Close should kill the process and clean up.
	if err := proc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestStartProcessTypedArgs verifies that typed ProcessOptions fields produce
// the correct CLI arguments passed to the child process.
func TestStartProcessTypedArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")
	argsFile := filepath.Join(dir, "args.txt")

	// Script dumps all args to a file, then exits.
	script := `#!/bin/sh
echo "$@" > ` + argsFile + `
exit 0
`
	if err := os.WriteFile(fakeBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	ctx := context.Background()
	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath:   fakeBinary,
		Model:        "o3",
		Sandbox:      codex.SandboxModeReadOnly,
		ApprovalMode: "full-auto",
		Config:       map[string]string{"key1": "val1"},
		ExecArgs:     []string{"--extra", "flag"},
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	_ = proc.Wait()
	_ = proc.Close()

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	args := string(data)

	for _, want := range []string{
		"exec", "--experimental-json",
		"--model o3",
		"--sandbox read-only",
		"--full-auto",
		"--config key1=val1",
		"--extra flag",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing expected substring %q", args, want)
		}
	}

	// Verify typed fields are emitted before ExecArgs so positional arguments
	// cannot move SDK-owned safety flags into prompt text.
	modelIdx := strings.Index(args, "--model")
	extraIdx := strings.Index(args, "--extra")
	if modelIdx > extraIdx {
		t.Errorf("typed flags should come before ExecArgs: model at %d, extra at %d", modelIdx, extraIdx)
	}
}

// TestBuildArgsDefaults verifies that empty ProcessOptions produce minimal args.
func TestBuildArgsDefaults(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")
	argsFile := filepath.Join(dir, "args.txt")

	script := `#!/bin/sh
for arg in "$@"; do echo "$arg"; done > ` + argsFile + `
exit 0
`
	if err := os.WriteFile(fakeBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	ctx := context.Background()
	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	_ = proc.Wait()
	_ = proc.Close()

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{"exec", "--experimental-json"}
	if !slices.Equal(lines, want) {
		t.Errorf("default args = %v, want %v", lines, want)
	}
}

// TestStartProcessBadBinary verifies that StartProcess returns an error
// when the binary doesn't exist.
func TestStartProcessBadBinary(t *testing.T) {
	ctx := context.Background()
	_, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath: "/nonexistent/codex-binary-that-does-not-exist",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestStartProcessBadBinaryDoesNotLeakFileDescriptors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fd-leak check requires /proc/self/fd on linux")
	}

	startFDs, err := countOpenFDs()
	if err != nil {
		t.Skipf("failed to count starting file descriptors: %v", err)
	}

	const attempts = 64
	for i := 0; i < attempts; i++ {
		_, startErr := codex.StartProcess(context.Background(), &codex.ProcessOptions{
			BinaryPath: "/nonexistent/codex-binary-that-does-not-exist",
		})
		if startErr == nil {
			t.Fatal("expected error for nonexistent binary")
		}
	}

	endFDs, err := countOpenFDs()
	if err != nil {
		t.Fatalf("count ending file descriptors: %v", err)
	}

	const maxAllowedFDGrowth = 3
	if growth := endFDs - startFDs; growth > maxAllowedFDGrowth {
		t.Fatalf("file descriptor growth = %d after %d failed starts; want <= %d", growth, attempts, maxAllowedFDGrowth)
	}
}

func TestNewProcessFromClientNilPanics(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic when NewProcessFromClient is called with nil client")
		}
		msg, ok := recovered.(error)
		if !ok {
			t.Fatalf("panic value type = %T; want error", recovered)
		}
		if msg.Error() != "process client must not be nil" {
			t.Fatalf("panic message = %q; want %q", msg.Error(), "process client must not be nil")
		}
	}()

	_ = codex.NewProcessFromClient(nil)
}

// TestStartProcessExecArgsWithEndOfOptions verifies that StartProcess rejects
// ExecArgs containing "--" (end-of-options marker), which would bypass typed flag safety.
func TestStartProcessExecArgsWithEndOfOptions(t *testing.T) {
	ctx := context.Background()
	_, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath: "/nonexistent/binary",
		ExecArgs:   []string{"--foo", "--", "--bar"},
	})
	if err == nil {
		t.Fatal("expected error when ExecArgs contains '--'")
	}
	if !strings.Contains(err.Error(), "--") && !strings.Contains(err.Error(), "end-of-options") {
		t.Errorf("error message should mention '--' or end-of-options, got: %v", err)
	}
}

// TestStartProcessExecArgsWithTypedFlags verifies that StartProcess rejects
// ExecArgs containing typed safety flags that could override critical settings.
func TestStartProcessExecArgsWithTypedFlags(t *testing.T) {
	rejectedFlags := []string{
		"--model",
		"--sandbox",
		"--config",
		"--experimental-json",
		"--ask-for-approval",
		"--full-auto",
		"--dangerously-bypass-approvals-and-sandbox",
		"-dangerously-bypass-approvals-and-sandbox",
	}

	for _, flag := range rejectedFlags {
		t.Run(flag, func(t *testing.T) {
			ctx := context.Background()
			_, err := codex.StartProcess(ctx, &codex.ProcessOptions{
				BinaryPath: "/nonexistent/binary",
				ExecArgs:   []string{"--safe-flag", flag, "value"},
			})
			if err == nil {
				t.Fatalf("expected error when ExecArgs contains %q", flag)
			}
			if !strings.Contains(err.Error(), flag) {
				t.Errorf("error message should mention %q, got: %v", flag, err)
			}
		})
	}
}

// TestStartProcessExecArgsWithTypedFlagsCombinedForm verifies that
// --flag=value combined forms are also rejected.
func TestStartProcessExecArgsWithTypedFlagsCombinedForm(t *testing.T) {
	rejectedFlags := []string{
		"--model",
		"--sandbox",
		"--config",
		"--experimental-json",
		"--dangerously-bypass-approvals-and-sandbox",
		"-dangerously-bypass-approvals-and-sandbox",
	}

	for _, flag := range rejectedFlags {
		t.Run(flag+"=value", func(t *testing.T) {
			ctx := context.Background()
			_, err := codex.StartProcess(ctx, &codex.ProcessOptions{
				BinaryPath: "/nonexistent/binary",
				ExecArgs:   []string{flag + "=evil-value"},
			})
			if err == nil {
				t.Fatalf("expected error when ExecArgs contains %q", flag+"=evil-value")
			}
			if !strings.Contains(err.Error(), flag) {
				t.Errorf("error message should mention %q, got: %v", flag, err)
			}
		})
	}
}

// TestStartProcessExecArgsWithSingleDashTypedFlags verifies that both
// single-dash long-form and current short aliases for typed safety flags are rejected.
func TestStartProcessExecArgsWithSingleDashTypedFlags(t *testing.T) {
	rejectedFlags := []string{"-model", "-sandbox", "-config", "-experimental-json", "-m", "-s", "-c", "-a"}

	for _, flag := range rejectedFlags {
		t.Run(flag, func(t *testing.T) {
			ctx := context.Background()
			_, err := codex.StartProcess(ctx, &codex.ProcessOptions{
				BinaryPath: "/nonexistent/binary",
				ExecArgs:   []string{flag, "value"},
			})
			if err == nil {
				t.Fatalf("expected error when ExecArgs contains %q", flag)
			}
			if !strings.Contains(err.Error(), "typed safety flags") {
				t.Errorf("error should mention typed safety flags, got: %v", err)
			}
		})

		t.Run(flag+"=value", func(t *testing.T) {
			ctx := context.Background()
			_, err := codex.StartProcess(ctx, &codex.ProcessOptions{
				BinaryPath: "/nonexistent/binary",
				ExecArgs:   []string{flag + "=evil-value"},
			})
			if err == nil {
				t.Fatalf("expected error when ExecArgs contains %q", flag+"=evil-value")
			}
			if !strings.Contains(err.Error(), "typed safety flags") {
				t.Errorf("error should mention typed safety flags, got: %v", err)
			}
		})
	}
}

// TestStartProcessExecArgsWithAttachedShortTypedFlags verifies that attached
// short-option values are rejected for blocked safety aliases.
func TestStartProcessExecArgsWithAttachedShortTypedFlags(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantFlag string
	}{
		{
			name:     "model short alias with attached value",
			arg:      "-mfoo",
			wantFlag: "--model",
		},
		{
			name:     "sandbox short alias with attached value",
			arg:      "-sdanger-full-access",
			wantFlag: "--sandbox",
		},
		{
			name:     "config short alias with attached value",
			arg:      "-capproval_policy=never",
			wantFlag: "--config",
		},
		{
			name:     "approval short alias with attached value",
			arg:      "-aon-request",
			wantFlag: "--ask-for-approval",
		},
		{
			name:     "sandbox-prefixed token parsed as short alias",
			arg:      "-server",
			wantFlag: "--sandbox",
		},
		{
			name:     "model-prefixed token parsed as short alias",
			arg:      "-metadata",
			wantFlag: "--model",
		},
		{
			name:     "config-prefixed token parsed as short alias",
			arg:      "-configurable",
			wantFlag: "--config",
		},
		{
			name:     "approval-prefixed token parsed as short alias",
			arg:      "-all",
			wantFlag: "--ask-for-approval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := codex.StartProcess(ctx, &codex.ProcessOptions{
				BinaryPath: "/nonexistent/binary",
				ExecArgs:   []string{tt.arg},
			})
			if err == nil {
				t.Fatalf("expected error when ExecArgs contains %q", tt.arg)
			}
			if !strings.Contains(err.Error(), "typed safety flags") {
				t.Fatalf("error should mention typed safety flags, got: %v", err)
			}
			if !strings.Contains(err.Error(), tt.wantFlag) {
				t.Fatalf("error should mention %q, got: %v", tt.wantFlag, err)
			}
		})
	}
}

func TestStartProcessApprovalModeRejectsUnknownValue(t *testing.T) {
	ctx := context.Background()
	_, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath:   "/nonexistent/binary",
		ApprovalMode: "ask",
	})
	if err == nil {
		t.Fatal("expected error for unsupported approval mode")
	}
	if !strings.Contains(err.Error(), "ApprovalMode") {
		t.Fatalf("expected error to mention ApprovalMode, got: %v", err)
	}
}

func TestStartProcessApprovalModeRejectsConfigConflict(t *testing.T) {
	ctx := context.Background()
	_, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath:   "/nonexistent/binary",
		ApprovalMode: "full-auto",
		Config: map[string]string{
			"approval_policy": "never",
		},
	})
	if err == nil {
		t.Fatal("expected error for conflicting approval inputs")
	}
	if !strings.Contains(err.Error(), "ApprovalMode conflicts") {
		t.Fatalf("expected conflict error, got: %v", err)
	}
}

// TestStartProcessExecArgsAllowsNonSafetyFlags verifies that non-overlapping
// opaque ExecArgs survive validation and only fail later at exec.
func TestStartProcessExecArgsAllowsNonSafetyFlags(t *testing.T) {
	tests := []struct {
		name     string
		execArgs []string
	}{
		{
			name:     "long form flags",
			execArgs: []string{"--some-other-flag=value", "--another=123"},
		},
		{
			name:     "short flag with attached value",
			execArgs: []string{"-xtrace"},
		},
		{
			name:     "short flag with equals value",
			execArgs: []string{"-p=/tmp/cache"},
		},
		{
			name:     "mixed positional and non-safety flags",
			execArgs: []string{"serve", "-xdebug", "--verbose"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// StartProcess will fail because the binary doesn't exist, but it
			// should fail at exec, not at flag validation.
			_, err := codex.StartProcess(ctx, &codex.ProcessOptions{
				BinaryPath: "/nonexistent/binary",
				ExecArgs:   tt.execArgs,
			})
			if err == nil {
				t.Fatal("expected error (binary not found), got nil")
			}
			// Verify we reached the exec stage (not rejected by flag validation).
			if strings.Contains(err.Error(), "typed safety flags") {
				t.Fatalf("non-safety flags should not be rejected, got: %v", err)
			}
			if !strings.Contains(err.Error(), "codex") && !strings.Contains(err.Error(), "nonexistent") &&
				!strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "not found") {
				t.Fatalf("expected exec-stage error (binary not found), got: %v", err)
			}
		})
	}
}

// TestStartProcessStartupContextCancellationDoesNotTerminateChild verifies
// that the startup context only gates StartProcess itself, not the lifetime
// of a successfully started child process.
func TestStartProcessStartupContextCancellationDoesNotTerminateChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")

	script := `#!/bin/sh
while true; do sleep 1; done
`
	if err := os.WriteFile(fakeBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	// Cancel the startup context after the child has been returned.
	cancel()

	waitErrCh := make(chan error, 1)
	go func() {
		waitErrCh <- proc.Wait()
	}()

	select {
	case err := <-waitErrCh:
		t.Fatalf("process exited after startup context cancellation: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	if err := proc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case err := <-waitErrCh:
		if err != nil {
			t.Fatalf("Wait() after Close returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for process exit after Close")
	}
}

func TestProcessWaitReturnsNilAfterSuccessfulClose(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix signal semantics")
	}

	dir := t.TempDir()
	fakeBinary := writeProcessScriptBinary(t, dir, `#!/bin/sh
trap 'exit 0' INT
while true; do sleep 1; done
`)

	proc, err := codex.StartProcess(context.Background(), &codex.ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	if err := proc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("Wait() after Close = %v, want nil", err)
	}
}

// TestProcessWait verifies that Wait returns after the process exits.
func TestProcessWait(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")

	// Script that exits immediately with code 0.
	script := `#!/bin/sh
exit 0
`
	if err := os.WriteFile(fakeBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	ctx := context.Background()
	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	if err := proc.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	// Close after Wait should be a clean no-op.
	if err := proc.Close(); err != nil {
		t.Fatalf("Close after Wait: %v", err)
	}
}

func TestStartProcessNilOptionsRequiresBinaryPath(t *testing.T) {
	ctx := context.Background()
	_, err := codex.StartProcess(ctx, nil)
	if err == nil {
		t.Fatal("expected error for missing BinaryPath")
	}
	if !strings.Contains(err.Error(), "BinaryPath") {
		t.Errorf("expected error to mention BinaryPath, got: %v", err)
	}
}

func TestStartProcessRejectsRelativeBinaryPath(t *testing.T) {
	ctx := context.Background()
	_, err := codex.StartProcess(ctx, &codex.ProcessOptions{BinaryPath: "./codex"})
	if err == nil {
		t.Fatal("expected error for relative BinaryPath")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("expected error to mention absolute path requirement, got: %v", err)
	}
}

func TestStartProcessMinimalEnvByDefault(t *testing.T) {
	t.Setenv("CODEX_SDK_GO_TEST_SECRET", "should-not-leak")
	requiredEnv := requiredMinimalEnvForRuntime(t)
	for key, value := range requiredEnv {
		t.Setenv(key, value)
	}

	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.txt")
	fakeBinary := writeEnvDumpBinary(t, dir, envFile)

	proc, err := codex.StartProcess(context.Background(), &codex.ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	_ = proc.Wait()
	_ = proc.Close()

	values := readEnvFile(t, envFile)
	if values["CODEX_SDK_GO_TEST_SECRET"] == "should-not-leak" {
		t.Fatal("child inherited secret env var by default")
	}
	for key, want := range requiredEnv {
		if got := values[key]; got != want {
			t.Fatalf("%s = %q; want %q", key, got, want)
		}
	}
}

func TestStartProcessCanInheritParentEnv(t *testing.T) {
	t.Setenv("CODEX_SDK_GO_TEST_SECRET", "expected-leak")

	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.txt")
	fakeBinary := writeEnvDumpBinary(t, dir, envFile)

	proc, err := codex.StartProcess(context.Background(), &codex.ProcessOptions{
		BinaryPath:       fakeBinary,
		InheritParentEnv: true,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	_ = proc.Wait()
	_ = proc.Close()

	values := readEnvFile(t, envFile)
	if values["CODEX_SDK_GO_TEST_SECRET"] != "expected-leak" {
		t.Fatal("child should inherit parent env when InheritParentEnv is true")
	}
}

func TestStartProcessNilContext(t *testing.T) {
	var nilCtx context.Context
	_, err := codex.StartProcess(nilCtx, &codex.ProcessOptions{})
	if !errors.Is(err, codex.ErrNilContext) {
		t.Fatalf("StartProcess(nil, ...) error = %v; want ErrNilContext", err)
	}
}

// TestEnsureInitRetryAfterFailure verifies that ensureInit retries the
// initialize handshake after a transient failure instead of latching the error.
func TestEnsureInitRetryAfterFailure(t *testing.T) {
	mock := NewMockTransport()
	mock.SetSendError(fmt.Errorf("transient network error"))

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First call should fail because Send returns an error for "initialize".
	_, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err == nil {
		t.Fatal("expected StartConversation to fail on transient init error")
	}

	// Clear the error and set up proper responses for initialize and thread/start.
	mock.SetSendError(nil)

	_ = mock.SetResponseData("initialize", validInitializeResponseData("codex-test/1.0"))
	_ = mock.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread-1")))

	// Second call should succeed, proving ensureInit retried.
	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("expected StartConversation to succeed after clearing error: %v", err)
	}

	if conv.ThreadID() != "thread-1" {
		t.Errorf("ThreadID() = %q, want %q", conv.ThreadID(), "thread-1")
	}
}

func TestEnsureInitRetryAfterInvalidInitializeResponse(t *testing.T) {
	mock := NewMockTransport()
	mock.SetResponse("initialize", codex.Response{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{"codexHome":"/tmp/codex-home","platformOs":"linux","userAgent":"codex-test/1.0"}`),
	})

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err == nil {
		t.Fatal("expected StartConversation to fail on invalid initialize response")
	}
	if !strings.Contains(err.Error(), "missing platformFamily") {
		t.Fatalf("error = %q, want missing platformFamily", err.Error())
	}

	_ = mock.SetResponseData("initialize", validInitializeResponseData("codex-test/1.0"))
	_ = mock.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread-1")))

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("expected StartConversation to succeed after fixing initialize response: %v", err)
	}
	if conv.ThreadID() != "thread-1" {
		t.Errorf("ThreadID() = %q, want %q", conv.ThreadID(), "thread-1")
	}
}

// TestProcessCloseIdempotent verifies Close can be called multiple times.
func TestProcessCloseIdempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")

	script := `#!/bin/sh
exit 0
`
	if err := os.WriteFile(fakeBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	ctx := context.Background()
	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	// Wait for process to exit naturally.
	_ = proc.Wait()

	// Call Close multiple times — should not panic.
	_ = proc.Close()
	_ = proc.Close()
	_ = proc.Close()
}

// TestProcessCloseForceKill verifies that Close force-kills a process that
// ignores SIGINT after the grace period expires.
func TestProcessCloseForceKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix signal semantics")
	}

	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")

	// Script that traps SIGINT and ignores it, forcing the force-kill path.
	script := `#!/bin/sh
trap '' INT
while true; do sleep 1; done
`
	if err := os.WriteFile(fakeBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	ctx := context.Background()
	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	// Close should complete within a reasonable time even though the
	// process ignores SIGINT — the force-kill fires after the grace period.
	done := make(chan error, 1)
	go func() {
		done <- proc.Close()
	}()

	select {
	case <-done:
		// Close completed (error or nil is fine — the process was killed)
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not complete within 10s — force-kill path may be broken")
	}
}

func TestProcessCloseClosesChildStdin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")

	// Ignore SIGINT so the process only exits when Close delivers stdin EOF.
	script := `#!/bin/sh
trap '' INT
cat >/dev/null
exit 0
`
	if err := os.WriteFile(fakeBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	proc, err := codex.StartProcess(context.Background(), &codex.ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	start := time.Now()
	if err := proc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 1500*time.Millisecond {
		t.Fatalf("Close took %v; want stdin EOF shutdown well before forced kill", elapsed)
	}
}

func TestProcessCloseDrainsFinalStdoutOnSignalShutdown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix signal semantics")
	}

	dir := t.TempDir()
	fakeBinary := writeProcessScriptBinary(t, dir, `#!/bin/sh
emit_shutdown() {
  printf '%s\n' '{"jsonrpc":"2.0","method":"thread/realtime/error","params":{"threadId":"thread-1","message":"sigint"}}'
}
trap 'emit_shutdown; sleep 1; exit 0' INT
IFS= read -r line || exit 1
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"codexHome":"/tmp/codex-home","platformFamily":"unix","platformOs":"linux","userAgent":"fake-codex/0.0.1"}}'
while :; do :; done
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath:    fakeBinary,
		ClientOptions: []codex.ClientOption{codex.WithRequestTimeout(2 * time.Second)},
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	received := make(chan codex.ThreadRealtimeErrorNotification, 1)
	proc.Client.OnThreadRealtimeError(func(notif codex.ThreadRealtimeErrorNotification) {
		received <- notif
	})

	_, err = proc.Client.Initialize(ctx, codex.InitializeParams{
		ClientInfo: codex.ClientInfo{Name: "codex-sdk-go", Version: "test"},
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if err := proc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	waitForRealtimeErrorMessage(t, received, "sigint")
}

func TestProcessCloseTreatsInterruptExitCode130AsExpected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix signal semantics")
	}

	dir := t.TempDir()
	fakeBinary := writeProcessScriptBinary(t, dir, `#!/bin/sh
emit_shutdown() {
  printf '%s\n' '{"jsonrpc":"2.0","method":"thread/realtime/error","params":{"threadId":"thread-1","message":"sigint-130"}}'
}
trap 'emit_shutdown; sleep 1; exit 130' INT
IFS= read -r line || exit 1
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"codexHome":"/tmp/codex-home","platformFamily":"unix","platformOs":"linux","userAgent":"fake-codex/0.0.1"}}'
while :; do :; done
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath:    fakeBinary,
		ClientOptions: []codex.ClientOption{codex.WithRequestTimeout(2 * time.Second)},
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	received := make(chan codex.ThreadRealtimeErrorNotification, 1)
	proc.Client.OnThreadRealtimeError(func(notif codex.ThreadRealtimeErrorNotification) {
		received <- notif
	})

	_, err = proc.Client.Initialize(ctx, codex.InitializeParams{
		ClientInfo: codex.ClientInfo{Name: "codex-sdk-go", Version: "test"},
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if err := proc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	waitForRealtimeErrorMessage(t, received, "sigint-130")
}

func TestProcessCloseDrainsFinalStdoutOnStdinEOFShutdown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	fakeBinary := writeProcessScriptBinary(t, dir, `#!/bin/sh
trap '' INT
IFS= read -r line || exit 1
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"codexHome":"/tmp/codex-home","platformFamily":"unix","platformOs":"linux","userAgent":"fake-codex/0.0.1"}}'
while IFS= read -r line; do
  :
done
printf '%s\n' '{"jsonrpc":"2.0","method":"thread/realtime/error","params":{"threadId":"thread-1","message":"eof"}}'
sleep 1
exit 0
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath:    fakeBinary,
		ClientOptions: []codex.ClientOption{codex.WithRequestTimeout(2 * time.Second)},
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	received := make(chan codex.ThreadRealtimeErrorNotification, 1)
	proc.Client.OnThreadRealtimeError(func(notif codex.ThreadRealtimeErrorNotification) {
		received <- notif
	})

	_, err = proc.Client.Initialize(ctx, codex.InitializeParams{
		ClientInfo: codex.ClientInfo{Name: "codex-sdk-go", Version: "test"},
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if err := proc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	waitForRealtimeErrorMessage(t, received, "eof")
}

func TestProcessCloseKillsSpawnedDescendants(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix process-group semantics")
	}

	dir := t.TempDir()
	childPIDFile := filepath.Join(dir, "child.pid")
	fakeBinary := writeProcessScriptBinary(t, dir, `#!/bin/sh
trap 'exit 0' INT
sleep 1000 &
echo "$!" > "`+childPIDFile+`"
while :; do sleep 1; done
`)

	proc, err := codex.StartProcess(context.Background(), &codex.ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	childPIDText := waitForFileContents(t, childPIDFile)
	childPID, err := strconv.Atoi(childPIDText)
	if err != nil {
		t.Fatalf("parse child pid %q: %v", childPIDText, err)
	}
	if !pidIsAlive(childPID) {
		t.Fatalf("spawned child %d is not running", childPID)
	}

	if err := proc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	waitForPIDExit(t, childPID)
}

// TestStartProcessCustomStderr verifies stderr redirection.
func TestStartProcessCustomStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")
	stderrFile := filepath.Join(dir, "stderr.log")

	script := `#!/bin/sh
echo "error output" >&2
exit 0
`
	if err := os.WriteFile(fakeBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	f, err := os.Create(stderrFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	ctx := context.Background()
	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
		BinaryPath: fakeBinary,
		Stderr:     f,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	_ = proc.Wait()
	_ = proc.Close()
	_ = f.Close()

	data, err := os.ReadFile(stderrFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("stderr file is empty, expected error output")
	}
}

func TestStartProcessNilStderrDoesNotForwardToParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")
	parentStderr := filepath.Join(dir, "parent-stderr.log")

	script := `#!/bin/sh
echo "sensitive child stderr" >&2
exit 0
`
	if err := os.WriteFile(fakeBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	f, err := os.Create(parentStderr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	originalStderr := os.Stderr
	os.Stderr = f
	defer func() { os.Stderr = originalStderr }()

	proc, err := codex.StartProcess(context.Background(), &codex.ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	_ = proc.Wait()
	_ = proc.Close()
	_ = f.Close()

	data, err := os.ReadFile(parentStderr)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("parent stderr captured %q; want empty output", string(data))
	}
}

// delayedCountingTransport delays initialize responses and counts how many
// times each method is called. Used to test that ensureInit serializes
// concurrent callers so only one initialize request is sent.
type delayedCountingTransport struct {
	delay      time.Duration
	callCounts sync.Map // method → *atomic.Int32
	reqHandler codex.RequestHandler
	mu         sync.Mutex
}

func (d *delayedCountingTransport) Send(ctx context.Context, req codex.Request) (codex.Response, error) {
	counter, _ := d.callCounts.LoadOrStore(req.Method, &atomic.Int32{})
	counter.(*atomic.Int32).Add(1)

	if req.Method == "initialize" {
		select {
		case <-time.After(d.delay):
		case <-ctx.Done():
			return codex.Response{}, ctx.Err()
		}
	}

	return codex.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  json.RawMessage(`{"codexHome":"/tmp/codex-home","platformFamily":"unix","platformOs":"linux","userAgent":"test"}`),
	}, nil
}

func (d *delayedCountingTransport) Notify(_ context.Context, _ codex.Notification) error { return nil }
func (d *delayedCountingTransport) OnRequest(h codex.RequestHandler) {
	d.mu.Lock()
	d.reqHandler = h
	d.mu.Unlock()
}
func (d *delayedCountingTransport) OnNotify(_ codex.NotificationHandler) {}
func (d *delayedCountingTransport) Close() error                         { return nil }

func (d *delayedCountingTransport) methodCount(method string) int32 {
	counter, ok := d.callCounts.Load(method)
	if !ok {
		return 0
	}
	return counter.(*atomic.Int32).Load()
}

// TestEnsureInitConcurrentCallers verifies that two goroutines calling
// StartConversation concurrently on a fresh Process result in exactly one
// initialize request (ensureInit serializes callers via initMu).
func TestEnsureInitConcurrentCallers(t *testing.T) {
	transport := &delayedCountingTransport{delay: 50 * time.Millisecond}

	// Set up a thread/start response so StartConversation succeeds.
	transport.callCounts.LoadOrStore("thread/start", &atomic.Int32{})

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const concurrency = 5
	errs := make(chan error, concurrency)

	// Override the thread/start response to return valid data.
	origSend := transport.Send
	_ = origSend // used via the transport interface

	var startBarrier sync.WaitGroup
	startBarrier.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			startBarrier.Done()
			startBarrier.Wait() // all goroutines start ~simultaneously
			_, err := proc.StartConversation(ctx, codex.ConversationOptions{})
			errs <- err
		}()
	}

	// Collect results — we expect thread/start to fail because our transport
	// returns a minimal JSON result, but ensureInit should succeed.
	var initFailed bool
	for i := 0; i < concurrency; i++ {
		err := <-errs
		if err != nil && strings.Contains(err.Error(), "initialize") {
			initFailed = true
		}
	}

	if initFailed {
		t.Fatal("initialize failed unexpectedly")
	}

	initCount := transport.methodCount("initialize")
	if initCount != 1 {
		t.Errorf("expected exactly 1 initialize call, got %d", initCount)
	}
}

func TestStartConversationAfterManualInitializeDoesNotReinitialize(t *testing.T) {
	mock := NewMockTransport()
	_ = mock.SetResponseData("initialize", validInitializeResponseData("codex-test/1.0"))
	_ = mock.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread-1")))

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := proc.Client.Initialize(ctx, codex.InitializeParams{
		ClientInfo: codex.ClientInfo{Name: "test-client", Version: "1.0.0"},
		Capabilities: &codex.InitializeCapabilities{
			ExperimentalAPI:           true,
			OptOutNotificationMethods: []string{"thread/started"},
		},
	}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}
	defer func() { _ = conv.Close() }()

	if conv.ThreadID() != "thread-1" {
		t.Fatalf("ThreadID() = %q, want %q", conv.ThreadID(), "thread-1")
	}
	if got := mock.MethodCallCount("initialize"); got != 1 {
		t.Fatalf("initialize call count = %d, want 1", got)
	}
}

// TestRunAfterTransportClose verifies that calling Run on a Process whose
// transport has been closed produces a clear error.
func TestRunAfterTransportClose(t *testing.T) {
	mock := NewMockTransport()

	_ = mock.SetResponseData("initialize", validInitializeResponseData("codex-test/1.0"))
	fixture := validProcessThreadStartResponse(validProcessThreadPayload("thread-1"))
	fixture["thread"].(map[string]interface{})["ephemeral"] = false
	_ = mock.SetResponseData("thread/start", fixture)

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()

	// StartConversation initializes and creates a thread (no turn wait).
	_, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	// Close the transport to simulate process shutdown.
	_ = mock.Close()

	// Run should fail because the transport is closed.
	_, err = proc.Run(ctx, codex.RunOptions{Prompt: "after close"})
	if err == nil {
		t.Fatal("expected error from Run after transport close, got nil")
	}
}

func TestStartConversationOnZeroValueProcessReturnsError(t *testing.T) {
	var proc codex.Process
	_, err := proc.StartConversation(context.Background(), codex.ConversationOptions{})
	if err == nil {
		t.Fatal("expected error from StartConversation on zero-value Process")
	}
	if !strings.Contains(err.Error(), "process client must not be nil") {
		t.Fatalf("error = %v; want nil-client error", err)
	}
}

func countOpenFDs() (int, error) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}
