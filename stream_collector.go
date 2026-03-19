package codex

import (
	"encoding/json"
	"strings"
	"sync"
)

const (
	streamCollectorErrorHistoryLimit       = 256
	streamCollectorOutputDeltaHistoryLimit = 512
	streamCollectorRawOutputChunkLimit     = 512
	streamCollectorRawOutputBytesLimit     = 256 * 1024
)

// NormalizedStreamError is a helper view that normalizes stream- and
// notification-level errors into a consistent shape while preserving message text.
type NormalizedStreamError struct {
	Kind         string
	Message      string
	ThreadID     *string
	TurnID       *string
	SourceMethod *string
	Raw          json.RawMessage
}

// CommandExecutionLifecycle tracks start/completion state and output deltas for
// a command execution thread item.
type CommandExecutionLifecycle struct {
	ItemID              string
	Started             bool
	Completed           bool
	Status              *CommandExecutionStatus
	StartedItem         *CommandExecutionThreadItem
	CompletedItem       *CommandExecutionThreadItem
	OutputDeltas        []string
	DroppedOutputDeltas int
	AggregatedOutput    string
}

// McpToolCallLifecycle tracks start/completion state for an MCP tool call item.
type McpToolCallLifecycle struct {
	ItemID        string
	Started       bool
	Completed     bool
	Status        *McpToolCallStatus
	StartedItem   *McpToolCallThreadItem
	CompletedItem *McpToolCallThreadItem
}

// WebSearchLifecycle tracks start/completion state for a web search item.
type WebSearchLifecycle struct {
	ItemID        string
	Started       bool
	Completed     bool
	StartedItem   *WebSearchThreadItem
	CompletedItem *WebSearchThreadItem
}

// FileChangeLifecycle tracks start/completion state for a file-change item.
type FileChangeLifecycle struct {
	ItemID        string
	Started       bool
	Completed     bool
	Status        *PatchApplyStatus
	StartedItem   *FileChangeThreadItem
	CompletedItem *FileChangeThreadItem
}

// StreamSummary is a convenience snapshot over a streamed run.
type StreamSummary struct {
	LatestPlanText   *string
	LatestPlanItemID *string
	LatestTokenUsage *ThreadTokenUsage

	NormalizedErrors        []NormalizedStreamError
	DroppedNormalizedErrors int

	CommandExecutions map[string]CommandExecutionLifecycle
	McpToolCalls      map[string]McpToolCallLifecycle
	WebSearches       map[string]WebSearchLifecycle
	FileChanges       map[string]FileChangeLifecycle
}

// StreamCollector accumulates a convenience summary from streamed events and
// selected notifications. It is safe for concurrent use.
type StreamCollector struct {
	mu sync.Mutex

	latestPlanText   *string
	latestPlanItemID *string
	latestTokenUsage *ThreadTokenUsage

	normalizedErrors        []NormalizedStreamError
	droppedNormalizedErrors int

	commandExecutions   map[string]CommandExecutionLifecycle
	commandOutputChunks map[string][]string
	commandOutputBytes  map[string]int
	mcpToolCalls        map[string]McpToolCallLifecycle
	webSearches         map[string]WebSearchLifecycle
	fileChanges         map[string]FileChangeLifecycle
}

// NewStreamCollector constructs a ready-to-use collector.
func NewStreamCollector() *StreamCollector {
	return &StreamCollector{
		commandExecutions:   make(map[string]CommandExecutionLifecycle),
		commandOutputChunks: make(map[string][]string),
		commandOutputBytes:  make(map[string]int),
		mcpToolCalls:        make(map[string]McpToolCallLifecycle),
		webSearches:         make(map[string]WebSearchLifecycle),
		fileChanges:         make(map[string]FileChangeLifecycle),
	}
}

// Process ingests one stream tuple from Stream.Events().
func (c *StreamCollector) Process(event Event, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil {
		c.appendErrorLocked(NormalizedStreamError{
			Kind:    "stream_error",
			Message: err.Error(),
		})
		return
	}
	if event == nil {
		return
	}

	switch e := event.(type) {
	case *PlanDelta:
		c.mergePlanDeltaLocked(e)
	case *ItemStarted:
		c.ingestStartedItemLocked(e.Item.Value)
	case *ItemCompleted:
		c.ingestCompletedItemLocked(e.Item.Value)
	case *TurnCompleted:
		if e.Turn.Error != nil {
			c.appendErrorLocked(NormalizedStreamError{
				Kind:         "turn_error",
				Message:      e.Turn.Error.Message,
				TurnID:       cloneStringPtr(&e.Turn.ID),
				SourceMethod: cloneStringPtr(Ptr(notifyTurnCompleted)),
				Raw:          append(json.RawMessage(nil), e.Turn.Error.Raw...),
			})
		}
	}
}

// Summary returns a deep-copied snapshot of the current collector state.
func (c *StreamCollector) Summary() StreamSummary {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := StreamSummary{
		LatestPlanText:          cloneStringPtr(c.latestPlanText),
		LatestPlanItemID:        cloneStringPtr(c.latestPlanItemID),
		NormalizedErrors:        make([]NormalizedStreamError, len(c.normalizedErrors)),
		DroppedNormalizedErrors: c.droppedNormalizedErrors,
		CommandExecutions:       make(map[string]CommandExecutionLifecycle, len(c.commandExecutions)),
		McpToolCalls:            make(map[string]McpToolCallLifecycle, len(c.mcpToolCalls)),
		WebSearches:             make(map[string]WebSearchLifecycle, len(c.webSearches)),
		FileChanges:             make(map[string]FileChangeLifecycle, len(c.fileChanges)),
	}

	if c.latestTokenUsage != nil {
		out.LatestTokenUsage = cloneThreadTokenUsage(c.latestTokenUsage)
	}

	for i, err := range c.normalizedErrors {
		out.NormalizedErrors[i] = cloneNormalizedStreamError(err)
	}
	for k, v := range c.commandExecutions {
		out.CommandExecutions[k] = cloneCommandExecutionLifecycle(
			v,
			c.commandOutputChunks[k],
		)
	}
	for k, v := range c.mcpToolCalls {
		out.McpToolCalls[k] = cloneMcpToolCallLifecycle(v)
	}
	for k, v := range c.webSearches {
		out.WebSearches[k] = cloneWebSearchLifecycle(v)
	}
	for k, v := range c.fileChanges {
		out.FileChanges[k] = cloneFileChangeLifecycle(v)
	}

	return out
}

func (c *StreamCollector) processCommandExecutionOutputDelta(n CommandExecutionOutputDeltaNotification) {
	c.mu.Lock()
	defer c.mu.Unlock()

	lc := c.commandExecutions[n.ItemID]
	lc.ItemID = n.ItemID
	lc.OutputDeltas, lc.DroppedOutputDeltas = appendBoundedHistory(
		lc.OutputDeltas,
		n.Delta,
		lc.DroppedOutputDeltas,
		streamCollectorOutputDeltaHistoryLimit,
	)
	chunks := append(c.commandOutputChunks[n.ItemID], n.Delta)
	bytes := c.commandOutputBytes[n.ItemID] + len(n.Delta)
	chunks, bytes = trimBoundedStringHistory(
		chunks,
		bytes,
		streamCollectorRawOutputChunkLimit,
		streamCollectorRawOutputBytesLimit,
	)
	c.commandOutputChunks[n.ItemID] = chunks
	c.commandOutputBytes[n.ItemID] = bytes
	c.commandExecutions[n.ItemID] = lc
}

func (c *StreamCollector) processThreadTokenUsageUpdated(n ThreadTokenUsageUpdatedNotification) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tu := n.TokenUsage
	c.latestTokenUsage = &tu
}

func (c *StreamCollector) processSystemError(n ErrorNotification) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.appendErrorLocked(NormalizedStreamError{
		Kind:         "system_error",
		Message:      n.Error.Message,
		ThreadID:     cloneStringPtr(&n.ThreadID),
		TurnID:       cloneStringPtr(&n.TurnID),
		SourceMethod: cloneStringPtr(Ptr(notifyError)),
		Raw:          append(json.RawMessage(nil), n.Raw...),
	})
}

func (c *StreamCollector) processThreadRealtimeError(n ThreadRealtimeErrorNotification) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.appendErrorLocked(NormalizedStreamError{
		Kind:         "realtime_error",
		Message:      n.Message,
		ThreadID:     cloneStringPtr(&n.ThreadID),
		SourceMethod: cloneStringPtr(Ptr(notifyRealtimeError)),
		Raw:          append(json.RawMessage(nil), n.Raw...),
	})
}

func (c *StreamCollector) appendErrorLocked(e NormalizedStreamError) {
	c.normalizedErrors, c.droppedNormalizedErrors = appendBoundedHistory(
		c.normalizedErrors,
		e,
		c.droppedNormalizedErrors,
		streamCollectorErrorHistoryLimit,
	)
}

func (c *StreamCollector) mergePlanDeltaLocked(p *PlanDelta) {
	if p == nil {
		return
	}
	if c.latestPlanItemID == nil || *c.latestPlanItemID != p.ItemID {
		c.latestPlanItemID = Ptr(p.ItemID)
		c.latestPlanText = Ptr(p.Delta)
		return
	}
	if c.latestPlanText == nil {
		c.latestPlanText = Ptr(p.Delta)
		return
	}
	combined := *c.latestPlanText + p.Delta
	c.latestPlanText = Ptr(combined)
}

func (c *StreamCollector) ingestStartedItemLocked(item ThreadItem) {
	switch v := item.(type) {
	case *CommandExecutionThreadItem:
		lc := c.commandExecutions[v.ID]
		lc.ItemID = v.ID
		lc.Started = true
		status := v.Status
		lc.Status = &status
		lc.StartedItem = cloneCommandExecutionItem(v)
		c.commandExecutions[v.ID] = lc
	case *McpToolCallThreadItem:
		lc := c.mcpToolCalls[v.ID]
		lc.ItemID = v.ID
		lc.Started = true
		status := v.Status
		lc.Status = &status
		lc.StartedItem = cloneMcpToolCallItem(v)
		c.mcpToolCalls[v.ID] = lc
	case *WebSearchThreadItem:
		lc := c.webSearches[v.ID]
		lc.ItemID = v.ID
		lc.Started = true
		lc.StartedItem = cloneWebSearchItem(v)
		c.webSearches[v.ID] = lc
	case *FileChangeThreadItem:
		lc := c.fileChanges[v.ID]
		lc.ItemID = v.ID
		lc.Started = true
		status := v.Status
		lc.Status = &status
		lc.StartedItem = cloneFileChangeItem(v)
		c.fileChanges[v.ID] = lc
	}
}

func (c *StreamCollector) ingestCompletedItemLocked(item ThreadItem) {
	switch v := item.(type) {
	case *PlanThreadItem:
		c.latestPlanItemID = Ptr(v.ID)
		c.latestPlanText = Ptr(v.Text)
	case *CommandExecutionThreadItem:
		lc := c.commandExecutions[v.ID]
		lc.ItemID = v.ID
		lc.Completed = true
		status := v.Status
		lc.Status = &status
		lc.CompletedItem = cloneCommandExecutionItem(v)
		if v.AggregatedOutput != nil {
			lc.AggregatedOutput = *v.AggregatedOutput
		} else if chunks := c.commandOutputChunks[v.ID]; len(chunks) > 0 {
			lc.AggregatedOutput = strings.Join(chunks, "")
		}
		delete(c.commandOutputChunks, v.ID)
		delete(c.commandOutputBytes, v.ID)
		c.commandExecutions[v.ID] = lc
	case *McpToolCallThreadItem:
		lc := c.mcpToolCalls[v.ID]
		lc.ItemID = v.ID
		lc.Completed = true
		status := v.Status
		lc.Status = &status
		lc.CompletedItem = cloneMcpToolCallItem(v)
		c.mcpToolCalls[v.ID] = lc
	case *WebSearchThreadItem:
		lc := c.webSearches[v.ID]
		lc.ItemID = v.ID
		lc.Completed = true
		lc.CompletedItem = cloneWebSearchItem(v)
		c.webSearches[v.ID] = lc
	case *FileChangeThreadItem:
		lc := c.fileChanges[v.ID]
		lc.ItemID = v.ID
		lc.Completed = true
		status := v.Status
		lc.Status = &status
		lc.CompletedItem = cloneFileChangeItem(v)
		c.fileChanges[v.ID] = lc
	}
}

func cloneCommandExecutionItem(in *CommandExecutionThreadItem) *CommandExecutionThreadItem {
	if in == nil {
		return nil
	}
	cp := *in
	if in.CommandActions != nil {
		cp.CommandActions = cloneCommandActions(in.CommandActions)
	}
	cp.AggregatedOutput = cloneStringPtr(in.AggregatedOutput)
	cp.DurationMs = cloneInt64Ptr(in.DurationMs)
	cp.ExitCode = cloneInt32Ptr(in.ExitCode)
	cp.ProcessId = cloneStringPtr(in.ProcessId)
	return &cp
}

func cloneMcpToolCallItem(in *McpToolCallThreadItem) *McpToolCallThreadItem {
	if in == nil {
		return nil
	}
	cp := *in
	cp.Arguments = cloneJSONValue(in.Arguments)
	cp.Result = cloneMcpToolCallResult(in.Result)
	cp.Error = cloneMcpToolCallError(in.Error)
	cp.DurationMs = cloneInt64Ptr(in.DurationMs)
	return &cp
}

func cloneWebSearchItem(in *WebSearchThreadItem) *WebSearchThreadItem {
	if in == nil {
		return nil
	}
	cp := *in
	if in.Action != nil {
		action := cloneWebSearchActionWrapper(*in.Action)
		cp.Action = &action
	}
	return &cp
}

func cloneFileChangeItem(in *FileChangeThreadItem) *FileChangeThreadItem {
	if in == nil {
		return nil
	}
	cp := *in
	cp.Changes = cloneFileUpdateChanges(in.Changes)
	return &cp
}

func cloneInt64Ptr(v *int64) *int64 {
	if v == nil {
		return nil
	}
	n := *v
	return &n
}

func cloneInt32Ptr(v *int32) *int32 {
	if v == nil {
		return nil
	}
	n := *v
	return &n
}

func cloneThreadTokenUsage(v *ThreadTokenUsage) *ThreadTokenUsage {
	if v == nil {
		return nil
	}
	cp := *v
	cp.ModelContextWindow = cloneInt64Ptr(v.ModelContextWindow)
	return &cp
}

func cloneNormalizedStreamError(in NormalizedStreamError) NormalizedStreamError {
	in.ThreadID = cloneStringPtr(in.ThreadID)
	in.TurnID = cloneStringPtr(in.TurnID)
	in.SourceMethod = cloneStringPtr(in.SourceMethod)
	in.Raw = append(json.RawMessage(nil), in.Raw...)
	return in
}

func cloneCommandExecutionLifecycle(in CommandExecutionLifecycle, outputChunks []string) CommandExecutionLifecycle {
	cp := in
	cp.Status = cloneCommandExecutionStatusPtr(in.Status)
	cp.StartedItem = cloneCommandExecutionItem(in.StartedItem)
	cp.CompletedItem = cloneCommandExecutionItem(in.CompletedItem)
	cp.OutputDeltas = append([]string(nil), in.OutputDeltas...)
	if len(outputChunks) > 0 {
		cp.AggregatedOutput = strings.Join(outputChunks, "")
	}
	return cp
}

func cloneMcpToolCallLifecycle(in McpToolCallLifecycle) McpToolCallLifecycle {
	cp := in
	cp.Status = cloneMcpToolCallStatusPtr(in.Status)
	cp.StartedItem = cloneMcpToolCallItem(in.StartedItem)
	cp.CompletedItem = cloneMcpToolCallItem(in.CompletedItem)
	return cp
}

func cloneWebSearchLifecycle(in WebSearchLifecycle) WebSearchLifecycle {
	cp := in
	cp.StartedItem = cloneWebSearchItem(in.StartedItem)
	cp.CompletedItem = cloneWebSearchItem(in.CompletedItem)
	return cp
}

func cloneFileChangeLifecycle(in FileChangeLifecycle) FileChangeLifecycle {
	cp := in
	cp.Status = clonePatchApplyStatusPtr(in.Status)
	cp.StartedItem = cloneFileChangeItem(in.StartedItem)
	cp.CompletedItem = cloneFileChangeItem(in.CompletedItem)
	return cp
}

func cloneCommandExecutionStatusPtr(v *CommandExecutionStatus) *CommandExecutionStatus {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneMcpToolCallStatusPtr(v *McpToolCallStatus) *McpToolCallStatus {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func clonePatchApplyStatusPtr(v *PatchApplyStatus) *PatchApplyStatus {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func appendBoundedHistory[T any](history []T, next T, dropped int, limit int) ([]T, int) {
	if limit <= 0 {
		return history, dropped
	}
	if len(history) < limit {
		return append(history, next), dropped
	}
	copy(history, history[1:])
	history[len(history)-1] = next
	return history, dropped + 1
}

func trimBoundedStringHistory(history []string, totalBytes int, maxChunks int, maxBytes int) ([]string, int) {
	if maxChunks > 0 {
		for len(history) > maxChunks {
			totalBytes -= len(history[0])
			history = history[1:]
		}
	}
	if maxBytes > 0 {
		for len(history) > 0 && totalBytes > maxBytes {
			totalBytes -= len(history[0])
			history = history[1:]
		}
	}
	return history, totalBytes
}
