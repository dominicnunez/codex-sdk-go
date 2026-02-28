// Package codex provides a Go client for the Codex CLI JSON-RPC 2.0 protocol.
//
// It implements full bidirectional communication over stdio, including typed
// service methods, streaming notification listeners, and approval handler
// callbacks. The public API maps 1:1 to the Codex protocol schemas.
//
// Basic usage with an existing transport:
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
//
// Using StartProcess for automatic process management:
//
//	proc, err := codex.StartProcess(ctx, nil)
//	if err != nil { ... }
//	defer proc.Close()
//
//	resp, err := proc.Client.Initialize(ctx, codex.InitializeParams{
//		ClientInfo: codex.ClientInfo{Name: "my-app", Version: "1.0.0"},
//	})
//
// Using Run for a single-turn conversation (simplest usage):
//
//	proc, err := codex.StartProcess(ctx, &codex.ProcessOptions{
//		Model:        "o3",
//		Sandbox:      codex.SandboxModeReadOnly,
//		ApprovalMode: "full-auto",
//	})
//	if err != nil { ... }
//	defer proc.Close()
//
//	result, err := proc.Run(ctx, codex.RunOptions{
//		Prompt: "Explain what this project does",
//	})
//	if err != nil { ... }
//	fmt.Println(result.Response)
package codex
