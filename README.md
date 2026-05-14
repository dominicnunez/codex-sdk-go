# codex-sdk-go

Idiomatic Go SDK for the [OpenAI Codex](https://github.com/openai/codex) JSON-RPC 2.0 protocol. Stdlib only, zero external dependencies.

Built against the [Codex app-server protocol schemas](specs/) — full coverage of all current request methods, 40+ notification types, and 9 server→client approval flows.

## Installation

```go
import codex "github.com/dominicnunez/codex-sdk-go/sdk"
```

## Requirements

Go 1.25+

## Quick Start

This SDK is protocol-only. It provides typed JSON-RPC requests, notifications,
responses, and approval handlers over a caller-provided `codex.Transport`.

Process management, stdio framing, WebSocket framing, and other runtime concerns
are intentionally outside this package.

```go
func run(ctx context.Context, transport codex.Transport) error {
	client := codex.NewClient(transport, codex.WithRequestTimeout(30*time.Second))

	// Initialize handshake
	_, err := client.Initialize(ctx, codex.InitializeParams{
		ClientInfo: codex.ClientInfo{
			Name:    "my-codex-client",
			Version: "1.0.0",
		},
	})
	if err != nil {
		return err
	}

	// Listen for protocol notifications
	client.OnAgentMessageDelta(func(notif codex.AgentMessageDeltaNotification) {
		fmt.Print(notif.Delta)
	})

	// Start a thread and turn
	threadResp, err := client.Thread.Start(ctx, codex.ThreadStartParams{
		Model: codex.Ptr("gpt-4"),
	})
	if err != nil {
		return err
	}

	_, err = client.Turn.Start(ctx, codex.TurnStartParams{
		ThreadID: threadResp.Thread.ID,
		Input: []codex.UserInput{
			&codex.TextUserInput{Text: "What is the capital of France?"},
		},
	})
	return err
}
```

## Approval Handlers

Codex is bidirectional — the server sends requests back to the client for approval. Register handlers to respond:

```go
client.SetApprovalHandlers(codex.ApprovalHandlers{
	OnFileChangeRequestApproval: func(ctx context.Context, params codex.FileChangeRequestApprovalParams) (codex.FileChangeRequestApprovalResponse, error) {
		return codex.FileChangeRequestApprovalResponse{Decision: "accept"}, nil
	},
	OnCommandExecutionRequestApproval: func(ctx context.Context, params codex.CommandExecutionRequestApprovalParams) (codex.CommandExecutionRequestApprovalResponse, error) {
		return codex.CommandExecutionRequestApprovalResponse{
			Decision: codex.CommandExecutionApprovalDecisionWrapper{Value: "accept"},
		}, nil
	},
})
```

Unhandled approval types return JSON-RPC method-not-found (`-32601`).

## Architecture

JSON-RPC 2.0 over a pluggable transport layer. The protocol is bidirectional:
- **Client → Server:** Requests and notifications
- **Server → Client:** Approval requests and streaming notifications

Services: `client.Thread`, `client.Turn`, `client.Account`, `client.Config`, `client.Model`, `client.Skills`, `client.Apps`, `client.Mcp`, `client.Command`, `client.Review`, `client.Feedback`, `client.ExternalAgent`, `client.Experimental`, `client.System`

## Origin

Built from 150+ JSON schemas in the [OpenAI Codex](https://github.com/openai/codex) app-server protocol. This is an unofficial community SDK.

## Contributing

Issues and PRs welcome on GitHub.

### Local Hooks

This repo uses shared Git hooks in `.githooks/`.

Install hooks once after cloning:

```bash
git config --local core.hooksPath .githooks
```

Verify the setting:

```bash
git config --local --get core.hooksPath
```

Hook behavior:

- `pre-commit`: runs `gofmt` on staged Go files, re-stages them, then runs `golangci-lint run --new`
- `pre-push`: runs `go test ./...`, `go test -race ./...`, `golangci-lint run ./...`, and `go mod tidy -diff`

To bypass hooks intentionally for a one-off operation, use Git's standard `--no-verify` flag.
