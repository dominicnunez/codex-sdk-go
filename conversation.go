package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
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
}

// errTurnInProgress is returned when a Turn or TurnStreamed call is made
// while another turn is already executing on the same Conversation.
var errTurnInProgress = errors.New("a turn is already in progress on this conversation")

// Conversation manages a persistent thread across multiple turns.
// Concurrent Turn or TurnStreamed calls on the same Conversation are
// not supported — the second call returns errTurnInProgress.
type Conversation struct {
	process    *Process
	threadID   string
	thread     Thread
	mu         sync.Mutex
	activeTurn bool
}

// ThreadID returns the underlying thread ID.
func (c *Conversation) ThreadID() string {
	return c.threadID
}

// Thread returns a deep-copy snapshot of the latest thread state.
// The returned Thread is fully isolated from the Conversation's internal
// state — mutations to the snapshot do not affect the Conversation.
// ThreadItemWrapper values within Items are cloned via JSON round-trip.
func (c *Conversation) Thread() Thread {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.thread
	t.Name = cloneStringPtr(c.thread.Name)
	t.AgentNickname = cloneStringPtr(c.thread.AgentNickname)
	t.AgentRole = cloneStringPtr(c.thread.AgentRole)
	t.Path = cloneStringPtr(c.thread.Path)
	if c.thread.GitInfo != nil {
		g := *c.thread.GitInfo
		g.Branch = cloneStringPtr(g.Branch)
		g.OriginURL = cloneStringPtr(g.OriginURL)
		g.SHA = cloneStringPtr(g.SHA)
		t.GitInfo = &g
	}
	t.Turns = make([]Turn, len(c.thread.Turns))
	copy(t.Turns, c.thread.Turns)
	for i, turn := range t.Turns {
		t.Turns[i].Items = make([]ThreadItemWrapper, len(turn.Items))
		for j, item := range turn.Items {
			t.Turns[i].Items[j] = cloneThreadItemWrapper(item)
		}
		if turn.Error != nil {
			e := *turn.Error
			e.CodexErrorInfo = append(json.RawMessage(nil), turn.Error.CodexErrorInfo...)
			t.Turns[i].Error = &e
		}
	}
	return t
}

// cloneThreadItemWrapper deep-copies a ThreadItemWrapper via JSON round-trip.
// Panics on marshal/unmarshal failure — these indicate a bug in a type's JSON
// methods and must not be silently swallowed (returning the original would
// break the deep-copy isolation guarantee).
func cloneThreadItemWrapper(w ThreadItemWrapper) ThreadItemWrapper {
	if w.Value == nil {
		return w
	}
	b, err := json.Marshal(w)
	if err != nil {
		panic(fmt.Sprintf("cloneThreadItemWrapper: marshal failed: %v", err))
	}
	var clone ThreadItemWrapper
	if err := json.Unmarshal(b, &clone); err != nil {
		panic(fmt.Sprintf("cloneThreadItemWrapper: unmarshal failed: %v", err))
	}
	return clone
}

func cloneStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

// StartConversation creates a thread and returns a Conversation handle.
func (p *Process) StartConversation(ctx context.Context, opts ConversationOptions) (*Conversation, error) {
	if err := p.ensureInit(ctx); err != nil {
		return nil, err
	}

	params := ThreadStartParams{
		Ephemeral: Ptr(false),
	}
	if opts.Instructions != nil {
		params.DeveloperInstructions = opts.Instructions
	}
	if opts.Model != nil {
		params.Model = opts.Model
	}
	if opts.Personality != nil {
		params.Personality = opts.Personality
	}
	if opts.ApprovalPolicy != nil {
		params.ApprovalPolicy = opts.ApprovalPolicy
	}

	resp, err := p.Client.Thread.Start(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("thread/start: %w", err)
	}

	return &Conversation{
		process:  p,
		threadID: resp.Thread.ID,
		thread:   resp.Thread,
	}, nil
}

func (c *Conversation) buildTurnParams(opts TurnOptions) TurnStartParams {
	params := TurnStartParams{
		ThreadID: c.threadID,
		Input:    []UserInput{&TextUserInput{Text: opts.Prompt}},
	}
	if opts.Effort != nil {
		params.Effort = opts.Effort
	}
	if opts.Model != nil {
		params.Model = opts.Model
	}
	if opts.CollaborationMode != nil {
		params.CollaborationMode = opts.CollaborationMode
	}
	return params
}

// Turn executes a blocking turn on the existing thread, like Run() but multi-turn.
// Concurrent calls to Turn or TurnStreamed on the same Conversation are not
// supported and return an error.
func (c *Conversation) Turn(ctx context.Context, opts TurnOptions) (*RunResult, error) {
	if opts.Prompt == "" {
		return nil, errors.New("prompt is required")
	}

	if err := c.process.ensureInit(ctx); err != nil {
		return nil, err
	}

	c.mu.Lock()
	if c.activeTurn {
		c.mu.Unlock()
		return nil, errTurnInProgress
	}
	c.activeTurn = true
	thread := c.thread
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.activeTurn = false
		c.mu.Unlock()
	}()

	return executeTurn(ctx, turnLifecycleParams{
		client:     c.process.Client,
		turnParams: c.buildTurnParams(opts),
		thread:     thread,
		threadID:   c.threadID,
		onComplete: func(turn Turn) {
			c.mu.Lock()
			c.thread.Turns = append(c.thread.Turns, turn)
			c.mu.Unlock()
		},
	})
}

// TurnStreamed executes a streaming turn on the existing thread.
func (c *Conversation) TurnStreamed(ctx context.Context, opts TurnOptions) *Stream {
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

	go c.turnStreamedLifecycle(ctx, opts, ch, s)

	return s
}

func (c *Conversation) turnStreamedLifecycle(ctx context.Context, opts TurnOptions, ch chan<- eventOrErr, s *Stream) {
	defer close(ch)
	defer close(s.done)

	if opts.Prompt == "" {
		streamSendErr(ctx, ch, errors.New("prompt is required"))
		return
	}

	if err := c.process.ensureInit(ctx); err != nil {
		streamSendErr(ctx, ch, err)
		return
	}

	c.mu.Lock()
	if c.activeTurn {
		c.mu.Unlock()
		streamSendErr(ctx, ch, errTurnInProgress)
		return
	}
	c.activeTurn = true
	thread := c.thread
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.activeTurn = false
		c.mu.Unlock()
	}()

	executeStreamedTurn(ctx, turnLifecycleParams{
		client:     c.process.Client,
		turnParams: c.buildTurnParams(opts),
		thread:     thread,
		threadID:   c.threadID,
		onComplete: func(turn Turn) {
			c.mu.Lock()
			c.thread.Turns = append(c.thread.Turns, turn)
			c.mu.Unlock()
		},
	}, ch, s)
}
