package appserver

import (
	"encoding/json"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const (
	streamCollectorErrorHistoryLimit       = 256
	streamCollectorOutputDeltaHistoryLimit = 512
	streamCollectorOutputDeltaBytesLimit   = 64 * 1024
	streamCollectorPlanTextBytesLimit      = 64 * 1024
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
	ItemID                  string
	ThreadID                string
	TurnID                  string
	Started                 bool
	Completed               bool
	Status                  *CommandExecutionStatus
	StartedItem             *CommandExecutionThreadItem
	CompletedItem           *CommandExecutionThreadItem
	OutputDeltas            []string
	DroppedOutputDeltas     int
	DroppedOutputDeltaBytes int
	AggregatedOutput        string
}

// McpToolCallLifecycle tracks start/completion state for an MCP tool call item.
type McpToolCallLifecycle struct {
	ItemID        string
	ThreadID      string
	TurnID        string
	Started       bool
	Completed     bool
	Status        *McpToolCallStatus
	StartedItem   *McpToolCallThreadItem
	CompletedItem *McpToolCallThreadItem
}

// WebSearchLifecycle tracks start/completion state for a web search item.
type WebSearchLifecycle struct {
	ItemID        string
	ThreadID      string
	TurnID        string
	Started       bool
	Completed     bool
	StartedItem   *WebSearchThreadItem
	CompletedItem *WebSearchThreadItem
}

// FileChangeLifecycle tracks start/completion state for a file-change item.
type FileChangeLifecycle struct {
	ItemID        string
	ThreadID      string
	TurnID        string
	Started       bool
	Completed     bool
	Status        *PatchApplyStatus
	StartedItem   *FileChangeThreadItem
	CompletedItem *FileChangeThreadItem
}

// StreamSummary is a convenience snapshot over a streamed run.
type StreamSummary struct {
	LatestPlanText             *string
	LatestPlanItemID           *string
	LatestTokenUsage           *ThreadTokenUsage
	DroppedLatestPlanTextBytes int

	NormalizedErrors        []NormalizedStreamError
	DroppedNormalizedErrors int

	// Keys use the bare item ID when it is unique in the summary and switch to
	// thread/turn/item scoping only when duplicate item IDs need disambiguation.
	CommandExecutions map[string]CommandExecutionLifecycle
	McpToolCalls      map[string]McpToolCallLifecycle
	WebSearches       map[string]WebSearchLifecycle
	FileChanges       map[string]FileChangeLifecycle
}

// StreamCollector accumulates a convenience summary from streamed events and
// selected notifications. It is safe for concurrent use.
type StreamCollector struct {
	mu sync.Mutex

	latestPlanText             *string
	latestPlanItemID           *string
	latestTokenUsage           *ThreadTokenUsage
	droppedLatestPlanTextBytes int

	normalizedErrors        []NormalizedStreamError
	droppedNormalizedErrors int

	commandExecutions       map[string]CommandExecutionLifecycle
	commandOutputChunks     map[string][]string
	commandOutputDeltaBytes map[string]int
	commandOutputBytes      map[string]int
	mcpToolCalls            map[string]McpToolCallLifecycle
	webSearches             map[string]WebSearchLifecycle
	fileChanges             map[string]FileChangeLifecycle
}

// NewStreamCollector constructs a ready-to-use collector.
func NewStreamCollector() *StreamCollector {
	return &StreamCollector{
		commandExecutions:       make(map[string]CommandExecutionLifecycle),
		commandOutputChunks:     make(map[string][]string),
		commandOutputDeltaBytes: make(map[string]int),
		commandOutputBytes:      make(map[string]int),
		mcpToolCalls:            make(map[string]McpToolCallLifecycle),
		webSearches:             make(map[string]WebSearchLifecycle),
		fileChanges:             make(map[string]FileChangeLifecycle),
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
		c.ingestStartedItemLocked(e.ThreadID, e.TurnID, e.Item.Value)
	case *ItemCompleted:
		c.ingestCompletedItemLocked(e.ThreadID, e.TurnID, e.Item.Value)
	case *TurnCompleted:
		if e.Turn.Error != nil {
			c.appendErrorLocked(NormalizedStreamError{
				Kind:         "turn_error",
				Message:      e.Turn.Error.Message,
				ThreadID:     cloneStringPtr(&e.ThreadID),
				TurnID:       cloneStringPtr(&e.Turn.ID),
				SourceMethod: cloneStringPtr(Ptr(protocol.NotifyTurnCompleted)),
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
		LatestPlanText:             cloneStringPtr(c.latestPlanText),
		LatestPlanItemID:           cloneStringPtr(c.latestPlanItemID),
		DroppedLatestPlanTextBytes: c.droppedLatestPlanTextBytes,
		NormalizedErrors:           make([]NormalizedStreamError, len(c.normalizedErrors)),
		DroppedNormalizedErrors:    c.droppedNormalizedErrors,
		CommandExecutions:          make(map[string]CommandExecutionLifecycle, len(c.commandExecutions)),
		McpToolCalls:               make(map[string]McpToolCallLifecycle, len(c.mcpToolCalls)),
		WebSearches:                make(map[string]WebSearchLifecycle, len(c.webSearches)),
		FileChanges:                make(map[string]FileChangeLifecycle, len(c.fileChanges)),
	}

	if c.latestTokenUsage != nil {
		out.LatestTokenUsage = cloneThreadTokenUsage(c.latestTokenUsage)
	}

	commandExecutionCounts := countLifecycleItemIDs(c.commandExecutions, func(v CommandExecutionLifecycle) string { return v.ItemID })
	mcpToolCallCounts := countLifecycleItemIDs(c.mcpToolCalls, func(v McpToolCallLifecycle) string { return v.ItemID })
	webSearchCounts := countLifecycleItemIDs(c.webSearches, func(v WebSearchLifecycle) string { return v.ItemID })
	fileChangeCounts := countLifecycleItemIDs(c.fileChanges, func(v FileChangeLifecycle) string { return v.ItemID })

	for i, err := range c.normalizedErrors {
		out.NormalizedErrors[i] = cloneNormalizedStreamError(err)
	}
	for k, v := range c.commandExecutions {
		summaryKey := summaryLifecycleKey(v.ThreadID, v.TurnID, v.ItemID, commandExecutionCounts[v.ItemID] > 1)
		out.CommandExecutions[summaryKey] = cloneCommandExecutionLifecycle(
			v,
			c.commandOutputChunks[k],
		)
	}
	for _, v := range c.mcpToolCalls {
		summaryKey := summaryLifecycleKey(v.ThreadID, v.TurnID, v.ItemID, mcpToolCallCounts[v.ItemID] > 1)
		out.McpToolCalls[summaryKey] = cloneMcpToolCallLifecycle(v)
	}
	for _, v := range c.webSearches {
		summaryKey := summaryLifecycleKey(v.ThreadID, v.TurnID, v.ItemID, webSearchCounts[v.ItemID] > 1)
		out.WebSearches[summaryKey] = cloneWebSearchLifecycle(v)
	}
	for _, v := range c.fileChanges {
		summaryKey := summaryLifecycleKey(v.ThreadID, v.TurnID, v.ItemID, fileChangeCounts[v.ItemID] > 1)
		out.FileChanges[summaryKey] = cloneFileChangeLifecycle(v)
	}

	return out
}

func (c *StreamCollector) processCommandExecutionOutputDelta(n CommandExecutionOutputDeltaNotification) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := streamLifecycleKey(n.ThreadID, n.TurnID, n.ItemID)
	lc := c.commandExecutions[key]
	lc.ItemID = n.ItemID
	lc.ThreadID = n.ThreadID
	lc.TurnID = n.TurnID
	historyBytes := c.commandOutputDeltaBytes[key]
	lc.OutputDeltas, historyBytes, lc.DroppedOutputDeltas, lc.DroppedOutputDeltaBytes = appendBoundedStringHistory(
		lc.OutputDeltas,
		historyBytes,
		n.Delta,
		lc.DroppedOutputDeltas,
		lc.DroppedOutputDeltaBytes,
		streamCollectorOutputDeltaHistoryLimit,
		streamCollectorOutputDeltaBytesLimit,
	)
	c.commandOutputDeltaBytes[key] = historyBytes
	chunks := append(c.commandOutputChunks[key], n.Delta)
	bytes := c.commandOutputBytes[key] + len(n.Delta)
	chunks, bytes = trimBoundedStringHistory(
		chunks,
		bytes,
		streamCollectorRawOutputChunkLimit,
		streamCollectorRawOutputBytesLimit,
	)
	c.commandOutputChunks[key] = chunks
	c.commandOutputBytes[key] = bytes
	c.commandExecutions[key] = lc
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
		SourceMethod: cloneStringPtr(Ptr(protocol.NotifyError)),
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
		SourceMethod: cloneStringPtr(Ptr(protocol.NotifyRealtimeError)),
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
		c.setLatestPlanTextLocked(p.ItemID, p.Delta)
		return
	}
	if c.latestPlanText == nil {
		c.setLatestPlanTextLocked(p.ItemID, p.Delta)
		return
	}
	c.latestPlanItemID = Ptr(p.ItemID)
	combined, droppedBytes := appendBoundedStringSuffix(*c.latestPlanText, p.Delta, streamCollectorPlanTextBytesLimit)
	c.latestPlanText = Ptr(combined)
	c.droppedLatestPlanTextBytes += droppedBytes
}

func (c *StreamCollector) ingestStartedItemLocked(threadID string, turnID string, item ThreadItem) {
	switch v := item.(type) {
	case *CommandExecutionThreadItem:
		updateLifecycleStateLocked(c.commandExecutions, threadID, turnID, v.ID, func(_ string, lc *CommandExecutionLifecycle) {
			setLifecycleScope(&lc.ItemID, &lc.ThreadID, &lc.TurnID, v.ID, threadID, turnID)
			lc.Started = true
			lc.Status = Ptr(v.Status)
			lc.StartedItem = cloneCommandExecutionItem(v)
		})
	case *McpToolCallThreadItem:
		updateLifecycleStateLocked(c.mcpToolCalls, threadID, turnID, v.ID, func(_ string, lc *McpToolCallLifecycle) {
			setLifecycleScope(&lc.ItemID, &lc.ThreadID, &lc.TurnID, v.ID, threadID, turnID)
			lc.Started = true
			lc.Status = Ptr(v.Status)
			lc.StartedItem = cloneMcpToolCallItem(v)
		})
	case *WebSearchThreadItem:
		updateLifecycleStateLocked(c.webSearches, threadID, turnID, v.ID, func(_ string, lc *WebSearchLifecycle) {
			setLifecycleScope(&lc.ItemID, &lc.ThreadID, &lc.TurnID, v.ID, threadID, turnID)
			lc.Started = true
			lc.StartedItem = cloneWebSearchItem(v)
		})
	case *FileChangeThreadItem:
		updateLifecycleStateLocked(c.fileChanges, threadID, turnID, v.ID, func(_ string, lc *FileChangeLifecycle) {
			setLifecycleScope(&lc.ItemID, &lc.ThreadID, &lc.TurnID, v.ID, threadID, turnID)
			lc.Started = true
			lc.Status = Ptr(v.Status)
			lc.StartedItem = cloneFileChangeItem(v)
		})
	}
}

func (c *StreamCollector) ingestCompletedItemLocked(threadID string, turnID string, item ThreadItem) {
	switch v := item.(type) {
	case *PlanThreadItem:
		c.setLatestPlanTextLocked(v.ID, v.Text)
	case *CommandExecutionThreadItem:
		key := updateLifecycleStateLocked(c.commandExecutions, threadID, turnID, v.ID, func(key string, lc *CommandExecutionLifecycle) {
			setLifecycleScope(&lc.ItemID, &lc.ThreadID, &lc.TurnID, v.ID, threadID, turnID)
			lc.Completed = true
			lc.Status = Ptr(v.Status)
			lc.CompletedItem = cloneCommandExecutionItem(v)
			if v.AggregatedOutput != nil {
				lc.AggregatedOutput = *v.AggregatedOutput
			} else if chunks := c.commandOutputChunks[key]; len(chunks) > 0 {
				lc.AggregatedOutput = strings.Join(chunks, "")
			}
		})
		delete(c.commandOutputChunks, key)
		delete(c.commandOutputDeltaBytes, key)
		delete(c.commandOutputBytes, key)
	case *McpToolCallThreadItem:
		updateLifecycleStateLocked(c.mcpToolCalls, threadID, turnID, v.ID, func(_ string, lc *McpToolCallLifecycle) {
			setLifecycleScope(&lc.ItemID, &lc.ThreadID, &lc.TurnID, v.ID, threadID, turnID)
			lc.Completed = true
			lc.Status = Ptr(v.Status)
			lc.CompletedItem = cloneMcpToolCallItem(v)
		})
	case *WebSearchThreadItem:
		updateLifecycleStateLocked(c.webSearches, threadID, turnID, v.ID, func(_ string, lc *WebSearchLifecycle) {
			setLifecycleScope(&lc.ItemID, &lc.ThreadID, &lc.TurnID, v.ID, threadID, turnID)
			lc.Completed = true
			lc.CompletedItem = cloneWebSearchItem(v)
		})
	case *FileChangeThreadItem:
		updateLifecycleStateLocked(c.fileChanges, threadID, turnID, v.ID, func(_ string, lc *FileChangeLifecycle) {
			setLifecycleScope(&lc.ItemID, &lc.ThreadID, &lc.TurnID, v.ID, threadID, turnID)
			lc.Completed = true
			lc.Status = Ptr(v.Status)
			lc.CompletedItem = cloneFileChangeItem(v)
		})
	}
}

func updateLifecycleStateLocked[T any](states map[string]T, threadID string, turnID string, itemID string, update func(string, *T)) string {
	key := streamLifecycleKey(threadID, turnID, itemID)
	state := states[key]
	update(key, &state)
	states[key] = state
	return key
}

func setLifecycleScope(targetItemID *string, targetThreadID *string, targetTurnID *string, itemID string, threadID string, turnID string) {
	*targetItemID = itemID
	*targetThreadID = threadID
	*targetTurnID = turnID
}

func streamLifecycleKey(threadID string, turnID string, itemID string) string {
	if threadID == "" && turnID == "" {
		return itemID
	}
	return threadID + "\x1f" + turnID + "\x1f" + itemID
}

func summaryLifecycleKey(threadID string, turnID string, itemID string, duplicate bool) string {
	if !duplicate || (threadID == "" && turnID == "") {
		return itemID
	}
	return threadID + "/" + turnID + "/" + itemID
}

func countLifecycleItemIDs[T any](states map[string]T, itemID func(T) string) map[string]int {
	counts := make(map[string]int, len(states))
	for _, state := range states {
		counts[itemID(state)]++
	}
	return counts
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
	cp.DurationMs = clonePtr(in.DurationMs)
	cp.ExitCode = clonePtr(in.ExitCode)
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
	cp.DurationMs = clonePtr(in.DurationMs)
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

func cloneThreadTokenUsage(v *ThreadTokenUsage) *ThreadTokenUsage {
	if v == nil {
		return nil
	}
	cp := *v
	cp.ModelContextWindow = clonePtr(v.ModelContextWindow)
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
	cp.Status = clonePtr(in.Status)
	cp.StartedItem = cloneCommandExecutionItem(in.StartedItem)
	cp.CompletedItem = cloneCommandExecutionItem(in.CompletedItem)
	cp.OutputDeltas = append([]string(nil), in.OutputDeltas...)
	if len(outputChunks) > 0 {
		cp.AggregatedOutput = strings.Join(outputChunks, "")
	}
	return cp
}

func (c *StreamCollector) setLatestPlanTextLocked(itemID string, text string) {
	c.latestPlanItemID = Ptr(itemID)
	retained, droppedBytes := retainSuffixWithinByteLimit(text, streamCollectorPlanTextBytesLimit)
	c.latestPlanText = Ptr(retained)
	c.droppedLatestPlanTextBytes = droppedBytes
}

func cloneMcpToolCallLifecycle(in McpToolCallLifecycle) McpToolCallLifecycle {
	cp := in
	cp.Status = clonePtr(in.Status)
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
	cp.Status = clonePtr(in.Status)
	cp.StartedItem = cloneFileChangeItem(in.StartedItem)
	cp.CompletedItem = cloneFileChangeItem(in.CompletedItem)
	return cp
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

func appendBoundedStringHistory(history []string, historyBytes int, next string, droppedEntries int, droppedBytes int, maxEntries int, maxBytes int) ([]string, int, int, int) {
	if maxEntries <= 0 || maxBytes <= 0 {
		return history, 0, droppedEntries + 1, droppedBytes + len(next)
	}

	retainedNext, trimmedBytes := retainSuffixWithinByteLimit(next, maxBytes)
	droppedBytes += trimmedBytes

	history = append(history, retainedNext)
	historyBytes += len(retainedNext)

	for len(history) > maxEntries {
		droppedBytes += len(history[0])
		historyBytes -= len(history[0])
		history = history[1:]
		droppedEntries++
	}

	for len(history) > 0 && historyBytes > maxBytes {
		if len(history) == 1 {
			trimmed, trimmedFromEntry := retainSuffixWithinByteLimit(history[0], maxBytes)
			history[0] = trimmed
			historyBytes = len(trimmed)
			droppedBytes += trimmedFromEntry
			break
		}

		droppedBytes += len(history[0])
		historyBytes -= len(history[0])
		history = history[1:]
		droppedEntries++
	}

	return history, historyBytes, droppedEntries, droppedBytes
}

func appendBoundedStringSuffix(existing string, next string, maxBytes int) (string, int) {
	return retainSuffixWithinByteLimit(existing+next, maxBytes)
}

func retainSuffixWithinByteLimit(text string, maxBytes int) (string, int) {
	if maxBytes <= 0 {
		return "", len(text)
	}
	if len(text) <= maxBytes {
		return text, 0
	}

	start := len(text) - maxBytes
	for start < len(text) && !utf8.RuneStart(text[start]) {
		start++
	}
	if start >= len(text) {
		return "", len(text)
	}
	return text[start:], start
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
