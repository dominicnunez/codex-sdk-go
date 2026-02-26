# codex-sdk-go

Go SDK for the OpenAI Codex CLI JSON-RPC 2.0 protocol.

**Note:** This SDK is for OpenAI's Codex CLI, not to be confused with OpenCode by Anomaly Co.

## Installation

```bash
go get github.com/dominicnunez/codex-sdk-go
```

## Requirements

- **Go 1.22+** (developed with Go 1.22; PRD specifies 1.25+ for future compatibility)
- **Zero external dependencies** (stdlib only)

## Features

- **Full Protocol Coverage**: All 38 request methods and 40+ notification types
- **Bidirectional Communication**: Handle both client→server and server→client requests
- **Type-Safe API**: Strongly-typed request/response/notification structures
- **Transport Abstraction**: Stdio transport included, extensible for WebSocket or custom transports
- **Approval Handlers**: Register callbacks for server→client approval flows
- **Thread-Safe**: Concurrent request/notification handling with proper synchronization

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func main() {
	// Create stdio transport (communicates with Codex CLI via stdin/stdout)
	transport := codex.NewStdioTransport(os.Stdin, os.Stdout)
	defer transport.Close()

	// Create client with optional request timeout
	client := codex.NewClient(transport, codex.WithRequestTimeout(30*time.Second))

	ctx := context.Background()

	// Step 1: Initialize handshake (v1 protocol)
	initResp, err := client.Initialize(ctx, codex.InitializeParams{
		ClientInfo: codex.ClientInfo{
			Name:    "my-codex-client",
			Version: "1.0.0",
		},
	})
	if err != nil {
		log.Fatalf("Initialize failed: %v", err)
	}
	fmt.Printf("Connected to %s\n", initResp.UserAgent)

	// Step 2: Register notification listeners for streaming events
	client.OnThreadStarted(func(notif codex.ThreadStartedNotification) {
		fmt.Printf("Thread started: %s\n", notif.Thread.ID)
	})

	client.OnTurnStarted(func(notif codex.TurnStartedNotification) {
		fmt.Printf("Turn started: %s\n", notif.TurnID)
	})

	client.OnTurnCompleted(func(notif codex.TurnCompletedNotification) {
		fmt.Printf("Turn completed: %s\n", notif.TurnID)
	})

	client.OnAgentMessageDelta(func(notif codex.AgentMessageDeltaNotification) {
		// Stream agent responses in real-time
		fmt.Print(notif.Delta)
	})

	// Step 3: Start a thread
	threadResp, err := client.Thread.Start(ctx, codex.ThreadStartParams{
		Model:         codex.Ptr("gpt-4"),
		ModelProvider: codex.Ptr("openai"),
	})
	if err != nil {
		log.Fatalf("Thread.Start failed: %v", err)
	}
	threadID := threadResp.Thread.ID
	fmt.Printf("Thread ID: %s\n", threadID)

	// Step 4: Start a turn (send a message)
	turnResp, err := client.Turn.Start(ctx, codex.TurnStartParams{
		ThreadID: threadID,
		UserInput: []codex.UserInput{
			&codex.TextUserInput{Text: "What is the capital of France?"},
		},
	})
	if err != nil {
		log.Fatalf("Turn.Start failed: %v", err)
	}
	fmt.Printf("Turn ID: %s\n", turnResp.TurnID)

	// The client will receive streaming notifications as the turn executes:
	// - AgentMessageDelta: incremental response text
	// - ItemStarted/ItemCompleted: execution steps (file changes, commands, etc.)
	// - TurnCompleted: final turn status
}
```

## Approval Handlers

The Codex protocol is bidirectional: the server can send requests back to the client for approval (e.g., applying patches, executing commands, file changes). Register handlers to respond to these requests:

```go
client.SetApprovalHandlers(codex.ApprovalHandlers{
	OnFileChangeRequestApproval: func(ctx context.Context, params codex.FileChangeRequestApprovalParams) (codex.FileChangeRequestApprovalResponse, error) {
		// Review the proposed file changes
		fmt.Printf("File changes requested:\n")
		for path, change := range params.FileChanges {
			fmt.Printf("  %s: %s\n", path, change.ChangeType)
		}

		// Return approval decision
		return codex.FileChangeRequestApprovalResponse{
			Decision: "approved", // or "denied"
		}, nil
	},

	OnCommandExecutionRequestApproval: func(ctx context.Context, params codex.CommandExecutionRequestApprovalParams) (codex.CommandExecutionRequestApprovalResponse, error) {
		// Review the command to be executed
		fmt.Printf("Command: %s\n", params.Command)

		// Return approval decision (6 variants: accept, acceptForSession, decline, cancel, acceptWithExecpolicyAmendment, applyNetworkPolicyAmendment)
		return codex.CommandExecutionRequestApprovalResponse{
			Decision: codex.CommandExecutionApprovalDecisionWrapper{
				Value: "accept",
			},
		}, nil
	},

	OnSkillRequestApproval: func(ctx context.Context, params codex.SkillRequestApprovalParams) (codex.SkillRequestApprovalResponse, error) {
		// Approve or deny skill execution
		return codex.SkillRequestApprovalResponse{
			Decision: "approved",
		}, nil
	},

	// Add handlers for other approval types as needed:
	// - OnApplyPatchApproval (deprecated legacy API)
	// - OnExecCommandApproval
	// - OnDynamicToolCall
	// - OnToolRequestUserInput
	// - OnFuzzyFileSearch
	// - OnChatgptAuthTokensRefresh
})
```

**Important:** If an approval handler is not set and the server sends that request type, the client will return a JSON-RPC method-not-found error (`-32601`). This allows you to selectively enable approval flows.

## Available Services

The client provides typed service accessors for all protocol domains:

```go
// Thread management
client.Thread.Start(ctx, params)
client.Thread.Read(ctx, params)
client.Thread.List(ctx, params)
client.Thread.Resume(ctx, params)
client.Thread.Fork(ctx, params)
client.Thread.Rollback(ctx, params)
client.Thread.Archive(ctx, params)
// ... and more

// Turn operations (messages/interactions)
client.Turn.Start(ctx, params)
client.Turn.Interrupt(ctx, params)
client.Turn.Steer(ctx, params)

// Account & authentication
client.Account.Get(ctx, params)
client.Account.Login(ctx, params)
client.Account.GetRateLimits(ctx)

// Configuration
client.Config.Read(ctx, params)
client.Config.Write(ctx, params)

// Models
client.Model.List(ctx, params)

// Skills (custom capabilities)
client.Skills.List(ctx, params)
client.Skills.ConfigWrite(ctx, params)

// Apps (connectors/integrations)
client.Apps.List(ctx, params)

// MCP (Model Context Protocol) servers
client.Mcp.ListServerStatus(ctx, params)
client.Mcp.OauthLogin(ctx, params)

// Command execution
client.Command.Exec(ctx, params)

// Code review
client.Review.Start(ctx, params)

// Feedback submission
client.Feedback.Upload(ctx, params)

// External agent config migration
client.ExternalAgent.ConfigDetect(ctx, params)
client.ExternalAgent.ConfigImport(ctx, params)

// Experimental features
client.Experimental.FeatureList(ctx, params)

// System operations
client.System.WindowsSandboxSetupStart(ctx, params)
```

## Notification Listeners

Register listeners for server→client notifications (streaming events):

```go
// Thread notifications
client.OnThreadStarted(func(notif codex.ThreadStartedNotification) { /* ... */ })
client.OnThreadClosed(func(notif codex.ThreadClosedNotification) { /* ... */ })
client.OnThreadStatusChanged(func(notif codex.ThreadStatusChangedNotification) { /* ... */ })

// Turn notifications
client.OnTurnStarted(func(notif codex.TurnStartedNotification) { /* ... */ })
client.OnTurnCompleted(func(notif codex.TurnCompletedNotification) { /* ... */ })

// Streaming deltas
client.OnAgentMessageDelta(func(notif codex.AgentMessageDeltaNotification) { /* ... */ })
client.OnFileChangeOutputDelta(func(notif codex.FileChangeOutputDeltaNotification) { /* ... */ })
client.OnReasoningTextDelta(func(notif codex.ReasoningTextDeltaNotification) { /* ... */ })

// Account notifications
client.OnAccountUpdated(func(notif codex.AccountUpdatedNotification) { /* ... */ })
client.OnAccountRateLimitsUpdated(func(notif codex.AccountRateLimitsUpdatedNotification) { /* ... */ })

// Config notifications
client.OnConfigWarning(func(notif codex.ConfigWarningNotification) { /* ... */ })

// Model notifications
client.OnModelRerouted(func(notif codex.ModelReroutedNotification) { /* ... */ })

// App notifications
client.OnAppListUpdated(func(notif codex.AppListUpdatedNotification) { /* ... */ })

// MCP notifications
client.OnMcpServerOauthLoginCompleted(func(notif codex.McpServerOauthLoginCompletedNotification) { /* ... */ })
client.OnMcpToolCallProgress(func(notif codex.McpToolCallProgressNotification) { /* ... */ })

// Realtime audio/video notifications (EXPERIMENTAL)
client.OnThreadRealtimeStarted(func(notif codex.ThreadRealtimeStartedNotification) { /* ... */ })
client.OnThreadRealtimeOutputAudioDelta(func(notif codex.ThreadRealtimeOutputAudioDeltaNotification) { /* ... */ })

// System notifications
client.OnError(func(notif codex.ErrorNotification) { /* ... */ })
client.OnContextCompacted(func(notif codex.ContextCompactedNotification) { /* ... */ })

// ... and 30+ more notification types
```

See [dispatch.go](dispatch.go) for the complete list of notification methods.

## Helper Functions

```go
// Ptr returns a pointer to a value (useful for optional fields)
title := codex.Ptr("My Custom Title")
params := codex.InitializeParams{
	ClientInfo: codex.ClientInfo{
		Name:    "my-client",
		Version: "1.0.0",
		Title:   title, // *string
	},
}
```

## Architecture

This SDK implements JSON-RPC 2.0 over a pluggable transport layer:

1. **Transport Layer** (`transport.go`, `stdio.go`): Handles message encoding, delivery, and bidirectional communication
2. **Client Layer** (`client.go`): Request/response matching, timeout handling, error wrapping
3. **Service Layer** (`thread.go`, `turn.go`, etc.): Typed methods for each protocol domain
4. **Message Router** (`dispatch.go`): Routes incoming notifications and approval requests to registered handlers

The protocol is **bidirectional**:
- **Client → Server**: Requests (expect response) and Notifications (fire-and-forget)
- **Server → Client**: Requests (approval flows) and Notifications (streaming events)

## Error Handling

The SDK provides typed errors for different failure modes:

```go
import "errors"

resp, err := client.Thread.Start(ctx, params)
if err != nil {
	var rpcErr *codex.RPCError
	var timeoutErr *codex.TimeoutError
	var transportErr *codex.TransportError

	switch {
	case errors.As(err, &rpcErr):
		// JSON-RPC error from server
		fmt.Printf("RPC error %d: %s\n", rpcErr.Code(), rpcErr.Message())
	case errors.As(err, &timeoutErr):
		// Request timeout or context cancellation
		fmt.Println("Request timed out")
	case errors.As(err, &transportErr):
		// Connection/IO failure
		fmt.Printf("Transport error: %v\n", transportErr)
	}
}
```

## Testing

Run all tests:

```bash
go test ./...
```

Run tests with race detection:

```bash
go test -race ./...
```

The SDK includes comprehensive test coverage with 180+ tests covering all protocol operations, notification dispatch, approval handler flows, and concurrent request handling.

## License

MIT License - see [LICENSE.md](LICENSE.md)

## Contributing

This is an unofficial SDK for the OpenAI Codex CLI protocol. For issues or contributions, please open an issue on GitHub.

## References

- **JSON-RPC 2.0 Specification**: https://www.jsonrpc.org/specification
- **Protocol Specs**: See `specs/` directory for 150+ JSON schemas defining the complete protocol
