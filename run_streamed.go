package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"sync"
)

const streamChannelBuffer = 64

// eventOrErr pairs an Event with an error for channel transport.
type eventOrErr struct {
	event Event
	err   error
}

// Stream holds the streaming iterator and result for a RunStreamed call.
type Stream struct {
	// Events yields (Event, error) pairs. Iterate with a range-over-func loop.
	// Iteration ends when the turn completes, an error occurs, or the context is cancelled.
	Events iter.Seq2[Event, error]

	result *RunResult
	done   chan struct{}
	mu     sync.Mutex
}

// Result returns the RunResult after the stream has completed.
// Blocks until the turn finishes. Returns nil if the turn errored
// (the error was already surfaced through the Events iterator).
func (s *Stream) Result() *RunResult {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result
}

// RunStreamed executes a single-turn conversation like Run, but yields events
// through an iterator instead of blocking until completion. Returns immediately;
// the lifecycle runs in a background goroutine.
func (p *Process) RunStreamed(ctx context.Context, opts RunOptions) *Stream {
	ch := make(chan eventOrErr, streamChannelBuffer)
	s := &Stream{
		done: make(chan struct{}),
	}

	s.Events = func(yield func(Event, error) bool) {
		for eoe := range ch {
			if !yield(eoe.event, eoe.err) {
				return
			}
		}
	}

	go p.runStreamedLifecycle(ctx, opts, ch, s)

	return s
}

// send attempts a non-blocking send on ch, respecting context cancellation.
func streamSend(ctx context.Context, ch chan<- eventOrErr, eoe eventOrErr) bool {
	select {
	case ch <- eoe:
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *Process) runStreamedLifecycle(ctx context.Context, opts RunOptions, ch chan<- eventOrErr, s *Stream) {
	defer close(ch)
	defer close(s.done)

	if opts.Prompt == "" {
		streamSend(ctx, ch, eventOrErr{err: errors.New("prompt is required")})
		return
	}

	// Idempotent initialize handshake.
	p.initOnce.Do(func() {
		_, p.initErr = p.Client.Initialize(ctx, InitializeParams{
			ClientInfo: ClientInfo{Name: "codex-sdk-go", Version: "0.1.0"},
		})
	})
	if p.initErr != nil {
		streamSend(ctx, ch, eventOrErr{err: fmt.Errorf("initialize: %w", p.initErr)})
		return
	}

	// Start a thread.
	threadParams := ThreadStartParams{
		Ephemeral: Ptr(true),
	}
	if opts.Instructions != nil {
		threadParams.DeveloperInstructions = opts.Instructions
	}
	if opts.Model != nil {
		threadParams.Model = opts.Model
	}
	if opts.Personality != nil {
		threadParams.Personality = opts.Personality
	}
	if opts.ApprovalPolicy != nil {
		threadParams.ApprovalPolicy = opts.ApprovalPolicy
	}

	threadResp, err := p.Client.Thread.Start(ctx, threadParams)
	if err != nil {
		streamSend(ctx, ch, eventOrErr{err: fmt.Errorf("thread/start: %w", err)})
		return
	}

	// Register internal listeners before starting the turn to avoid missing events.
	var (
		items      []ThreadItemWrapper
		itemsMu    sync.Mutex
		unsubFuncs []func()
	)
	defer func() {
		for _, unsub := range unsubFuncs {
			unsub()
		}
	}()

	on := func(method string, handler NotificationHandler) {
		unsub := p.Client.addNotificationListener(method, handler)
		unsubFuncs = append(unsubFuncs, unsub)
	}

	turnDone := make(chan TurnCompletedNotification, 1)

	on(notifyTurnStarted, func(_ context.Context, notif Notification) {
		var n TurnStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		streamSend(ctx, ch, eventOrErr{event: &TurnStarted{Turn: n.Turn, ThreadID: n.ThreadID}})
	})

	on(notifyAgentMessageDelta, func(_ context.Context, notif Notification) {
		var n AgentMessageDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		streamSend(ctx, ch, eventOrErr{event: &TextDelta{Delta: n.Delta, ItemID: n.ItemID}})
	})

	on(notifyReasoningTextDelta, func(_ context.Context, notif Notification) {
		var n ReasoningTextDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		streamSend(ctx, ch, eventOrErr{event: &ReasoningDelta{Delta: n.Delta, ItemID: n.ItemID, ContentIndex: n.ContentIndex}})
	})

	on(notifyReasoningSummaryTextDelta, func(_ context.Context, notif Notification) {
		var n ReasoningSummaryTextDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		streamSend(ctx, ch, eventOrErr{event: &ReasoningSummaryDelta{Delta: n.Delta, ItemID: n.ItemID, SummaryIndex: n.SummaryIndex}})
	})

	on(notifyPlanDelta, func(_ context.Context, notif Notification) {
		var n PlanDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		streamSend(ctx, ch, eventOrErr{event: &PlanDelta{Delta: n.Delta, ItemID: n.ItemID}})
	})

	on(notifyFileChangeOutputDelta, func(_ context.Context, notif Notification) {
		var n FileChangeOutputDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		streamSend(ctx, ch, eventOrErr{event: &FileChangeDelta{Delta: n.Delta, ItemID: n.ItemID}})
	})

	on(notifyItemStarted, func(_ context.Context, notif Notification) {
		var n ItemStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		streamSend(ctx, ch, eventOrErr{event: &ItemStarted{Item: n.Item}})
	})

	on(notifyItemCompleted, func(_ context.Context, notif Notification) {
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		itemsMu.Lock()
		items = append(items, n.Item)
		itemsMu.Unlock()
		streamSend(ctx, ch, eventOrErr{event: &ItemCompleted{Item: n.Item}})
	})

	on(notifyTurnCompleted, func(_ context.Context, notif Notification) {
		var n TurnCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		select {
		case turnDone <- n:
		default:
		}
	})

	// Start the turn.
	turnParams := TurnStartParams{
		ThreadID: threadResp.Thread.ID,
		Input:    []UserInput{&TextUserInput{Text: opts.Prompt}},
	}
	if opts.Effort != nil {
		turnParams.Effort = opts.Effort
	}

	if _, err := p.Client.Turn.Start(ctx, turnParams); err != nil {
		streamSend(ctx, ch, eventOrErr{err: fmt.Errorf("turn/start: %w", err)})
		return
	}

	// Wait for turn completion or context cancellation.
	select {
	case completed := <-turnDone:
		// Emit TurnCompleted event before checking for errors.
		streamSend(ctx, ch, eventOrErr{event: &TurnCompleted{Turn: completed.Turn}})

		if completed.Turn.Error != nil {
			// Force-send the error even if ctx is done — this is a terminal event.
			ch <- eventOrErr{err: fmt.Errorf("turn error: %s", completed.Turn.Error.Message)}
			return
		}

		itemsMu.Lock()
		collectedItems := make([]ThreadItemWrapper, len(items))
		copy(collectedItems, items)
		itemsMu.Unlock()

		result := &RunResult{
			Thread: threadResp.Thread,
			Turn:   completed.Turn,
			Items:  collectedItems,
		}

		// Extract response text from the last agentMessage item.
		for i := len(collectedItems) - 1; i >= 0; i-- {
			if msg, ok := collectedItems[i].Value.(*AgentMessageThreadItem); ok {
				result.Response = msg.Text
				break
			}
		}

		s.mu.Lock()
		s.result = result
		s.mu.Unlock()

	case <-ctx.Done():
		// Force-send — ctx is already done so streamSend would fail.
		ch <- eventOrErr{err: ctx.Err()}
	}
}
