package codex

import (
	"context"
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

// Conversation manages a persistent thread across multiple turns.
type Conversation struct {
	process  *Process
	threadID string
	thread   Thread
	mu       sync.Mutex
}

// ThreadID returns the underlying thread ID.
func (c *Conversation) ThreadID() string {
	return c.threadID
}

// Thread returns a snapshot of the latest thread state.
// The returned value is a deep copy; mutations do not affect the Conversation.
func (c *Conversation) Thread() Thread {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.thread
	t.Turns = make([]Turn, len(c.thread.Turns))
	copy(t.Turns, c.thread.Turns)
	return t
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
func (c *Conversation) Turn(ctx context.Context, opts TurnOptions) (*RunResult, error) {
	if opts.Prompt == "" {
		return nil, errors.New("prompt is required")
	}

	if err := c.process.ensureInit(ctx); err != nil {
		return nil, err
	}

	return executeTurn(ctx, turnLifecycleParams{
		client:     c.process.Client,
		turnParams: c.buildTurnParams(opts),
		thread:     c.thread,
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
		streamSendErr(ch, errors.New("prompt is required"))
		return
	}

	if err := c.process.ensureInit(ctx); err != nil {
		streamSendErr(ch, err)
		return
	}

	executeStreamedTurn(ctx, turnLifecycleParams{
		client:     c.process.Client,
		turnParams: c.buildTurnParams(opts),
		thread:     c.thread,
		threadID:   c.threadID,
		onComplete: func(turn Turn) {
			c.mu.Lock()
			c.thread.Turns = append(c.thread.Turns, turn)
			c.mu.Unlock()
		},
	}, ch, s)
}
