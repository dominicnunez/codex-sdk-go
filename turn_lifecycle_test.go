package codex_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func writeStdioResult(enc *json.Encoder, id codex.RequestID, result interface{}) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	if err := enc.Encode(codex.Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	}); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return nil
}

func writeStdioNotification(enc *json.Encoder, method string, params interface{}) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal notification params: %w", err)
	}
	if err := enc.Encode(codex.Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}); err != nil {
		return fmt.Errorf("encode notification: %w", err)
	}
	return nil
}

func serveLifecycleOverStdio(serverReader io.Reader, serverWriter io.Writer, threadID, turnID, itemID, itemText string) error {
	scanner := bufio.NewScanner(serverReader)
	enc := json.NewEncoder(serverWriter)

	for scanner.Scan() {
		var req codex.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return fmt.Errorf("unmarshal request: %w", err)
		}

		switch req.Method {
		case "initialize":
			if err := writeStdioResult(enc, req.ID, validInitializeResponseData("codex-test/1.0")); err != nil {
				return err
			}
		case "thread/start":
			if err := writeStdioResult(enc, req.ID, validProcessThreadStartResponse(validProcessThreadPayload(threadID))); err != nil {
				return err
			}
		case "turn/start":
			if err := writeStdioResult(enc, req.ID, map[string]interface{}{
				"turn": map[string]interface{}{
					"id":     turnID,
					"status": "inProgress",
					"items":  []interface{}{},
				},
			}); err != nil {
				return err
			}
			if err := writeStdioNotification(enc, "item/completed", map[string]interface{}{
				"threadId": threadID,
				"turnId":   turnID,
				"item": map[string]interface{}{
					"type": "agentMessage",
					"id":   itemID,
					"text": itemText,
				},
			}); err != nil {
				return err
			}
			if err := writeStdioNotification(enc, "turn/completed", map[string]interface{}{
				"threadId": threadID,
				"turn": map[string]interface{}{
					"id":     turnID,
					"status": "completed",
					"items":  []interface{}{},
				},
			}); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("unexpected method %q", req.Method)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}
	return nil
}

func serveBurstLifecycleOverStdio(serverReader io.Reader, serverWriter io.Writer, threadID, turnID string, itemCount int) error {
	scanner := bufio.NewScanner(serverReader)
	enc := json.NewEncoder(serverWriter)

	for scanner.Scan() {
		var req codex.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return fmt.Errorf("unmarshal request: %w", err)
		}

		switch req.Method {
		case "initialize":
			if err := writeStdioResult(enc, req.ID, validInitializeResponseData("codex-test/1.0")); err != nil {
				return err
			}
		case "thread/start":
			if err := writeStdioResult(enc, req.ID, validProcessThreadStartResponse(validProcessThreadPayload(threadID))); err != nil {
				return err
			}
		case "turn/start":
			if err := writeStdioResult(enc, req.ID, map[string]interface{}{
				"turn": map[string]interface{}{
					"id":     turnID,
					"status": "inProgress",
					"items":  []interface{}{},
				},
			}); err != nil {
				return err
			}
			for i := 1; i <= itemCount; i++ {
				if err := writeStdioNotification(enc, "item/completed", map[string]interface{}{
					"threadId": threadID,
					"turnId":   turnID,
					"item": map[string]interface{}{
						"type": "agentMessage",
						"id":   fmt.Sprintf("item-%03d", i),
						"text": fmt.Sprintf("message %03d", i),
					},
				}); err != nil {
					return err
				}
			}
			if err := writeStdioNotification(enc, "turn/completed", map[string]interface{}{
				"threadId": threadID,
				"turn": map[string]interface{}{
					"id":     turnID,
					"status": "completed",
					"items":  []interface{}{},
				},
			}); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("unexpected method %q", req.Method)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}
	return nil
}

func blockFirstItemCompleted(client *codex.Client) (<-chan struct{}, chan struct{}) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var blocked atomic.Bool

	client.OnItemCompleted(func(codex.ItemCompletedNotification) {
		if !blocked.CompareAndSwap(false, true) {
			return
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
	})

	return started, release
}

// notifyDuringSendTransport wraps a MockTransport and fires a turn/completed
// notification during the turn/start RPC — before returning the response.
// This simulates a fast server that pushes notifications while the client is
// still inside Send, proving that notification listeners are already registered
// at that point.
type notifyDuringSendTransport struct {
	*MockTransport
	notifHandler codex.NotificationHandler
	threadID     string
}

type staleMalformedTurnCompletedTransport struct {
	*MockTransport
	notifHandler codex.NotificationHandler
	threadID     string
	turnStarts   int
}

func (t *notifyDuringSendTransport) OnNotify(handler codex.NotificationHandler) {
	t.notifHandler = handler
	t.MockTransport.OnNotify(handler)
}

func (t *staleMalformedTurnCompletedTransport) OnNotify(handler codex.NotificationHandler) {
	t.notifHandler = handler
	t.MockTransport.OnNotify(handler)
}

func (t *notifyDuringSendTransport) Send(ctx context.Context, req codex.Request) (codex.Response, error) {
	resp, err := t.MockTransport.Send(ctx, req)
	if err != nil {
		return resp, err
	}

	// After the mock returns the turn/start response but before we return it
	// to the caller, inject item/completed and turn/completed notifications.
	// The caller (executeTurn) has already registered listeners before calling
	// Send, so these notifications must be handled correctly.
	if req.Method == "turn/start" && t.notifHandler != nil {
		t.notifHandler(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "item/completed",
			Params:  json.RawMessage(`{"threadId":"` + t.threadID + `","turnId":"turn-1","item":{"type":"agentMessage","id":"item-early","text":"early bird"}}`),
		})
		t.notifHandler(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "turn/completed",
			Params:  json.RawMessage(`{"threadId":"` + t.threadID + `","turn":{"id":"turn-1","status":"completed","items":[]}}`),
		})
	}

	return resp, nil
}

func (t *staleMalformedTurnCompletedTransport) Send(ctx context.Context, req codex.Request) (codex.Response, error) {
	if req.Method == "turn/start" {
		t.turnStarts++
		turnID := fmt.Sprintf("turn-%d", t.turnStarts)
		if err := t.SetResponseData("turn/start", map[string]interface{}{
			"turn": map[string]interface{}{
				"id":     turnID,
				"status": "inProgress",
				"items":  []interface{}{},
			},
		}); err != nil {
			return codex.Response{}, err
		}
	}

	resp, err := t.MockTransport.Send(ctx, req)
	if err != nil {
		return resp, err
	}

	if req.Method == "turn/start" && t.notifHandler != nil {
		turnID := fmt.Sprintf("turn-%d", t.turnStarts)
		if t.turnStarts == 2 {
			t.notifHandler(ctx, codex.Notification{
				JSONRPC: "2.0",
				Method:  "turn/completed",
				Params:  json.RawMessage(`{"threadId":"` + t.threadID + `","turn":{}}`),
			})
		}
		t.notifHandler(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "item/completed",
			Params: json.RawMessage(
				`{"threadId":"` + t.threadID + `","turnId":"` + turnID + `","item":{"type":"agentMessage","id":"item-` + turnID + `","text":"response for ` + turnID + `"}}`,
			),
		})
		t.notifHandler(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "turn/completed",
			Params:  json.RawMessage(`{"threadId":"` + t.threadID + `","turn":{"id":"` + turnID + `","status":"completed","items":[]}}`),
		})
	}

	return resp, nil
}

func TestRunNotificationBeforeTurnStartResponse(t *testing.T) {
	base := NewMockTransport()

	_ = base.SetResponseData("initialize", validInitializeResponseData("codex-test/1.0"))
	_ = base.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread-1")))
	_ = base.SetResponseData("turn/start", map[string]interface{}{
		"turn": map[string]interface{}{
			"id":     "turn-1",
			"status": "inProgress",
			"items":  []interface{}{},
		},
	})

	transport := &notifyDuringSendTransport{
		MockTransport: base,
		threadID:      "thread-1",
	}

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, codex.RunOptions{Prompt: "early notification"})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.Response != "early bird" {
		t.Errorf("Response = %q, want %q", result.Response, "early bird")
	}
	if len(result.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(result.Items))
	}
	if len(result.Turn.Items) != 1 {
		t.Errorf("len(Turn.Items) = %d, want 1", len(result.Turn.Items))
	}
	if len(result.Thread.Turns) != 1 {
		t.Fatalf("len(Thread.Turns) = %d, want 1", len(result.Thread.Turns))
	}
	if len(result.Thread.Turns[0].Items) != 1 {
		t.Errorf("len(Thread.Turns[0].Items) = %d, want 1", len(result.Thread.Turns[0].Items))
	}
	if result.Turn.ID != "turn-1" {
		t.Errorf("Turn.ID = %q, want %q", result.Turn.ID, "turn-1")
	}
}

func TestRunStreamedNotificationBeforeTurnStartResponse(t *testing.T) {
	base := NewMockTransport()

	_ = base.SetResponseData("initialize", validInitializeResponseData("codex-test/1.0"))
	_ = base.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread-1")))
	_ = base.SetResponseData("turn/start", map[string]interface{}{
		"turn": map[string]interface{}{
			"id":     "turn-1",
			"status": "inProgress",
			"items":  []interface{}{},
		},
	})

	transport := &notifyDuringSendTransport{
		MockTransport: base,
		threadID:      "thread-1",
	}

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "early notification"})

	var events []codex.Event
	for event, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, event)
	}

	// Expect at least ItemCompleted and TurnCompleted events.
	var gotItemCompleted, gotTurnCompleted bool
	for _, e := range events {
		switch e.(type) {
		case *codex.ItemCompleted:
			gotItemCompleted = true
		case *codex.TurnCompleted:
			gotTurnCompleted = true
		}
	}

	if !gotItemCompleted {
		t.Error("missing ItemCompleted event from early notification")
	}
	if !gotTurnCompleted {
		t.Error("missing TurnCompleted event from early notification")
	}

	result := stream.Result()
	if result == nil {
		t.Fatal("Result() returned nil")
		return
	}
	if result.Response != "early bird" {
		t.Errorf("Response = %q, want %q", result.Response, "early bird")
	}
	if len(result.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(result.Items))
	}
}

func TestConversationTurnIgnoresUnattributableMalformedCompletionOnReusedThread(t *testing.T) {
	base := NewMockTransport()

	_ = base.SetResponseData("initialize", validInitializeResponseData("codex-test/1.0"))
	_ = base.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread-1")))

	transport := &staleMalformedTurnCompletedTransport{
		MockTransport: base,
		threadID:      "thread-1",
	}

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation() error = %v", err)
	}

	first, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "first"})
	if err != nil {
		t.Fatalf("Turn 1 error = %v", err)
	}
	if first.Turn.ID != "turn-1" {
		t.Fatalf("Turn 1 ID = %q; want %q", first.Turn.ID, "turn-1")
	}

	second, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "second"})
	if err != nil {
		t.Fatalf("Turn 2 error = %v", err)
	}
	if second.Turn.ID != "turn-2" {
		t.Fatalf("Turn 2 ID = %q; want %q", second.Turn.ID, "turn-2")
	}
	if second.Response != "response for turn-2" {
		t.Fatalf("Turn 2 response = %q; want %q", second.Response, "response for turn-2")
	}
}

func TestConversationTurnStreamedIgnoresUnattributableMalformedCompletionOnReusedThread(t *testing.T) {
	base := NewMockTransport()

	_ = base.SetResponseData("initialize", validInitializeResponseData("codex-test/1.0"))
	_ = base.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread-1")))

	transport := &staleMalformedTurnCompletedTransport{
		MockTransport: base,
		threadID:      "thread-1",
	}

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation() error = %v", err)
	}

	first := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "first"})
	for _, err := range first.Events() {
		if err != nil {
			t.Fatalf("Turn 1 streamed error = %v", err)
		}
	}
	if first.Result() == nil || first.Result().Turn.ID != "turn-1" {
		t.Fatalf("Turn 1 streamed result = %#v; want turn-1", first.Result())
	}

	second := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "second"})
	for _, err := range second.Events() {
		if err != nil {
			t.Fatalf("Turn 2 streamed error = %v", err)
		}
	}

	result := second.Result()
	if result == nil {
		t.Fatal("Turn 2 streamed result is nil")
		return
	}
	if result.Turn.ID != "turn-2" {
		t.Fatalf("Turn 2 streamed ID = %q; want %q", result.Turn.ID, "turn-2")
	}
	if result.Response != "response for turn-2" {
		t.Fatalf("Turn 2 streamed response = %q; want %q", result.Response, "response for turn-2")
	}
}

func TestRunWaitsForBlockedItemCompletedHandler(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	const (
		threadID = "thread-1"
		turnID   = "turn-1"
		itemID   = "item-1"
		itemText = "final answer"
	)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- serveLifecycleOverStdio(serverReader, serverWriter, threadID, turnID, itemID, itemText)
	}()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	itemHandlingStarted := make(chan struct{}, 1)
	releaseItem := make(chan struct{})
	client.OnItemCompleted(func(codex.ItemCompletedNotification) {
		select {
		case itemHandlingStarted <- struct{}{}:
		default:
		}
		<-releaseItem
	})

	proc := codex.NewProcessFromClient(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *codex.RunResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := proc.Run(ctx, codex.RunOptions{Prompt: "hello"})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case <-itemHandlingStarted:
	case err := <-errCh:
		t.Fatalf("Run() returned early with error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for item/completed handler to block")
	}

	select {
	case result := <-resultCh:
		t.Fatalf("Run() finished before item handler was released: %#v", result)
	case err := <-errCh:
		t.Fatalf("Run() failed before item handler was released: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseItem)

	select {
	case err := <-errCh:
		t.Fatalf("Run() error: %v", err)
	case result := <-resultCh:
		if result.Response != itemText {
			t.Fatalf("Response = %q; want %q", result.Response, itemText)
		}
		if len(result.Items) != 1 {
			t.Fatalf("len(Items) = %d; want 1", len(result.Items))
		}
		if len(result.Turn.Items) != 1 {
			t.Fatalf("len(Turn.Items) = %d; want 1", len(result.Turn.Items))
		}
		select {
		case err := <-serverErrCh:
			if err != nil {
				t.Fatalf("stdio server error: %v", err)
			}
		default:
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Run() to finish")
	}
}

func TestRunStreamedWaitsForBlockedItemCompletedHandler(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	const (
		threadID = "thread-1"
		turnID   = "turn-1"
		itemID   = "item-1"
		itemText = "streamed answer"
	)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- serveLifecycleOverStdio(serverReader, serverWriter, threadID, turnID, itemID, itemText)
	}()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	itemHandlingStarted := make(chan struct{}, 1)
	releaseItem := make(chan struct{})
	client.OnItemCompleted(func(codex.ItemCompletedNotification) {
		select {
		case itemHandlingStarted <- struct{}{}:
		default:
		}
		<-releaseItem
	})

	proc := codex.NewProcessFromClient(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})
	streamResultCh := make(chan *codex.RunResult, 1)
	streamErrCh := make(chan error, 1)
	go func() {
		for _, err := range stream.Events() {
			if err != nil {
				streamErrCh <- err
				return
			}
		}
		streamResultCh <- stream.Result()
	}()

	select {
	case <-itemHandlingStarted:
	case err := <-streamErrCh:
		t.Fatalf("RunStreamed() returned early with error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for streamed item/completed handler to block")
	}

	select {
	case result := <-streamResultCh:
		t.Fatalf("RunStreamed() finished before item handler was released: %#v", result)
	case err := <-streamErrCh:
		t.Fatalf("RunStreamed() failed before item handler was released: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseItem)

	select {
	case err := <-streamErrCh:
		t.Fatalf("RunStreamed() error: %v", err)
	case result := <-streamResultCh:
		if result == nil {
			t.Fatal("Result() returned nil")
			return
		}
		if result.Response != itemText {
			t.Fatalf("Response = %q; want %q", result.Response, itemText)
		}
		if len(result.Items) != 1 {
			t.Fatalf("len(Items) = %d; want 1", len(result.Items))
		}
		if len(result.Turn.Items) != 1 {
			t.Fatalf("len(Turn.Items) = %d; want 1", len(result.Turn.Items))
		}
		select {
		case err := <-serverErrCh:
			if err != nil {
				t.Fatalf("stdio server error: %v", err)
			}
		default:
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for RunStreamed() to finish")
	}
}

func TestRunCompletesWithAllItemsUnderTurnNotificationBacklog(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	const (
		threadID  = "thread-burst"
		turnID    = "turn-burst"
		itemCount = 200
	)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- serveBurstLifecycleOverStdio(serverReader, serverWriter, threadID, turnID, itemCount)
	}()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	itemHandlingStarted, releaseItem := blockFirstItemCompleted(client)
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *codex.RunResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := proc.Run(ctx, codex.RunOptions{Prompt: "burst"})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case <-itemHandlingStarted:
	case err := <-errCh:
		t.Fatalf("Run() returned early with error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first item/completed handler to block")
	}

	select {
	case result := <-resultCh:
		t.Fatalf("Run() finished before releasing blocked handler: %#v", result)
	case err := <-errCh:
		t.Fatalf("Run() failed before releasing blocked handler: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseItem)

	select {
	case err := <-errCh:
		t.Fatalf("Run() error: %v", err)
	case result := <-resultCh:
		if got := len(result.Items); got != itemCount {
			t.Fatalf("len(Items) = %d; want %d", got, itemCount)
		}
		if got := len(result.Turn.Items); got != itemCount {
			t.Fatalf("len(Turn.Items) = %d; want %d", got, itemCount)
		}
		if result.Response != "message 200" {
			t.Fatalf("Response = %q; want %q", result.Response, "message 200")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Run() to finish")
	}

	select {
	case err := <-serverErrCh:
		if err != nil {
			t.Fatalf("stdio server error: %v", err)
		}
	default:
	}
}

func TestConversationTurnCompletesWithAllItemsUnderTurnNotificationBacklog(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	const (
		threadID  = "thread-conversation"
		turnID    = "turn-conversation"
		itemCount = 200
	)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- serveBurstLifecycleOverStdio(serverReader, serverWriter, threadID, turnID, itemCount)
	}()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	itemHandlingStarted, releaseItem := blockFirstItemCompleted(client)
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation() error: %v", err)
	}

	resultCh := make(chan *codex.RunResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "burst"})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case <-itemHandlingStarted:
	case err := <-errCh:
		t.Fatalf("Turn() returned early with error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first item/completed handler to block")
	}

	select {
	case result := <-resultCh:
		t.Fatalf("Turn() finished before releasing blocked handler: %#v", result)
	case err := <-errCh:
		t.Fatalf("Turn() failed before releasing blocked handler: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseItem)

	select {
	case err := <-errCh:
		t.Fatalf("Turn() error: %v", err)
	case result := <-resultCh:
		if got := len(result.Items); got != itemCount {
			t.Fatalf("len(Items) = %d; want %d", got, itemCount)
		}
		if got := len(result.Turn.Items); got != itemCount {
			t.Fatalf("len(Turn.Items) = %d; want %d", got, itemCount)
		}
		if result.Response != "message 200" {
			t.Fatalf("Response = %q; want %q", result.Response, "message 200")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Turn() to finish")
	}

	select {
	case err := <-serverErrCh:
		if err != nil {
			t.Fatalf("stdio server error: %v", err)
		}
	default:
	}
}

func TestRunStreamedCompletesWithAllItemsUnderTurnNotificationBacklog(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	defer func() { _ = clientReader.Close() }()
	defer func() { _ = serverWriter.Close() }()
	defer func() { _ = serverReader.Close() }()
	defer func() { _ = clientWriter.Close() }()

	const (
		threadID  = "thread-streamed"
		turnID    = "turn-streamed"
		itemCount = 200
	)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- serveBurstLifecycleOverStdio(serverReader, serverWriter, threadID, turnID, itemCount)
	}()

	transport := codex.NewStdioTransport(clientReader, clientWriter)
	defer func() { _ = transport.Close() }()

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	itemHandlingStarted, releaseItem := blockFirstItemCompleted(client)
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "burst"})
	doneCh := make(chan struct{})
	streamErrCh := make(chan error, 1)
	go func() {
		defer close(doneCh)
		for _, err := range stream.Events() {
			if err != nil {
				streamErrCh <- err
				return
			}
		}
	}()

	select {
	case <-itemHandlingStarted:
	case err := <-streamErrCh:
		t.Fatalf("RunStreamed() returned early with error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first item/completed handler to block")
	}

	select {
	case <-doneCh:
		t.Fatal("RunStreamed() finished before releasing blocked handler")
	case err := <-streamErrCh:
		t.Fatalf("RunStreamed() failed before releasing blocked handler: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseItem)

	select {
	case err := <-streamErrCh:
		t.Fatalf("RunStreamed() error: %v", err)
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for RunStreamed() to finish")
	}

	result := stream.Result()
	if result == nil {
		t.Fatal("Result() returned nil")
		return
	}
	if got := len(result.Items); got != itemCount {
		t.Fatalf("len(Items) = %d; want %d", got, itemCount)
	}
	if got := len(result.Turn.Items); got != itemCount {
		t.Fatalf("len(Turn.Items) = %d; want %d", got, itemCount)
	}
	if result.Response != "message 200" {
		t.Fatalf("Response = %q; want %q", result.Response, "message 200")
	}

	select {
	case err := <-serverErrCh:
		if err != nil {
			t.Fatalf("stdio server error: %v", err)
		}
	default:
	}
}
