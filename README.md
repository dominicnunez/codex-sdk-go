# codex-sdk-go

Idiomatic Go SDK for the [OpenAI Codex](https://github.com/openai/codex) JSON-RPC 2.0 protocol, app-server runtime helpers, exec helpers, and Codex login helpers. Stdlib only, zero external dependencies.

Built against the [Codex app-server protocol schemas](appserver/protocol/schema/json/) — full coverage of all current request methods, 40+ notification types, and 9 server→client approval flows.

## Installation

```bash
go get github.com/dominicnunez/codex-sdk-go
```

Import the package that matches the layer you need:

```go
import codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
```

## Requirements

- Go 1.25+
- A Codex CLI binary for `appserver` and `exec` process helpers
- An absolute `ProcessOptions.BinaryPath` when spawning Codex from this SDK

## Packages

| Package | Purpose | Upstream owner |
| --- | --- | --- |
| `appserver/protocol` | Typed JSON-RPC requests, responses, notifications, approval handlers, and schema coverage | `codex-rs/app-server-protocol` |
| `appserver/transport` | Newline-delimited JSON-RPC stdio transport and notification ordering | `codex-rs/app-server-transport/src/transport` |
| `appserver` | `codex app-server --listen stdio://` process startup and client lifecycle | `codex-rs/app-server`, `codex-rs/app-server-client` |
| `exec` | Single-turn `Run`, streamed turns, persistent conversations, and Codex CLI process lifecycle | `codex-rs/exec`, `codex-rs/app-server-client` |
| `login` | Codex OAuth authorization-code flow with PKCE and local/manual callback handling | `codex-rs/login` |
| `login/auth` | Credential storage, JWT claim extraction, redaction, and `chatgptAuthTokens` payload helpers | `codex-rs/login`, `codex-rs/app-server-protocol` |

## Quick Start

The `appserver/protocol` package is protocol-only. It provides typed JSON-RPC requests,
notifications, responses, and approval handlers over a caller-provided `codex.Transport`.

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

## Exec Helper

Use `exec` when you want this SDK to spawn the Codex CLI and execute a single turn.

```go
import (
	"context"
	"fmt"
	osexec "os/exec"

	codexexec "github.com/dominicnunez/codex-sdk-go/exec"
)

func runCodex(ctx context.Context) error {
	binary, err := osexec.LookPath("codex")
	if err != nil {
		return err
	}

	process, err := codexexec.StartProcess(ctx, &codexexec.ProcessOptions{
		BinaryPath: binary,
		Sandbox:    codexexec.SandboxModeWorkspaceWrite,
	})
	if err != nil {
		return err
	}
	defer process.Close()

	result, err := process.Run(ctx, codexexec.RunOptions{
		Prompt: "Summarize this repository.",
	})
	if err != nil {
		return err
	}

	fmt.Println(result.Response)
	return nil
}
```

Use `RunStreamed` for range-over-func streaming events, or `StartConversation` when you need a persistent thread across multiple turns.

## Codex OAuth

Use `login` and `login/auth` to obtain ChatGPT/Codex subscription-backed tokens and pass them to the app-server as `chatgptAuthTokens`.

```go
import (
	"context"
	"fmt"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"github.com/dominicnunez/codex-sdk-go/login"
	"github.com/dominicnunez/codex-sdk-go/login/auth"
)

func loginWithAuthTokens(ctx context.Context, client *codex.Client) error {
	creds, err := login.Login(ctx, login.LoginOptions{
		Config: login.Config{Originator: "my-codex-client"},
		OnAuthURL: func(ctx context.Context, authURL string) error {
			fmt.Println("Open this URL:", authURL)
			return nil
		},
		ManualCode: func(ctx context.Context, prompt login.AuthPrompt) (string, error) {
			fmt.Println(prompt.Message)
			var input string
			_, err := fmt.Scanln(&input)
			return input, err
		},
	})
	if err != nil {
		return err
	}

	payload, err := auth.NewAuthTokensLoginParams(creds)
	if err != nil {
		return err
	}

	_, err = client.Account.Login(ctx, &codex.ChatgptAuthTokensLoginAccountParams{
		AccessToken:      payload.AccessToken,
		ChatgptAccountId: payload.ChatGPTAccountID,
		ChatgptPlanType:  payload.ChatGPTPlanType,
	})
	return err
}
```

This is a Codex app-server auth bridge. It is not OpenAI Platform API-key auth and it does not make ChatGPT subscriptions usable with generic `api.openai.com` API endpoints.

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

Runtime helpers import the protocol package instead of redefining schema-owned
types, so `appserver/protocol/schema/json/` remains the source of truth for the app-server contract.

Process helpers reject relative binary paths and secret-bearing CLI config keys. Pass credentials through supported environment variables or `login/auth` payloads instead of command-line arguments.

## Origin

Built from 150+ JSON schemas in the [OpenAI Codex](https://github.com/openai/codex) app-server protocol and selected upstream login/runtime behavior. Local package structure follows the upstream `codex-rs/` owner paths where practical while preserving Go package boundaries. This is an unofficial community SDK.

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
