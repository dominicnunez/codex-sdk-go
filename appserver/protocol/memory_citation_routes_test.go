package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	protocol "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const (
	malformedCitationItem = `{"type":"agentMessage","id":"m","text":"t","memoryCitation":{"entries":[{}],"threadIds":[]}}`
	validCitationItem     = `{"type":"agentMessage","id":"m","text":"t","memoryCitation":{"entries":[{"lineEnd":3,"lineStart":1,"note":"note","path":"relative/path"}],"threadIds":["thread-ref"]}}`
)

func TestMemoryCitationTypedNotificationRoutes(t *testing.T) {
	itemStarted := `{"startedAtMs":1,"threadId":"thread-1","turnId":"turn-1","item":%s}`
	itemCompleted := `{"completedAtMs":1,"threadId":"thread-1","turnId":"turn-1","item":%s}`
	turnStarted := `{"threadId":"thread-1","turn":{"id":"turn-1","status":"inProgress","items":[%s]}}`
	turnCompleted := `{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[%s]}}`
	tests := []struct {
		name, method, envelope string
		register               func(*protocol.Client, func(protocol.ThreadItemWrapper), func(protocol.ThreadItemWrapper)) func()
	}{
		{
			name:   "item started",
			method: "item/started", envelope: itemStarted,
			register: func(c *protocol.Client, on, add func(protocol.ThreadItemWrapper)) func() {
				c.OnItemStarted(func(n protocol.ItemStartedNotification) { on(n.Item) })
				return c.AddItemStartedListener(func(n protocol.ItemStartedNotification) { add(n.Item) })
			},
		},
		{
			name:   "item completed",
			method: "item/completed", envelope: itemCompleted,
			register: func(c *protocol.Client, on, add func(protocol.ThreadItemWrapper)) func() {
				c.OnItemCompleted(func(n protocol.ItemCompletedNotification) { on(n.Item) })
				return c.AddItemCompletedListener(func(n protocol.ItemCompletedNotification) { add(n.Item) })
			},
		},
		{
			name:   "turn started",
			method: "turn/started", envelope: turnStarted,
			register: func(c *protocol.Client, on, add func(protocol.ThreadItemWrapper)) func() {
				c.OnTurnStarted(func(n protocol.TurnStartedNotification) { on(n.Turn.Items[0]) })
				return c.AddTurnStartedListener(func(n protocol.TurnStartedNotification) { add(n.Turn.Items[0]) })
			},
		},
		{
			name:   "turn completed",
			method: "turn/completed", envelope: turnCompleted,
			register: func(c *protocol.Client, on, add func(protocol.ThreadItemWrapper)) func() {
				c.OnTurnCompleted(func(n protocol.TurnCompletedNotification) { on(n.Turn.Items[0]) })
				return c.AddTurnCompletedListener(func(n protocol.TurnCompletedNotification) { add(n.Turn.Items[0]) })
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			transport := NewMockTransport()
			var onItems, addedItems []*protocol.MemoryCitation
			var reported []string
			client := protocol.NewClient(transport, protocol.WithHandlerErrorCallback(func(method string, err error) {
				if err == nil {
					t.Errorf("nil error reported for %s", method)
				}
				reported = append(reported, method)
			}))
			t.Cleanup(func() { _ = client.Close() })
			remove := tc.register(client, func(item protocol.ThreadItemWrapper) {
				onItems = append(onItems, item.Value.(*protocol.AgentMessageThreadItem).MemoryCitation)
			}, func(item protocol.ThreadItemWrapper) {
				addedItems = append(addedItems, item.Value.(*protocol.AgentMessageThreadItem).MemoryCitation)
			})
			defer remove()
			inject := func(item string) {
				payload := fmt.Sprintf(tc.envelope, item)
				transport.InjectServerNotification(context.Background(), protocol.Notification{JSONRPC: "2.0", Method: tc.method, Params: json.RawMessage(payload)})
			}
			inject(malformedCitationItem)
			if len(onItems) != 0 || len(addedItems) != 0 || !reflect.DeepEqual(reported, []string{tc.method, tc.method}) {
				t.Fatalf("invalid item route side effects: on=%d add=%d reports=%v", len(onItems), len(addedItems), reported)
			}
			inject(validCitationItem)
			if len(onItems) != 1 || len(addedItems) != 1 || len(reported) != 2 {
				t.Fatalf("valid recovery: on=%d add=%d reports=%v", len(onItems), len(addedItems), reported)
			}
			want := &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 3, LineStart: 1, Note: "note", Path: "relative/path"}}, ThreadIDs: []string{"thread-ref"}}
			assertMemoryCitationEqual(t, onItems[0], want)
			assertMemoryCitationEqual(t, addedItems[0], want)
		})
	}
}

func threadWithCitationItem(t *testing.T, id, rawItem string) json.RawMessage {
	t.Helper()
	thread := validProcessThreadPayload(id)
	thread["turns"] = []any{map[string]any{
		"id": "turn-1", "status": "completed", "items": []json.RawMessage{json.RawMessage(rawItem)},
	}}
	encoded, err := json.Marshal(thread)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestMemoryCitationThreadStartedCacheAdmission(t *testing.T) {
	transport := NewMockTransport()
	var reports, onCalls, addCalls, updates int
	var reportMethods []string
	client := protocol.NewClient(transport, protocol.WithHandlerErrorCallback(func(method string, err error) {
		if err == nil {
			t.Errorf("nil handler error from %s", method)
		}
		reports++
		reportMethods = append(reportMethods, method)
	}))
	t.Cleanup(func() { _ = client.Close() })
	client.OnThreadStarted(func(protocol.ThreadStartedNotification) { onCalls++ })
	remove := client.AddThreadStartedListener(func(protocol.ThreadStartedNotification) { addCalls++ })
	defer remove()
	prior := protocol.Thread{ID: "thread-1", Preview: "prior"}
	client.CacheThreadState(prior)
	removeState := client.AddThreadStateListener(prior.ID, func(protocol.Thread) { updates++ }, nil)
	defer removeState()
	updates = 0 // registration replays the existing cached snapshot
	inject := func(item string) {
		thread := threadWithCitationItem(t, prior.ID, item)
		payload := append([]byte(`{"thread":`), thread...)
		payload = append(payload, '}')
		transport.InjectServerNotification(context.Background(), protocol.Notification{JSONRPC: "2.0", Method: "thread/started", Params: payload})
	}
	inject(malformedCitationItem)
	if onCalls != 0 || addCalls != 0 || reports != 3 {
		t.Fatalf("malformed thread/started published: on=%d add=%d reports=%v", onCalls, addCalls, reportMethods)
	}
	state, ok := client.ThreadStateSnapshot(prior.ID)
	if !ok || !reflect.DeepEqual(state, prior) || updates != 0 {
		t.Fatalf("malformed thread/started changed cache: state=%+v updates=%d", state, updates)
	}
	if !reflect.DeepEqual(reportMethods, []string{"thread/started", "thread/started", "thread/started"}) {
		t.Fatalf("handler error origins = %v", reportMethods)
	}
	inject(validCitationItem)
	if onCalls != 1 || addCalls != 1 || reports != 3 || updates != 1 {
		t.Fatalf("valid thread/started recovery: on=%d add=%d reports=%v updates=%d", onCalls, addCalls, reportMethods, updates)
	}
	state, ok = client.ThreadStateSnapshot(prior.ID)
	if !ok || len(state.Turns) != 1 || updates != 1 {
		t.Fatalf("valid thread/started not cached: state=%+v updates=%d", state, updates)
	}
	citation := state.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation
	assertMemoryCitationEqual(t, citation, &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 3, LineStart: 1, Note: "note", Path: "relative/path"}}, ThreadIDs: []string{"thread-ref"}})
}

func TestMemoryCitationPublicServiceRoutes(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		invalid, valid string
		call           func(*protocol.Client) (*protocol.MemoryCitation, error)
	}{
		{
			name: "thread items list", method: "thread/items/list",
			invalid: `{"data":[{"turnId":"turn-1","item":` + malformedCitationItem + `}]}`,
			valid:   `{"data":[{"turnId":"turn-1","item":` + validCitationItem + `}]}`,
			call: func(c *protocol.Client) (*protocol.MemoryCitation, error) {
				resp, err := c.Thread.ItemsList(context.Background(), protocol.ThreadItemsListParams{ThreadID: "thread-1"})
				if err != nil {
					return nil, err
				}
				return resp.Data[0].Item.Value.(*protocol.AgentMessageThreadItem).MemoryCitation, nil
			},
		},
		{
			name: "thread turns list", method: "thread/turns/list",
			invalid: `{"data":[{"id":"turn-1","status":"completed","items":[` + malformedCitationItem + `]}]}`,
			valid:   `{"data":[{"id":"turn-1","status":"completed","items":[` + validCitationItem + `]}]}`,
			call: func(c *protocol.Client) (*protocol.MemoryCitation, error) {
				resp, err := c.Thread.TurnsList(context.Background(), protocol.ThreadTurnsListParams{ThreadID: "thread-1"})
				if err != nil {
					return nil, err
				}
				return resp.Data[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation, nil
			},
		},
		{
			name: "turn start", method: "turn/start",
			invalid: `{"turn":{"id":"turn-1","status":"completed","items":[` + malformedCitationItem + `]}}`,
			valid:   `{"turn":{"id":"turn-1","status":"completed","items":[` + validCitationItem + `]}}`,
			call: func(c *protocol.Client) (*protocol.MemoryCitation, error) {
				resp, err := c.Turn.Start(context.Background(), protocol.TurnStartParams{ThreadID: "thread-1", Input: []protocol.UserInput{&protocol.TextUserInput{Text: "prompt"}}})
				if err != nil {
					return nil, err
				}
				return resp.Turn.Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation, nil
			},
		},
		{
			name: "review start", method: "review/start",
			invalid: `{"reviewThreadId":"review-1","turn":{"id":"turn-1","status":"completed","items":[` + malformedCitationItem + `]}}`,
			valid:   `{"reviewThreadId":"review-1","turn":{"id":"turn-1","status":"completed","items":[` + validCitationItem + `]}}`,
			call: func(c *protocol.Client) (*protocol.MemoryCitation, error) {
				resp, err := c.Review.Start(context.Background(), protocol.ReviewStartParams{ThreadID: "thread-1", Target: protocol.ReviewTargetWrapper{Value: &protocol.UncommittedChangesReviewTarget{}}})
				if err != nil {
					return nil, err
				}
				return resp.Turn.Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation, nil
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := protocol.NewClient(transport)
			t.Cleanup(func() { _ = client.Close() })
			transport.SetResponse(tc.method, protocol.Response{Result: json.RawMessage(tc.invalid)})
			if value, err := tc.call(client); err == nil || value != nil {
				t.Fatalf("malformed service response admitted: value=%#v err=%v", value, err)
			}
			transport.SetResponse(tc.method, protocol.Response{Result: json.RawMessage(tc.valid)})
			value, err := tc.call(client)
			if err != nil {
				t.Fatalf("valid recovery failed: %v", err)
			}
			assertMemoryCitationEqual(t, value, &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 3, LineStart: 1, Note: "note", Path: "relative/path"}}, ThreadIDs: []string{"thread-ref"}})
		})
	}
}

func TestMemoryCitationThreadStartWithStateListenerAdmission(t *testing.T) {
	transport := NewMockTransport()
	client := protocol.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	calls := 0
	makeResponse := func(item string) map[string]any {
		thread := validProcessThreadPayload("thread-start-citation")
		thread["turns"] = []any{map[string]any{"id": "turn-1", "status": "completed", "items": []json.RawMessage{json.RawMessage(item)}}}
		return validProcessThreadStartResponse(thread)
	}
	if err := transport.SetResponseData("thread/start", makeResponse(malformedCitationItem)); err != nil {
		t.Fatal(err)
	}
	response, generation, unsubscribe, err := client.Thread.StartWithStateListener(context.Background(), protocol.ThreadStartParams{}, func(protocol.Thread) { calls++ }, func() { calls++ })
	if err == nil || response.Thread.ID != "" || generation != 0 || unsubscribe == nil || calls != 0 {
		t.Fatalf("malformed start admitted observation: response=%+v generation=%d calls=%d err=%v", response, generation, calls, err)
	}
	if state, ok := client.ThreadStateSnapshot("thread-start-citation"); ok || state.ID != "" || client.ThreadStateGeneration("thread-start-citation") != 0 {
		t.Fatalf("malformed start published cached state: state=%+v ok=%t generation=%d", state, ok, client.ThreadStateGeneration("thread-start-citation"))
	}
	client.CacheThreadState(protocol.Thread{ID: "thread-start-citation", Preview: "independent publication"})
	if calls != 0 {
		t.Fatalf("failed start left an active observer: callback count=%d", calls)
	}
	independent, ok := client.ThreadStateSnapshot("thread-start-citation")
	if !ok || independent.Preview != "independent publication" {
		t.Fatalf("independent cache publication missing: state=%+v ok=%t", independent, ok)
	}
	unsubscribe()
	if err := transport.SetResponseData("thread/start", makeResponse(validCitationItem)); err != nil {
		t.Fatal(err)
	}
	response, generation, unsubscribe, err = client.Thread.StartWithStateListener(context.Background(), protocol.ThreadStartParams{}, func(protocol.Thread) { calls++ }, nil)
	if err != nil || response.Thread.ID != "thread-start-citation" || generation == 0 || calls != 1 {
		t.Fatalf("valid start recovery: response=%+v generation=%d calls=%d err=%v", response, generation, calls, err)
	}
	defer unsubscribe()
	citation := response.Thread.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation
	assertMemoryCitationEqual(t, citation, &protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 3, LineStart: 1, Note: "note", Path: "relative/path"}}, ThreadIDs: []string{"thread-ref"}})
}

func TestMemoryCitationReadAndThreadStateOwnership(t *testing.T) {
	const id = "thread-citation-ownership"
	transport := NewMockTransport()
	client := protocol.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	responsePayload := validProcessThreadPayload(id)
	responsePayload["turns"] = []any{map[string]any{
		"id": "turn-ownership", "status": "completed", "items": []json.RawMessage{json.RawMessage(`{"type":"agentMessage","id":"m","text":"t","memoryCitation":{"entries":[{"lineEnd":5,"lineStart":2,"note":"initial","path":"opaque/path"},{"lineEnd":9,"lineStart":4,"note":"second","path":"relative/../second"}],"threadIds":["one","two"]}}`)},
	}}
	if err := transport.SetResponseData("thread/read", map[string]any{"thread": responsePayload}); err != nil {
		t.Fatal(err)
	}
	var listenerSnapshot protocol.Thread
	listenerCalls := 0
	remove := client.AddThreadStateListener(id, func(thread protocol.Thread) {
		listenerCalls++
		listenerSnapshot = thread
	}, nil)
	defer remove()
	response, err := client.Thread.Read(context.Background(), protocol.ThreadReadParams{ThreadID: id})
	if err != nil {
		t.Fatal(err)
	}
	if listenerCalls != 1 {
		t.Fatalf("read listener updates = %d, want 1", listenerCalls)
	}
	want := protocol.MemoryCitation{
		Entries: []protocol.MemoryCitationEntry{
			{LineEnd: 5, LineStart: 2, Note: "initial", Path: "opaque/path"},
			{LineEnd: 9, LineStart: 4, Note: "second", Path: "relative/../second"},
		},
		ThreadIDs: []string{"one", "two"},
	}
	responseCitation := response.Thread.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation
	cacheSnapshot, ok := client.ThreadStateSnapshot(id)
	if !ok {
		t.Fatal("read response was not cached")
	}
	cacheCitation := cacheSnapshot.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation
	listenerCitation := listenerSnapshot.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation
	if !reflect.DeepEqual(*responseCitation, want) || !reflect.DeepEqual(*cacheCitation, want) || !reflect.DeepEqual(*listenerCitation, want) {
		t.Fatalf("initial values differ: response=%+v cache=%+v listener=%+v", responseCitation, cacheCitation, listenerCitation)
	}

	responseCitation.Entries[0].Note = "mutated-response"
	responseCitation.ThreadIDs[0] = "mutated-response-id"
	cacheAgain, _ := client.ThreadStateSnapshot(id)
	assertMemoryCitationEqual(t, cacheAgain.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation, &want)
	assertMemoryCitationEqual(t, listenerSnapshot.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation, &want)

	cacheSnapshot.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation.Entries[1].Path = "mutated-cache-snapshot"
	cacheSnapshot.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation.ThreadIDs[1] = "mutated-cache-id"
	cacheAgain, _ = client.ThreadStateSnapshot(id)
	assertMemoryCitationEqual(t, cacheAgain.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation, &want)
	assertMemoryCitationEqual(t, listenerSnapshot.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation, &want)

	listenerSnapshot.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation.Entries[0].Path = "mutated-listener"
	listenerSnapshot.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation.ThreadIDs[0] = "mutated-listener-id"
	cacheAgain, _ = client.ThreadStateSnapshot(id)
	assertMemoryCitationEqual(t, cacheAgain.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation, &want)

	callerOwned := protocol.Thread{ID: "separate-cache-write", Turns: []protocol.Turn{{ID: "turn", Status: protocol.TurnStatusCompleted, Items: []protocol.ThreadItemWrapper{{Value: &protocol.AgentMessageThreadItem{
		ID: "m", MemoryCitation: &protocol.MemoryCitation{Entries: append([]protocol.MemoryCitationEntry(nil), want.Entries...), ThreadIDs: append([]string(nil), want.ThreadIDs...)}, Text: "t",
	}}}}}}
	client.CacheThreadState(callerOwned)
	callerOwned.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation.Entries[0].Note = "mutated-caller"
	stored, ok := client.ThreadStateSnapshot("separate-cache-write")
	if !ok {
		t.Fatal("caller cache write missing")
	}
	assertMemoryCitationEqual(t, stored.Turns[0].Items[0].Value.(*protocol.AgentMessageThreadItem).MemoryCitation, &want)
}
