package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func isSignalExitError(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	return exitErr.ProcessState != nil && !exitErr.Exited()
}

func TestProcessExitErrorSurfacesUnexpectedSignalExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal-based exit errors require unix")
	}

	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	proc := &Process{
		cmd:      cmd,
		waitDone: make(chan struct{}),
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("signal: %v", err)
	}

	proc.doWait()

	if err := proc.processExitError(); err == nil {
		t.Fatal("processExitError() = nil, want unexpected signal exit")
	} else if !isSignalExitError(err) {
		t.Fatalf("processExitError() = %v, want signal exit error", err)
	}
}

func TestBuildArgsEmitsTypedFlagsBeforeExecArgs(t *testing.T) {
	opts := &ProcessOptions{
		Model:        "o3",
		Sandbox:      SandboxModeReadOnly,
		ApprovalMode: approvalModeFullAuto,
		Config: map[string]string{
			"beta":  "second",
			"alpha": "first",
		},
		ExecArgs: []string{"summarize this repository", "--verbose"},
	}

	args, err := opts.buildArgs()
	if err != nil {
		t.Fatalf("buildArgs: %v", err)
	}

	want := []string{
		"exec",
		"--experimental-json",
		"--model",
		"o3",
		"--sandbox",
		string(SandboxModeReadOnly),
		"--full-auto",
		"--config",
		"alpha=first",
		"--config",
		"beta=second",
		"summarize this repository",
		"--verbose",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("buildArgs() = %v; want %v", args, want)
	}
}

func TestBuildArgsRejectsSensitiveConfigKeys(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "api key", key: "model_providers.openai.api_key"},
		{name: "access token", key: "auth.access_token"},
		{name: "client secret", key: "oauth.clientSecret"},
		{name: "github token", key: "github_token"},
		{name: "oauth token", key: "oauth_token"},
		{name: "oauth camel token", key: "oauthToken"},
		{name: "id token", key: "auth.id_token"},
		{name: "id camel token", key: "idToken"},
		{name: "jwt", key: "jwt"},
		{name: "jwt token", key: "session.jwt_token"},
		{name: "cookie", key: "http.cookie"},
		{name: "session cookie", key: "sessionCookie"},
		{name: "authorization", key: "headers.authorization"},
		{name: "authorization header", key: "authorization_header"},
		{name: "auth header", key: "authHeader"},
		{name: "password", key: "database.password"},
		{name: "plural password", key: "database.passwords"},
		{name: "private endpoint", key: "private_endpoint"},
		{name: "plural secret", key: "app.secrets"},
		{name: "camel plural secret", key: "clientSecrets"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ProcessOptions{Config: map[string]string{tt.key: "sensitive"}}
			_, err := opts.buildArgs()
			if !errors.Is(err, errSensitiveProcessConfigKey) {
				t.Fatalf("buildArgs() error = %v; want %v", err, errSensitiveProcessConfigKey)
			}
		})
	}
}

func TestBuildArgsAllowsNonSensitiveConfigKeys(t *testing.T) {
	opts := &ProcessOptions{Config: map[string]string{
		processConfigApprovalPolicyKey:   "on-request",
		"model":                          "o3",
		"model_provider":                 "openai",
		"max_tokens":                     "1024",
		"model_auto_compact_token_limit": "200000",
		"key1":                           "val1",
	}}

	args, err := opts.buildArgs()
	if err != nil {
		t.Fatalf("buildArgs() error = %v", err)
	}

	if !slices.Contains(args, "--config") {
		t.Fatalf("args = %v; want --config flag", args)
	}
}

func TestDefaultChildEnvKeysForGOOS(t *testing.T) {
	tests := []struct {
		name       string
		goos       string
		wantKeys   []string
		rejectKeys []string
	}{
		{
			name:       "unix baseline",
			goos:       "linux",
			wantKeys:   []string{"HOME", "PATH", "TMPDIR", "SHELL", "USER"},
			rejectKeys: []string{"APPDATA", "LOCALAPPDATA", "USERPROFILE"},
		},
		{
			name:       "windows baseline",
			goos:       "windows",
			wantKeys:   []string{"HOME", "PATH", "APPDATA", "LOCALAPPDATA", "USERPROFILE", "SYSTEMROOT"},
			rejectKeys: []string{"SHELL", "USER"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := defaultChildEnvKeysForGOOS(tt.goos)
			keySet := make(map[string]struct{}, len(keys))
			for _, key := range keys {
				keySet[key] = struct{}{}
			}

			for _, key := range tt.wantKeys {
				if _, ok := keySet[key]; !ok {
					t.Fatalf("defaultChildEnvKeysForGOOS(%q) missing %q", tt.goos, key)
				}
			}
			for _, key := range tt.rejectKeys {
				if _, ok := keySet[key]; ok {
					t.Fatalf("defaultChildEnvKeysForGOOS(%q) unexpectedly includes %q", tt.goos, key)
				}
			}
		})
	}
}

func TestMinimalChildEnvForGOOSUsesPlatformAllowlist(t *testing.T) {
	lookupFrom := func(values map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) {
			value, ok := values[key]
			return value, ok
		}
	}

	t.Run("windows preserves profile and appdata", func(t *testing.T) {
		values := map[string]string{
			"PATH":         `C:\Windows\System32`,
			"USERPROFILE":  `C:\Users\kai`,
			"APPDATA":      `C:\Users\kai\AppData\Roaming`,
			"LOCALAPPDATA": `C:\Users\kai\AppData\Local`,
			"SECRET_TOKEN": "redact-me",
		}

		env := minimalChildEnvForGOOS("windows", lookupFrom(values))
		got := make(map[string]string, len(env))
		for _, entry := range env {
			key, value, ok := strings.Cut(entry, "=")
			if !ok {
				t.Fatalf("malformed env entry %q", entry)
			}
			got[key] = value
		}

		for key, want := range map[string]string{
			"PATH":         values["PATH"],
			"USERPROFILE":  values["USERPROFILE"],
			"APPDATA":      values["APPDATA"],
			"LOCALAPPDATA": values["LOCALAPPDATA"],
		} {
			if got[key] != want {
				t.Fatalf("%s = %q; want %q", key, got[key], want)
			}
		}
		if _, ok := got["SECRET_TOKEN"]; ok {
			t.Fatal("minimal child env should not include non-allowlisted variables")
		}
	})

	t.Run("unix preserves home and tmpdir", func(t *testing.T) {
		values := map[string]string{
			"HOME":         "/home/kai",
			"PATH":         "/usr/bin:/bin",
			"TMPDIR":       "/tmp/codex",
			"SECRET_TOKEN": "redact-me",
		}

		env := minimalChildEnvForGOOS("linux", lookupFrom(values))
		got := make(map[string]string, len(env))
		for _, entry := range env {
			key, value, ok := strings.Cut(entry, "=")
			if !ok {
				t.Fatalf("malformed env entry %q", entry)
			}
			got[key] = value
		}

		for key, want := range map[string]string{
			"HOME":   values["HOME"],
			"PATH":   values["PATH"],
			"TMPDIR": values["TMPDIR"],
		} {
			if got[key] != want {
				t.Fatalf("%s = %q; want %q", key, got[key], want)
			}
		}
		if _, ok := got["SECRET_TOKEN"]; ok {
			t.Fatal("minimal child env should not include non-allowlisted variables")
		}
	})
}

type errCloser struct {
	err   error
	calls int
}

func (c *errCloser) Close() error {
	c.calls++
	return c.err
}

var _ io.Closer = (*errCloser)(nil)

func TestCloseIgnoresStdinCloseErrorAfterWait(t *testing.T) {
	closer := &errCloser{err: os.ErrClosed}
	proc := &Process{
		stdin:    closer,
		waitDone: make(chan struct{}),
	}
	close(proc.waitDone)

	if err := proc.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	if closer.calls != 1 {
		t.Fatalf("stdin Close calls = %d, want 1", closer.calls)
	}
	if proc.stdin != nil {
		t.Fatal("stdin should be cleared after Close")
	}
}

func TestCloseReturnsUnexpectedStdinCloseErrorWhileRunning(t *testing.T) {
	sentinel := errors.New("boom")
	closer := &errCloser{err: sentinel}
	proc := &Process{
		stdin:    closer,
		waitDone: make(chan struct{}),
	}

	err := proc.Close()
	if !errors.Is(err, sentinel) {
		t.Fatalf("Close() error = %v, want wrapped sentinel", err)
	}
	if closer.calls != 1 {
		t.Fatalf("stdin Close calls = %d, want 1", closer.calls)
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
			Result:  json.RawMessage(`{"codexHome":"/tmp/codex-home","platformFamily":"unix","platformOs":"linux","userAgent":"codex-test/1.0"}`),
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

func TestEnsureInitWaitingCallerRejectsNilContext(t *testing.T) {
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

	var nilCtx context.Context
	if err := proc.ensureInit(nilCtx); !errors.Is(err, ErrNilContext) {
		t.Fatalf("ensureInit(nil) error = %v, want ErrNilContext", err)
	}

	close(transport.unblockInit)
	if err := <-firstDone; err != nil {
		t.Fatalf("first ensureInit failed: %v", err)
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
			Result:  json.RawMessage(`{"codexHome":"/tmp/codex-home","platformFamily":"unix","platformOs":"linux","userAgent":"codex-test/1.0"}`),
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

type recordingInitializeTransport struct {
	mu         sync.Mutex
	lastParams InitializeParams
}

func (t *recordingInitializeTransport) Send(_ context.Context, req Request) (Response, error) {
	if req.Method != methodInitialize {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if err := json.Unmarshal(req.Params, &t.lastParams); err != nil {
		return Response{}, err
	}
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  json.RawMessage(`{"codexHome":"/tmp/codex-home","platformFamily":"unix","platformOs":"linux","userAgent":"codex-test/1.0"}`),
	}, nil
}

func (t *recordingInitializeTransport) Notify(_ context.Context, _ Notification) error { return nil }
func (t *recordingInitializeTransport) OnRequest(_ RequestHandler)                     {}
func (t *recordingInitializeTransport) OnNotify(_ NotificationHandler)                 {}
func (t *recordingInitializeTransport) Close() error                                   { return nil }

func (t *recordingInitializeTransport) params() InitializeParams {
	t.mu.Lock()
	defer t.mu.Unlock()
	return cloneInitializeParams(t.lastParams)
}

func TestEnsureInitUsesConfiguredInitializeParams(t *testing.T) {
	transport := &recordingInitializeTransport{}
	client := NewClient(transport, WithRequestTimeout(5*time.Second))
	proc := NewProcessFromClient(client)
	proc.initializeParams = InitializeParams{
		ClientInfo: ClientInfo{Name: "custom-client", Version: "2.0.0"},
		Capabilities: &InitializeCapabilities{
			ExperimentalAPI:           true,
			OptOutNotificationMethods: []string{"thread/started"},
		},
	}

	if err := proc.ensureInit(context.Background()); err != nil {
		t.Fatalf("ensureInit failed: %v", err)
	}

	got := transport.params()
	if got.ClientInfo.Name != "custom-client" || got.ClientInfo.Version != "2.0.0" {
		t.Fatalf("initialize client info = %+v, want custom-client/2.0.0", got.ClientInfo)
	}
	if got.Capabilities == nil || !got.Capabilities.ExperimentalAPI {
		t.Fatalf("initialize capabilities = %+v, want experimental API enabled", got.Capabilities)
	}
	if !slices.Equal(got.Capabilities.OptOutNotificationMethods, []string{"thread/started"}) {
		t.Fatalf("optOutNotificationMethods = %v, want [thread/started]", got.Capabilities.OptOutNotificationMethods)
	}
}

func TestProcessCloseNoSignalModeSkipsLongGraceWait(t *testing.T) {
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

	proc, err := StartProcess(context.Background(), &ProcessOptions{
		BinaryPath: fakeBinary,
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}
	proc.shutdownMode = processShutdownModeNoSignal

	start := time.Now()
	if err := proc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Fatalf("Close took %v; want no-signal shutdown to skip the long grace wait", elapsed)
	}
}
