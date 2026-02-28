package codex

import (
	"context"
	"encoding/json"
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

	// CollaborationMode optionally configures multi-agent collaboration for this turn.
	CollaborationMode *CollaborationMode
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

// buildThreadParams converts RunOptions into ThreadStartParams.
func buildThreadParams(opts RunOptions) ThreadStartParams {
	params := ThreadStartParams{
		Ephemeral: Ptr(true),
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
	return params
}

// buildTurnParams converts RunOptions and a thread ID into TurnStartParams.
func buildTurnParams(opts RunOptions, threadID string) TurnStartParams {
	params := TurnStartParams{
		ThreadID: threadID,
		Input:    []UserInput{&TextUserInput{Text: opts.Prompt}},
	}
	if opts.Effort != nil {
		params.Effort = opts.Effort
	}
	if opts.CollaborationMode != nil {
		params.CollaborationMode = opts.CollaborationMode
	}
	return params
}

// buildRunResult assembles a RunResult from collected items and turn data.
func buildRunResult(thread Thread, turn Turn, items []ThreadItemWrapper) *RunResult {
	result := &RunResult{
		Thread: thread,
		Turn:   turn,
		Items:  items,
	}
	// Extract response text from the last agentMessage item.
	for i := len(items) - 1; i >= 0; i-- {
		if msg, ok := items[i].Value.(*AgentMessageThreadItem); ok {
			result.Response = msg.Text
			break
		}
	}
	return result
}

// Run executes a single-turn conversation: creates a thread, starts a turn
// with the given prompt, collects items until the turn completes, and returns
// the result. This is the simplest way to get a response from the Codex CLI.
func (p *Process) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	if opts.Prompt == "" {
		return nil, errors.New("prompt is required")
	}

	if err := p.ensureInit(ctx); err != nil {
		return nil, err
	}

	threadResp, err := p.Client.Thread.Start(ctx, buildThreadParams(opts))
	if err != nil {
		return nil, fmt.Errorf("thread/start: %w", err)
	}

	// Collect items and wait for turn completion via internal listeners,
	// which don't clobber user-registered handlers.
	var (
		items []ThreadItemWrapper
		mu    sync.Mutex
		done  = make(chan TurnCompletedNotification, 1)
	)

	unsubItem := p.Client.addNotificationListener(notifyItemCompleted, func(_ context.Context, notif Notification) {
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		mu.Lock()
		items = append(items, n.Item)
		mu.Unlock()
	})

	unsubTurn := p.Client.addNotificationListener(notifyTurnCompleted, func(_ context.Context, notif Notification) {
		var n TurnCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			// Synthesize a failure so Run() doesn't hang on malformed JSON.
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

	if _, err := p.Client.Turn.Start(ctx, buildTurnParams(opts, threadResp.Thread.ID)); err != nil {
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

		return buildRunResult(threadResp.Thread, completed.Turn, collectedItems), nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
