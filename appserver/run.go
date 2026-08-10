package appserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// RunOptions configures a single-turn Run() call.
type RunOptions struct {
	// Prompt is the user prompt text (required).
	Prompt string

	// Instructions optionally sets developer instructions for the thread.
	Instructions *string

	// Cwd optionally pins both the thread and turn to an absolute working directory.
	Cwd *string

	// Config optionally provides thread-scoped configuration overrides.
	Config json.RawMessage

	// Model optionally overrides the model for this turn.
	Model *string

	// Effort optionally sets reasoning effort for this turn.
	Effort *ReasoningEffort

	// Personality optionally sets the personality for the thread.
	Personality *Personality

	// ApprovalPolicy optionally sets the approval policy for the thread.
	ApprovalPolicy *AskForApproval

	// Sandbox optionally sets the thread sandbox mode.
	Sandbox *SandboxMode

	// SandboxPolicy optionally applies a more granular sandbox policy to the turn.
	SandboxPolicy SandboxPolicy

	// CollaborationMode optionally configures multi-agent collaboration for this turn.
	CollaborationMode *CollaborationMode

	// OutputSchema optionally constrains the model response to a JSON schema.
	OutputSchema interface{}
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
		Cwd:       opts.Cwd,
		Ephemeral: Ptr(true),
	}
	if len(opts.Config) > 0 {
		params.Config = append(json.RawMessage(nil), opts.Config...)
	}
	applyThreadStartOptions(&params, opts.Instructions, opts.Model, opts.Personality, opts.ApprovalPolicy)
	if opts.Sandbox != nil {
		params.Sandbox = opts.Sandbox
	}
	return params
}

// buildTurnParams converts RunOptions and a thread ID into TurnStartParams.
func buildTurnParams(opts RunOptions, threadID string) TurnStartParams {
	params := newTurnStartParams(threadID, opts.Prompt)
	applyTurnStartOptions(&params, opts.Model, opts.Effort, opts.CollaborationMode, opts.OutputSchema)
	if opts.Cwd != nil {
		params.Cwd = opts.Cwd
	}
	if opts.SandboxPolicy != nil {
		params.SandboxPolicy = &opts.SandboxPolicy
	}
	return params
}

func applyThreadStartOptions(params *ThreadStartParams, instructions *string, model *string, personality *Personality, approvalPolicy *AskForApproval) {
	if instructions != nil {
		params.DeveloperInstructions = instructions
	}
	if model != nil {
		params.Model = model
	}
	if personality != nil {
		params.Personality = personality
	}
	if approvalPolicy != nil {
		params.ApprovalPolicy = approvalPolicy
	}
}

func newTurnStartParams(threadID string, prompt string) TurnStartParams {
	return TurnStartParams{
		ThreadID: threadID,
		Input:    []UserInput{&TextUserInput{Text: prompt}},
	}
}

func applyTurnStartOptions(params *TurnStartParams, model *string, effort *ReasoningEffort, collaborationMode *CollaborationMode, outputSchema interface{}) {
	if model != nil {
		params.Model = model
	}
	if effort != nil {
		params.Effort = effort
	}
	if collaborationMode != nil {
		params.CollaborationMode = collaborationMode
	}
	if outputSchema != nil {
		params.OutputSchema = outputSchema
	}
}

func validatePrompt(prompt string) error {
	if prompt == "" {
		return errors.New("prompt is required")
	}
	return nil
}

func validatePromptContext(ctx context.Context, prompt string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	return validatePrompt(prompt)
}

// buildRunResult assembles a RunResult from collected items and turn data.
func buildRunResult(thread Thread, turn Turn, items []ThreadItemWrapper) *RunResult {
	resultThread := cloneThreadState(thread)
	resultTurn := turnWithItems(turn, items)
	resultItems := cloneThreadItems(items)
	resultThread.Turns = append(resultThread.Turns, cloneTurn(resultTurn))

	result := &RunResult{
		Thread: resultThread,
		Turn:   resultTurn,
		Items:  resultItems,
	}
	// Extract response text from the last agentMessage item.
	for i := len(resultItems) - 1; i >= 0; i-- {
		if msg, ok := resultItems[i].Value.(*AgentMessageThreadItem); ok {
			result.Response = msg.Text
			break
		}
	}
	return result
}

func turnWithItems(turn Turn, items []ThreadItemWrapper) Turn {
	cp := turn
	cp.Items = cloneThreadItems(items)
	cp.Error = cloneTurnError(turn.Error)
	return cp
}

// Run executes a single-turn conversation: creates a thread, starts a turn
// with the given prompt, collects items until the turn completes, and returns
// the result. This is the simplest way to get a response from the Codex CLI.
func (p *Process) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	if err := validatePromptContext(ctx, opts.Prompt); err != nil {
		return nil, err
	}

	if err := p.ensureInit(ctx); err != nil {
		return nil, err
	}

	threadResp, err := p.Client.Thread.Start(ctx, buildThreadParams(opts))
	if err != nil {
		return nil, fmt.Errorf("thread/start: %w", err)
	}

	return executeTurn(ctx, turnLifecycleParams{
		client:                    p.Client,
		turnParams:                buildTurnParams(opts, threadResp.Thread.ID),
		thread:                    threadResp.Thread,
		threadID:                  threadResp.Thread.ID,
		allowMissingInitialTurnID: true,
	})
}
