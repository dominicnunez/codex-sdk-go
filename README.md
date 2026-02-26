# codex-sdk-go

Idiomatic Go SDK for the [OpenAI Codex CLI](https://github.com/openai/codex) JSON-RPC 2.0 protocol. Stdlib only, zero external dependencies.

Built against the [protocol schemas](specs/) extracted from the Codex TypeScript source — full coverage of all 38 request methods, 40+ notification types, and 9 server→client approval flows.

## Installation

```go
import codex "github.com/dominicnunez/codex-sdk-go"
```

## Requirements

Go 1.22+

## Quick Start

```go
transport := codex.NewStdioTransport(os.Stdin, os.Stdout)
defer transport.Close()

client := codex.NewClient(transport, codex.WithRequestTimeout(30*time.Second))

// Initialize handshake
initResp, err := client.Initialize(ctx, codex.InitializeParams{
	ClientInfo: codex.ClientInfo{
		Name:    "my-codex-client",
		Version: "1.0.0",
	},
})

// Listen for streaming events
client.OnAgentMessageDelta(func(notif codex.AgentMessageDeltaNotification) {
	fmt.Print(notif.Delta)
})

// Start a thread and turn
threadResp, _ := client.Thread.Start(ctx, codex.ThreadStartParams{
	Model: codex.Ptr("gpt-4"),
})

client.Turn.Start(ctx, codex.TurnStartParams{
	ThreadID: threadResp.Thread.ID,
	Input: []codex.UserInput{
		&codex.TextUserInput{Text: "What is the capital of France?"},
	},
})
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

Built from 150+ JSON schemas extracted from the [OpenAI Codex CLI](https://github.com/openai/codex) TypeScript source. This is an unofficial community SDK.

## Contributing

Issues and PRs welcome on GitHub.
