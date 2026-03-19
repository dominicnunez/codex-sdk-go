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

// streamChannelBuffer is the capacity of the bounded event queue between the
// lifecycle goroutine and the Events() iterator. The queue stays bounded so a
// stalled or absent consumer cannot grow memory without limit, while an active
// consumer receives burst-driven backpressure instead of a spurious failure.
const streamChannelBuffer = 64

// ErrStreamConsumed is returned when Events() is called on a Stream whose
// events have already been consumed by a prior iteration.
var ErrStreamConsumed = errors.New("stream events already consumed")

// ErrStreamOverflow is returned when the bounded stream buffer fills before an
// Events() consumer is attached to drain it.
var ErrStreamOverflow = errors.New("stream event buffer overflow")

// guardedChan wraps a bounded ring buffer with condition variables so
// producers can apply backpressure to an active consumer without growing
// memory without limit. If iteration stops early, the queue detaches and
// subsequent events are discarded so Result() can still complete.
type guardedChan struct {
	mu             sync.Mutex
	notEmpty       *sync.Cond
	notFull        *sync.Cond
	buf            []eventOrErr
	head           int
	size           int
	closed         bool
	detached       bool
	consumerActive bool
	terminalErr    error
}

func newGuardedChan(size int) *guardedChan {
	g := &guardedChan{buf: make([]eventOrErr, size)}
	g.notEmpty = sync.NewCond(&g.mu)
	g.notFull = sync.NewCond(&g.mu)
	return g
}

// send writes an event/error pair to the bounded queue. When an Events()
// consumer is active, a full queue applies backpressure until capacity is
// available. If no consumer has attached yet, the stream fails with
// [ErrStreamOverflow] instead of blocking indefinitely.
func (g *guardedChan) send(eoe eventOrErr) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		switch {
		case g.closed || g.detached || g.terminalErr != nil:
			return
		case g.size < len(g.buf):
			tail := (g.head + g.size) % len(g.buf)
			g.buf[tail] = eoe
			g.size++
			g.notEmpty.Signal()
			return
		case !g.consumerActive:
			if g.terminalErr == nil {
				g.terminalErr = ErrStreamOverflow
			}
			g.closed = true
			g.notEmpty.Broadcast()
			g.notFull.Broadcast()
			return
		default:
			g.notFull.Wait()
		}
	}
}

// setTerminalError records the terminal stream error exactly once.
func (g *guardedChan) setTerminalError(err error) {
	if err == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.terminalErr != nil {
		return
	}
	g.terminalErr = err
	g.notFull.Broadcast()
	g.notEmpty.Broadcast()
}

func (g *guardedChan) fail(err error) {
	if err == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.terminalErr == nil {
		g.terminalErr = err
	}
	g.closed = true
	g.notEmpty.Broadcast()
	g.notFull.Broadcast()
}

func (g *guardedChan) terminalError() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.terminalErr
}

func (g *guardedChan) closeOnce() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return
	}
	g.closed = true
	g.notEmpty.Broadcast()
	g.notFull.Broadcast()
}

func (g *guardedChan) attachConsumer() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.detached {
		return
	}
	g.consumerActive = true
	g.notFull.Broadcast()
}

func (g *guardedChan) detachConsumer() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.detached {
		return
	}
	g.detached = true
	g.consumerActive = false
	for i := 0; i < g.size; i++ {
		idx := (g.head + i) % len(g.buf)
		g.buf[idx] = eventOrErr{}
	}
	g.head = 0
	g.size = 0
	g.notEmpty.Broadcast()
	g.notFull.Broadcast()
}

func (g *guardedChan) recv() (eventOrErr, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for g.size == 0 {
		if g.closed || g.detached {
			return eventOrErr{}, false
		}
		g.notEmpty.Wait()
	}

	eoe := g.buf[g.head]
	g.buf[g.head] = eventOrErr{}
	g.head = (g.head + 1) % len(g.buf)
	g.size--
	g.notFull.Signal()
	return eoe, true
}

// eventOrErr pairs an Event with an error for channel transport.
type eventOrErr struct {
	event Event
	err   error
}

// Stream holds the streaming iterator and result for a RunStreamed call.
type Stream struct {
	events iter.Seq2[Event, error]
	queue  *guardedChan

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
	if s.queue != nil {
		s.queue.attachConsumer()
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
	if err := validateContext(ctx); err != nil {
		return newErrorStream(err)
	}
	if opts.Prompt == "" {
		return newErrorStream(errors.New("prompt is required"))
	}

	g := newGuardedChan(streamChannelBuffer)
	s := &Stream{
		done:  make(chan struct{}),
		queue: g,
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
		queue:  g,
	}
}

func streamIterator(g *guardedChan) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		for {
			eoe, ok := g.recv()
			if !ok {
				break
			}
			if !yield(eoe.event, eoe.err) {
				g.detachConsumer()
				return
			}
		}
		if err := g.terminalError(); err != nil {
			_ = yield(nil, err)
		}
	}
}

// streamSendEvent sends an event on g and fails the stream if the bounded
// buffer overflows.
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
		client:                    p.Client,
		turnParams:                buildTurnParams(opts, threadResp.Thread.ID),
		thread:                    threadResp.Thread,
		threadID:                  threadResp.Thread.ID,
		allowMissingInitialTurnID: true,
		collector:                 collector,
	}, g, s)
}
