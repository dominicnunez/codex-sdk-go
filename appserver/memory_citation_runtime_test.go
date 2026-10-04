package appserver_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
)

const (
	badCitationItem  = `{"type":"agentMessage","id":"bad","text":"discard","memoryCitation":{"threadIds":[]}}`
	goodCitationItem = `{"type":"agentMessage","id":"good","text":"answer","memoryCitation":{"threadIds":["thread-source"],"entries":[{"lineEnd":9,"lineStart":2,"note":"memory note","path":"relative/file.md"}]}}`
	completedTurn    = `{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`
)

func injectCompletedItem(ctx context.Context, mock *MockTransport, raw string) {
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"completedAtMs":1,"threadId":"thread-1","turnId":"turn-1","item":` + raw + `}`),
	})
}

func injectCompletedTurn(ctx context.Context, mock *MockTransport) {
	mock.InjectServerNotification(ctx, codex.Notification{JSONRPC: "2.0", Method: "turn/completed", Params: json.RawMessage(completedTurn)})
}

func assertValidCitationRuntimeItem(t *testing.T, value codex.ThreadItem) {
	t.Helper()
	item, ok := value.(*codex.AgentMessageThreadItem)
	if !ok {
		t.Fatalf("valid item type = %T, want *AgentMessageThreadItem", value)
	}
	if item.ID != "good" || item.Text != "answer" || item.MemoryCitation == nil {
		t.Fatalf("valid item = %#v", item)
	}
	citation := item.MemoryCitation
	if len(citation.ThreadIDs) != 1 || citation.ThreadIDs[0] != "thread-source" || len(citation.Entries) != 1 {
		t.Fatalf("citation = %#v", citation)
	}
	entry := citation.Entries[0]
	if entry.LineStart != 2 || entry.LineEnd != 9 || entry.Note != "memory note" || entry.Path != "relative/file.md" {
		t.Fatalf("entry = %#v", entry)
	}
}

func TestRunCitationAdmissionFallbackThenRecovery(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type outcome struct {
		result *codex.RunResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := proc.Run(ctx, codex.RunOptions{Prompt: "synthetic citation admission"})
		done <- outcome{result, err}
	}()
	waitForMethodCallCount(t, mock, "turn/start", 1)
	injectCompletedItem(ctx, mock, badCitationItem)
	injectCompletedItem(ctx, mock, goodCitationItem)
	injectCompletedTurn(ctx, mock)
	got := <-done
	if got.err != nil {
		t.Fatalf("Run() error = %v", got.err)
	}
	if got.result == nil || len(got.result.Items) != 2 {
		t.Fatalf("Run() result = %#v", got.result)
	}
	unknown, ok := got.result.Items[0].Value.(*codex.UnknownThreadItem)
	if !ok {
		t.Fatalf("invalid item type = %T, want diagnostic UnknownThreadItem", got.result.Items[0].Value)
	}
	if unknown.Type != codex.UnmarshalErrorItemType {
		t.Fatalf("fallback type = %q", unknown.Type)
	}
	if string(unknown.Raw) != badCitationItem {
		t.Fatalf("fallback raw = %s, want exact invalid item", unknown.Raw)
	}
	assertValidCitationRuntimeItem(t, got.result.Items[1].Value)
	if got.result.Response != "answer" {
		t.Fatalf("Response = %q", got.result.Response)
	}
	if len(got.result.Thread.Turns) != 1 || len(got.result.Thread.Turns[0].Items) != 2 {
		t.Fatalf("thread snapshot = %#v", got.result.Thread.Turns)
	}
}

func TestRunStreamedCitationAdmissionFallbackThenRecovery(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "synthetic citation admission"})
	waitForRunStreamedReady(t, mock)
	injectCompletedItem(ctx, mock, badCitationItem)
	injectCompletedItem(ctx, mock, goodCitationItem)
	injectCompletedTurn(ctx, mock)
	var invalidEvent, validEvent bool
	var emittedValid *codex.AgentMessageThreadItem
	for event, err := range stream.Events() {
		if err != nil {
			t.Fatalf("Events() error = %v", err)
		}
		completed, ok := event.(*codex.ItemCompleted)
		if !ok {
			continue
		}
		switch value := completed.Item.Value.(type) {
		case *codex.UnknownThreadItem:
			if string(value.Raw) != badCitationItem {
				t.Fatalf("event fallback raw = %s", value.Raw)
			}
			invalidEvent = true
		case *codex.AgentMessageThreadItem:
			assertValidCitationRuntimeItem(t, value)
			emittedValid = value
			validEvent = true
		}
	}
	if !invalidEvent || !validEvent {
		t.Fatalf("fallback event=%t valid event=%t", invalidEvent, validEvent)
	}
	if emittedValid == nil {
		t.Fatal("missing emitted valid message")
	}
	emittedValid.MemoryCitation.Entries[0].Note = "mutated event"
	result := stream.Result()
	if result == nil || len(result.Items) != 2 {
		t.Fatalf("Result() = %#v", result)
	}
	unknown, ok := result.Items[0].Value.(*codex.UnknownThreadItem)
	if !ok || string(unknown.Raw) != badCitationItem {
		t.Fatalf("result fallback = %#v", result.Items[0].Value)
	}
	assertValidCitationRuntimeItem(t, result.Items[1].Value)
	// Events and Result are separate owned snapshots.
	if result.Items[1].Value.(*codex.AgentMessageThreadItem).MemoryCitation.Entries[0].Note != "memory note" {
		t.Fatal("event mutation changed Result snapshot")
	}
	result.Items[1].Value.(*codex.AgentMessageThreadItem).MemoryCitation.Entries[0].Note = "mutated result"
	if emittedValid.MemoryCitation.Entries[0].Note != "mutated event" {
		t.Fatal("Result mutation changed emitted event")
	}
}

func TestRunStreamedMalformedCitationCompletionIsFailedTurn(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "synthetic malformed completion"})
	waitForRunStreamedReady(t, mock)
	bad := `{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[` + badCitationItem + `]}}`
	mock.InjectServerNotification(ctx, codex.Notification{JSONRPC: "2.0", Method: "turn/completed", Params: json.RawMessage(bad)})
	var completionErr error
	for event, err := range stream.Events() {
		if err != nil {
			completionErr = err
			continue
		}
		_ = event
	}
	if completionErr == nil || !strings.Contains(completionErr.Error(), "failed to unmarshal turn/completed") || !strings.Contains(completionErr.Error(), "memoryCitation.entries is required") {
		t.Fatalf("completion error = %v", completionErr)
	}
	if got := stream.Result(); got != nil {
		t.Fatalf("Result() = %#v, want nil for failed malformed completion", got)
	}
}

func TestRunMalformedCitationCompletionReturnsFailedTurnError(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type outcome struct {
		result *codex.RunResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := proc.Run(ctx, codex.RunOptions{Prompt: "synthetic malformed completion"})
		done <- outcome{result, err}
	}()
	waitForMethodCallCount(t, mock, "turn/start", 1)
	bad := `{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[` + badCitationItem + `]}}`
	mock.InjectServerNotification(ctx, codex.Notification{JSONRPC: "2.0", Method: "turn/completed", Params: json.RawMessage(bad)})
	got := <-done
	if got.result != nil {
		t.Fatalf("Run() result = %#v, want nil for failed malformed completion", got.result)
	}
	if got.err == nil || !strings.Contains(got.err.Error(), "failed to unmarshal turn/completed") || !strings.Contains(got.err.Error(), "memoryCitation.entries is required") {
		t.Fatalf("Run() error = %v", got.err)
	}
}

func TestConversationCitationAdmissionAcrossReusedTurns(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conversation, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation() error = %v", err)
	}
	defer conversation.Close()

	type turnOutcome struct {
		result *codex.RunResult
		err    error
	}
	firstDone := make(chan turnOutcome, 1)
	go func() {
		result, turnErr := conversation.Turn(ctx, codex.TurnOptions{Prompt: "first synthetic turn"})
		firstDone <- turnOutcome{result, turnErr}
	}()
	waitForMethodCallCount(t, mock, "turn/start", 1)
	injectCompletedItem(ctx, mock, badCitationItem)
	injectCompletedItem(ctx, mock, goodCitationItem)
	injectCompletedTurn(ctx, mock)
	first := <-firstDone
	if first.err != nil || first.result == nil || len(first.result.Items) != 2 {
		t.Fatalf("first Turn() result=%#v err=%v", first.result, first.err)
	}
	if _, ok := first.result.Items[0].Value.(*codex.UnknownThreadItem); !ok {
		t.Fatalf("first invalid item type = %T", first.result.Items[0].Value)
	}
	assertValidCitationRuntimeItem(t, first.result.Items[1].Value)

	stream := conversation.TurnStreamed(ctx, codex.TurnOptions{Prompt: "second synthetic turn"})
	waitForMethodCallCount(t, mock, "turn/start", 2)
	injectCompletedItem(ctx, mock, goodCitationItem)
	injectCompletedTurn(ctx, mock)
	for _, err := range rangeStreamEvents(stream) {
		if err != nil {
			t.Fatalf("second TurnStreamed event error = %v", err)
		}
	}
	second := stream.Result()
	if second == nil || len(second.Items) != 1 {
		t.Fatalf("second TurnStreamed result = %#v", second)
	}
	assertValidCitationRuntimeItem(t, second.Items[0].Value)

	snapshot := conversation.Thread()
	if len(snapshot.Turns) != 2 || len(snapshot.Turns[0].Items) != 2 || len(snapshot.Turns[1].Items) != 1 {
		t.Fatalf("Conversation.Thread() turns = %#v", snapshot.Turns)
	}
	last := snapshot.Turns[1].Items[0].Value.(*codex.AgentMessageThreadItem)
	assertValidCitationRuntimeItem(t, last)
	last.MemoryCitation.Entries[0].Note = "snapshot mutation"
	if got := conversation.Thread().Turns[1].Items[0].Value.(*codex.AgentMessageThreadItem).MemoryCitation.Entries[0].Note; got != "memory note" {
		t.Fatalf("Conversation.Thread snapshot mutation reached internal state: %q", got)
	}
}

func rangeStreamEvents(stream *codex.Stream) []error {
	var errs []error
	for _, err := range stream.Events() {
		errs = append(errs, err)
	}
	return errs
}
