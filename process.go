package codex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"
)

const (
	defaultBinaryName = "codex"
	// processGracePeriod is how long Close waits after SIGINT before
	// sending SIGKILL. 3 seconds balances fast shutdown against giving
	// the CLI time to flush pending writes and release resources.
	processGracePeriod = 3 * time.Second

	// sdkVersion is sent to the server during initialization.
	// Update this value when cutting a new release.
	sdkVersion = "0.1.0"
)

// ProcessOptions configures how the Codex CLI process is spawned.
type ProcessOptions struct {
	// Path to the codex binary. If empty, "codex" is resolved from PATH.
	BinaryPath string

	// Extra arguments prepended before typed flags (so typed flags win via last-wins).
	// Use for forward-compat with new CLI flags not yet covered by typed fields.
	// Must not contain "--" (end-of-options marker), which would bypass typed flag safety.
	ExecArgs []string

	// Environment variables for the child process. Nil inherits the parent environment.
	Env []string

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

	// ApprovalMode sets --approval-mode (e.g. "full-auto", "suggest", "ask").
	ApprovalMode string

	// Config passes repeatable --config key=value flags.
	Config map[string]string
}

// Process wraps a running Codex CLI child process and its connected Client.
type Process struct {
	Client    *Client
	cmd       *exec.Cmd
	transport *StdioTransport
	closeOnce sync.Once
	waitOnce  sync.Once
	waitErr   error
	waitDone  chan struct{}
	initMu    sync.Mutex
	initDone  bool
}

// errEndOfOptionsInExecArgs is returned when ExecArgs contains "--", which
// would cause typed safety flags to be treated as positional arguments.
var errEndOfOptionsInExecArgs = errors.New(`ExecArgs must not contain "--" (end-of-options marker)`)

// buildArgs constructs the CLI argument list from typed fields and ExecArgs.
// ExecArgs are prepended before typed flags so that typed fields (Model,
// Sandbox, ApprovalMode, Config) always win via last-wins CLI parsing.
// This prevents untrusted ExecArgs from overriding safety-critical flags.
func (opts *ProcessOptions) buildArgs() ([]string, error) {
	for _, arg := range opts.ExecArgs {
		if arg == "--" {
			return nil, errEndOfOptionsInExecArgs
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
		args = append(args, "--approval-mode", opts.ApprovalMode)
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
	done := make(chan struct{})
	close(done)
	return &Process{Client: client, waitDone: done}
}

// StartProcess spawns "codex exec --experimental-json" as a child process,
// connects a StdioTransport to its stdin/stdout, and returns a ready-to-use Client.
// The returned Process must be closed when done.
func StartProcess(ctx context.Context, opts *ProcessOptions) (*Process, error) {
	if opts == nil {
		opts = &ProcessOptions{}
	}

	binary := opts.BinaryPath
	if binary == "" {
		binary = defaultBinaryName
	}

	args, err := opts.buildArgs()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = opts.Env
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
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start codex: %w", err)
	}

	// Process stdout is transport's reader; process stdin is transport's writer.
	transport := NewStdioTransport(stdout, stdin)
	client := NewClient(transport, opts.ClientOptions...)

	return &Process{
		Client:    client,
		cmd:       cmd,
		transport: transport,
		waitDone:  make(chan struct{}),
	}, nil
}

// Close stops the child process and closes the transport.
// Safe to call multiple times. No-op when created via NewProcessFromClient.
func (p *Process) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		// Close transport first to unblock any pending reads.
		if p.transport != nil {
			if err := p.transport.Close(); err != nil {
				closeErr = err
			}
		}

		// Try graceful interrupt, then force kill after grace period.
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Signal(os.Interrupt)

			go p.doWait()

			select {
			case <-p.waitDone:
			case <-time.After(processGracePeriod):
				_ = p.cmd.Process.Kill()
				<-p.waitDone
			}

			// Surface the process exit error unless it was caused by
			// our own interrupt/kill signal (expected during shutdown).
			if p.waitErr != nil && !isSignalError(p.waitErr) {
				closeErr = errors.Join(closeErr, p.waitErr)
			}
		}
	})
	return closeErr
}

// ensureInit runs the idempotent initialize handshake. On success the result is
// latched so future calls return immediately. On failure the next call retries,
// allowing recovery from transient errors.
func (p *Process) ensureInit(ctx context.Context) error {
	p.initMu.Lock()
	defer p.initMu.Unlock()
	if p.initDone {
		return nil
	}
	_, err := p.Client.Initialize(ctx, InitializeParams{
		ClientInfo: ClientInfo{Name: "codex-sdk-go", Version: sdkVersion},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	p.initDone = true
	return nil
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
