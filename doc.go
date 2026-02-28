// Package codex provides a Go client for the Codex CLI JSON-RPC 2.0 protocol.
//
// It implements full bidirectional communication over stdio, including typed
// service methods, streaming notification listeners, and approval handler
// callbacks. The public API maps 1:1 to the Codex protocol schemas.
//
// Basic usage:
//
//	transport := codex.NewStdioTransport(os.Stdin, os.Stdout)
//	defer transport.Close()
//
//	client := codex.NewClient(transport)
//
//	resp, err := client.Initialize(ctx, codex.InitializeParams{
//		ClientInfo: codex.ClientInfo{Name: "my-app", Version: "1.0.0"},
//	})
//
//	client.OnAgentMessageDelta(func(n codex.AgentMessageDeltaNotification) {
//		fmt.Print(n.Delta)
//	})
//
//	thread, err := client.Thread.Start(ctx, codex.ThreadStartParams{
//		Model: codex.Ptr("gpt-4"),
//	})
package codex
