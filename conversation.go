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
	process                  *Process
	threadID                 string
	thread                   Thread
	mu                       sync.Mutex
	activeTurn               bool
	hasCompletedTerminalTurn bool
}

// ThreadID returns the underlying thread ID.
func (c *Conversation) ThreadID() string {
	return c.threadID
}

// Thread returns a deep-copy snapshot of the latest thread state known to the
// client. The snapshot reflects thread metadata cached from thread service
// responses, thread lifecycle notifications, and turns completed through this
// Conversation. When no client-backed snapshot is available, it falls back to
// the Conversation's locally tracked state. The returned Thread is fully
// isolated from internal state, so mutating the snapshot does not affect the
// Conversation or client cache.
func (c *Conversation) Thread() Thread {
	if c.process != nil && c.process.Client != nil {
		if snapshot, ok := c.process.Client.threadStateSnapshot(c.threadID); ok {
			return snapshot
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneThreadState(c.thread)
}

func (c *Conversation) latestThreadStateLocked() Thread {
	if c.process != nil && c.process.Client != nil {
		if snapshot, ok := c.process.Client.threadStateSnapshot(c.threadID); ok {
			return snapshot
		}
	}
	return cloneThreadState(c.thread)
}

func (c *Conversation) applyCompletedThread(thread Thread) {
	c.mu.Lock()
	c.thread = cloneThreadState(thread)
	c.hasCompletedTerminalTurn = true
	c.mu.Unlock()
}

func cloneThreadState(thread Thread) Thread {
	t := thread
	t.Name = cloneStringPtr(thread.Name)
	t.AgentNickname = cloneStringPtr(thread.AgentNickname)
	t.AgentRole = cloneStringPtr(thread.AgentRole)
	t.Path = cloneStringPtr(thread.Path)
	if thread.GitInfo != nil {
		g := *thread.GitInfo
		g.Branch = cloneStringPtr(g.Branch)
		g.OriginURL = cloneStringPtr(g.OriginURL)
		g.SHA = cloneStringPtr(g.SHA)
		t.GitInfo = &g
	}
	t.Source = cloneSessionSourceWrapper(thread.Source)
	t.Status = cloneThreadStatusWrapper(thread.Status)
	t.Turns = make([]Turn, len(thread.Turns))
	for i, turn := range thread.Turns {
		t.Turns[i] = cloneTurn(turn)
	}
	return t
}

func cloneTurn(turn Turn) Turn {
	cp := turn
	cp.Items = cloneThreadItems(turn.Items)
	cp.Error = cloneTurnError(turn.Error)
	return cp
}

func cloneThreadItems(items []ThreadItemWrapper) []ThreadItemWrapper {
	if items == nil {
		return nil
	}
	out := make([]ThreadItemWrapper, len(items))
	for i, item := range items {
		out[i] = cloneThreadItemWrapper(item)
	}
	return out
}

func cloneTurnError(err *TurnError) *TurnError {
	if err == nil {
		return nil
	}
	cp := *err
	cp.CodexErrorInfo = append(json.RawMessage(nil), err.CodexErrorInfo...)
	cp.AdditionalDetails = cloneStringPtr(err.AdditionalDetails)
	return &cp
}

func cloneThreadItemWrapper(w ThreadItemWrapper) ThreadItemWrapper {
	if w.Value == nil {
		return w
	}
	switch v := w.Value.(type) {
	case *UserMessageThreadItem:
		cp := *v
		cp.Content = cloneUserInputs(v.Content)
		return ThreadItemWrapper{Value: &cp}
	case *AgentMessageThreadItem:
		cp := *v
		cp.Phase = cloneMessagePhasePtr(v.Phase)
		return ThreadItemWrapper{Value: &cp}
	case *PlanThreadItem:
		cp := *v
		return ThreadItemWrapper{Value: &cp}
	case *ReasoningThreadItem:
		cp := *v
		cp.Content = append([]string(nil), v.Content...)
		cp.Summary = append([]string(nil), v.Summary...)
		return ThreadItemWrapper{Value: &cp}
	case *CommandExecutionThreadItem:
		cp := *v
		cp.CommandActions = cloneCommandActions(v.CommandActions)
		cp.AggregatedOutput = cloneStringPtr(v.AggregatedOutput)
		cp.DurationMs = cloneInt64Ptr(v.DurationMs)
		cp.ExitCode = cloneInt32Ptr(v.ExitCode)
		cp.ProcessId = cloneStringPtr(v.ProcessId)
		return ThreadItemWrapper{Value: &cp}
	case *FileChangeThreadItem:
		cp := *v
		cp.Changes = cloneFileUpdateChanges(v.Changes)
		return ThreadItemWrapper{Value: &cp}
	case *McpToolCallThreadItem:
		cp := *v
		cp.Arguments = cloneJSONValue(v.Arguments)
		cp.Result = cloneMcpToolCallResult(v.Result)
		cp.Error = cloneMcpToolCallError(v.Error)
		cp.DurationMs = cloneInt64Ptr(v.DurationMs)
		return ThreadItemWrapper{Value: &cp}
	case *DynamicToolCallThreadItem:
		cp := *v
		cp.Arguments = cloneJSONValue(v.Arguments)
		cp.ContentItems = cloneDynamicToolCallOutputContentItems(v.ContentItems)
		cp.Success = cloneBoolPtr(v.Success)
		cp.DurationMs = cloneInt64Ptr(v.DurationMs)
		return ThreadItemWrapper{Value: &cp}
	case *CollabAgentToolCallThreadItem:
		cp := *v
		cp.AgentsStates = cloneCollabAgentStates(v.AgentsStates)
		cp.Model = cloneStringPtr(v.Model)
		cp.ReceiverThreadIds = append([]string(nil), v.ReceiverThreadIds...)
		cp.ReasoningEffort = cloneReasoningEffortPtr(v.ReasoningEffort)
		cp.Prompt = cloneStringPtr(v.Prompt)
		return ThreadItemWrapper{Value: &cp}
	case *WebSearchThreadItem:
		cp := *v
		if v.Action != nil {
			action := cloneWebSearchActionWrapper(*v.Action)
			cp.Action = &action
		}
		return ThreadItemWrapper{Value: &cp}
	case *ImageViewThreadItem:
		cp := *v
		return ThreadItemWrapper{Value: &cp}
	case *EnteredReviewModeThreadItem:
		cp := *v
		return ThreadItemWrapper{Value: &cp}
	case *ExitedReviewModeThreadItem:
		cp := *v
		return ThreadItemWrapper{Value: &cp}
	case *ContextCompactionThreadItem:
		cp := *v
		return ThreadItemWrapper{Value: &cp}
	case *UnknownThreadItem:
		cp := *v
		cp.Raw = append(json.RawMessage(nil), v.Raw...)
		return ThreadItemWrapper{Value: &cp}
	default:
		// Best-effort fallback for unexpected in-memory variants. If the JSON roundtrip
		// cannot clone the value, the fallback returns a zero wrapper instead of preserving it.
		return cloneThreadItemWrapperFallback(w)
	}
}

func cloneSessionSourceWrapper(w SessionSourceWrapper) SessionSourceWrapper {
	if w.Value == nil {
		return w
	}
	switch v := w.Value.(type) {
	case sessionSourceLiteral:
		return SessionSourceWrapper{Value: v}
	case SessionSourceSubAgent:
		return SessionSourceWrapper{Value: SessionSourceSubAgent{SubAgent: cloneSubAgentSource(v.SubAgent)}}
	case UnknownSessionSource:
		cp := v
		cp.Raw = append(json.RawMessage(nil), v.Raw...)
		return SessionSourceWrapper{Value: cp}
	default:
		return cloneSessionSourceWrapperFallback(w)
	}
}

func cloneThreadStatusWrapper(w ThreadStatusWrapper) ThreadStatusWrapper {
	if w.Value == nil {
		return w
	}
	switch v := w.Value.(type) {
	case ThreadStatusNotLoaded:
		return ThreadStatusWrapper{Value: v}
	case ThreadStatusIdle:
		return ThreadStatusWrapper{Value: v}
	case ThreadStatusSystemError:
		return ThreadStatusWrapper{Value: v}
	case ThreadStatusActive:
		cp := v
		cp.ActiveFlags = append([]ThreadActiveFlag(nil), v.ActiveFlags...)
		return ThreadStatusWrapper{Value: cp}
	case UnknownThreadStatus:
		cp := v
		cp.Raw = append(json.RawMessage(nil), v.Raw...)
		return ThreadStatusWrapper{Value: cp}
	default:
		return cloneThreadStatusWrapperFallback(w)
	}
}

func cloneSubAgentSource(src SubAgentSource) SubAgentSource {
	if src == nil {
		return nil
	}
	switch v := src.(type) {
	case subAgentSourceLiteral:
		return v
	case SubAgentSourceThreadSpawn:
		cp := v
		return cp
	case SubAgentSourceOther:
		cp := v
		return cp
	case UnknownSubAgentSource:
		cp := v
		cp.Raw = append(json.RawMessage(nil), v.Raw...)
		return cp
	default:
		return cloneSubAgentSourceFallback(src)
	}
}

func cloneUserInputs(in []UserInput) []UserInput {
	if in == nil {
		return nil
	}
	out := make([]UserInput, len(in))
	for i, input := range in {
		out[i] = cloneUserInput(input)
	}
	return out
}

func cloneUserInput(in UserInput) UserInput {
	if in == nil {
		return nil
	}
	switch v := in.(type) {
	case *TextUserInput:
		cp := *v
		cp.TextElements = cloneTextElements(v.TextElements)
		return &cp
	case *ImageUserInput:
		cp := *v
		return &cp
	case *LocalImageUserInput:
		cp := *v
		return &cp
	case *SkillUserInput:
		cp := *v
		return &cp
	case *MentionUserInput:
		cp := *v
		return &cp
	case *UnknownUserInput:
		cp := *v
		cp.Raw = append(json.RawMessage(nil), v.Raw...)
		return &cp
	default:
		return cloneUserInputFallback(in)
	}
}

func cloneTextElements(in []TextElement) []TextElement {
	if in == nil {
		return nil
	}
	out := make([]TextElement, len(in))
	for i, element := range in {
		out[i] = element
		out[i].Placeholder = cloneStringPtr(element.Placeholder)
	}
	return out
}

func cloneCommandActions(in []CommandActionWrapper) []CommandActionWrapper {
	if in == nil {
		return nil
	}
	out := make([]CommandActionWrapper, len(in))
	for i, action := range in {
		out[i] = cloneCommandActionWrapper(action)
	}
	return out
}

func cloneCommandActionWrapper(w CommandActionWrapper) CommandActionWrapper {
	switch v := w.Value.(type) {
	case *ReadCommandAction:
		cp := *v
		return CommandActionWrapper{Value: &cp}
	case *ListFilesCommandAction:
		cp := *v
		cp.Path = cloneStringPtr(v.Path)
		return CommandActionWrapper{Value: &cp}
	case *SearchCommandAction:
		cp := *v
		cp.Path = cloneStringPtr(v.Path)
		cp.Query = cloneStringPtr(v.Query)
		return CommandActionWrapper{Value: &cp}
	case *UnknownCommandAction:
		cp := *v
		return CommandActionWrapper{Value: &cp}
	default:
		return cloneCommandActionWrapperFallback(w)
	}
}

func cloneFileUpdateChanges(in []FileUpdateChange) []FileUpdateChange {
	if in == nil {
		return nil
	}
	out := make([]FileUpdateChange, len(in))
	for i, change := range in {
		out[i] = change
		out[i].Kind = clonePatchChangeKindWrapper(change.Kind)
	}
	return out
}

func clonePatchChangeKindWrapper(w PatchChangeKindWrapper) PatchChangeKindWrapper {
	switch v := w.Value.(type) {
	case *AddPatchChangeKind:
		return PatchChangeKindWrapper{Value: &AddPatchChangeKind{}}
	case *DeletePatchChangeKind:
		return PatchChangeKindWrapper{Value: &DeletePatchChangeKind{}}
	case *UpdatePatchChangeKind:
		cp := *v
		cp.MovePath = cloneStringPtr(v.MovePath)
		return PatchChangeKindWrapper{Value: &cp}
	case *UnknownPatchChangeKind:
		cp := *v
		cp.Raw = append(json.RawMessage(nil), v.Raw...)
		return PatchChangeKindWrapper{Value: &cp}
	default:
		return clonePatchChangeKindWrapperFallback(w)
	}
}

func cloneMcpToolCallResult(in *McpToolCallResult) *McpToolCallResult {
	if in == nil {
		return nil
	}
	out := &McpToolCallResult{
		Content:           make([]interface{}, len(in.Content)),
		StructuredContent: cloneJSONValue(in.StructuredContent),
	}
	for i, item := range in.Content {
		out.Content[i] = cloneJSONValue(item)
	}
	return out
}

func cloneMcpToolCallError(in *McpToolCallError) *McpToolCallError {
	if in == nil {
		return nil
	}
	cp := *in
	return &cp
}

func cloneDynamicToolCallOutputContentItems(in []DynamicToolCallOutputContentItemWrapper) []DynamicToolCallOutputContentItemWrapper {
	if in == nil {
		return nil
	}
	out := make([]DynamicToolCallOutputContentItemWrapper, len(in))
	for i, item := range in {
		out[i] = cloneDynamicToolCallOutputContentItemWrapper(item)
	}
	return out
}

func cloneDynamicToolCallOutputContentItemWrapper(w DynamicToolCallOutputContentItemWrapper) DynamicToolCallOutputContentItemWrapper {
	switch v := w.Value.(type) {
	case *InputTextDynamicToolCallOutputContentItem:
		cp := *v
		return DynamicToolCallOutputContentItemWrapper{Value: &cp}
	case *InputImageDynamicToolCallOutputContentItem:
		cp := *v
		return DynamicToolCallOutputContentItemWrapper{Value: &cp}
	case *UnknownDynamicToolCallOutputContentItem:
		cp := *v
		cp.Raw = append(json.RawMessage(nil), v.Raw...)
		return DynamicToolCallOutputContentItemWrapper{Value: &cp}
	default:
		return cloneDynamicToolCallOutputContentItemWrapperFallback(w)
	}
}

func cloneCollabAgentStates(in map[string]CollabAgentState) map[string]CollabAgentState {
	if in == nil {
		return nil
	}
	out := make(map[string]CollabAgentState, len(in))
	for key, value := range in {
		cp := value
		cp.Message = cloneStringPtr(value.Message)
		out[key] = cp
	}
	return out
}

func cloneWebSearchActionWrapper(w WebSearchActionWrapper) WebSearchActionWrapper {
	switch v := w.Value.(type) {
	case *SearchWebSearchAction:
		cp := *v
		cp.Query = cloneStringPtr(v.Query)
		cp.Queries = cloneStringSlicePtr(v.Queries)
		return WebSearchActionWrapper{Value: &cp}
	case *OpenPageWebSearchAction:
		cp := *v
		cp.URL = cloneStringPtr(v.URL)
		return WebSearchActionWrapper{Value: &cp}
	case *FindInPageWebSearchAction:
		cp := *v
		cp.URL = cloneStringPtr(v.URL)
		cp.Pattern = cloneStringPtr(v.Pattern)
		return WebSearchActionWrapper{Value: &cp}
	case *OtherWebSearchAction:
		return WebSearchActionWrapper{Value: &OtherWebSearchAction{}}
	case *UnknownWebSearchAction:
		cp := *v
		cp.Raw = append(json.RawMessage(nil), v.Raw...)
		return WebSearchActionWrapper{Value: &cp}
	default:
		return cloneWebSearchActionWrapperFallback(w)
	}
}

func cloneThreadItemWrapperFallback(w ThreadItemWrapper) ThreadItemWrapper {
	var clone ThreadItemWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return ThreadItemWrapper{}
}

func cloneSessionSourceWrapperFallback(w SessionSourceWrapper) SessionSourceWrapper {
	var clone SessionSourceWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return SessionSourceWrapper{}
}

func cloneThreadStatusWrapperFallback(w ThreadStatusWrapper) ThreadStatusWrapper {
	var clone ThreadStatusWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return ThreadStatusWrapper{}
}

func cloneSubAgentSourceFallback(src SubAgentSource) SubAgentSource {
	var clone SubAgentSource
	if cloneViaJSON(src, &clone) {
		return clone
	}
	return nil
}

func cloneUserInputFallback(input UserInput) UserInput {
	var clone UserInput
	if cloneViaJSON(input, &clone) {
		return clone
	}
	return nil
}

func cloneCommandActionWrapperFallback(w CommandActionWrapper) CommandActionWrapper {
	var clone CommandActionWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return CommandActionWrapper{}
}

func clonePatchChangeKindWrapperFallback(w PatchChangeKindWrapper) PatchChangeKindWrapper {
	var clone PatchChangeKindWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return PatchChangeKindWrapper{}
}

func cloneDynamicToolCallOutputContentItemWrapperFallback(w DynamicToolCallOutputContentItemWrapper) DynamicToolCallOutputContentItemWrapper {
	var clone DynamicToolCallOutputContentItemWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return DynamicToolCallOutputContentItemWrapper{}
}

func cloneWebSearchActionWrapperFallback(w WebSearchActionWrapper) WebSearchActionWrapper {
	var clone WebSearchActionWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return WebSearchActionWrapper{}
}

func cloneViaJSON(in, out interface{}) bool {
	b, err := json.Marshal(in)
	if err != nil {
		return false
	}
	return json.Unmarshal(b, out) == nil
}

func cloneJSONValue(in interface{}) interface{} {
	if in == nil {
		return nil
	}
	var out interface{}
	if cloneViaJSON(in, &out) {
		return out
	}
	return nil
}

func cloneMessagePhasePtr(in *MessagePhase) *MessagePhase {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func cloneReasoningEffortPtr(in *ReasoningEffort) *ReasoningEffort {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func cloneStringSlicePtr(in *[]string) *[]string {
	if in == nil {
		return nil
	}
	out := append([]string(nil), (*in)...)
	return &out
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
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
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
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
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
	thread := c.latestThreadStateLocked()
	allowMissingInitialTurnID := !c.hasCompletedTerminalTurn
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.activeTurn = false
		c.mu.Unlock()
	}()

	return executeTurn(ctx, turnLifecycleParams{
		client:                    c.process.Client,
		turnParams:                c.buildTurnParams(opts),
		thread:                    thread,
		threadID:                  c.threadID,
		allowMissingInitialTurnID: allowMissingInitialTurnID,
		onComplete:                c.applyCompletedThread,
	})
}

// TurnStreamed executes a streaming turn on the existing thread.
func (c *Conversation) TurnStreamed(ctx context.Context, opts TurnOptions) *Stream {
	if err := validateContext(ctx); err != nil {
		return newErrorStream(err)
	}
	g := newGuardedChan(streamChannelBuffer)
	s := &Stream{
		done:  make(chan struct{}),
		queue: g,
	}

	s.events = streamIterator(g)

	go c.turnStreamedLifecycle(ctx, opts, g, s)

	return s
}

func (c *Conversation) turnStreamedLifecycle(ctx context.Context, opts TurnOptions, g *guardedChan, s *Stream) {
	defer g.closeOnce()
	defer close(s.done)

	if opts.Prompt == "" {
		streamSendErr(g, errors.New("prompt is required"))
		return
	}

	if err := c.process.ensureInit(ctx); err != nil {
		streamSendErr(g, err)
		return
	}

	c.mu.Lock()
	if c.activeTurn {
		c.mu.Unlock()
		streamSendErr(g, errTurnInProgress)
		return
	}
	c.activeTurn = true
	thread := c.latestThreadStateLocked()
	allowMissingInitialTurnID := !c.hasCompletedTerminalTurn
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.activeTurn = false
		c.mu.Unlock()
	}()

	executeStreamedTurn(ctx, turnLifecycleParams{
		client:                    c.process.Client,
		turnParams:                c.buildTurnParams(opts),
		thread:                    thread,
		threadID:                  c.threadID,
		allowMissingInitialTurnID: allowMissingInitialTurnID,
		onComplete:                c.applyCompletedThread,
	}, g, s)
}
