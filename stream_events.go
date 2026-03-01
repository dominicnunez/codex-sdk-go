package codex

// Event is a sealed interface for streaming events yielded by Stream.Events.
// Concrete types: TurnStarted, TextDelta, ReasoningDelta, ReasoningSummaryDelta,
// PlanDelta, FileChangeDelta, ItemStarted, ItemCompleted, CollabToolCallEvent, TurnCompleted.
type Event interface {
	streamEvent()
}

// TurnStarted is emitted when a turn begins.
type TurnStarted struct {
	Turn     Turn
	ThreadID string
}

func (*TurnStarted) streamEvent() {}

// TextDelta is emitted for incremental agent message text.
type TextDelta struct {
	Delta  string
	ItemID string
}

func (*TextDelta) streamEvent() {}

// ReasoningDelta is emitted for incremental reasoning text.
type ReasoningDelta struct {
	Delta        string
	ItemID       string
	ContentIndex int64
}

func (*ReasoningDelta) streamEvent() {}

// ReasoningSummaryDelta is emitted for incremental reasoning summary text.
type ReasoningSummaryDelta struct {
	Delta        string
	ItemID       string
	SummaryIndex int64
}

func (*ReasoningSummaryDelta) streamEvent() {}

// PlanDelta is emitted for incremental plan text.
type PlanDelta struct {
	Delta  string
	ItemID string
}

func (*PlanDelta) streamEvent() {}

// FileChangeDelta is emitted for incremental file change diff text.
type FileChangeDelta struct {
	Delta  string
	ItemID string
}

func (*FileChangeDelta) streamEvent() {}

// ItemStarted is emitted when a thread item begins.
type ItemStarted struct {
	Item ThreadItemWrapper
}

func (*ItemStarted) streamEvent() {}

// ItemCompleted is emitted when a thread item finishes.
type ItemCompleted struct {
	Item ThreadItemWrapper
}

func (*ItemCompleted) streamEvent() {}

// CollabToolCallPhase distinguishes started from completed collab events.
type CollabToolCallPhase string

const (
	CollabToolCallStartedPhase   CollabToolCallPhase = "started"
	CollabToolCallCompletedPhase CollabToolCallPhase = "completed"
)

// CollabToolCallEvent is emitted when a collab tool call begins or finishes
// (item/started or item/completed with a collabAgentToolCall item).
// Emitted before the generic ItemStarted/ItemCompleted event.
type CollabToolCallEvent struct {
	Phase             CollabToolCallPhase
	ID                string
	Tool              CollabAgentTool
	Status            CollabAgentToolCallStatus
	AgentsStates      map[string]CollabAgentState
	ReceiverThreadIds []string
	SenderThreadId    string
	Prompt            *string
}

func (*CollabToolCallEvent) streamEvent() {}

func newCollabEvent(phase CollabToolCallPhase, c *CollabAgentToolCallThreadItem) *CollabToolCallEvent {
	states := make(map[string]CollabAgentState, len(c.AgentsStates))
	for k, v := range c.AgentsStates {
		states[k] = v
	}
	ids := make([]string, len(c.ReceiverThreadIds))
	copy(ids, c.ReceiverThreadIds)

	return &CollabToolCallEvent{
		Phase:             phase,
		ID:                c.ID,
		Tool:              c.Tool,
		Status:            c.Status,
		AgentsStates:      states,
		ReceiverThreadIds: ids,
		SenderThreadId:    c.SenderThreadId,
		Prompt:            cloneStringPtr(c.Prompt),
	}
}

// TurnCompleted is emitted when the turn finishes.
type TurnCompleted struct {
	Turn Turn
}

func (*TurnCompleted) streamEvent() {}
