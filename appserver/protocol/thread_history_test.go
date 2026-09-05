package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestPaginatedHistoryItemsAndTurns(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	transport.SetResponse("thread/items/list", codex.Response{Result: json.RawMessage(`{"data":[{"turnId":"tu1","item":{"type":"agentMessage","id":"i1","text":"hello"}}],"nextCursor":"next-item","backwardsCursor":"back-item"}`)})
	sortDirection := codex.SortDirectionDesc
	items, err := client.Thread.ItemsList(context.Background(), codex.ThreadItemsListParams{
		ThreadID: "t1", TurnID: strPtr("tu1"), Cursor: strPtr("anchor"), Limit: codex.Ptr(uint32(0)), SortDirection: &sortDirection,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items.Data) != 1 || items.Data[0].TurnID != "tu1" || items.NextCursor == nil || *items.NextCursor != "next-item" || items.BackwardsCursor == nil || *items.BackwardsCursor != "back-item" {
		t.Fatalf("items = %+v", items)
	}
	item, ok := items.Data[0].Item.Value.(*codex.AgentMessageThreadItem)
	if !ok || item.Text != "hello" {
		t.Fatalf("item = %#v", items.Data[0].Item.Value)
	}
	var params map[string]interface{}
	if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
		t.Fatal(err)
	}
	if params["threadId"] != "t1" || params["turnId"] != "tu1" || params["cursor"] != "anchor" || params["limit"] != float64(0) || params["sortDirection"] != "desc" {
		t.Fatalf("params = %v", params)
	}

	transport.SetResponse("thread/turns/list", codex.Response{Result: json.RawMessage(`{"data":[{"id":"tu1","status":"completed","items":[],"itemsView":"summary","startedAt":10,"completedAt":12,"durationMs":2000}],"nextCursor":"next-turn","backwardsCursor":"back-turn"}`)})
	view := codex.TurnItemsViewSummary
	turns, err := client.Thread.TurnsList(context.Background(), codex.ThreadTurnsListParams{ThreadID: "t1", ItemsView: &view})
	if err != nil {
		t.Fatal(err)
	}
	if len(turns.Data) != 1 || turns.Data[0].ItemsView != view || turns.Data[0].DurationMs == nil || *turns.Data[0].DurationMs != 2000 || turns.Data[0].StartedAt == nil || *turns.Data[0].StartedAt != 10 || turns.Data[0].CompletedAt == nil || *turns.Data[0].CompletedAt != 12 {
		t.Fatalf("turns = %+v", turns)
	}
	if turns.NextCursor == nil || *turns.NextCursor != "next-turn" || turns.BackwardsCursor == nil || *turns.BackwardsCursor != "back-turn" {
		t.Fatalf("turn cursors = %+v", turns)
	}
	if err := json.Unmarshal(transport.GetSentRequest(1).Params, &params); err != nil {
		t.Fatal(err)
	}
	if params["itemsView"] != "summary" {
		t.Fatalf("params = %v", params)
	}
}

func TestPaginatedHistoryRevertAndResumeMetadata(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	thread := validThreadPayload("t1")
	thread["historyMode"] = "paginated"
	thread["model"] = "model-1"
	thread["reasoningEffort"] = "high"
	if err := transport.SetResponseData("thread/revert", map[string]interface{}{
		"thread": thread, "itemsBackwardsCursor": "item-anchor", "turnsBackwardsCursor": "turn-anchor",
	}); err != nil {
		t.Fatal(err)
	}
	response, err := client.Thread.Revert(context.Background(), codex.ThreadRevertParams{ThreadID: "t1", BeforeTurnID: "tu2"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Thread.HistoryMode != codex.ThreadHistoryModePaginated || response.Thread.Model == nil || *response.Thread.Model != "model-1" || response.Thread.ReasoningEffort == nil || *response.Thread.ReasoningEffort != "high" {
		t.Fatalf("thread = %+v", response.Thread)
	}
	if response.ItemsBackwardsCursor == nil || *response.ItemsBackwardsCursor != "item-anchor" || response.TurnsBackwardsCursor == nil || *response.TurnsBackwardsCursor != "turn-anchor" {
		t.Fatalf("cursors = %+v", response)
	}
	var params map[string]string
	if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
		t.Fatal(err)
	}
	if params["beforeTurnId"] != "tu2" || params["threadId"] != "t1" {
		t.Fatalf("params = %v", params)
	}

	fixture := validThreadLifecycleResponse(thread)
	fixture["itemsBackwardsCursor"] = "item-anchor"
	fixture["turnsBackwardsCursor"] = "turn-anchor"
	if err := transport.SetResponseData("thread/resume", fixture); err != nil {
		t.Fatal(err)
	}
	resume, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if resume.ItemsBackwardsCursor == nil || *resume.ItemsBackwardsCursor != "item-anchor" || resume.TurnsBackwardsCursor == nil || *resume.TurnsBackwardsCursor != "turn-anchor" {
		t.Fatalf("resume cursors = %+v", resume)
	}
}

func TestPaginatedHistoryLegacyDefaults(t *testing.T) {
	var turn codex.Turn
	if err := json.Unmarshal([]byte(`{"id":"tu1","status":"completed","items":[]}`), &turn); err != nil {
		t.Fatal(err)
	}
	if turn.ItemsView != codex.TurnItemsViewFull {
		t.Fatalf("itemsView = %s", turn.ItemsView)
	}
	data, err := json.Marshal(validThreadPayload("t1"))
	if err != nil {
		t.Fatal(err)
	}
	var thread codex.Thread
	if err := json.Unmarshal(data, &thread); err != nil {
		t.Fatal(err)
	}
	if thread.HistoryMode != codex.ThreadHistoryModeLegacy {
		t.Fatalf("historyMode = %s", thread.HistoryMode)
	}
}

func TestPaginatedHistoryRejectsInvalidRequests(t *testing.T) {
	for _, call := range []func(*codex.Client) error{
		func(c *codex.Client) error {
			_, err := c.Thread.ItemsList(context.Background(), codex.ThreadItemsListParams{})
			return err
		},
		func(c *codex.Client) error {
			_, err := c.Thread.TurnsList(context.Background(), codex.ThreadTurnsListParams{})
			return err
		},
		func(c *codex.Client) error {
			_, err := c.Thread.Revert(context.Background(), codex.ThreadRevertParams{ThreadID: "t1"})
			return err
		},
		func(c *codex.Client) error {
			_, err := c.Thread.Revert(context.Background(), codex.ThreadRevertParams{BeforeTurnID: "tu1"})
			return err
		},
		func(c *codex.Client) error {
			_, err := c.Thread.ItemsList(context.Background(), codex.ThreadItemsListParams{ThreadID: "t1", SortDirection: codex.Ptr(codex.SortDirection("invalid"))})
			return err
		},
		func(c *codex.Client) error {
			_, err := c.Thread.TurnsList(context.Background(), codex.ThreadTurnsListParams{ThreadID: "t1", ItemsView: codex.Ptr(codex.TurnItemsView("invalid"))})
			return err
		},
	} {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		err := call(client)
		_ = client.Close()
		if err == nil {
			t.Fatal("invalid request accepted")
		}
		if transport.GetSentRequest(0) != nil {
			t.Fatal("invalid request reached transport")
		}
	}
}

func TestPaginatedHistoryRejectsMalformedResponses(t *testing.T) {
	for _, tc := range []struct {
		data   string
		target interface{}
	}{
		{`{}`, &codex.ThreadItemsListResponse{}},
		{`{"data":null}`, &codex.ThreadTurnsListResponse{}},
		{`{"data":[{"turnId":"tu1"}]}`, &codex.ThreadItemsListResponse{}},
		{`{"data":[{"item":null,"turnId":"tu1"}]}`, &codex.ThreadItemsListResponse{}},
		{`{"thread":null}`, &codex.ThreadRevertResponse{}},
		{`{"thread":{}}`, &codex.ThreadRevertResponse{}},
		{`{"id":"tu1","status":"completed","items":[],"itemsView":"invalid"}`, &codex.Turn{}},
	} {
		if err := json.Unmarshal([]byte(tc.data), tc.target); err == nil {
			t.Fatalf("accepted %s into %T", tc.data, tc.target)
		}
	}
}
