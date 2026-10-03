package appserver

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/dominicnunez/codex-sdk-go/internal/deepcopy"
)

// ConversationOptions configures the thread created by StartConversation.
type ConversationOptions struct {
	Instructions   *string
	Model          *string
	Personality    *Personality
	ApprovalPolicy *AskForApproval
}

// TurnOptions configures an individual turn within a conversation.
type TurnOptions struct {
	Prompt            string
	Effort            *ReasoningEffort
	Model             *string
	CollaborationMode *CollaborationMode
	OutputSchema      interface{}
}

// errTurnInProgress is returned when a Turn or TurnStreamed call is made
// while another turn is already executing on the same Conversation.
var errTurnInProgress = errors.New("a turn is already in progress on this conversation")
var errConversationClosed = errors.New("conversation is closed")
var errConversationUninitialized = errors.New("conversation must be created with StartConversation")

// Conversation manages a persistent thread across multiple turns.
// Concurrent Turn or TurnStreamed calls on the same Conversation are
// not supported — the second call returns errTurnInProgress.
type Conversation struct {
	process   *Process
	threadID  string
	state     *conversationState
	release   func()
	cleanup   *runtime.Cleanup
	closeOnce sync.Once
}

type conversationState struct {
	mu             sync.Mutex
	thread         Thread
	activeTurn     bool
	closed         bool
	hasStartedTurn bool
}

func newConversationState(thread Thread) *conversationState {
	return &conversationState{thread: cloneThreadState(thread)}
}

func (s *conversationState) snapshot() Thread {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneThreadState(s.thread)
}

func (s *conversationState) storeSnapshot(thread Thread) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.thread = cloneThreadState(thread)
	s.mu.Unlock()
}

func (s *conversationState) startTurn() (Thread, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Thread{}, false, errConversationClosed
	}
	if s.activeTurn {
		return Thread{}, false, errTurnInProgress
	}
	s.activeTurn = true
	return cloneThreadState(s.thread), !s.hasStartedTurn, nil
}

func (s *conversationState) ensureOpen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errConversationClosed
	}
	return nil
}

func (s *conversationState) finishTurn() {
	s.mu.Lock()
	s.activeTurn = false
	s.mu.Unlock()
}

func (s *conversationState) markTurnStarted() {
	s.mu.Lock()
	s.hasStartedTurn = true
	s.mu.Unlock()
}

func (s *conversationState) applyCompletedThread(thread Thread) {
	s.mu.Lock()
	s.thread = cloneThreadState(thread)
	s.mu.Unlock()
}

func (s *conversationState) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

// ThreadID returns the underlying thread ID.
func (c *Conversation) ThreadID() string {
	return c.threadID
}

// Thread returns a deep-copy snapshot of the latest thread state tracked by
// this conversation. The snapshot is kept current from thread service
// responses, metadata notifications, and turns completed through the
// Conversation. The returned Thread is fully isolated from internal state, so
// mutating the snapshot does not affect the Conversation or client cache.
func (c *Conversation) Thread() Thread {
	if c == nil || c.state == nil {
		return Thread{}
	}
	return c.state.snapshot()
}

func (c *Conversation) applyCompletedThread(thread Thread) {
	if c == nil || c.state == nil {
		return
	}
	c.state.applyCompletedThread(thread)
}

func (c *Conversation) ensureInitialized() error {
	switch {
	case c == nil:
		return errConversationUninitialized
	case c.state == nil:
		return errConversationUninitialized
	case c.process == nil:
		return errConversationUninitialized
	case c.process.Client == nil:
		return errConversationUninitialized
	default:
		return nil
	}
}

// Close releases conversation-local resources. Safe to call multiple times.
func (c *Conversation) Close() error {
	if c == nil {
		return nil
	}

	c.closeOnce.Do(func() {
		if c.cleanup != nil {
			c.cleanup.Stop()
			c.cleanup = nil
		}
		if c.state != nil {
			c.state.close()
		}
		if c.release != nil {
			c.release()
			c.release = nil
		}
	})
	return nil
}

// Protocol snapshots use the same deep-copy boundary as the client cache.
// Copy the complete exported shape so new fields cannot silently retain aliases
// and nil, empty, dynamic union types and raw metadata survive unchanged.
func cloneThreadState(thread Thread) Thread { return cloneArbitraryValue(thread) }
func cloneTurn(turn Turn) Turn              { return cloneArbitraryValue(turn) }
func cloneThreadItems(items []ThreadItemWrapper) []ThreadItemWrapper {
	return cloneArbitraryValue(items)
}
func cloneThreadItemWrapper(w ThreadItemWrapper) ThreadItemWrapper { return cloneArbitraryValue(w) }
func cloneUserInput(in UserInput) UserInput                        { return cloneArbitraryValue(in) }
func cloneArbitraryValue[T any](in T) T                            { return deepcopy.Value(in) }
func cloneStringPtr(s *string) *string                             { return clonePtr(s) }
func clonePtr[T any](in *T) *T {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

// StartConversation creates a thread and returns a Conversation handle.
// Call Close on the returned Conversation when it is no longer needed so its
// thread-state listener is released promptly.
func (p *Process) StartConversation(ctx context.Context, opts ConversationOptions) (*Conversation, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := p.ensureInit(ctx); err != nil {
		return nil, err
	}

	params := ThreadStartParams{
		Ephemeral: Ptr(false),
	}
	applyThreadStartOptions(&params, opts.Instructions, opts.Model, opts.Personality, opts.ApprovalPolicy)

	resp, err := p.Client.Thread.Start(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("thread/start: %w", err)
	}

	state := newConversationState(resp.Thread)
	conv := &Conversation{
		process:  p,
		threadID: resp.Thread.ID,
		state:    state,
	}
	unsubscribe := p.Client.AddThreadStateListener(resp.Thread.ID, state.storeSnapshot, state.close)
	conv.release = unsubscribe
	if snapshot, ok := p.Client.ThreadStateSnapshot(resp.Thread.ID); ok {
		state.storeSnapshot(snapshot)
	}
	cleanup := runtime.AddCleanup(conv, func(unsub func()) {
		if unsub != nil {
			unsub()
		}
	}, unsubscribe)
	conv.cleanup = &cleanup

	return conv, nil
}

func (c *Conversation) buildTurnParams(opts TurnOptions) TurnStartParams {
	params := newTurnStartParams(c.threadID, opts.Prompt)
	applyTurnStartOptions(&params, opts.Model, opts.Effort, opts.CollaborationMode, opts.OutputSchema)
	return params
}

func (c *Conversation) buildTurnLifecycleParams(opts TurnOptions, thread Thread, allowMissingInitialTurnID bool) turnLifecycleParams {
	return turnLifecycleParams{
		client:                    c.process.Client,
		turnParams:                c.buildTurnParams(opts),
		thread:                    thread,
		threadID:                  c.threadID,
		allowMissingInitialTurnID: allowMissingInitialTurnID,
		onStart:                   c.state.markTurnStarted,
		onComplete:                c.applyCompletedThread,
	}
}

// Turn executes a blocking turn on the existing thread, like Run() but multi-turn.
// Concurrent calls to Turn or TurnStreamed on the same Conversation are not
// supported and return an error.
func (c *Conversation) Turn(ctx context.Context, opts TurnOptions) (*RunResult, error) {
	if err := validatePromptContext(ctx, opts.Prompt); err != nil {
		return nil, err
	}
	if err := c.ensureInitialized(); err != nil {
		return nil, err
	}
	if err := c.state.ensureOpen(); err != nil {
		return nil, err
	}
	if err := c.process.ensureInit(ctx); err != nil {
		return nil, err
	}

	thread, allowMissingInitialTurnID, err := c.state.startTurn()
	if err != nil {
		return nil, err
	}

	defer func() {
		c.state.finishTurn()
	}()

	return executeTurn(ctx, c.buildTurnLifecycleParams(opts, thread, allowMissingInitialTurnID))
}

// TurnStreamed executes a streaming turn on the existing thread.
func (c *Conversation) TurnStreamed(ctx context.Context, opts TurnOptions) *Stream {
	if err := validateContext(ctx); err != nil {
		return newErrorStream(err)
	}
	if err := c.ensureInitialized(); err != nil {
		return newErrorStream(err)
	}
	if err := c.state.ensureOpen(); err != nil {
		return newErrorStream(err)
	}
	g, s := newActiveStream(streamChannelBuffer)

	go c.turnStreamedLifecycle(ctx, opts, g, s)

	return s
}

func (c *Conversation) turnStreamedLifecycle(ctx context.Context, opts TurnOptions, g *guardedChan, s *Stream) {
	defer g.closeOnce()
	defer close(s.done)

	if err := validatePrompt(opts.Prompt); err != nil {
		streamSendErr(g, err)
		return
	}

	if err := c.process.ensureInit(ctx); err != nil {
		streamSendErr(g, err)
		return
	}

	thread, allowMissingInitialTurnID, err := c.state.startTurn()
	if err != nil {
		streamSendErr(g, err)
		return
	}

	defer func() {
		c.state.finishTurn()
	}()

	executeStreamedTurn(ctx, c.buildTurnLifecycleParams(opts, thread, allowMissingInitialTurnID), g, s)
}
