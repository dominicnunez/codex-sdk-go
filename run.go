package codex

import (
	"context"
	"errors"
	"fmt"
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
	if opts.Model != nil {
		params.Model = opts.Model
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

	return executeTurn(ctx, turnLifecycleParams{
		client:     p.Client,
		turnParams: buildTurnParams(opts, threadResp.Thread.ID),
		thread:     threadResp.Thread,
		threadID:   threadResp.Thread.ID,
	})
}
