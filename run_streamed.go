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

// streamSendEvent performs a non-blocking send of an event on ch.
// Events are best-effort: if the consumer fell behind, the event is dropped.
func streamSendEvent(ch chan<- eventOrErr, event Event) {
	select {
	case ch <- eventOrErr{event: event}:
	default:
	}
}

// streamSendErr performs a blocking send of an error on ch.
// Errors must not be lost — they are terminal and signal the consumer to stop.
func streamSendErr(ch chan<- eventOrErr, err error) {
	ch <- eventOrErr{err: err}
}

// streamListen registers a notification listener that unmarshals the
// notification params into N, converts it to an Event, and sends it on ch.
// Notifications with a threadId that does not match threadID are ignored.
func streamListen[N any](on func(string, NotificationHandler), method string, ch chan<- eventOrErr, threadID string, convert func(N) Event) {
	on(method, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != threadID {
			return
		}
		var n N
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		streamSendEvent(ch, convert(n))
	})
}

func (p *Process) runStreamedLifecycle(ctx context.Context, opts RunOptions, ch chan<- eventOrErr, s *Stream) {
	defer close(ch)
	defer close(s.done)

	if opts.Prompt == "" {
		streamSendErr(ch, errors.New("prompt is required"))
		return
	}

	if err := p.ensureInit(ctx); err != nil {
		streamSendErr(ch, err)
		return
	}

	threadResp, err := p.Client.Thread.Start(ctx, buildThreadParams(opts))
	if err != nil {
		streamSendErr(ch, fmt.Errorf("thread/start: %w", err))
		return
	}

	executeStreamedTurn(ctx, turnLifecycleParams{
		client:     p.Client,
		turnParams: buildTurnParams(opts, threadResp.Thread.ID),
		thread:     threadResp.Thread,
		threadID:   threadResp.Thread.ID,
	}, ch, s)
}
