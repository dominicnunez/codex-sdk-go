package codex_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestStartProcess verifies that StartProcess spawns a child process,
// creates a working Client, and Close cleans up properly.
func TestStartProcess(t *testing.T) {
	// Create a fake "codex" binary that reads stdin and writes a JSON-RPC response.
	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "fake-codex")

	// The script ignores args, reads one line from stdin, and writes a valid
	// JSON-RPC initialize response back.
	script := `#!/bin/sh
read line
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2.0","capabilities":{}}}'
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
		"--approval-mode full-auto",
		"--config key1=val1",
		"--extra flag",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing expected substring %q", args, want)
		}
	}

	// Verify --extra flag comes after typed flags (user overrides last)
	modelIdx := strings.Index(args, "--model")
	extraIdx := strings.Index(args, "--extra")
	if extraIdx < modelIdx {
		t.Errorf("ExecArgs should come after typed flags: model at %d, extra at %d", modelIdx, extraIdx)
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

// TestStartProcessContextCancellation verifies that canceling the context
// causes the process to terminate.
func TestStartProcessContextCancellation(t *testing.T) {
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

	// Cancel the context — this should cause the process to be killed
	// (via exec.CommandContext).
	cancel()

	// Wait for the process to exit before calling Close.
	_ = proc.Wait()

	if err := proc.Close(); err != nil {
		// Close may return an error since the process was already killed,
		// which is acceptable behavior.
		t.Logf("Close after cancel: %v (expected)", err)
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

	// Close after Wait should be safe (idempotent).
	if err := proc.Close(); err != nil {
		t.Logf("Close after Wait: %v (expected)", err)
	}
}

// TestStartProcessNilOptions verifies that nil options use defaults.
func TestStartProcessNilOptions(t *testing.T) {
	// This will fail because "codex" isn't in PATH in CI, but it should
	// fail with a start error, not a panic.
	ctx := context.Background()
	_, err := codex.StartProcess(ctx, nil)
	if err == nil {
		t.Log("StartProcess with nil options succeeded (codex binary found in PATH)")
	}
	// Either error or success is acceptable — we just verify no panic.
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

	_ = mock.SetResponseData("initialize", map[string]interface{}{
		"userAgent": "codex-test/1.0",
	})

	_ = mock.SetResponseData("thread/start", map[string]interface{}{
		"approvalPolicy": "never",
		"cwd":            "/tmp",
		"model":          "o3",
		"modelProvider":  "openai",
		"sandbox":        map[string]interface{}{"type": "readOnly"},
		"thread": map[string]interface{}{
			"id":            "thread-1",
			"cliVersion":    "1.0.0",
			"createdAt":     1700000000,
			"cwd":           "/tmp",
			"modelProvider": "openai",
			"preview":       "",
			"source":        "exec",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     1700000000,
			"ephemeral":     true,
		},
	})

	// Second call should succeed, proving ensureInit retried.
	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("expected StartConversation to succeed after clearing error: %v", err)
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
