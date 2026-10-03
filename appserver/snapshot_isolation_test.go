package appserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
)

func TestCollectorItemMetadataOwnership(t *testing.T) {
	for _, completed := range []bool{false, true} {
		item := &codex.McpToolCallThreadItem{ID: "mcp", PluginID: ptr("original")}
		collector := codex.NewStreamCollector()
		wrapper := codex.ThreadItemWrapper{Value: item}
		if completed {
			collector.Process(&codex.ItemCompleted{Item: wrapper}, nil)
		} else {
			collector.Process(&codex.ItemStarted{Item: wrapper}, nil)
		}
		*item.PluginID = "input mutation"
		get := func() *codex.McpToolCallThreadItem {
			lifecycle := collector.Summary().McpToolCalls["mcp"]
			if completed {
				return lifecycle.CompletedItem
			}
			return lifecycle.StartedItem
		}
		first := get()
		if *first.PluginID != "original" {
			t.Fatalf("completed=%v: input changed stored metadata: %q", completed, *first.PluginID)
		}
		*first.PluginID = "snapshot mutation"
		if *get().PluginID != "original" {
			t.Fatalf("completed=%v: snapshot changed stored metadata", completed)
		}
	}
}

const ownershipCommandJSON = `{"type":"commandExecution","id":"command","command":"pwd","commandActions":[],"cwd":"/workspace","source":"agent","status":"completed","pluginId":"original","scriptPath":"/workspace/script"}`
const ownershipMCPJSON = `{"type":"mcpToolCall","id":"mcp","server":"server","tool":"tool","status":"completed","arguments":{"nested":["original"]},"pluginId":"original","readOnlyHint":true,"mcpAppResourceUri":"ui://original","appContext":{"connectorId":"original","actionName":"original","appName":"original","linkId":"original","resourceUri":"ui://original"},"mcpAppUi":{"preferredModelDisplayMode":"inline","resourceUri":"ui://original"},"result":{"content":[],"structuredContent":{"nested":["original"]}}}`
const ownershipDynamicJSON = `{"type":"dynamicToolCall","id":"dynamic","tool":"tool","namespace":"original","status":"completed","arguments":{},"contentItems":[],"success":true}`
const ownershipCollabJSON = `{"type":"collabAgentToolCall","id":"collab","tool":"spawnAgent","status":"completed","agentsStates":{"agent":{"status":"completed","message":"original"}},"receiverThreadIds":["agent"],"senderThreadId":"thread-1"}`

func ownershipWireItems(t *testing.T) []codex.ThreadItemWrapper {
	t.Helper()
	var items []codex.ThreadItemWrapper
	if err := json.Unmarshal([]byte(`[`+ownershipCommandJSON+`,`+ownershipMCPJSON+`,`+ownershipDynamicJSON+`,`+ownershipCollabJSON+`]`), &items); err != nil {
		t.Fatal(err)
	}
	return items
}

func mutateWireItemMetadata(items []codex.ThreadItemWrapper) {
	*items[0].Value.(*codex.CommandExecutionThreadItem).PluginID = "mutated"
	*items[0].Value.(*codex.CommandExecutionThreadItem).Source = "mutated"
	*items[0].Value.(*codex.CommandExecutionThreadItem).ScriptPath = "mutated"
	mcp := items[1].Value.(*codex.McpToolCallThreadItem)
	*mcp.PluginID = "mutated"
	mcp.AppContext.ConnectorID = "mutated"
	*mcp.AppContext.ActionName = "mutated"
	mcp.McpAppUI.ResourceURI = "mutated"
	mcp.Arguments.(map[string]any)["nested"].([]any)[0] = "mutated"
	*items[2].Value.(*codex.DynamicToolCallThreadItem).Namespace = "mutated"
	*items[3].Value.(*codex.CollabAgentToolCallThreadItem).AgentsStates["agent"].Message = "mutated"
}

func TestConversationCacheAndSnapshotItemOwnership(t *testing.T) {
	process, _ := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conversation, err := process.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conversation.Close() }()
	thread := conversation.Thread()
	thread.Turns = []codex.Turn{{ID: "old", Status: codex.TurnStatusCompleted, Items: ownershipWireItems(t)}}
	process.Client.CacheThreadState(thread)
	mutateWireItemMetadata(thread.Turns[0].Items)
	want := ownershipWireItems(t)
	first := conversation.Thread()
	if !reflect.DeepEqual(first.Turns[0].Items, want) {
		t.Fatal("input mutation changed cached conversation")
	}
	mutateWireItemMetadata(first.Turns[0].Items)
	if !reflect.DeepEqual(conversation.Thread().Turns[0].Items, want) {
		t.Fatal("conversation snapshot retained item aliases")
	}
	cache, ok := process.Client.ThreadStateSnapshot(conversation.ThreadID())
	if !ok || !reflect.DeepEqual(cache.Turns[0].Items, want) {
		t.Fatal("conversation mutation changed client cache")
	}
	mutateWireItemMetadata(cache.Turns[0].Items)
	if !reflect.DeepEqual(conversation.Thread().Turns[0].Items, want) {
		t.Fatal("cache snapshot changed conversation")
	}
}

func TestStreamEventsCollectorAndResultOwnership(t *testing.T) {
	for _, conversationMode := range []bool{false, true} {
		process, mock := mockProcess(t)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		collector := codex.NewStreamCollector()
		var conversation *codex.Conversation
		var stream *codex.Stream
		if conversationMode {
			var err error
			conversation, err = process.StartConversation(ctx, codex.ConversationOptions{})
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			stream = conversation.TurnStreamed(ctx, codex.TurnOptions{Prompt: "ownership"})
		} else {
			stream = process.RunStreamedWithCollector(ctx, codex.RunOptions{Prompt: "ownership"}, collector)
		}
		waitForMethodCallCount(t, mock, "turn/start", 1)
		itemsSeen := make(chan struct{})
		consumerDone := make(chan error, 1)
		go func() {
			count := 0
			for event, err := range stream.Events() {
				if err != nil {
					consumerDone <- err
					return
				}
				if conversationMode {
					collector.Process(event, nil)
				}
				switch value := event.(type) {
				case *codex.CollabToolCallEvent:
					*value.AgentsStates["agent"].Message = "collab event mutation"
				case *codex.ItemCompleted:
					switch item := value.Item.Value.(type) {
					case *codex.CommandExecutionThreadItem:
						*item.PluginID = "event mutation"
					case *codex.McpToolCallThreadItem:
						item.AppContext.ConnectorID = "event mutation"
					case *codex.DynamicToolCallThreadItem:
						*item.Namespace = "event mutation"
					case *codex.CollabAgentToolCallThreadItem:
						if *item.AgentsStates["agent"].Message != "original" {
							consumerDone <- context.Canceled
							return
						}
						*item.AgentsStates["agent"].Message = "generic event mutation"
					}
					count++
					if count == 4 {
						close(itemsSeen)
					}
				case *codex.TurnCompleted:
					if value.Turn.StartedAt != nil {
						*value.Turn.StartedAt = 99
					}
				}
			}
			consumerDone <- nil
		}()
		for _, item := range []string{ownershipCommandJSON, ownershipMCPJSON, ownershipDynamicJSON, ownershipCollabJSON} {
			mock.InjectServerNotification(ctx, codex.Notification{Method: "item/completed", Params: json.RawMessage(`{"completedAtMs":1,"threadId":"thread-1","turnId":"turn-1","item":` + item + `}`)})
		}
		select {
		case <-itemsSeen:
		case err := <-consumerDone:
			cancel()
			t.Fatalf("consumer stopped before item handshake: %v", err)
		case <-ctx.Done():
			cancel()
			t.Fatal("item consumer handshake timed out")
		}
		mock.InjectServerNotification(ctx, codex.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[],"startedAt":1,"completedAt":2,"durationMs":1}}`)})
		if err := <-consumerDone; err != nil {
			cancel()
			t.Fatal(err)
		}
		result := stream.Result()
		if result == nil || !reflect.DeepEqual(result.Items, ownershipWireItems(t)) || result.Turn.StartedAt == nil || *result.Turn.StartedAt != 1 {
			cancel()
			t.Fatalf("event mutation reached result: %+v", result)
		}
		mutateWireItemMetadata(result.Items)
		want := ownershipWireItems(t)
		if !reflect.DeepEqual(result.Turn.Items, want) || !reflect.DeepEqual(result.Thread.Turns[len(result.Thread.Turns)-1].Items, want) || !reflect.DeepEqual(stream.Result().Items, want) {
			cancel()
			t.Fatal("result views or repeated Result share references")
		}
		summary := collector.Summary()
		if *summary.CommandExecutions["command"].CompletedItem.PluginID != "original" || summary.McpToolCalls["mcp"].CompletedItem.AppContext.ConnectorID != "original" {
			cancel()
			t.Fatal("event mutation reached collector")
		}
		if conversation != nil {
			if !reflect.DeepEqual(conversation.Thread().Turns[len(conversation.Thread().Turns)-1].Items, want) {
				cancel()
				t.Fatal("result mutation reached conversation")
			}
			_ = conversation.Close()
		}
		cancel()
	}
}

func TestCanceledStreamPartialCollectorOwnership(t *testing.T) {
	process, mock := mockProcess(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	collector := codex.NewStreamCollector()
	stream := process.RunStreamedWithCollector(ctx, codex.RunOptions{Prompt: "partial ownership"}, collector)
	waitForMethodCallCount(t, mock, "turn/start", 1)
	seen := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for event, err := range stream.Events() {
			if err != nil {
				continue
			}
			if item, ok := event.(*codex.ItemStarted); ok {
				*item.Item.Value.(*codex.CommandExecutionThreadItem).PluginID = "event mutation"
				close(seen)
			}
		}
	}()
	mock.InjectServerNotification(ctx, codex.Notification{Method: "item/started", Params: json.RawMessage(`{"startedAtMs":1,"threadId":"thread-1","turnId":"turn-1","item":` + ownershipCommandJSON + `}`)})
	select {
	case <-seen:
	case <-time.After(5 * time.Second):
		t.Fatal("partial item handshake timed out")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("canceled stream did not finish")
	}
	if stream.Result() != nil {
		t.Fatal("canceled operation returned success")
	}
	first := collector.Summary().CommandExecutions["command"]
	if *first.StartedItem.PluginID != "original" {
		t.Fatal("partial collector retained event alias")
	}
	*first.StartedItem.PluginID = "snapshot mutation"
	if *collector.Summary().CommandExecutions["command"].StartedItem.PluginID != "original" {
		t.Fatal("partial snapshot retained alias after cancellation")
	}
}

func BenchmarkConversationItemSnapshots(b *testing.B) {
	for _, count := range []int{1, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			mock := NewMockTransport()
			if err := mock.SetResponseData("initialize", validInitializeResponseData("codex-test/1.0")); err != nil {
				b.Fatal(err)
			}
			if err := mock.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread-1"))); err != nil {
				b.Fatal(err)
			}
			process := codex.NewProcessFromClient(codex.NewClient(mock))
			conversation, err := process.StartConversation(context.Background(), codex.ConversationOptions{})
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = conversation.Close() }()
			thread := conversation.Thread()
			thread.Turns = make([]codex.Turn, count)
			for i := range thread.Turns {
				var item codex.ThreadItemWrapper
				if err := json.Unmarshal([]byte(ownershipMCPJSON), &item); err != nil {
					b.Fatal(err)
				}
				thread.Turns[i] = codex.Turn{ID: fmt.Sprint(i), Status: codex.TurnStatusCompleted, Items: []codex.ThreadItemWrapper{item}}
			}
			process.Client.CacheThreadState(thread)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				snapshot := conversation.Thread()
				if len(snapshot.Turns) != count {
					b.Fatal("snapshot history lost")
				}
			}
		})
	}
}
