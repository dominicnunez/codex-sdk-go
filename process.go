package codex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	defaultBinaryName  = "codex"
	processGracePeriod = 3 * time.Second
)

// ProcessOptions configures how the Codex CLI process is spawned.
type ProcessOptions struct {
	// Path to the codex binary. If empty, "codex" is resolved from PATH.
	BinaryPath string

	// Extra arguments passed after "codex exec --experimental-json".
	ExecArgs []string

	// Environment variables for the child process. Nil inherits the parent environment.
	Env []string

	// Working directory for the child process. Empty inherits the parent.
	Dir string

	// Stderr writer for the child process. Nil defaults to os.Stderr.
	Stderr *os.File

	// Client options forwarded to NewClient.
	ClientOptions []ClientOption
}

// Process wraps a running Codex CLI child process and its connected Client.
type Process struct {
	Client    *Client
	cmd       *exec.Cmd
	transport *StdioTransport
	closeOnce sync.Once
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

	args := []string{"exec", "--experimental-json"}
	args = append(args, opts.ExecArgs...)

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
	}, nil
}

// Close stops the child process and closes the transport.
// Safe to call multiple times.
func (p *Process) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		// Close transport first to unblock any pending reads.
		if err := p.transport.Close(); err != nil {
			closeErr = err
		}

		// Try graceful interrupt, then force kill after grace period.
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Signal(os.Interrupt)

			done := make(chan struct{})
			go func() {
				_ = p.cmd.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(processGracePeriod):
				_ = p.cmd.Process.Kill()
				<-done
			}
		}
	})
	return closeErr
}

// Wait waits for the child process to exit and returns the exit error.
func (p *Process) Wait() error {
	return p.cmd.Wait()
}
