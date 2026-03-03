package codex_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestStreamCollectorProcessLifecycleAndPlan(t *testing.T) {
	collector := codex.NewStreamCollector()

	collector.Process(&codex.PlanDelta{ItemID: "plan-1", Delta: "step 1"}, nil)
	collector.Process(&codex.PlanDelta{ItemID: "plan-1", Delta: " + step 2"}, nil)

	collector.Process(&codex.ItemStarted{
		Item: codex.ThreadItemWrapper{
			Value: &codex.CommandExecutionThreadItem{
				ID:             "cmd-1",
				Command:        "ls",
				CommandActions: []codex.CommandActionWrapper{},
				Cwd:            "/tmp",
				Status:         codex.CommandExecutionStatusInProgress,
			},
		},
	}, nil)

	collector.Process(&codex.ItemCompleted{
		Item: codex.ThreadItemWrapper{
			Value: &codex.CommandExecutionThreadItem{
				ID:               "cmd-1",
				Command:          "ls",
				CommandActions:   []codex.CommandActionWrapper{},
				Cwd:              "/tmp",
				Status:           codex.CommandExecutionStatusCompleted,
				AggregatedOutput: ptr("output"),
			},
		},
	}, nil)

	collector.Process(&codex.ItemCompleted{
		Item: codex.ThreadItemWrapper{
			Value: &codex.PlanThreadItem{
				ID:   "plan-final",
				Text: "final plan",
			},
		},
	}, nil)

	summary := collector.Summary()

	if summary.LatestPlanText == nil || *summary.LatestPlanText != "final plan" {
		t.Fatalf("latest plan text = %v, want %q", summary.LatestPlanText, "final plan")
	}
	if summary.LatestPlanItemID == nil || *summary.LatestPlanItemID != "plan-final" {
		t.Fatalf("latest plan item id = %v, want %q", summary.LatestPlanItemID, "plan-final")
	}

	cmd, ok := summary.CommandExecutions["cmd-1"]
	if !ok {
		t.Fatal("expected command execution lifecycle for cmd-1")
	}
	if !cmd.Started || !cmd.Completed {
		t.Fatalf("command lifecycle started/completed = %v/%v, want true/true", cmd.Started, cmd.Completed)
	}
	if cmd.Status == nil || *cmd.Status != codex.CommandExecutionStatusCompleted {
		t.Fatalf("command status = %v, want %v", cmd.Status, codex.CommandExecutionStatusCompleted)
	}
	if cmd.AggregatedOutput != "output" {
		t.Fatalf("aggregated output = %q, want %q", cmd.AggregatedOutput, "output")
	}
}

func TestRunStreamedWithCollectorCapturesNotificationConveniences(t *testing.T) {
	proc, mock := mockProcess(t)
	collector := codex.NewStreamCollector()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamedWithCollector(ctx, codex.RunOptions{Prompt: "collect"}, collector)
	waitForRunStreamedReady(t, mock)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/commandExecution/outputDelta",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"cmd-1","delta":"out"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/tokenUsage/updated",
		Params: json.RawMessage(`{
			"threadId":"thread-1",
			"turnId":"turn-1",
			"tokenUsage":{
				"last":{"cachedInputTokens":0,"inputTokens":10,"outputTokens":5,"reasoningOutputTokens":0,"totalTokens":15},
				"total":{"cachedInputTokens":1,"inputTokens":20,"outputTokens":10,"reasoningOutputTokens":1,"totalTokens":31},
				"modelContextWindow":128000
			}
		}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "error",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","willRetry":false,"error":{"message":"system failed"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/realtime/error",
		Params:  json.RawMessage(`{"threadId":"thread-1","message":"realtime failed"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/started",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"commandExecution","id":"cmd-1","command":"ls","commandActions":[],"cwd":"/tmp","status":"inProgress"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"commandExecution","id":"cmd-1","command":"ls","commandActions":[],"cwd":"/tmp","status":"completed","aggregatedOutput":"out"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	for _, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	}

	summary := collector.Summary()
	if summary.LatestTokenUsage == nil {
		t.Fatal("expected token usage summary")
	}
	if summary.LatestTokenUsage.Last.InputTokens != 10 {
		t.Fatalf("last input tokens = %d, want 10", summary.LatestTokenUsage.Last.InputTokens)
	}
	if summary.LatestTokenUsage.Total.TotalTokens != 31 {
		t.Fatalf("total tokens = %d, want 31", summary.LatestTokenUsage.Total.TotalTokens)
	}

	cmd, ok := summary.CommandExecutions["cmd-1"]
	if !ok {
		t.Fatal("expected command execution lifecycle for cmd-1")
	}
	if !cmd.Started || !cmd.Completed {
		t.Fatalf("command lifecycle started/completed = %v/%v, want true/true", cmd.Started, cmd.Completed)
	}
	if len(cmd.OutputDeltas) != 1 || cmd.OutputDeltas[0] != "out" {
		t.Fatalf("output deltas = %#v, want [\"out\"]", cmd.OutputDeltas)
	}
	if cmd.AggregatedOutput != "out" {
		t.Fatalf("aggregated output = %q, want %q", cmd.AggregatedOutput, "out")
	}

	if len(summary.NormalizedErrors) < 2 {
		t.Fatalf("expected >=2 normalized errors, got %d", len(summary.NormalizedErrors))
	}
}

func TestStreamCollectorSummaryIsDeepCopied(t *testing.T) {
	proc, mock := mockProcess(t)
	collector := codex.NewStreamCollector()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamedWithCollector(ctx, codex.RunOptions{Prompt: "collect"}, collector)
	waitForRunStreamedReady(t, mock)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/commandExecution/outputDelta",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"cmd-copy","delta":"chunk"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "error",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","willRetry":false,"error":{"message":"system failed"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/started",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"commandExecution","id":"cmd-copy","command":"ls","commandActions":[],"cwd":"/tmp","status":"inProgress","processId":"123"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"commandExecution","id":"cmd-copy","command":"ls","commandActions":[],"cwd":"/tmp","status":"completed","aggregatedOutput":"chunk"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/started",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"mcpToolCall","id":"mcp-copy","server":"local","tool":"search","status":"inProgress","arguments":{"outer":{"inner":"value"}}}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"mcpToolCall","id":"mcp-copy","server":"local","tool":"search","status":"completed","arguments":{"outer":{"inner":"value"}},"result":{"content":[{"kind":"text","value":"ok"}],"structuredContent":{"result":{"value":"ok"}}}}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/started",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"webSearch","id":"web-copy","query":"original query","action":{"type":"search","query":"start query","queries":["start-a","start-b"]}}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"webSearch","id":"web-copy","query":"original query","action":{"type":"findInPage","url":"https://example.com","pattern":"needle"}}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/started",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"fileChange","id":"file-copy","status":"inProgress","changes":[{"path":"old.txt","diff":"@@ -1 +1 @@","kind":{"type":"update","move_path":"new.txt"}}]}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"fileChange","id":"file-copy","status":"completed","changes":[{"path":"old.txt","diff":"@@ -1 +1 @@","kind":{"type":"update","move_path":"new.txt"}}]}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	for _, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	}

	summary := collector.Summary()
	cmd := summary.CommandExecutions["cmd-copy"]
	*cmd.Status = codex.CommandExecutionStatusFailed
	cmd.StartedItem.Command = "rm -rf /"
	cmd.StartedItem.ProcessId = ptr("999")
	cmd.OutputDeltas[0] = "mutated"
	summary.CommandExecutions["cmd-copy"] = cmd
	mcp := summary.McpToolCalls["mcp-copy"]
	*mcp.Status = codex.McpToolCallStatusFailed
	mcp.StartedItem.Arguments.(map[string]interface{})["outer"] = "changed"
	mcp.CompletedItem.Result.StructuredContent.(map[string]interface{})["result"] = "changed"
	summary.McpToolCalls["mcp-copy"] = mcp
	web := summary.WebSearches["web-copy"]
	searchAction := web.StartedItem.Action.Value.(*codex.SearchWebSearchAction)
	*searchAction.Query = "changed query"
	(*searchAction.Queries)[0] = "changed-list-query"
	findAction := web.CompletedItem.Action.Value.(*codex.FindInPageWebSearchAction)
	*findAction.Pattern = "changed-pattern"
	summary.WebSearches["web-copy"] = web
	file := summary.FileChanges["file-copy"]
	*file.Status = codex.PatchApplyStatusFailed
	updateKind := file.StartedItem.Changes[0].Kind.Value.(*codex.UpdatePatchChangeKind)
	*updateKind.MovePath = "changed.txt"
	summary.FileChanges["file-copy"] = file
	summary.NormalizedErrors[0].Message = "changed"
	*summary.NormalizedErrors[0].ThreadID = "mutated-thread"

	after := collector.Summary()
	got := after.CommandExecutions["cmd-copy"]
	if got.Status == nil || *got.Status != codex.CommandExecutionStatusCompleted {
		t.Fatalf("status leaked mutation: %v", got.Status)
	}
	if got.StartedItem == nil || got.StartedItem.Command != "ls" {
		t.Fatalf("started item command leaked mutation: %v", got.StartedItem)
	}
	if got.StartedItem.ProcessId == nil || *got.StartedItem.ProcessId != "123" {
		t.Fatalf("process id leaked mutation: %v", got.StartedItem.ProcessId)
	}
	if len(got.OutputDeltas) != 1 || got.OutputDeltas[0] != "chunk" {
		t.Fatalf("output deltas leaked mutation: %v", got.OutputDeltas)
	}
	mcpAfter := after.McpToolCalls["mcp-copy"]
	if mcpAfter.Status == nil || *mcpAfter.Status != codex.McpToolCallStatusCompleted {
		t.Fatalf("mcp status leaked mutation: %v", mcpAfter.Status)
	}
	mcpArgs := mcpAfter.StartedItem.Arguments.(map[string]interface{})
	if _, ok := mcpArgs["outer"].(map[string]interface{}); !ok {
		t.Fatalf("mcp arguments leaked mutation: %#v", mcpArgs)
	}
	mcpResult := mcpAfter.CompletedItem.Result.StructuredContent.(map[string]interface{})
	if _, ok := mcpResult["result"].(map[string]interface{}); !ok {
		t.Fatalf("mcp structured content leaked mutation: %#v", mcpResult)
	}
	webAfter := after.WebSearches["web-copy"]
	webStart := webAfter.StartedItem.Action.Value.(*codex.SearchWebSearchAction)
	if webStart.Query == nil || *webStart.Query != "start query" {
		t.Fatalf("web start action leaked query mutation: %#v", webStart)
	}
	if webStart.Queries == nil || len(*webStart.Queries) != 2 || (*webStart.Queries)[0] != "start-a" {
		t.Fatalf("web start action leaked queries mutation: %#v", webStart.Queries)
	}
	webCompleted := webAfter.CompletedItem.Action.Value.(*codex.FindInPageWebSearchAction)
	if webCompleted.Pattern == nil || *webCompleted.Pattern != "needle" {
		t.Fatalf("web completed action leaked pattern mutation: %#v", webCompleted)
	}
	fileAfter := after.FileChanges["file-copy"]
	if fileAfter.Status == nil || *fileAfter.Status != codex.PatchApplyStatusCompleted {
		t.Fatalf("file status leaked mutation: %v", fileAfter.Status)
	}
	fileUpdateKind := fileAfter.StartedItem.Changes[0].Kind.Value.(*codex.UpdatePatchChangeKind)
	if fileUpdateKind.MovePath == nil || *fileUpdateKind.MovePath != "new.txt" {
		t.Fatalf("file change kind leaked mutation: %#v", fileUpdateKind)
	}
	if len(after.NormalizedErrors) == 0 || after.NormalizedErrors[0].Message != "system failed" {
		t.Fatalf("normalized error message leaked mutation: %v", after.NormalizedErrors)
	}
	if after.NormalizedErrors[0].ThreadID == nil || *after.NormalizedErrors[0].ThreadID != "thread-1" {
		t.Fatalf("normalized error thread id leaked mutation: %v", after.NormalizedErrors[0].ThreadID)
	}
}

func TestStreamCollectorBoundsNormalizedErrorHistory(t *testing.T) {
	collector := codex.NewStreamCollector()

	const totalErrors = 600
	for i := 0; i < totalErrors; i++ {
		collector.Process(nil, fmt.Errorf("err-%d", i))
	}

	summary := collector.Summary()
	if len(summary.NormalizedErrors) >= totalErrors {
		t.Fatalf("normalized errors retained = %d; want bounded history smaller than %d", len(summary.NormalizedErrors), totalErrors)
	}
	if summary.DroppedNormalizedErrors == 0 {
		t.Fatal("expected dropped normalized error count to be tracked")
	}
}

func TestStreamCollectorBoundsOutputDeltaHistory(t *testing.T) {
	proc, mock := mockProcess(t)
	collector := codex.NewStreamCollector()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamedWithCollector(ctx, codex.RunOptions{Prompt: "collect output deltas"}, collector)
	waitForRunStreamedReady(t, mock)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/started",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"commandExecution","id":"cmd-many","command":"echo","commandActions":[],"cwd":"/tmp","status":"inProgress"}}`),
	})
	for i := 0; i < 1200; i++ {
		mock.InjectServerNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "item/commandExecution/outputDelta",
			Params: json.RawMessage(fmt.Sprintf(
				`{"threadId":"thread-1","turnId":"turn-1","itemId":"cmd-many","delta":"d%04d"}`,
				i,
			)),
		})
	}
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"commandExecution","id":"cmd-many","command":"echo","commandActions":[],"cwd":"/tmp","status":"completed","aggregatedOutput":"done"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	for _, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	}

	summary := collector.Summary()
	cmd, ok := summary.CommandExecutions["cmd-many"]
	if !ok {
		t.Fatal("expected command execution lifecycle for cmd-many")
	}
	if len(cmd.OutputDeltas) >= 1200 {
		t.Fatalf("output delta history retained = %d; want bounded history smaller than %d", len(cmd.OutputDeltas), 1200)
	}
	if cmd.DroppedOutputDeltas == 0 {
		t.Fatal("expected dropped output delta count to be tracked")
	}
}

func TestStreamCollectorBoundsRawOutputChunksAndFinalizesOnCompletion(t *testing.T) {
	proc, mock := mockProcess(t)
	collector := codex.NewStreamCollector()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamedWithCollector(ctx, codex.RunOptions{Prompt: "collect raw chunks"}, collector)
	waitForRunStreamedReady(t, mock)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/started",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"commandExecution","id":"cmd-raw","command":"echo","commandActions":[],"cwd":"/tmp","status":"inProgress"}}`),
	})

	const totalChunks = 2000
	chunk := strings.Repeat("x", 1024)
	for i := 0; i < totalChunks; i++ {
		mock.InjectServerNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "item/commandExecution/outputDelta",
			Params: json.RawMessage(fmt.Sprintf(
				`{"threadId":"thread-1","turnId":"turn-1","itemId":"cmd-raw","delta":"%s"}`,
				chunk,
			)),
		})
	}

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"commandExecution","id":"cmd-raw","command":"echo","commandActions":[],"cwd":"/tmp","status":"completed"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	for _, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	}

	summary := collector.Summary()
	cmd, ok := summary.CommandExecutions["cmd-raw"]
	if !ok {
		t.Fatal("expected command execution lifecycle for cmd-raw")
	}
	if !cmd.Completed {
		t.Fatal("expected command execution lifecycle to be completed")
	}
	if cmd.AggregatedOutput == "" {
		t.Fatal("expected bounded aggregated output to be retained after completion")
	}

	fullLength := totalChunks * len(chunk)
	if len(cmd.AggregatedOutput) >= fullLength {
		t.Fatalf("aggregated output length = %d; want bounded length smaller than %d", len(cmd.AggregatedOutput), fullLength)
	}
}
