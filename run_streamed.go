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
	mu          sync.RWMutex
	ch          chan eventOrErr
	closed      bool
	terminalErr error
}

func newGuardedChan(size int) *guardedChan {
	return &guardedChan{ch: make(chan eventOrErr, size)}
}

// send writes an event/error pair to the channel. When the channel is full, the
// oldest queued element is dropped so lifecycle completion never depends on
// the consumer draining Events().
func (g *guardedChan) send(eoe eventOrErr) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.closed {
		return
	}
	select {
	case g.ch <- eoe:
		return
	default:
	}

	// Buffer is full: evict one queued event and retry once. If a concurrent
	// sender filled the slot before this retry, we intentionally drop the new
	// event to keep delivery non-blocking.
	select {
	case <-g.ch:
	default:
	}
	select {
	case g.ch <- eoe:
	default:
	}
}

// setTerminalError records the terminal stream error exactly once.
// It is stored out-of-band from the lossy event buffer so it cannot be dropped
// when producers are contending on a full channel.
func (g *guardedChan) setTerminalError(err error) {
	if err == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || g.terminalErr != nil {
		return
	}
	g.terminalErr = err
}

func (g *guardedChan) terminalError() error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.terminalErr
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
	return p.runStreamedWithCollector(ctx, opts, nil)
}

// RunStreamedWithCollector executes RunStreamed and feeds all streamed events
// (plus selected notification-derived conveniences) into collector.
// The collector is optional; passing nil behaves like RunStreamed.
func (p *Process) RunStreamedWithCollector(ctx context.Context, opts RunOptions, collector *StreamCollector) *Stream {
	return p.runStreamedWithCollector(ctx, opts, collector)
}

func (p *Process) runStreamedWithCollector(ctx context.Context, opts RunOptions, collector *StreamCollector) *Stream {
	if opts.Prompt == "" {
		return newErrorStream(errors.New("prompt is required"))
	}

	g := newGuardedChan(streamChannelBuffer)
	s := &Stream{
		done: make(chan struct{}),
	}

	s.events = streamIterator(g)

	go p.runStreamedLifecycle(ctx, opts, g, s, collector)

	return s
}

// newErrorStream returns a Stream that yields a single error and completes
// immediately. Used for synchronous validation failures in RunStreamed.
func newErrorStream(err error) *Stream {
	done := make(chan struct{})
	close(done)
	g := newGuardedChan(1)
	g.setTerminalError(err)
	g.closeOnce()
	return &Stream{
		done:   done,
		events: streamIterator(g),
	}
}

func streamIterator(g *guardedChan) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		for eoe := range g.ch {
			if !yield(eoe.event, eoe.err) {
				return
			}
		}
		if err := g.terminalError(); err != nil {
			_ = yield(nil, err)
		}
	}
}

// streamSendEvent sends an event on g using bounded-loss semantics.
func streamSendEvent(g *guardedChan, event Event) {
	g.send(eventOrErr{event: event})
}

// streamSendErr records the terminal stream error.
func streamSendErr(g *guardedChan, err error) {
	g.setTerminalError(err)
}

// streamListen registers a notification listener that unmarshals the
// notification params into N, filters by thread, converts it to an Event,
// and sends it on g.
func streamListen[N any](on func(string, NotificationHandler), method string, g *guardedChan, threadID string, reportErr func(string, error), onEvent func(Event), threadIDOf func(N) string, convert func(N) Event) {
	on(method, func(_ context.Context, notif Notification) {
		var n N
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			reportErr(method, fmt.Errorf("unmarshal %s: %w", method, err))
			return
		}
		if threadIDOf(n) != threadID {
			return
		}
		ev := convert(n)
		if onEvent != nil {
			onEvent(ev)
		}
		streamSendEvent(g, ev)
	})
}

func (p *Process) runStreamedLifecycle(ctx context.Context, opts RunOptions, g *guardedChan, s *Stream, collector *StreamCollector) {
	defer g.closeOnce()
	defer close(s.done)

	if err := p.ensureInit(ctx); err != nil {
		if collector != nil {
			collector.Process(nil, err)
		}
		streamSendErr(g, err)
		return
	}

	threadResp, err := p.Client.Thread.Start(ctx, buildThreadParams(opts))
	if err != nil {
		if collector != nil {
			collector.Process(nil, fmt.Errorf("thread/start: %w", err))
		}
		streamSendErr(g, fmt.Errorf("thread/start: %w", err))
		return
	}

	executeStreamedTurn(ctx, turnLifecycleParams{
		client:     p.Client,
		turnParams: buildTurnParams(opts, threadResp.Thread.ID),
		thread:     threadResp.Thread,
		threadID:   threadResp.Thread.ID,
		collector:  collector,
	}, g, s)
}
