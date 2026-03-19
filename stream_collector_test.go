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

func TestStreamCollectorScopesRepeatedItemIDsByThreadAndTurn(t *testing.T) {
	collector := codex.NewStreamCollector()

	collector.Process(&codex.ItemStarted{
		ThreadID: "thread-1",
		TurnID:   "turn-1",
		Item: codex.ThreadItemWrapper{
			Value: &codex.CommandExecutionThreadItem{
				ID:             "cmd-1",
				Command:        "ls",
				CommandActions: []codex.CommandActionWrapper{},
				Cwd:            "/tmp/one",
				Status:         codex.CommandExecutionStatusInProgress,
			},
		},
	}, nil)
	collector.Process(&codex.ItemCompleted{
		ThreadID: "thread-1",
		TurnID:   "turn-1",
		Item: codex.ThreadItemWrapper{
			Value: &codex.CommandExecutionThreadItem{
				ID:               "cmd-1",
				Command:          "ls",
				CommandActions:   []codex.CommandActionWrapper{},
				Cwd:              "/tmp/one",
				Status:           codex.CommandExecutionStatusCompleted,
				AggregatedOutput: ptr("first output"),
			},
		},
	}, nil)
	collector.Process(&codex.ItemStarted{
		ThreadID: "thread-2",
		TurnID:   "turn-2",
		Item: codex.ThreadItemWrapper{
			Value: &codex.CommandExecutionThreadItem{
				ID:             "cmd-1",
				Command:        "pwd",
				CommandActions: []codex.CommandActionWrapper{},
				Cwd:            "/tmp/two",
				Status:         codex.CommandExecutionStatusInProgress,
			},
		},
	}, nil)

	summary := collector.Summary()
	if got := len(summary.CommandExecutions); got != 2 {
		t.Fatalf("command execution count = %d, want 2", got)
	}

	var (
		first       codex.CommandExecutionLifecycle
		second      codex.CommandExecutionLifecycle
		firstFound  bool
		secondFound bool
	)
	for _, lifecycle := range summary.CommandExecutions {
		switch {
		case lifecycle.ThreadID == "thread-1" && lifecycle.TurnID == "turn-1":
			first = lifecycle
			firstFound = true
		case lifecycle.ThreadID == "thread-2" && lifecycle.TurnID == "turn-2":
			second = lifecycle
			secondFound = true
		}
	}

	if !firstFound {
		t.Fatal("missing lifecycle for thread-1/turn-1")
	}
	if !first.Started || !first.Completed {
		t.Fatalf("first lifecycle started/completed = %v/%v, want true/true", first.Started, first.Completed)
	}
	if first.CompletedItem == nil || first.CompletedItem.Command != "ls" {
		t.Fatalf("first completed item = %+v, want ls", first.CompletedItem)
	}
	if first.AggregatedOutput != "first output" {
		t.Fatalf("first aggregated output = %q, want first output", first.AggregatedOutput)
	}

	if !secondFound {
		t.Fatal("missing lifecycle for thread-2/turn-2")
	}
	if !second.Started || second.Completed {
		t.Fatalf("second lifecycle started/completed = %v/%v, want true/false", second.Started, second.Completed)
	}
	if second.StartedItem == nil || second.StartedItem.Command != "pwd" {
		t.Fatalf("second started item = %+v, want pwd", second.StartedItem)
	}
	if second.AggregatedOutput != "" {
		t.Fatalf("second aggregated output = %q, want empty", second.AggregatedOutput)
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

func TestRunStreamedWithCollectorPreservesRawNormalizedErrorPayloads(t *testing.T) {
	proc, mock := mockProcess(t)
	collector := codex.NewStreamCollector()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamedWithCollector(ctx, codex.RunOptions{Prompt: "collect raw errors"}, collector)
	waitForRunStreamedReady(t, mock)

	systemRaw := json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","willRetry":false,"error":{"message":"system failed","additionalDetails":"details"}}`)
	realtimeRaw := json.RawMessage(`{"threadId":"thread-1","message":"realtime failed"}`)
	turnRaw := json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"failed","items":[],"error":{"message":"turn failed","additionalDetails":"details","codexErrorInfo":{"code":"E_TURN"}}}}`)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "error",
		Params:  systemRaw,
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "thread/realtime/error",
		Params:  realtimeRaw,
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  turnRaw,
	})

	var streamErr error
	for _, err := range stream.Events() {
		if err != nil {
			streamErr = err
		}
	}
	if streamErr == nil || !strings.Contains(streamErr.Error(), "turn error: turn failed") {
		t.Fatalf("stream error = %v, want turn failure", streamErr)
	}

	summary := collector.Summary()
	if len(summary.NormalizedErrors) != 4 {
		t.Fatalf("normalized errors = %d, want 4", len(summary.NormalizedErrors))
	}

	errorsByKind := make(map[string]codex.NormalizedStreamError, len(summary.NormalizedErrors))
	for _, normalized := range summary.NormalizedErrors {
		errorsByKind[normalized.Kind] = normalized
	}

	systemErr, ok := errorsByKind["system_error"]
	if !ok {
		t.Fatal("missing system_error")
	}
	if string(systemErr.Raw) != string(systemRaw) {
		t.Fatalf("system raw = %s, want %s", string(systemErr.Raw), string(systemRaw))
	}

	realtimeErr, ok := errorsByKind["realtime_error"]
	if !ok {
		t.Fatal("missing realtime_error")
	}
	if string(realtimeErr.Raw) != string(realtimeRaw) {
		t.Fatalf("realtime raw = %s, want %s", string(realtimeErr.Raw), string(realtimeRaw))
	}

	turnErr, ok := errorsByKind["turn_error"]
	if !ok {
		t.Fatal("missing turn_error")
	}
	if turnErr.TurnID == nil || *turnErr.TurnID != "turn-1" {
		t.Fatalf("turn error turn id = %v, want turn-1", turnErr.TurnID)
	}
	if turnErr.SourceMethod == nil || *turnErr.SourceMethod != "turn/completed" {
		t.Fatalf("turn error source method = %v, want turn/completed", turnErr.SourceMethod)
	}
	if !strings.Contains(string(turnErr.Raw), `"code":"E_TURN"`) {
		t.Fatalf("turn raw = %s, want codexErrorInfo payload", string(turnErr.Raw))
	}

	streamErrSummary, ok := errorsByKind["stream_error"]
	if !ok {
		t.Fatal("missing stream_error")
	}
	if len(streamErrSummary.Raw) != 0 {
		t.Fatalf("stream error raw = %s, want empty", string(streamErrSummary.Raw))
	}

	summary.NormalizedErrors[0].Raw[0] = '{'
	after := collector.Summary()
	for _, normalized := range after.NormalizedErrors {
		if normalized.Kind == "system_error" && string(normalized.Raw) != string(systemRaw) {
			t.Fatalf("normalized error raw leaked mutation: %s", string(normalized.Raw))
		}
	}
}

func TestStreamCollectorSummaryIsDeepCopied(t *testing.T) {
	collector := codex.NewStreamCollector()
	commandPath := ptr("/workspace")
	commandQuery := ptr("needle")
	collector.Process(&codex.ItemStarted{Item: codex.ThreadItemWrapper{Value: &codex.CommandExecutionThreadItem{
		ID:      "cmd-copy",
		Command: "ls",
		CommandActions: []codex.CommandActionWrapper{{
			Value: &codex.SearchCommandAction{
				Command: "rg needle",
				Path:    commandPath,
				Query:   commandQuery,
			},
		}},
		Cwd:       "/tmp",
		Status:    codex.CommandExecutionStatusInProgress,
		ProcessId: ptr("123"),
	}}}, nil)
	collector.Process(&codex.ItemCompleted{Item: codex.ThreadItemWrapper{Value: &codex.CommandExecutionThreadItem{
		ID:      "cmd-copy",
		Command: "ls",
		CommandActions: []codex.CommandActionWrapper{{
			Value: &codex.SearchCommandAction{
				Command: "rg needle",
				Path:    ptr("/workspace"),
				Query:   ptr("needle"),
			},
		}},
		Cwd:              "/tmp",
		Status:           codex.CommandExecutionStatusCompleted,
		AggregatedOutput: ptr("chunk"),
	}}}, nil)
	collector.Process(&codex.ItemStarted{Item: codex.ThreadItemWrapper{Value: &codex.McpToolCallThreadItem{
		ID:        "mcp-copy",
		Server:    "local",
		Tool:      "search",
		Status:    codex.McpToolCallStatusInProgress,
		Arguments: map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}},
	}}}, nil)
	collector.Process(&codex.ItemCompleted{Item: codex.ThreadItemWrapper{Value: &codex.McpToolCallThreadItem{
		ID:        "mcp-copy",
		Server:    "local",
		Tool:      "search",
		Status:    codex.McpToolCallStatusCompleted,
		Arguments: map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}},
		Result: &codex.McpToolCallResult{
			Content: []interface{}{
				map[string]interface{}{"kind": "text", "value": "ok"},
			},
			StructuredContent: map[string]interface{}{"result": map[string]interface{}{"value": "ok"}},
		},
	}}}, nil)
	collector.Process(&codex.ItemStarted{Item: codex.ThreadItemWrapper{Value: &codex.WebSearchThreadItem{
		ID:    "web-copy",
		Query: "original query",
		Action: &codex.WebSearchActionWrapper{Value: &codex.SearchWebSearchAction{
			Query:   ptr("start query"),
			Queries: &[]string{"start-a", "start-b"},
		}},
	}}}, nil)
	collector.Process(&codex.ItemCompleted{Item: codex.ThreadItemWrapper{Value: &codex.WebSearchThreadItem{
		ID:    "web-copy",
		Query: "original query",
		Action: &codex.WebSearchActionWrapper{Value: &codex.FindInPageWebSearchAction{
			URL:     ptr("https://example.com"),
			Pattern: ptr("needle"),
		}},
	}}}, nil)
	movePath := ptr("new.txt")
	collector.Process(&codex.ItemStarted{Item: codex.ThreadItemWrapper{Value: &codex.FileChangeThreadItem{
		ID:     "file-copy",
		Status: codex.PatchApplyStatusInProgress,
		Changes: []codex.FileUpdateChange{{
			Path: "old.txt",
			Diff: "@@ -1 +1 @@",
			Kind: codex.PatchChangeKindWrapper{Value: &codex.UpdatePatchChangeKind{
				MovePath: movePath,
			}},
		}},
	}}}, nil)
	collector.Process(&codex.ItemCompleted{Item: codex.ThreadItemWrapper{Value: &codex.FileChangeThreadItem{
		ID:     "file-copy",
		Status: codex.PatchApplyStatusCompleted,
		Changes: []codex.FileUpdateChange{{
			Path: "old.txt",
			Diff: "@@ -1 +1 @@",
			Kind: codex.PatchChangeKindWrapper{Value: &codex.UpdatePatchChangeKind{
				MovePath: ptr("new.txt"),
			}},
		}},
	}}}, nil)
	collector.Process(nil, fmt.Errorf("system failed"))

	summary := collector.Summary()
	cmd, ok := summary.CommandExecutions["cmd-copy"]
	if !ok {
		t.Fatalf("missing command execution summary: %#v", summary)
	}
	if cmd.Status == nil || cmd.StartedItem == nil || cmd.CompletedItem == nil {
		t.Fatalf("incomplete command execution summary: %#v", cmd)
	}
	*cmd.Status = codex.CommandExecutionStatusFailed
	cmd.StartedItem.Command = "rm -rf /"
	cmdAction := cmd.StartedItem.CommandActions[0].Value.(*codex.SearchCommandAction)
	*cmdAction.Path = "/mutated"
	*cmdAction.Query = "changed-needle"
	cmd.StartedItem.ProcessId = ptr("999")
	summary.CommandExecutions["cmd-copy"] = cmd
	mcp, ok := summary.McpToolCalls["mcp-copy"]
	if !ok {
		t.Fatalf("missing mcp tool call summary: %#v", summary)
	}
	if mcp.Status == nil || mcp.StartedItem == nil || mcp.CompletedItem == nil {
		t.Fatalf("incomplete mcp tool call summary: %#v", mcp)
	}
	*mcp.Status = codex.McpToolCallStatusFailed
	mcp.StartedItem.Arguments.(map[string]interface{})["outer"] = "changed"
	mcp.CompletedItem.Result.StructuredContent.(map[string]interface{})["result"] = "changed"
	summary.McpToolCalls["mcp-copy"] = mcp
	web, ok := summary.WebSearches["web-copy"]
	if !ok {
		t.Fatalf("missing web search summary: %#v", summary)
	}
	if web.StartedItem == nil || web.CompletedItem == nil {
		t.Fatalf("incomplete web search summary: %#v", web)
	}
	searchAction := web.StartedItem.Action.Value.(*codex.SearchWebSearchAction)
	*searchAction.Query = "changed query"
	(*searchAction.Queries)[0] = "changed-list-query"
	findAction := web.CompletedItem.Action.Value.(*codex.FindInPageWebSearchAction)
	*findAction.Pattern = "changed-pattern"
	summary.WebSearches["web-copy"] = web
	file, ok := summary.FileChanges["file-copy"]
	if !ok {
		t.Fatalf("missing file change summary: %#v", summary)
	}
	if file.Status == nil || file.StartedItem == nil {
		t.Fatalf("incomplete file change summary: %#v", file)
	}
	*file.Status = codex.PatchApplyStatusFailed
	updateKind := file.StartedItem.Changes[0].Kind.Value.(*codex.UpdatePatchChangeKind)
	*updateKind.MovePath = "changed.txt"
	summary.FileChanges["file-copy"] = file
	summary.NormalizedErrors[0].Message = "changed"

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
	if len(got.StartedItem.CommandActions) != 1 {
		t.Fatalf("command actions length leaked mutation: %d", len(got.StartedItem.CommandActions))
	}
	gotAction := got.StartedItem.CommandActions[0].Value.(*codex.SearchCommandAction)
	if gotAction.Path == nil || *gotAction.Path != "/workspace" {
		t.Fatalf("command action path leaked mutation: %#v", gotAction)
	}
	if gotAction.Query == nil || *gotAction.Query != "needle" {
		t.Fatalf("command action query leaked mutation: %#v", gotAction)
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

func TestStreamCollectorScopesRepeatedItemIDsAcrossRuns(t *testing.T) {
	collector := codex.NewStreamCollector()

	collector.Process(&codex.ItemStarted{
		ThreadID: "thread-1",
		TurnID:   "turn-1",
		Item: codex.ThreadItemWrapper{
			Value: &codex.CommandExecutionThreadItem{
				ID:             "cmd-1",
				Command:        "ls",
				CommandActions: []codex.CommandActionWrapper{},
				Cwd:            "/tmp/one",
				Status:         codex.CommandExecutionStatusInProgress,
			},
		},
	}, nil)
	collector.Process(&codex.ItemCompleted{
		ThreadID: "thread-1",
		TurnID:   "turn-1",
		Item: codex.ThreadItemWrapper{
			Value: &codex.CommandExecutionThreadItem{
				ID:               "cmd-1",
				Command:          "ls",
				CommandActions:   []codex.CommandActionWrapper{},
				Cwd:              "/tmp/one",
				Status:           codex.CommandExecutionStatusCompleted,
				AggregatedOutput: ptr("first"),
			},
		},
	}, nil)
	collector.Process(&codex.ItemStarted{
		ThreadID: "thread-2",
		TurnID:   "turn-2",
		Item: codex.ThreadItemWrapper{
			Value: &codex.CommandExecutionThreadItem{
				ID:             "cmd-1",
				Command:        "pwd",
				CommandActions: []codex.CommandActionWrapper{},
				Cwd:            "/tmp/two",
				Status:         codex.CommandExecutionStatusInProgress,
			},
		},
	}, nil)

	summary := collector.Summary()
	if len(summary.CommandExecutions) != 2 {
		t.Fatalf("command execution summaries = %d, want 2", len(summary.CommandExecutions))
	}

	first, ok := summary.CommandExecutions["thread-1/turn-1/cmd-1"]
	if !ok {
		t.Fatalf("missing first scoped lifecycle: %#v", summary.CommandExecutions)
	}
	if !first.Started || !first.Completed {
		t.Fatalf("first lifecycle started/completed = %v/%v, want true/true", first.Started, first.Completed)
	}
	if first.ThreadID != "thread-1" || first.TurnID != "turn-1" {
		t.Fatalf("first lifecycle scope = %q/%q, want thread-1/turn-1", first.ThreadID, first.TurnID)
	}
	if first.AggregatedOutput != "first" {
		t.Fatalf("first aggregated output = %q, want %q", first.AggregatedOutput, "first")
	}

	second, ok := summary.CommandExecutions["thread-2/turn-2/cmd-1"]
	if !ok {
		t.Fatalf("missing second scoped lifecycle: %#v", summary.CommandExecutions)
	}
	if !second.Started || second.Completed {
		t.Fatalf("second lifecycle started/completed = %v/%v, want true/false", second.Started, second.Completed)
	}
	if second.ThreadID != "thread-2" || second.TurnID != "turn-2" {
		t.Fatalf("second lifecycle scope = %q/%q, want thread-2/turn-2", second.ThreadID, second.TurnID)
	}
	if second.StartedItem == nil || second.StartedItem.Command != "pwd" {
		t.Fatalf("second started item = %#v, want pwd", second.StartedItem)
	}
}
