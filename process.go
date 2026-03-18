package codex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultBinaryName = "codex"
	// processGracePeriod is how long Close waits after SIGINT before
	// sending SIGKILL. 3 seconds balances fast shutdown against giving
	// the CLI time to flush pending writes and release resources.
	processGracePeriod = 3 * time.Second
	// processEOFGracePeriod gives a child a brief chance to exit after stdin
	// closes on platforms where the SDK cannot send a graceful interrupt.
	processEOFGracePeriod = 100 * time.Millisecond

	// sdkVersion is sent to the server during initialization.
	// Update this value when cutting a new release.
	sdkVersion = "0.2.0"
)

type processShutdownMode uint8

const (
	processShutdownModeUnset processShutdownMode = iota
	processShutdownModeGraceful
	processShutdownModeNoSignal
)

// ProcessOptions configures how the Codex CLI process is spawned.
type ProcessOptions struct {
	// Path to the codex binary. Must be an absolute path to a trusted binary.
	// Relative paths and PATH lookup are rejected to avoid binary hijacking.
	BinaryPath string

	// Extra arguments prepended before typed flags (so typed flags win via last-wins).
	// Use for forward-compat with new CLI flags not yet covered by typed fields.
	// Must not contain "--" (end-of-options marker), which would bypass typed flag safety.
	ExecArgs []string

	// Environment variables for the child process. Nil uses a minimal allowlist
	// from the parent environment unless InheritParentEnv is true.
	Env []string

	// InheritParentEnv forwards the full parent environment to the child when
	// Env is nil. Disabled by default to avoid leaking unrelated secrets.
	InheritParentEnv bool

	// Working directory for the child process. Empty inherits the parent.
	Dir string

	// Stderr writer for the child process. Nil defaults to os.Stderr.
	Stderr io.Writer

	// Client options forwarded to NewClient.
	ClientOptions []ClientOption

	// Model sets --model for the Codex CLI.
	Model string

	// Sandbox sets --sandbox (e.g. "read-only", "workspace-write", "danger-full-access").
	Sandbox SandboxMode

	// ApprovalMode controls approval behavior for process startup.
	// Supported value: "full-auto" (emits --full-auto).
	// For other policies, set Config["approval_policy"] directly.
	ApprovalMode string

	// Config passes repeatable --config key=value flags.
	Config map[string]string
}

// Process wraps a running Codex CLI child process and its connected Client.
type Process struct {
	Client       *Client
	cmd          *exec.Cmd
	transport    *StdioTransport
	stdin        io.Closer
	closeOnce    sync.Once
	waitOnce     sync.Once
	waitErr      error
	waitDone     chan struct{}
	initMu       sync.Mutex
	initDone     bool
	initWait     chan struct{}
	shutdownMode processShutdownMode
}

// errEndOfOptionsInExecArgs is returned when ExecArgs contains "--", which
// would cause typed safety flags to be treated as positional arguments.
var errEndOfOptionsInExecArgs = errors.New(`ExecArgs must not contain "--" (end-of-options marker)`)

var errTypedFlagInExecArgs = errors.New("ExecArgs must not contain typed safety flags")
var errNilProcessClient = errors.New("process client must not be nil")

const approvalModeFullAuto = "full-auto"

var defaultChildEnvKeys = []string{
	"HOME",
	"LANG",
	"LC_ALL",
	"LC_CTYPE",
	"PATH",
	"SHELL",
	"SYSTEMROOT",
	"TMP",
	"TEMP",
	"TMPDIR",
	"USER",
}

// rejectedExecArgFlagAliases canonicalizes all blocked safety flags (including
// short aliases and compatibility aliases) to the preferred long option.
var rejectedExecArgFlagAliases = map[string]string{
	"--model":             "--model",
	"-model":              "--model",
	"--sandbox":           "--sandbox",
	"-sandbox":            "--sandbox",
	"--config":            "--config",
	"-config":             "--config",
	"--experimental-json": "--experimental-json",
	"-experimental-json":  "--experimental-json",
	"--ask-for-approval":  "--ask-for-approval",
	"-ask-for-approval":   "--ask-for-approval",
	"--full-auto":         "--full-auto",
	"-full-auto":          "--full-auto",
	"--dangerously-bypass-approvals-and-sandbox": "--dangerously-bypass-approvals-and-sandbox",
	"-dangerously-bypass-approvals-and-sandbox":  "--dangerously-bypass-approvals-and-sandbox",
	"-m": "--model",
	"-s": "--sandbox",
	"-c": "--config",
	"-a": "--ask-for-approval",
}

var rejectedShortFlagAliases = map[string]string{
	"-m": "--model",
	"-s": "--sandbox",
	"-c": "--config",
	"-a": "--ask-for-approval",
}

func canonicalRejectedExecArgFlag(arg string) (string, bool) {
	if !strings.HasPrefix(arg, "-") || arg == "-" {
		return "", false
	}

	token := arg
	if i := strings.IndexByte(token, '='); i >= 0 {
		token = token[:i]
	}

	if canonical, ok := rejectedExecArgFlagAliases[token]; ok {
		return canonical, true
	}

	// Reject short alias forms with attached values (for example -mo3, -sread-only).
	for shortAlias, canonical := range rejectedShortFlagAliases {
		if strings.HasPrefix(token, shortAlias) && len(token) > len(shortAlias) {
			return canonical, true
		}
	}

	return "", false
}

// buildArgs constructs the CLI argument list from typed fields and ExecArgs.
// ExecArgs are prepended before typed flags so that typed fields (Model,
// Sandbox, ApprovalMode, Config) always win via last-wins CLI parsing.
// This prevents untrusted ExecArgs from overriding safety-critical flags.
func (opts *ProcessOptions) buildArgs() ([]string, error) {
	for _, arg := range opts.ExecArgs {
		if arg == "--" {
			return nil, errEndOfOptionsInExecArgs
		}
		if canonicalFlag, blocked := canonicalRejectedExecArgFlag(arg); blocked {
			return nil, fmt.Errorf("%w: %s", errTypedFlagInExecArgs, canonicalFlag)
		}
	}

	args := []string{"exec", "--experimental-json"}

	args = append(args, opts.ExecArgs...)

	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.Sandbox != "" {
		args = append(args, "--sandbox", string(opts.Sandbox))
	}
	if opts.ApprovalMode != "" {
		switch opts.ApprovalMode {
		case approvalModeFullAuto:
			args = append(args, "--full-auto")
		default:
			return nil, fmt.Errorf("ApprovalMode %q is unsupported; use %q or set Config[\"approval_policy\"]", opts.ApprovalMode, approvalModeFullAuto)
		}
	}
	keys := make([]string, 0, len(opts.Config))
	for k := range opts.Config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--config", k+"="+opts.Config[k])
	}

	return args, nil
}

// NewProcessFromClient wraps an existing Client in a Process. This is useful
// for testing or when managing the Codex CLI process lifecycle externally.
// Close on the returned Process is a no-op since there is no child process.
func NewProcessFromClient(client *Client) *Process {
	if client == nil {
		panic(errNilProcessClient)
	}
	done := make(chan struct{})
	close(done)
	return &Process{Client: client, waitDone: done}
}

// StartProcess spawns "codex exec --experimental-json" as a child process,
// connects a StdioTransport to its stdin/stdout, and returns a ready-to-use Client.
// The returned Process must be closed when done.
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

	args, err := opts.buildArgs()
	if err != nil {
		return nil, err
	}

	binary, err := resolveBinaryPath(opts.BinaryPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(context.WithoutCancel(ctx), binary, args...)
	cmd.Env = resolveProcessEnv(opts)
	cmd.Dir = opts.Dir

	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	cmd.Stderr = stderr

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
		return nil, fmt.Errorf("start codex: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = stdout.Close()
		_ = stdin.Close()
		stopStartedCommand(cmd)
		return nil, err
	}

	// Process stdout is transport's reader; process stdin is transport's writer.
	transport := NewStdioTransport(stdout, stdin)
	client := NewClient(transport, opts.ClientOptions...)

	return &Process{
		Client:       client,
		cmd:          cmd,
		transport:    transport,
		stdin:        stdin,
		waitDone:     make(chan struct{}),
		shutdownMode: defaultProcessShutdownMode(),
	}, nil
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
	env := make([]string, 0, len(defaultChildEnvKeys))
	for _, key := range defaultChildEnvKeys {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+val)
		}
	}
	return env
}

func stopStartedCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

// Close stops the child process and closes the transport.
// Safe to call multiple times. No-op when created via NewProcessFromClient.
func (p *Process) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		closeErr = errors.Join(closeErr, p.closeStdin())
		closeErr = errors.Join(closeErr, p.closeTransport())
		closeErr = errors.Join(closeErr, p.closeChildProcess())
	})
	return closeErr
}

func (p *Process) closeStdin() error {
	if p.stdin == nil {
		return nil
	}
	if err := p.stdin.Close(); err != nil {
		return fmt.Errorf("close stdin: %w", err)
	}
	return nil
}

func (p *Process) closeTransport() error {
	if p.transport == nil {
		return nil
	}
	return p.transport.Close()
}

func (p *Process) closeChildProcess() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	process := p.cmd.Process
	shutdownMode := p.effectiveShutdownMode()

	go p.doWait()

	var closeErr error
	if shutdownMode == processShutdownModeGraceful {
		closeErr = errors.Join(closeErr, p.signalProcessShutdown(process))
	}
	closeErr = errors.Join(closeErr, p.waitForProcessExit(process, shutdownMode))
	return errors.Join(closeErr, p.processExitError())
}

func (p *Process) effectiveShutdownMode() processShutdownMode {
	if p.shutdownMode != processShutdownModeUnset {
		return p.shutdownMode
	}
	return defaultProcessShutdownMode()
}

func (p *Process) signalProcessShutdown(process *os.Process) error {
	if err := requestProcessShutdown(process); err != nil && !isExpectedProcessStopError(err) {
		return fmt.Errorf("signal process: %w", err)
	}
	return nil
}

func (p *Process) waitForProcessExit(process *os.Process, shutdownMode processShutdownMode) error {
	gracePeriod := processEOFGracePeriod
	if shutdownMode == processShutdownModeGraceful {
		gracePeriod = processGracePeriod
	}

	select {
	case <-p.waitDone:
		return nil
	case <-time.After(gracePeriod):
	}

	if err := process.Kill(); err != nil && !isExpectedProcessStopError(err) {
		return fmt.Errorf("kill process: %w", err)
	}
	<-p.waitDone
	return nil
}

func (p *Process) processExitError() error {
	// Surface the process exit error unless it was caused by
	// our own interrupt/kill signal (expected during shutdown).
	if p.waitErr != nil && !isSignalError(p.waitErr) {
		return p.waitErr
	}
	return nil
}

// ensureInit runs the idempotent initialize handshake. On success the result is
// latched so future calls return immediately. On failure the next call retries,
// allowing recovery from transient errors.
func (p *Process) ensureInit(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	for {
		p.initMu.Lock()
		if p.initDone {
			p.initMu.Unlock()
			return nil
		}
		if p.Client == nil {
			p.initMu.Unlock()
			return errNilProcessClient
		}
		if wait := p.initWait; wait != nil {
			p.initMu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		wait := make(chan struct{})
		p.initWait = wait
		p.initMu.Unlock()

		_, err := p.Client.Initialize(ctx, InitializeParams{
			ClientInfo: ClientInfo{Name: "codex-sdk-go", Version: sdkVersion},
		})

		p.initMu.Lock()
		if err == nil {
			p.initDone = true
		}
		p.initWait = nil
		close(wait)
		p.initMu.Unlock()

		if err != nil {
			return fmt.Errorf("initialize: %w", err)
		}
		return nil
	}
}

// doWait runs cmd.Wait exactly once and stores the result.
func (p *Process) doWait() {
	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
		close(p.waitDone)
	})
}

// isSignalError returns true if the error is an exec.ExitError caused by a signal
// (as opposed to a non-zero exit code). Signal-caused exits are expected during
// Process.Close shutdown and should not be surfaced as errors.
func isSignalError(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	return exitErr.ProcessState != nil && !exitErr.Exited()
}

func isExpectedProcessStopError(err error) bool {
	return errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH)
}

// Wait waits for the child process to exit and returns the exit error.
// Returns nil immediately when created via NewProcessFromClient (no child process).
// Safe to call concurrently with Close.
func (p *Process) Wait() error {
	if p.cmd == nil {
		return nil
	}
	p.doWait()
	return p.waitErr
}
