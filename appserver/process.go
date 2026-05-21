package appserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	codextransport "github.com/dominicnunez/codex-sdk-go/appserver/transport"
	processctl "github.com/dominicnunez/codex-sdk-go/internal/process"
)

const (
	defaultBinaryName       = "codex"
	appServerCommand        = "app-server"
	appServerListenFlag     = "--listen"
	appServerStdioListen    = "stdio://"
	appServerGracePeriod    = 3 * time.Second
	appServerEOFGracePeriod = 100 * time.Millisecond
)

var errNilProcessClient = errors.New("app-server process client must not be nil")

type processShutdownMode = processctl.ShutdownMode

const (
	processShutdownModeUnset    = processctl.ShutdownModeUnset
	processShutdownModeGraceful = processctl.ShutdownModeGraceful
)

type processShutdownAttempt = processctl.ShutdownAttempt

const (
	processShutdownAttemptInterrupt = processctl.ShutdownAttemptInterrupt
	processShutdownAttemptKill      = processctl.ShutdownAttemptKill
)

// ProcessOptions configures how the Codex app-server process is spawned.
type ProcessOptions struct {
	// Path to the codex binary. Must be an absolute path to a trusted binary.
	// Relative paths and PATH lookup are rejected to avoid binary hijacking.
	BinaryPath string

	// Environment variables for the child process. Nil uses a minimal allowlist
	// from the parent environment unless InheritParentEnv is true. Use Env for
	// sensitive values supported by the child process; command-line arguments can
	// be visible in process listings.
	Env []string

	// InheritParentEnv forwards the full parent environment to the child when
	// Env is nil. Disabled by default to avoid leaking unrelated secrets.
	InheritParentEnv bool

	// Working directory for the child process. Empty inherits the parent.
	Dir string

	// Stderr writer for the child process. Nil discards child stderr output.
	Stderr io.Writer

	// Client options forwarded to NewClient.
	ClientOptions []ClientOption

	// InitializeParams overrides the default initialize handshake.
	// Nil uses the SDK default client info.
	InitializeParams *InitializeParams
}

// Process wraps a running Codex app-server child process and its connected Client.
type Process struct {
	Client *Client

	cmd              *exec.Cmd
	transport        *codextransport.StdioTransport
	stdin            io.Closer
	initializeParams InitializeParams
	processTree      processctl.Tree
	closeOnce        sync.Once
	waitOnce         sync.Once
	waitErr          error
	waitDone         chan struct{}
	shutdownMode     processShutdownMode
	shutdownStep     atomic.Uint32

	initNotifyMu   sync.Mutex
	initNotifyDone bool
	initNotifyWait chan struct{}
}

// NewProcessFromClient wraps an existing app-server Client in a Process. This
// is useful for tests or for callers that manage the runtime process lifecycle
// outside this package. Close on the returned Process is a no-op because there
// is no child process owned by the wrapper.
func NewProcessFromClient(client *Client) *Process {
	if client == nil {
		panic(errNilProcessClient)
	}
	done := make(chan struct{})
	close(done)
	return &Process{
		Client:           client,
		waitDone:         done,
		initializeParams: defaultInitializeParams(),
	}
}

var commonChildEnvKeys = []string{
	"HOME",
	"LANG",
	"LC_ALL",
	"LC_CTYPE",
	"PATH",
	"TEMP",
	"TMP",
	"TMPDIR",
}

var unixChildEnvKeys = []string{
	"SHELL",
	"USER",
}

var windowsChildEnvKeys = []string{
	"APPDATA",
	"LOCALAPPDATA",
	"PATHEXT",
	"PROGRAMDATA",
	"SYSTEMDRIVE",
	"SYSTEMROOT",
	"USERPROFILE",
	"USERNAME",
}

// StartProcess spawns "codex app-server --listen stdio://" as a child process
// and connects a StdioTransport to its stdin/stdout. High-level helper methods
// initialize lazily; direct Client users should call Initialize before using v2
// app-server methods.
func StartProcess(ctx context.Context, opts *ProcessOptions) (*Process, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if opts == nil {
		opts = &ProcessOptions{}
	}

	args := opts.buildArgs()
	binary, err := resolveBinaryPath(opts.BinaryPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(context.WithoutCancel(ctx), binary, args...)
	processctl.ConfigureCommand(cmd)
	cmd.Env = resolveProcessEnv(opts)
	cmd.Dir = opts.Dir
	cmd.Stderr = opts.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stdin.Close()
		return nil, fmt.Errorf("start codex app-server: %w", err)
	}

	processTree, err := processctl.AttachTree(cmd)
	if err != nil {
		_ = stdout.Close()
		_ = stdin.Close()
		stopStartedCommand(cmd)
		return nil, fmt.Errorf("attach process tree: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = stdout.Close()
		_ = stdin.Close()
		stopStartedCommand(cmd)
		_ = processTree.Close()
		return nil, err
	}

	transport := codextransport.NewStdioTransport(stdout, stdin)
	client := NewClient(transport, opts.ClientOptions...)

	return &Process{
		Client:           client,
		cmd:              cmd,
		transport:        transport,
		stdin:            stdin,
		initializeParams: resolveProcessInitializeParams(opts),
		processTree:      processTree,
		waitDone:         make(chan struct{}),
		shutdownMode:     defaultProcessShutdownMode(),
	}, nil
}

func (opts *ProcessOptions) buildArgs() []string {
	return []string{appServerCommand, appServerListenFlag, appServerStdioListen}
}

func defaultInitializeParams() InitializeParams {
	return InitializeParams{
		ClientInfo: ClientInfo{Name: "codex-sdk-go", Version: sdkVersion},
	}
}

func resolveProcessInitializeParams(opts *ProcessOptions) InitializeParams {
	if opts == nil || opts.InitializeParams == nil {
		return defaultInitializeParams()
	}
	return cloneInitializeParams(*opts.InitializeParams)
}

func cloneInitializeParams(params InitializeParams) InitializeParams {
	cp := params
	cp.ClientInfo = params.ClientInfo
	cp.ClientInfo.Title = cloneStringPtr(params.ClientInfo.Title)
	if params.Capabilities != nil {
		capabilities := *params.Capabilities
		capabilities.OptOutNotificationMethods = append([]string(nil), params.Capabilities.OptOutNotificationMethods...)
		cp.Capabilities = &capabilities
	}
	return cp
}

func resolveBinaryPath(binaryPath string) (string, error) {
	if binaryPath == "" {
		return "", fmt.Errorf("BinaryPath is required and must be an absolute path to %q", defaultBinaryName)
	}
	if !filepath.IsAbs(binaryPath) {
		return "", fmt.Errorf("BinaryPath must be absolute: %q", binaryPath)
	}
	return filepath.Clean(binaryPath), nil
}

func resolveProcessEnv(opts *ProcessOptions) []string {
	if opts.Env != nil {
		return opts.Env
	}
	if opts.InheritParentEnv {
		return os.Environ()
	}
	return minimalChildEnv()
}

func minimalChildEnv() []string {
	return minimalChildEnvForGOOS(runtime.GOOS, os.LookupEnv)
}

func minimalChildEnvForGOOS(goos string, lookupEnv func(string) (string, bool)) []string {
	keys := defaultChildEnvKeysForGOOS(goos)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		if val, ok := lookupEnv(key); ok {
			env = append(env, key+"="+val)
		}
	}
	return env
}

func defaultChildEnvKeysForGOOS(goos string) []string {
	keys := append([]string{}, commonChildEnvKeys...)
	switch goos {
	case "windows":
		return append(keys, windowsChildEnvKeys...)
	default:
		return append(keys, unixChildEnvKeys...)
	}
}

func stopStartedCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

// Initialize performs the app-server initialize handshake and sends the
// follow-up initialized notification expected by the app-server lifecycle.
// Successful calls are idempotent; failed calls can be retried.
func (p *Process) Initialize(ctx context.Context) (InitializeResponse, error) {
	if err := validateContext(ctx); err != nil {
		return InitializeResponse{}, err
	}
	if p.Client == nil {
		return InitializeResponse{}, errNilProcessClient
	}

	params := p.initializeParams
	if params.ClientInfo.Name == "" && params.ClientInfo.Version == "" && params.Capabilities == nil {
		params = defaultInitializeParams()
	}

	resp, err := p.Client.Initialize(ctx, params)
	if err != nil {
		return InitializeResponse{}, fmt.Errorf("initialize: %w", err)
	}
	if err := p.notifyInitialized(ctx); err != nil {
		return InitializeResponse{}, fmt.Errorf("initialized notification: %w", err)
	}
	return resp, nil
}

func (p *Process) ensureInit(ctx context.Context) error {
	_, err := p.Initialize(ctx)
	return err
}

func (p *Process) notifyInitialized(ctx context.Context) error {
	if p.transport == nil {
		return nil
	}

	for {
		p.initNotifyMu.Lock()
		if p.initNotifyDone {
			p.initNotifyMu.Unlock()
			return nil
		}
		if wait := p.initNotifyWait; wait != nil {
			p.initNotifyMu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		wait := make(chan struct{})
		p.initNotifyWait = wait
		p.initNotifyMu.Unlock()

		err := p.transport.Notify(ctx, Notification{Method: protocol.NotifyInitialized})

		p.initNotifyMu.Lock()
		if err == nil {
			p.initNotifyDone = true
		}
		p.initNotifyWait = nil
		close(wait)
		p.initNotifyMu.Unlock()

		return err
	}
}

// Close stops the child process, waits for its stdout reader to drain, and
// then closes the transport. Safe to call multiple times.
func (p *Process) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		closeErr = errors.Join(closeErr, p.closeStdin())
		closeErr = errors.Join(closeErr, p.closeChildProcess())
		closeErr = errors.Join(closeErr, p.closeTransport())
	})
	return closeErr
}

func (p *Process) closeStdin() error {
	if p.stdin == nil {
		return nil
	}
	if err := p.stdin.Close(); err != nil {
		if p.waitCompleted() || isExpectedPipeCloseError(err) {
			p.stdin = nil
			return nil
		}
		return fmt.Errorf("close stdin: %w", err)
	}
	p.stdin = nil
	return nil
}

func (p *Process) closeTransport() error {
	if p.transport == nil {
		return nil
	}
	return p.transport.Close()
}

func (p *Process) closeChildProcess() (closeErr error) {
	if p.cmd == nil || p.cmd.Process == nil {
		return p.processTree.Close()
	}

	process := p.cmd.Process
	shutdownMode := p.effectiveShutdownMode()
	defer func() {
		closeErr = errors.Join(closeErr, p.processTree.Close())
	}()

	go p.doWait()

	if shutdownMode == processShutdownModeGraceful {
		closeErr = errors.Join(closeErr, p.signalProcessShutdown(process))
	}
	closeErr = errors.Join(closeErr, p.waitForProcessExit(process, shutdownMode))
	if p.waitCompleted() {
		p.waitForTransportReaderStop()
	}
	return errors.Join(closeErr, p.processExitError())
}

func defaultProcessShutdownMode() processShutdownMode {
	return processctl.DefaultShutdownMode()
}

func (p *Process) effectiveShutdownMode() processShutdownMode {
	if p.shutdownMode != processShutdownModeUnset {
		return p.shutdownMode
	}
	return defaultProcessShutdownMode()
}

func (p *Process) signalProcessShutdown(process *os.Process) error {
	if err := p.processTree.RequestShutdown(process); err != nil && !isExpectedProcessStopError(err) {
		return fmt.Errorf("signal process: %w", err)
	}
	p.recordShutdownAttempt(processShutdownAttemptInterrupt)
	return nil
}

func (p *Process) waitForProcessExit(process *os.Process, shutdownMode processShutdownMode) error {
	gracePeriod := appServerEOFGracePeriod
	if shutdownMode == processShutdownModeGraceful {
		gracePeriod = appServerGracePeriod
	}

	if p.processTree.WaitForExit(p.waitDone, process, gracePeriod) {
		return nil
	}

	if err := p.processTree.ForceKill(process); err != nil && !isExpectedProcessStopError(err) {
		return fmt.Errorf("kill process: %w", err)
	}
	p.recordShutdownAttempt(processShutdownAttemptKill)
	<-p.waitDone
	return nil
}

func (p *Process) processExitError() error {
	if p.waitErr == nil {
		return nil
	}
	if processctl.IsExpectedShutdownWaitError(p.waitErr, p.recordedShutdownAttempt()) {
		return nil
	}
	return p.waitErr
}

func (p *Process) recordShutdownAttempt(attempt processShutdownAttempt) {
	for {
		current := p.recordedShutdownAttempt()
		if current >= attempt {
			return
		}
		if p.shutdownStep.CompareAndSwap(uint32(current), uint32(attempt)) {
			return
		}
	}
}

func (p *Process) recordedShutdownAttempt() processShutdownAttempt {
	return processShutdownAttempt(p.shutdownStep.Load())
}

func (p *Process) waitForTransportReaderStop() {
	if p.transport == nil {
		return
	}
	<-p.transport.ReaderStopped()
}

func (p *Process) doWait() {
	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
		close(p.waitDone)
	})
}

func isExpectedProcessStopError(err error) bool {
	return processctl.IsExpectedStopError(err)
}

func isExpectedPipeCloseError(err error) bool {
	return errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EPIPE)
}

func (p *Process) waitCompleted() bool {
	if p.waitDone == nil {
		return false
	}
	select {
	case <-p.waitDone:
		return true
	default:
		return false
	}
}

// Wait waits for the child process to exit and returns the exit error.
// Safe to call concurrently with Close.
func (p *Process) Wait() error {
	if p.cmd == nil {
		return nil
	}
	p.doWait()
	return p.processExitError()
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	return nil
}
