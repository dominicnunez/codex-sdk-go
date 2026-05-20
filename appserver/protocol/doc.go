// Package protocol provides a Go client for the Codex JSON-RPC 2.0 protocol.
//
// It provides typed service methods, notification listeners, and approval
// handler callbacks over a caller-provided Transport. The public protocol
// types map to the Codex protocol schemas.
//
// Basic usage with an existing transport:
//
//	client := protocol.NewClient(transport)
//
//	resp, err := client.Initialize(ctx, protocol.InitializeParams{
//		ClientInfo: protocol.ClientInfo{Name: "my-app", Version: "1.0.0"},
//	})
//
//	client.OnAgentMessageDelta(func(n protocol.AgentMessageDeltaNotification) {
//		fmt.Print(n.Delta)
//	})
//
//	thread, err := client.Thread.Start(ctx, protocol.ThreadStartParams{
//		Model: protocol.Ptr("gpt-4"),
//	})
package protocol
