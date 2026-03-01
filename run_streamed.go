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

// streamChannelBuffer is the capacity of the event channel between the
// lifecycle goroutine and the Events() iterator. 64 is large enough to
// absorb bursts of rapid notifications (e.g. streaming text deltas)
// without blocking the notification dispatcher, while small enough to
// keep per-stream memory overhead negligible.
const streamChannelBuffer = 64

// ErrStreamConsumed is returned when Events() is called on a Stream whose
// events have already been consumed by a prior iteration.
var ErrStreamConsumed = errors.New("stream events already consumed")

// guardedChan wraps a channel with an RWMutex so that sends and close are
// mutually exclusive. Senders hold a read lock (concurrent sends are fine);
// the closer takes a write lock, ensuring no send is in flight when the
// channel is closed.
type guardedChan struct {
	mu     sync.RWMutex
	ch     chan eventOrErr
	closed bool
}

func newGuardedChan(size int) *guardedChan {
	return &guardedChan{ch: make(chan eventOrErr, size)}
}

// send writes an event/error pair to the channel, blocking until the send
// succeeds, ctx is cancelled, or the channel is already closed.
func (g *guardedChan) send(ctx context.Context, eoe eventOrErr) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.closed {
		return
	}
	select {
	case g.ch <- eoe:
	case <-ctx.Done():
	}
}

// trySend attempts a non-blocking send, falling back to a blocking send
// guarded by ctx. No-ops if the channel is closed or ctx cancelled.
func (g *guardedChan) trySend(ctx context.Context, eoe eventOrErr) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.closed {
		return
	}
	select {
	case g.ch <- eoe:
	default:
		select {
		case g.ch <- eoe:
		case <-ctx.Done():
		}
	}
}

func (g *guardedChan) closeOnce() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.closed {
		g.closed = true
		close(g.ch)
	}
}

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
	if opts.Prompt == "" {
		return newErrorStream(errors.New("prompt is required"))
	}

	g := newGuardedChan(streamChannelBuffer)
	s := &Stream{
		done: make(chan struct{}),
	}

	s.events = func(yield func(Event, error) bool) {
		for eoe := range g.ch {
			if !yield(eoe.event, eoe.err) {
				return
			}
		}
	}

	go p.runStreamedLifecycle(ctx, opts, g, s)

	return s
}

// newErrorStream returns a Stream that yields a single error and completes
// immediately. Used for synchronous validation failures in RunStreamed.
func newErrorStream(err error) *Stream {
	done := make(chan struct{})
	close(done)
	return &Stream{
		done: done,
		events: func(yield func(Event, error) bool) {
			yield(nil, err)
		},
	}
}

// streamSendEvent sends an event on g, blocking until the send succeeds,
// ctx is cancelled, or the channel is closed.
func streamSendEvent(ctx context.Context, g *guardedChan, event Event) {
	g.send(ctx, eventOrErr{event: event})
}

// streamSendErr sends a terminal error on g. It attempts a non-blocking send
// first (sufficient when buffer space remains), then falls back to a blocking
// send guarded by ctx to prevent goroutine leaks when the consumer stops reading.
func streamSendErr(ctx context.Context, g *guardedChan, err error) {
	g.trySend(ctx, eventOrErr{err: err})
}

// streamListen registers a notification listener that unmarshals the
// notification params into N, converts it to an Event, and sends it on g.
// Notifications with a threadId that does not match threadID are ignored.
func streamListen[N any](ctx context.Context, on func(string, NotificationHandler), method string, g *guardedChan, threadID string, reportErr func(string, error), convert func(N) Event) {
	on(method, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != threadID {
			return
		}
		var n N
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			reportErr(method, fmt.Errorf("unmarshal %s: %w", method, err))
			return
		}
		streamSendEvent(ctx, g, convert(n))
	})
}

func (p *Process) runStreamedLifecycle(ctx context.Context, opts RunOptions, g *guardedChan, s *Stream) {
	defer g.closeOnce()
	defer close(s.done)

	if err := p.ensureInit(ctx); err != nil {
		streamSendErr(ctx, g, err)
		return
	}

	threadResp, err := p.Client.Thread.Start(ctx, buildThreadParams(opts))
	if err != nil {
		streamSendErr(ctx, g, fmt.Errorf("thread/start: %w", err))
		return
	}

	executeStreamedTurn(ctx, turnLifecycleParams{
		client:     p.Client,
		turnParams: buildTurnParams(opts, threadResp.Thread.ID),
		thread:     threadResp.Thread,
		threadID:   threadResp.Thread.ID,
	}, g, s)
}
