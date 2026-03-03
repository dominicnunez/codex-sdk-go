package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// threadIDCarrier extracts the threadId from raw notification JSON for filtering.
type threadIDCarrier struct {
	ThreadID string `json:"threadId"`
}

// turnLifecycleParams configures a shared turn execution.
//
// Ordering assumption: notification listeners are registered before the
// Turn.Start RPC is sent. The server writes the RPC response before any
// turn-related notifications on the same stdio writer, so listeners are
// guaranteed to be in place before the first notification arrives. If the
// transport is ever replaced with one that multiplexes responses and
// notifications on separate channels, this ordering must be preserved.
type turnLifecycleParams struct {
	client     *Client
	turnParams TurnStartParams
	thread     Thread
	threadID   string
	onComplete func(Turn) // called on successful turn completion; nil = no-op
	collector  *StreamCollector
}

func isTerminalTurnStatus(status TurnStatus) bool {
	switch status {
	case TurnStatusCompleted, TurnStatusInterrupted, TurnStatusFailed:
		return true
	default:
		return false
	}
}

func invalidTurnCompletedNotification(err error) TurnCompletedNotification {
	return TurnCompletedNotification{
		Turn: Turn{
			Status: TurnStatusFailed,
			Error:  &TurnError{Message: "invalid turn/completed notification: " + err.Error()},
		},
	}
}

func validateTurnCompletedNotification(n TurnCompletedNotification) error {
	if n.ThreadID == "" {
		return fmt.Errorf("threadId is required")
	}
	if n.Turn.ID == "" {
		return fmt.Errorf("turn.id is required")
	}
	if !isTerminalTurnStatus(n.Turn.Status) {
		return fmt.Errorf("turn.status must be terminal, got %q", n.Turn.Status)
	}
	return nil
}

// executeTurn runs a blocking turn: registers listeners, starts the turn,
// collects items, and waits for completion or context cancellation.
// Listeners are filtered by threadID to avoid cross-contamination.
func executeTurn(ctx context.Context, p turnLifecycleParams) (*RunResult, error) {
	var (
		items []ThreadItemWrapper
		mu    sync.Mutex
		done  = make(chan TurnCompletedNotification, 1)
	)

	unsubItem := p.client.addNotificationListener(notifyItemCompleted, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			p.client.reportHandlerError(notifyItemCompleted, fmt.Errorf("unmarshal %s: %w", notifyItemCompleted, err))
			n.Item = ThreadItemWrapper{Value: &UnknownThreadItem{
				Type: UnmarshalErrorItemType,
				Raw:  append(json.RawMessage(nil), notif.Params...),
			}}
		}
		mu.Lock()
		items = append(items, n.Item)
		mu.Unlock()
	})

	unsubTurn := p.client.addNotificationListener(notifyTurnCompleted, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil {
			return
		}
		if carrier.ThreadID == "" {
			select {
			case done <- invalidTurnCompletedNotification(fmt.Errorf("threadId is required")):
			default:
			}
			return
		}
		if carrier.ThreadID != p.threadID {
			return
		}
		var n TurnCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			p.client.reportHandlerError(notifyTurnCompleted, fmt.Errorf("unmarshal %s: %w", notifyTurnCompleted, err))
			n = TurnCompletedNotification{
				Turn: Turn{Error: &TurnError{Message: "failed to unmarshal turn/completed: " + err.Error()}},
			}
		} else if err := validateTurnCompletedNotification(n); err != nil {
			p.client.reportHandlerError(notifyTurnCompleted, fmt.Errorf("validate %s: %w", notifyTurnCompleted, err))
			n = invalidTurnCompletedNotification(err)
		}
		select {
		case done <- n:
		default:
		}
	})

	defer unsubItem()
	defer unsubTurn()

	if _, err := p.client.Turn.Start(ctx, p.turnParams); err != nil {
		return nil, fmt.Errorf("turn/start: %w", err)
	}

	select {
	case completed := <-done:
		if completed.Turn.Error != nil {
			return nil, fmt.Errorf("turn error: %w", completed.Turn.Error)
		}

		mu.Lock()
		collectedItems := make([]ThreadItemWrapper, len(items))
		copy(collectedItems, items)
		mu.Unlock()

		if p.onComplete != nil {
			p.onComplete(completed.Turn)
		}

		return buildRunResult(p.thread, completed.Turn, collectedItems), nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// executeStreamedTurn runs the streaming lifecycle: registers filtered listeners,
// starts the turn, and sends events on ch until completion or context cancellation.
func executeStreamedTurn(ctx context.Context, p turnLifecycleParams, g *guardedChan, s *Stream) {
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
		unsub := p.client.addNotificationListener(method, handler)
		unsubFuncs = append(unsubFuncs, unsub)
	}
	emit := func(event Event) {
		if p.collector != nil {
			p.collector.Process(event, nil)
		}
		streamSendEvent(g, event)
	}
	emitErr := func(err error) {
		if p.collector != nil {
			p.collector.Process(nil, err)
		}
		streamSendErr(g, err)
	}
	onEvent := func(event Event) {
		if p.collector != nil {
			p.collector.Process(event, nil)
		}
	}

	turnDone := make(chan TurnCompletedNotification, 1)

	registerStreamDeltaListeners(p, g, on, onEvent)
	registerItemListeners(ctx, p, on, emit, &items, &itemsMu)
	registerTurnCompletedListener(p, on, turnDone)
	registerCollectorListeners(p, on)

	if _, err := p.client.Turn.Start(ctx, p.turnParams); err != nil {
		emitErr(fmt.Errorf("turn/start: %w", err))
		return
	}

	// Wait for turn completion or context cancellation.
	select {
	case completed := <-turnDone:
		emit(&TurnCompleted{Turn: completed.Turn})

		if completed.Turn.Error != nil {
			emitErr(fmt.Errorf("turn error: %w", completed.Turn.Error))
			return
		}

		itemsMu.Lock()
		collectedItems := make([]ThreadItemWrapper, len(items))
		copy(collectedItems, items)
		itemsMu.Unlock()

		if p.onComplete != nil {
			p.onComplete(completed.Turn)
		}

		s.mu.Lock()
		s.result = buildRunResult(p.thread, completed.Turn, collectedItems)
		s.mu.Unlock()

	case <-ctx.Done():
		emitErr(ctx.Err())
	}
}

func registerStreamDeltaListeners(p turnLifecycleParams, g *guardedChan, on func(string, NotificationHandler), onEvent func(Event)) {
	streamListen(on, notifyTurnStarted, g, p.threadID, p.client.reportHandlerError, onEvent, func(n TurnStartedNotification) string {
		return n.ThreadID
	}, func(n TurnStartedNotification) Event {
		return &TurnStarted{Turn: n.Turn, ThreadID: n.ThreadID}
	})
	streamListen(on, notifyAgentMessageDelta, g, p.threadID, p.client.reportHandlerError, onEvent, func(n AgentMessageDeltaNotification) string {
		return n.ThreadID
	}, func(n AgentMessageDeltaNotification) Event {
		return &TextDelta{Delta: n.Delta, ItemID: n.ItemID}
	})
	streamListen(on, notifyReasoningTextDelta, g, p.threadID, p.client.reportHandlerError, onEvent, func(n ReasoningTextDeltaNotification) string {
		return n.ThreadID
	}, func(n ReasoningTextDeltaNotification) Event {
		return &ReasoningDelta{Delta: n.Delta, ItemID: n.ItemID, ContentIndex: n.ContentIndex}
	})
	streamListen(on, notifyReasoningSummaryTextDelta, g, p.threadID, p.client.reportHandlerError, onEvent, func(n ReasoningSummaryTextDeltaNotification) string {
		return n.ThreadID
	}, func(n ReasoningSummaryTextDeltaNotification) Event {
		return &ReasoningSummaryDelta{Delta: n.Delta, ItemID: n.ItemID, SummaryIndex: n.SummaryIndex}
	})
	streamListen(on, notifyPlanDelta, g, p.threadID, p.client.reportHandlerError, onEvent, func(n PlanDeltaNotification) string {
		return n.ThreadID
	}, func(n PlanDeltaNotification) Event {
		return &PlanDelta{Delta: n.Delta, ItemID: n.ItemID}
	})
	streamListen(on, notifyFileChangeOutputDelta, g, p.threadID, p.client.reportHandlerError, onEvent, func(n FileChangeOutputDeltaNotification) string {
		return n.ThreadID
	}, func(n FileChangeOutputDeltaNotification) Event {
		return &FileChangeDelta{Delta: n.Delta, ItemID: n.ItemID}
	})
}

func registerItemListeners(ctx context.Context, p turnLifecycleParams, on func(string, NotificationHandler), emit func(Event), items *[]ThreadItemWrapper, itemsMu *sync.Mutex) {
	on(notifyItemStarted, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n ItemStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			p.client.reportHandlerError(notifyItemStarted, fmt.Errorf("unmarshal %s: %w", notifyItemStarted, err))
			return
		}
		if c, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
			emit(newCollabEvent(CollabToolCallStartedPhase, c))
		}
		emit(&ItemStarted{Item: n.Item})
	})

	on(notifyItemCompleted, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			p.client.reportHandlerError(notifyItemCompleted, fmt.Errorf("unmarshal %s: %w", notifyItemCompleted, err))
			n.Item = ThreadItemWrapper{Value: &UnknownThreadItem{
				Type: UnmarshalErrorItemType,
				Raw:  append(json.RawMessage(nil), notif.Params...),
			}}
		}
		itemsMu.Lock()
		*items = append(*items, n.Item)
		itemsMu.Unlock()
		if c, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
			emit(newCollabEvent(CollabToolCallCompletedPhase, c))
		}
		emit(&ItemCompleted{Item: n.Item})
	})

	_ = ctx // keeps signature consistent for future listener additions.
}

func registerTurnCompletedListener(p turnLifecycleParams, on func(string, NotificationHandler), turnDone chan<- TurnCompletedNotification) {
	on(notifyTurnCompleted, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil {
			return
		}
		if carrier.ThreadID == "" {
			select {
			case turnDone <- invalidTurnCompletedNotification(fmt.Errorf("threadId is required")):
			default:
			}
			return
		}
		if carrier.ThreadID != p.threadID {
			return
		}
		var n TurnCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			p.client.reportHandlerError(notifyTurnCompleted, fmt.Errorf("unmarshal %s: %w", notifyTurnCompleted, err))
			n = TurnCompletedNotification{
				Turn: Turn{Error: &TurnError{Message: "failed to unmarshal turn/completed: " + err.Error()}},
			}
		} else if err := validateTurnCompletedNotification(n); err != nil {
			p.client.reportHandlerError(notifyTurnCompleted, fmt.Errorf("validate %s: %w", notifyTurnCompleted, err))
			n = invalidTurnCompletedNotification(err)
		}
		select {
		case turnDone <- n:
		default:
		}
	})
}

func registerCollectorListeners(p turnLifecycleParams, on func(string, NotificationHandler)) {
	if p.collector == nil {
		return
	}

	on(notifyCommandExecutionOutputDelta, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n CommandExecutionOutputDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			p.client.reportHandlerError(notifyCommandExecutionOutputDelta, fmt.Errorf("unmarshal %s: %w", notifyCommandExecutionOutputDelta, err))
			return
		}
		p.collector.processCommandExecutionOutputDelta(n)
	})

	on(notifyThreadTokenUsageUpdated, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n ThreadTokenUsageUpdatedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			p.client.reportHandlerError(notifyThreadTokenUsageUpdated, fmt.Errorf("unmarshal %s: %w", notifyThreadTokenUsageUpdated, err))
			return
		}
		p.collector.processThreadTokenUsageUpdated(n)
	})

	on(notifyError, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n ErrorNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			p.client.reportHandlerError(notifyError, fmt.Errorf("unmarshal %s: %w", notifyError, err))
			return
		}
		p.collector.processSystemError(n)
	})

	on(notifyRealtimeError, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n ThreadRealtimeErrorNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			p.client.reportHandlerError(notifyRealtimeError, fmt.Errorf("unmarshal %s: %w", notifyRealtimeError, err))
			return
		}
		p.collector.processThreadRealtimeError(n)
	})
}
