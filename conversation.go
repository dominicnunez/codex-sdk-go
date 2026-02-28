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

// Thread returns the latest thread state.
func (c *Conversation) Thread() Thread {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.thread
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

	var (
		items []ThreadItemWrapper
		mu    sync.Mutex
		done  = make(chan TurnCompletedNotification, 1)
	)

	client := c.process.Client

	unsubItem := client.addNotificationListener(notifyItemCompleted, func(_ context.Context, notif Notification) {
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		mu.Lock()
		items = append(items, n.Item)
		mu.Unlock()
	})

	unsubTurn := client.addNotificationListener(notifyTurnCompleted, func(_ context.Context, notif Notification) {
		var n TurnCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			n = TurnCompletedNotification{
				Turn: Turn{Error: &TurnError{Message: "failed to unmarshal turn/completed: " + err.Error()}},
			}
		}
		select {
		case done <- n:
		default:
		}
	})

	defer unsubItem()
	defer unsubTurn()

	if _, err := client.Turn.Start(ctx, c.buildTurnParams(opts)); err != nil {
		return nil, fmt.Errorf("turn/start: %w", err)
	}

	select {
	case completed := <-done:
		if completed.Turn.Error != nil {
			return nil, fmt.Errorf("turn error: %s", completed.Turn.Error.Message)
		}

		mu.Lock()
		collectedItems := make([]ThreadItemWrapper, len(items))
		copy(collectedItems, items)
		mu.Unlock()

		c.mu.Lock()
		// Update thread turns with the completed turn.
		c.thread.Turns = append(c.thread.Turns, completed.Turn)
		c.mu.Unlock()

		return buildRunResult(c.thread, completed.Turn, collectedItems), nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
		streamSend(ch, eventOrErr{err: errors.New("prompt is required")})
		return
	}

	client := c.process.Client

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
		unsub := client.addNotificationListener(method, handler)
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

	on(notifyItemStarted, func(_ context.Context, notif Notification) {
		var n ItemStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		if collab, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
			streamSend(ch, eventOrErr{event: newCollabStarted(collab)})
		}
		streamSend(ch, eventOrErr{event: &ItemStarted{Item: n.Item}})
	})

	on(notifyItemCompleted, func(_ context.Context, notif Notification) {
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		itemsMu.Lock()
		items = append(items, n.Item)
		itemsMu.Unlock()
		if collab, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
			streamSend(ch, eventOrErr{event: newCollabCompleted(collab)})
		}
		streamSend(ch, eventOrErr{event: &ItemCompleted{Item: n.Item}})
	})

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

	if _, err := client.Turn.Start(ctx, c.buildTurnParams(opts)); err != nil {
		streamSend(ch, eventOrErr{err: fmt.Errorf("turn/start: %w", err)})
		return
	}

	select {
	case completed := <-turnDone:
		streamSend(ch, eventOrErr{event: &TurnCompleted{Turn: completed.Turn}})

		if completed.Turn.Error != nil {
			streamSend(ch, eventOrErr{err: fmt.Errorf("turn error: %s", completed.Turn.Error.Message)})
			return
		}

		itemsMu.Lock()
		collectedItems := make([]ThreadItemWrapper, len(items))
		copy(collectedItems, items)
		itemsMu.Unlock()

		c.mu.Lock()
		c.thread.Turns = append(c.thread.Turns, completed.Turn)
		c.mu.Unlock()

		s.mu.Lock()
		s.result = buildRunResult(c.thread, completed.Turn, collectedItems)
		s.mu.Unlock()

	case <-ctx.Done():
		streamSend(ch, eventOrErr{err: ctx.Err()})
	}
}
