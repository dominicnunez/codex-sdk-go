package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"sync"
	"sync/atomic"
)

const streamChannelBuffer = 64

// ErrStreamConsumed is returned when Events() is called on a Stream whose
// events have already been consumed by a prior iteration.
var ErrStreamConsumed = errors.New("stream events already consumed")

// eventOrErr pairs an Event with an error for channel transport.
type eventOrErr struct {
	event Event
	err   error
}

// Stream holds the streaming iterator and result for a RunStreamed call.
type Stream struct {
	events iter.Seq2[Event, error]

	result   *RunResult
	done     chan struct{}
	mu       sync.Mutex
	consumed atomic.Bool
}

// Events yields (Event, error) pairs. Iterate with a range-over-func loop.
// Iteration ends when the turn completes, an error occurs, or the context is cancelled.
// The iterator is single-use: subsequent calls return an iterator that yields
// a single ErrStreamConsumed error.
func (s *Stream) Events() iter.Seq2[Event, error] {
	if !s.consumed.CompareAndSwap(false, true) {
		return func(yield func(Event, error) bool) {
			yield(nil, ErrStreamConsumed)
		}
	}
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

// streamSendEvent sends an event on ch, blocking until the send succeeds or
// ctx is cancelled. This respects backpressure from the consumer instead of
// silently dropping events.
func streamSendEvent(ctx context.Context, ch chan<- eventOrErr, event Event) {
	select {
	case ch <- eventOrErr{event: event}:
	case <-ctx.Done():
	}
}

// streamSendErr sends a terminal error on ch. It attempts a non-blocking send
// first (sufficient when buffer space remains), then falls back to a blocking
// send guarded by ctx to prevent goroutine leaks when the consumer stops reading.
func streamSendErr(ctx context.Context, ch chan<- eventOrErr, err error) {
	select {
	case ch <- eventOrErr{err: err}:
	default:
		select {
		case ch <- eventOrErr{err: err}:
		case <-ctx.Done():
		}
	}
}

// streamListen registers a notification listener that unmarshals the
// notification params into N, converts it to an Event, and sends it on ch.
// Notifications with a threadId that does not match threadID are ignored.
func streamListen[N any](ctx context.Context, on func(string, NotificationHandler), method string, ch chan<- eventOrErr, threadID string, convert func(N) Event) {
	on(method, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != threadID {
			return
		}
		var n N
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		streamSendEvent(ctx, ch, convert(n))
	})
}

func (p *Process) runStreamedLifecycle(ctx context.Context, opts RunOptions, ch chan<- eventOrErr, s *Stream) {
	defer close(ch)
	defer close(s.done)

	if opts.Prompt == "" {
		streamSendErr(ctx, ch, errors.New("prompt is required"))
		return
	}

	if err := p.ensureInit(ctx); err != nil {
		streamSendErr(ctx, ch, err)
		return
	}

	threadResp, err := p.Client.Thread.Start(ctx, buildThreadParams(opts))
	if err != nil {
		streamSendErr(ctx, ch, fmt.Errorf("thread/start: %w", err))
		return
	}

	executeStreamedTurn(ctx, turnLifecycleParams{
		client:     p.Client,
		turnParams: buildTurnParams(opts, threadResp.Thread.ID),
		thread:     threadResp.Thread,
		threadID:   threadResp.Thread.ID,
	}, ch, s)
}
