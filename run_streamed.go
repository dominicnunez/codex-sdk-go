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
	events iter.Seq2[Event, error]

	result *RunResult
	done   chan struct{}
	mu     sync.Mutex
}

// Events yields (Event, error) pairs. Iterate with a range-over-func loop.
// Iteration ends when the turn completes, an error occurs, or the context is cancelled.
func (s *Stream) Events() iter.Seq2[Event, error] {
	return s.events
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

	s.events = func(yield func(Event, error) bool) {
		for eoe := range ch {
			if !yield(eoe.event, eoe.err) {
				return
			}
		}
	}

	go p.runStreamedLifecycle(ctx, opts, ch, s)

	return s
}

// streamSend performs a non-blocking send on ch. Events are best-effort: if the
// consumer fell behind or stopped reading, the event is dropped rather than
// blocking the transport dispatch goroutine.
func streamSend(ch chan<- eventOrErr, eoe eventOrErr) {
	select {
	case ch <- eoe:
	default:
	}
}

// streamListen registers a notification listener that unmarshals the
// notification params into N, converts it to an Event, and sends it on ch.
func streamListen[N any](on func(string, NotificationHandler), method string, ch chan<- eventOrErr, convert func(N) Event) {
	on(method, func(_ context.Context, notif Notification) {
		var n N
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		streamSend(ch, eventOrErr{event: convert(n)})
	})
}

func (p *Process) runStreamedLifecycle(ctx context.Context, opts RunOptions, ch chan<- eventOrErr, s *Stream) {
	defer close(ch)
	defer close(s.done)

	if opts.Prompt == "" {
		streamSend(ch, eventOrErr{err: errors.New("prompt is required")})
		return
	}

	if err := p.ensureInit(ctx); err != nil {
		streamSend(ch, eventOrErr{err: err})
		return
	}

	threadResp, err := p.Client.Thread.Start(ctx, buildThreadParams(opts))
	if err != nil {
		streamSend(ch, eventOrErr{err: fmt.Errorf("thread/start: %w", err)})
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

	streamListen(on, notifyTurnStarted, ch, func(n TurnStartedNotification) Event {
		return &TurnStarted{Turn: n.Turn, ThreadID: n.ThreadID}
	})

	streamListen(on, notifyAgentMessageDelta, ch, func(n AgentMessageDeltaNotification) Event {
		return &TextDelta{Delta: n.Delta, ItemID: n.ItemID}
	})

	streamListen(on, notifyReasoningTextDelta, ch, func(n ReasoningTextDeltaNotification) Event {
		return &ReasoningDelta{Delta: n.Delta, ItemID: n.ItemID, ContentIndex: n.ContentIndex}
	})

	streamListen(on, notifyReasoningSummaryTextDelta, ch, func(n ReasoningSummaryTextDeltaNotification) Event {
		return &ReasoningSummaryDelta{Delta: n.Delta, ItemID: n.ItemID, SummaryIndex: n.SummaryIndex}
	})

	streamListen(on, notifyPlanDelta, ch, func(n PlanDeltaNotification) Event {
		return &PlanDelta{Delta: n.Delta, ItemID: n.ItemID}
	})

	streamListen(on, notifyFileChangeOutputDelta, ch, func(n FileChangeOutputDeltaNotification) Event {
		return &FileChangeDelta{Delta: n.Delta, ItemID: n.ItemID}
	})

	streamListen(on, notifyItemStarted, ch, func(n ItemStartedNotification) Event {
		return &ItemStarted{Item: n.Item}
	})

	// item/completed is special: also appends to the collected items slice.
	on(notifyItemCompleted, func(_ context.Context, notif Notification) {
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		itemsMu.Lock()
		items = append(items, n.Item)
		itemsMu.Unlock()
		streamSend(ch, eventOrErr{event: &ItemCompleted{Item: n.Item}})
	})

	// turn/completed is special: signals turnDone and synthesizes on unmarshal failure.
	on(notifyTurnCompleted, func(_ context.Context, notif Notification) {
		var n TurnCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			n = TurnCompletedNotification{
				Turn: Turn{Error: &TurnError{Message: "failed to unmarshal turn/completed: " + err.Error()}},
			}
		}
		select {
		case turnDone <- n:
		default:
		}
	})

	if _, err := p.Client.Turn.Start(ctx, buildTurnParams(opts, threadResp.Thread.ID)); err != nil {
		streamSend(ch, eventOrErr{err: fmt.Errorf("turn/start: %w", err)})
		return
	}

	// Wait for turn completion or context cancellation.
	select {
	case completed := <-turnDone:
		// Emit TurnCompleted event before checking for errors.
		streamSend(ch, eventOrErr{event: &TurnCompleted{Turn: completed.Turn}})

		if completed.Turn.Error != nil {
			streamSend(ch, eventOrErr{err: fmt.Errorf("turn error: %s", completed.Turn.Error.Message)})
			return
		}

		itemsMu.Lock()
		collectedItems := make([]ThreadItemWrapper, len(items))
		copy(collectedItems, items)
		itemsMu.Unlock()

		s.mu.Lock()
		s.result = buildRunResult(threadResp.Thread, completed.Turn, collectedItems)
		s.mu.Unlock()

	case <-ctx.Done():
		streamSend(ch, eventOrErr{err: ctx.Err()})
	}
}
