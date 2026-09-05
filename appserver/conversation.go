package appserver

import (
	"context"
	"encoding/json"
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

func cloneThreadState(thread Thread) Thread {
	t := thread
	t.ForkedFromID = cloneStringPtr(thread.ForkedFromID)
	t.ParentThreadID = cloneStringPtr(thread.ParentThreadID)
	t.ProjectID = cloneStringPtr(thread.ProjectID)
	t.RecencyAt = clonePtr(thread.RecencyAt)
	t.Section = cloneArbitraryValue(thread.Section)
	t.SectionEnteredAt = clonePtr(thread.SectionEnteredAt)
	t.Model = cloneStringPtr(thread.Model)
	t.ReasoningEffort = cloneReasoningEffortPtr(thread.ReasoningEffort)
	t.ThreadSource = clonePtr(thread.ThreadSource)
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
	cp.StartedAt = clonePtr(turn.StartedAt)
	cp.CompletedAt = clonePtr(turn.CompletedAt)
	cp.DurationMs = clonePtr(turn.DurationMs)
	cp.Items = cloneThreadItems(turn.Items)
	cp.Error = cloneTurnError(turn.Error)
	return cp
}

func cloneThreadItems(items []ThreadItemWrapper) []ThreadItemWrapper {
	return cloneSlice(items, cloneThreadItemWrapper)
}

func cloneTurnError(err *TurnError) *TurnError {
	if err == nil {
		return nil
	}
	cp := *err
	cp.Misalignment = cloneArbitraryValue(err.Misalignment)
	cp.CodexErrorInfo = append(json.RawMessage(nil), err.CodexErrorInfo...)
	cp.AdditionalDetails = cloneStringPtr(err.AdditionalDetails)
	cp.Raw = append(json.RawMessage(nil), err.Raw...)
	return &cp
}

func cloneThreadItemWrapper(w ThreadItemWrapper) ThreadItemWrapper {
	if w.Value == nil {
		return w
	}
	// Keep these cases explicit: each schema variant has different pointer,
	// slice, raw JSON, or nested-union fields that need deep-copy isolation.
	switch v := w.Value.(type) {
	case *UserMessageThreadItem:
		cp := *v
		cp.Content = cloneUserInputs(v.Content)
		return ThreadItemWrapper{Value: &cp}
	case *AgentMessageThreadItem:
		cp := *v
		cp.Questions = cloneArbitraryValue(v.Questions)
		cp.MemoryCitation = cloneArbitraryValue(v.MemoryCitation)
		cp.Delivery = clonePtr(v.Delivery)
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
		cp.DurationMs = clonePtr(v.DurationMs)
		cp.ExitCode = clonePtr(v.ExitCode)
		cp.ProcessId = cloneStringPtr(v.ProcessId)
		return ThreadItemWrapper{Value: &cp}
	case *FileChangeThreadItem:
		cp := *v
		cp.Changes = cloneFileUpdateChanges(v.Changes)
		return ThreadItemWrapper{Value: &cp}
	case *McpToolCallThreadItem:
		cp := *v
		cp.McpAppResourceURI = cloneStringPtr(v.McpAppResourceURI)
		cp.AppContext = cloneArbitraryValue(v.AppContext)
		cp.PluginID = cloneStringPtr(v.PluginID)
		cp.ReadOnlyHint = cloneBoolPtr(v.ReadOnlyHint)
		cp.Arguments = cloneJSONValue(v.Arguments)
		cp.Result = cloneMcpToolCallResult(v.Result)
		cp.Error = cloneMcpToolCallError(v.Error)
		cp.DurationMs = clonePtr(v.DurationMs)
		return ThreadItemWrapper{Value: &cp}
	case *DynamicToolCallThreadItem:
		cp := *v
		cp.Arguments = cloneJSONValue(v.Arguments)
		cp.ContentItems = cloneDynamicToolCallOutputContentItems(v.ContentItems)
		cp.Success = cloneBoolPtr(v.Success)
		cp.DurationMs = clonePtr(v.DurationMs)
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
		// Best-effort fallback for unexpected in-memory variants. If the JSON
		// round-trip path does not work, preserve the in-memory value with a
		// reflective deep clone instead of silently dropping it.
		return cloneThreadItemWrapperFallback(w)
	}
}

func cloneSessionSourceWrapper(w SessionSourceWrapper) SessionSourceWrapper {
	if w.Value == nil {
		return w
	}
	switch v := w.Value.(type) {
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
	return cloneSlice(in, cloneUserInput)
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
	return cloneSlice(in, func(element TextElement) TextElement {
		element.Placeholder = cloneStringPtr(element.Placeholder)
		return element
	})
}

func cloneCommandActions(in []CommandActionWrapper) []CommandActionWrapper {
	return cloneSlice(in, cloneCommandActionWrapper)
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
	return cloneSlice(in, func(change FileUpdateChange) FileUpdateChange {
		change.Kind = clonePatchChangeKindWrapper(change.Kind)
		return change
	})
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
	return cloneSlice(in, cloneDynamicToolCallOutputContentItemWrapper)
}

// Keep this union clone explicit for schema-specific deep-copy isolation.
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

// Keep this union clone explicit for schema-specific deep-copy isolation.
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
	return ThreadItemWrapper{Value: cloneArbitraryValue(w.Value)}
}

func cloneSessionSourceWrapperFallback(w SessionSourceWrapper) SessionSourceWrapper {
	var clone SessionSourceWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return SessionSourceWrapper{Value: cloneArbitraryValue(w.Value)}
}

func cloneThreadStatusWrapperFallback(w ThreadStatusWrapper) ThreadStatusWrapper {
	var clone ThreadStatusWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return ThreadStatusWrapper{Value: cloneArbitraryValue(w.Value)}
}

func cloneSubAgentSourceFallback(src SubAgentSource) SubAgentSource {
	var clone SubAgentSource
	if cloneViaJSON(src, &clone) {
		return clone
	}
	return cloneArbitraryValue(src)
}

func cloneUserInputFallback(input UserInput) UserInput {
	var clone UserInput
	if cloneViaJSON(input, &clone) {
		return clone
	}
	return cloneArbitraryValue(input)
}

func cloneCommandActionWrapperFallback(w CommandActionWrapper) CommandActionWrapper {
	var clone CommandActionWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return CommandActionWrapper{Value: cloneArbitraryValue(w.Value)}
}

func clonePatchChangeKindWrapperFallback(w PatchChangeKindWrapper) PatchChangeKindWrapper {
	var clone PatchChangeKindWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return PatchChangeKindWrapper{Value: cloneArbitraryValue(w.Value)}
}

func cloneDynamicToolCallOutputContentItemWrapperFallback(w DynamicToolCallOutputContentItemWrapper) DynamicToolCallOutputContentItemWrapper {
	var clone DynamicToolCallOutputContentItemWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return DynamicToolCallOutputContentItemWrapper{Value: cloneArbitraryValue(w.Value)}
}

func cloneWebSearchActionWrapperFallback(w WebSearchActionWrapper) WebSearchActionWrapper {
	var clone WebSearchActionWrapper
	if cloneViaJSON(w, &clone) {
		return clone
	}
	return WebSearchActionWrapper{Value: cloneArbitraryValue(w.Value)}
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
	return cloneArbitraryValue(in)
}

func cloneArbitraryValue[T any](in T) T {
	return deepcopy.Value(in)
}

func cloneSlice[T any](in []T, clone func(T) T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	for i, value := range in {
		out[i] = clone(value)
	}
	return out
}

func cloneMessagePhasePtr(in *MessagePhase) *MessagePhase {
	return clonePtr(in)
}

func cloneReasoningEffortPtr(in *ReasoningEffort) *ReasoningEffort {
	return clonePtr(in)
}

func cloneBoolPtr(in *bool) *bool {
	return clonePtr(in)
}

func cloneStringSlicePtr(in *[]string) *[]string {
	if in == nil {
		return nil
	}
	out := append([]string(nil), (*in)...)
	return &out
}

func cloneStringPtr(s *string) *string {
	return clonePtr(s)
}

func clonePtr[T any](in *T) *T {
	if in == nil {
		return nil
	}
	v := *in
	return &v
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
