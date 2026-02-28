package codex

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// RunOptions configures a single-turn Run() call.
type RunOptions struct {
	// Prompt is the user prompt text (required).
	Prompt string

	// Instructions optionally sets developer instructions for the thread.
	Instructions *string

	// Model optionally overrides the model for this turn.
	Model *string

	// Effort optionally sets reasoning effort for this turn.
	Effort *ReasoningEffort

	// Personality optionally sets the personality for the thread.
	Personality *Personality

	// ApprovalPolicy optionally sets the approval policy for the thread.
	ApprovalPolicy *AskForApproval
}

// RunResult contains the output of a completed turn.
type RunResult struct {
	// Thread is the thread state after the turn.
	Thread Thread

	// Turn is the completed turn with items.
	Turn Turn

	// Items contains all items received via item/completed notifications during the turn.
	Items []ThreadItemWrapper

	// Response is the text from the last agentMessage item (convenience field).
	Response string
}

// Run executes a single-turn conversation: creates a thread, starts a turn
// with the given prompt, collects items until the turn completes, and returns
// the result. This is the simplest way to get a response from the Codex CLI.
func (p *Process) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	if opts.Prompt == "" {
		return nil, errors.New("prompt is required")
	}

	// Idempotent initialize handshake.
	p.initOnce.Do(func() {
		_, p.initErr = p.Client.Initialize(ctx, InitializeParams{
			ClientInfo: ClientInfo{Name: "codex-sdk-go", Version: "0.1.0"},
		})
	})
	if p.initErr != nil {
		return nil, fmt.Errorf("initialize: %w", p.initErr)
	}

	// Start a thread.
	threadParams := ThreadStartParams{
		Ephemeral: Ptr(true),
	}
	if opts.Instructions != nil {
		threadParams.DeveloperInstructions = opts.Instructions
	}
	if opts.Model != nil {
		threadParams.Model = opts.Model
	}
	if opts.Personality != nil {
		threadParams.Personality = opts.Personality
	}
	if opts.ApprovalPolicy != nil {
		threadParams.ApprovalPolicy = opts.ApprovalPolicy
	}

	threadResp, err := p.Client.Thread.Start(ctx, threadParams)
	if err != nil {
		return nil, fmt.Errorf("thread/start: %w", err)
	}

	// Collect items and wait for turn completion.
	var (
		items []ThreadItemWrapper
		mu    sync.Mutex
		done  = make(chan TurnCompletedNotification, 1)
	)

	p.Client.OnItemCompleted(func(n ItemCompletedNotification) {
		mu.Lock()
		items = append(items, n.Item)
		mu.Unlock()
	})

	p.Client.OnTurnCompleted(func(n TurnCompletedNotification) {
		select {
		case done <- n:
		default:
		}
	})

	// Clean up listeners when we return.
	defer func() {
		p.Client.OnItemCompleted(nil)
		p.Client.OnTurnCompleted(nil)
	}()

	// Start the turn.
	turnParams := TurnStartParams{
		ThreadID: threadResp.Thread.ID,
		Input:    []UserInput{&TextUserInput{Text: opts.Prompt}},
	}
	if opts.Effort != nil {
		turnParams.Effort = opts.Effort
	}

	if _, err := p.Client.Turn.Start(ctx, turnParams); err != nil {
		return nil, fmt.Errorf("turn/start: %w", err)
	}

	// Wait for completion or cancellation.
	select {
	case completed := <-done:
		if completed.Turn.Error != nil {
			return nil, fmt.Errorf("turn error: %s", completed.Turn.Error.Message)
		}

		mu.Lock()
		collectedItems := make([]ThreadItemWrapper, len(items))
		copy(collectedItems, items)
		mu.Unlock()

		result := &RunResult{
			Thread: threadResp.Thread,
			Turn:   completed.Turn,
			Items:  collectedItems,
		}

		// Extract response text from the last agentMessage item.
		for i := len(collectedItems) - 1; i >= 0; i-- {
			if msg, ok := collectedItems[i].Value.(*AgentMessageThreadItem); ok {
				result.Response = msg.Text
				break
			}
		}

		return result, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
