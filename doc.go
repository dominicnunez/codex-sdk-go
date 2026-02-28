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
//
// Using RunStreamed for streaming events via an iterator:
//
//	stream := proc.RunStreamed(ctx, codex.RunOptions{
//		Prompt: "Fix the bug in main.go",
//	})
//	for event, err := range stream.Events() {
//		if err != nil { log.Fatal(err) }
//		switch e := event.(type) {
//		case *codex.TextDelta:
//			fmt.Print(e.Delta)
//		case *codex.ItemCompleted:
//			fmt.Printf("\n[item completed: %T]\n", e.Item.Value)
//		case *codex.TurnCompleted:
//			fmt.Println("\n[turn done]")
//		}
//	}
//	result = stream.Result()
//
// Using Conversation for multi-turn sessions:
//
//	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{
//		Instructions: codex.Ptr("You are a helpful assistant"),
//	})
//	if err != nil { log.Fatal(err) }
//
//	r1, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "What files are in this project?"})
//	fmt.Println(r1.Response)
//
//	r2, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "Summarize the main one"})
//	fmt.Println(r2.Response)
//
// Using multi-agent collaboration with streaming and the AgentTracker:
//
//	tracker := codex.NewAgentTracker()
//	stream := proc.RunStreamed(ctx, codex.RunOptions{
//		Prompt: "Refactor the auth module",
//		CollaborationMode: &codex.CollaborationMode{
//			Mode:     codex.ModeKindDefault,
//			Settings: codex.CollaborationModeSettings{Model: "o3"},
//		},
//	})
//	for event, err := range stream.Events() {
//		if err != nil { log.Fatal(err) }
//		tracker.ProcessEvent(event)
//		switch e := event.(type) {
//		case *codex.CollabToolCallStarted:
//			fmt.Printf("[collab] %s started (tool=%s)\n", e.ID, e.Tool)
//		case *codex.CollabToolCallCompleted:
//			fmt.Printf("[collab] %s completed\n", e.ID)
//		case *codex.TextDelta:
//			fmt.Print(e.Delta)
//		}
//	}
//	fmt.Printf("Active agents: %d\n", tracker.ActiveCount())
package codex
