package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

type blockingInitializeTransport struct {
	unblockInit chan struct{}
	initCalls   atomic.Int32
}

func newBlockingInitializeTransport() *blockingInitializeTransport {
	return &blockingInitializeTransport{
		unblockInit: make(chan struct{}),
	}
}

func (t *blockingInitializeTransport) Send(ctx context.Context, req Request) (Response, error) {
	switch req.Method {
	case "initialize":
		t.initCalls.Add(1)
		select {
		case <-t.unblockInit:
		case <-ctx.Done():
			return Response{}, ctx.Err()
		}
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"userAgent":"codex-test/1.0"}`),
		}, nil
	default:
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}, nil
	}
}

func (t *blockingInitializeTransport) Notify(_ context.Context, _ Notification) error { return nil }
func (t *blockingInitializeTransport) OnRequest(_ RequestHandler)                     {}
func (t *blockingInitializeTransport) OnNotify(_ NotificationHandler)                 {}
func (t *blockingInitializeTransport) Close() error                                   { return nil }

func TestEnsureInitWaitingCallerRespectsContextCancellation(t *testing.T) {
	transport := newBlockingInitializeTransport()
	client := NewClient(transport, WithRequestTimeout(5*time.Second))
	proc := NewProcessFromClient(client)

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- proc.ensureInit(context.Background())
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if transport.initCalls.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if transport.initCalls.Load() == 0 {
		t.Fatal("initialize call did not start")
	}

	waiterCtx, cancelWaiter := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelWaiter()

	start := time.Now()
	err := proc.ensureInit(waiterCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ensureInit waiter error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Fatalf("waiter returned too late: %v", elapsed)
	}

	close(transport.unblockInit)
	if err := <-firstDone; err != nil {
		t.Fatalf("first ensureInit failed: %v", err)
	}

	if err := proc.ensureInit(context.Background()); err != nil {
		t.Fatalf("ensureInit after successful init returned error: %v", err)
	}
	if got := transport.initCalls.Load(); got != 1 {
		t.Fatalf("initialize call count = %d, want 1", got)
	}
}

type retryableInitializeTransport struct {
	mu              sync.Mutex
	initializeCalls int
}

func (t *retryableInitializeTransport) Send(_ context.Context, req Request) (Response, error) {
	switch req.Method {
	case "initialize":
		t.mu.Lock()
		t.initializeCalls++
		call := t.initializeCalls
		t.mu.Unlock()
		if call == 1 {
			return Response{}, fmt.Errorf("temporary init failure")
		}
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"userAgent":"codex-test/1.0"}`),
		}, nil
	default:
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}, nil
	}
}

func (t *retryableInitializeTransport) Notify(_ context.Context, _ Notification) error { return nil }
func (t *retryableInitializeTransport) OnRequest(_ RequestHandler)                     {}
func (t *retryableInitializeTransport) OnNotify(_ NotificationHandler)                 {}
func (t *retryableInitializeTransport) Close() error                                   { return nil }

func (t *retryableInitializeTransport) calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.initializeCalls
}

func TestEnsureInitFailureAllowsRetry(t *testing.T) {
	transport := &retryableInitializeTransport{}
	client := NewClient(transport, WithRequestTimeout(5*time.Second))
	proc := NewProcessFromClient(client)

	firstErr := proc.ensureInit(context.Background())
	if firstErr == nil {
		t.Fatal("expected first ensureInit to fail")
	}
	if !strings.Contains(firstErr.Error(), "initialize") {
		t.Fatalf("first ensureInit error = %v, want initialize context", firstErr)
	}

	secondErr := proc.ensureInit(context.Background())
	if secondErr != nil {
		t.Fatalf("expected second ensureInit to succeed, got: %v", secondErr)
	}
	if got := transport.calls(); got != 2 {
		t.Fatalf("initialize call count = %d, want 2", got)
	}
}
