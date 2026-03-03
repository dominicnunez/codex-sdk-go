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

func unmarshalThreadIDCarrier(params json.RawMessage) (threadIDCarrier, bool) {
	var carrier threadIDCarrier
	if err := json.Unmarshal(params, &carrier); err != nil {
		return threadIDCarrier{}, false
	}
	return carrier, true
}

func parseItemCompletedForThread(params json.RawMessage, threadID string) (ItemCompletedNotification, bool, error) {
	var n ItemCompletedNotification
	if err := json.Unmarshal(params, &n); err != nil {
		carrier, ok := unmarshalThreadIDCarrier(params)
		if !ok || carrier.ThreadID != threadID {
			return ItemCompletedNotification{}, false, nil
		}
		n.Item = ThreadItemWrapper{Value: &UnknownThreadItem{
			Type: UnmarshalErrorItemType,
			Raw:  append(json.RawMessage(nil), params...),
		}}
		return n, true, err
	}
	if n.ThreadID != threadID {
		return ItemCompletedNotification{}, false, nil
	}
	return n, true, nil
}

func parseTurnCompletedForThread(params json.RawMessage, threadID string) (turnCompletionCandidate, bool, error) {
	var n TurnCompletedNotification
	if err := json.Unmarshal(params, &n); err != nil {
		carrier, ok := unmarshalThreadIDCarrier(params)
		if !ok || carrier.ThreadID != threadID {
			return turnCompletionCandidate{}, false, nil
		}
		return turnCompletionCandidate{
			notification: TurnCompletedNotification{
				Turn: Turn{Error: &TurnError{Message: "failed to unmarshal turn/completed: " + err.Error()}},
			},
			allowMissingTurnID: true,
		}, true, err
	}
	if n.ThreadID == "" {
		return turnCompletionCandidate{
			notification:       invalidTurnCompletedNotification(fmt.Errorf("threadId is required")),
			allowMissingTurnID: true,
		}, true, nil
	}
	if n.ThreadID != threadID {
		return turnCompletionCandidate{}, false, nil
	}
	if err := validateTurnCompletedNotification(n); err != nil {
		return turnCompletionCandidate{
			notification:       invalidTurnCompletedNotification(err),
			allowMissingTurnID: true,
		}, true, err
	}
	return turnCompletionCandidate{notification: n}, true, nil
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

type turnCompletionCandidate struct {
	notification       TurnCompletedNotification
	allowMissingTurnID bool
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

func matchesActiveTurn(activeTurnID string, candidate turnCompletionCandidate) bool {
	n := candidate.notification
	if n.Turn.ID == activeTurnID {
		return true
	}
	return candidate.allowMissingTurnID && n.Turn.ID == ""
}

type blockingTurnState struct {
	mu                 sync.Mutex
	ready              bool
	turnID             string
	pendingItems       []ItemCompletedNotification
	pendingCompletions []turnCompletionCandidate
}

func (s *blockingTurnState) queueItem(n ItemCompletedNotification) (ThreadItemWrapper, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		s.pendingItems = append(s.pendingItems, n)
		return ThreadItemWrapper{}, false
	}
	if n.TurnID != s.turnID {
		return ThreadItemWrapper{}, false
	}
	return n.Item, true
}

func (s *blockingTurnState) queueCompletion(n turnCompletionCandidate) (TurnCompletedNotification, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		s.pendingCompletions = append(s.pendingCompletions, n)
		return TurnCompletedNotification{}, false
	}
	if !matchesActiveTurn(s.turnID, n) {
		return TurnCompletedNotification{}, false
	}
	return n.notification, true
}

func (s *blockingTurnState) start(turnID string) ([]ItemCompletedNotification, []turnCompletionCandidate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ready = true
	s.turnID = turnID
	items := s.pendingItems
	completions := s.pendingCompletions
	s.pendingItems = nil
	s.pendingCompletions = nil
	return items, completions
}

func waitForTurnCompletion(ctx context.Context, done <-chan TurnCompletedNotification) (TurnCompletedNotification, error) {
	select {
	case completed := <-done:
		return completed, nil
	case <-ctx.Done():
		return TurnCompletedNotification{}, ctx.Err()
	}
}

// executeTurn runs a blocking turn: registers listeners, starts the turn,
// collects items, and waits for completion or context cancellation.
// Listeners are filtered by threadID and active turnID to avoid cross-turn contamination.
func executeTurn(ctx context.Context, p turnLifecycleParams) (*RunResult, error) {
	var (
		items    []ThreadItemWrapper
		itemsMu  sync.Mutex
		state    blockingTurnState
		done     = make(chan TurnCompletedNotification, 1)
		sendDone = func(n TurnCompletedNotification) {
			select {
			case done <- n:
			default:
			}
		}
		appendItem = func(item ThreadItemWrapper) {
			itemsMu.Lock()
			items = append(items, item)
			itemsMu.Unlock()
		}
	)

	unsubItem := p.client.addNotificationListener(notifyItemCompleted, func(_ context.Context, notif Notification) {
		n, ok, err := parseItemCompletedForThread(notif.Params, p.threadID)
		if !ok {
			return
		}
		if err != nil {
			p.client.reportHandlerError(notifyItemCompleted, fmt.Errorf("unmarshal %s: %w", notifyItemCompleted, err))
		}
		item, ok := state.queueItem(n)
		if !ok {
			return
		}
		appendItem(item)
	})

	unsubTurn := p.client.addNotificationListener(notifyTurnCompleted, func(_ context.Context, notif Notification) {
		candidate, ok, err := parseTurnCompletedForThread(notif.Params, p.threadID)
		if !ok {
			return
		}
		if err != nil {
			if candidate.notification.Turn.Error != nil {
				p.client.reportHandlerError(notifyTurnCompleted, fmt.Errorf("unmarshal %s: %w", notifyTurnCompleted, err))
			} else {
				p.client.reportHandlerError(notifyTurnCompleted, fmt.Errorf("validate %s: %w", notifyTurnCompleted, err))
			}
		}
		completed, ok := state.queueCompletion(candidate)
		if !ok {
			return
		}
		sendDone(completed)
	})

	defer unsubItem()
	defer unsubTurn()

	startResp, err := p.client.Turn.Start(ctx, p.turnParams)
	if err != nil {
		return nil, fmt.Errorf("turn/start: %w", err)
	}
	if startResp.Turn.ID == "" {
		return nil, fmt.Errorf("turn/start: missing turn.id")
	}

	bufferedItems, bufferedCompletions := state.start(startResp.Turn.ID)

	for _, n := range bufferedItems {
		if n.TurnID == startResp.Turn.ID {
			appendItem(n.Item)
		}
	}
	for _, n := range bufferedCompletions {
		if !matchesActiveTurn(startResp.Turn.ID, n) {
			continue
		}
		sendDone(n.notification)
	}

	completed, err := waitForTurnCompletion(ctx, done)
	if err != nil {
		return nil, err
	}
	if completed.Turn.Error != nil {
		return nil, fmt.Errorf("turn error: %w", completed.Turn.Error)
	}

	itemsMu.Lock()
	collectedItems := make([]ThreadItemWrapper, len(items))
	copy(collectedItems, items)
	itemsMu.Unlock()

	if p.onComplete != nil {
		p.onComplete(completed.Turn)
	}

	return buildRunResult(p.thread, completed.Turn, collectedItems), nil
}

// executeStreamedTurn runs the streaming lifecycle: registers filtered listeners,
// starts the turn, and sends events on ch until completion or context cancellation.
func executeStreamedTurn(ctx context.Context, p turnLifecycleParams, g *guardedChan, s *Stream) {
	var (
		items                 []ThreadItemWrapper
		itemsMu               sync.Mutex
		turnStateMu           sync.Mutex
		turnReady             bool
		startedTurnID         string
		pendingTurnScoped     []func(string)
		pendingTurnCompletion []turnCompletionCandidate
		unsubFuncs            []func()
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

	dispatchTurnScoped := func(turnID string, fn func()) {
		turnStateMu.Lock()
		if !turnReady {
			capturedTurnID := turnID
			pendingTurnScoped = append(pendingTurnScoped, func(activeTurnID string) {
				if capturedTurnID == activeTurnID {
					fn()
				}
			})
			turnStateMu.Unlock()
			return
		}
		activeTurnID := startedTurnID
		turnStateMu.Unlock()

		if turnID != activeTurnID {
			return
		}
		fn()
	}

	queueTurnCompletionCandidate := func(n turnCompletionCandidate) {
		turnStateMu.Lock()
		if !turnReady {
			pendingTurnCompletion = append(pendingTurnCompletion, n)
			turnStateMu.Unlock()
			return
		}
		activeTurnID := startedTurnID
		turnStateMu.Unlock()

		if !matchesActiveTurn(activeTurnID, n) {
			return
		}
		select {
		case turnDone <- n.notification:
		default:
		}
	}

	registerStreamDeltaListeners(p, g, on, onEvent, dispatchTurnScoped)
	registerItemListeners(ctx, p, on, emit, &items, &itemsMu, dispatchTurnScoped)
	registerTurnCompletedListener(p, on, queueTurnCompletionCandidate)
	registerCollectorListeners(p, on, dispatchTurnScoped)

	startResp, err := p.client.Turn.Start(ctx, p.turnParams)
	if err != nil {
		emitErr(fmt.Errorf("turn/start: %w", err))
		return
	}
	if startResp.Turn.ID == "" {
		emitErr(fmt.Errorf("turn/start: missing turn.id"))
		return
	}

	turnStateMu.Lock()
	turnReady = true
	startedTurnID = startResp.Turn.ID
	pendingEvents := pendingTurnScoped
	pendingCompletions := pendingTurnCompletion
	pendingTurnScoped = nil
	pendingTurnCompletion = nil
	turnStateMu.Unlock()

	for _, pending := range pendingEvents {
		pending(startedTurnID)
	}
	for _, n := range pendingCompletions {
		if !matchesActiveTurn(startedTurnID, n) {
			continue
		}
		select {
		case turnDone <- n.notification:
		default:
		}
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

func registerStreamDeltaListeners(p turnLifecycleParams, g *guardedChan, on func(string, NotificationHandler), onEvent func(Event), dispatchTurnScoped func(string, func())) {
	streamListen(on, notifyTurnStarted, g, p.threadID, p.client.reportHandlerError, onEvent, func(n TurnStartedNotification) string {
		return n.ThreadID
	}, func(n TurnStartedNotification) Event {
		return &TurnStarted{Turn: n.Turn, ThreadID: n.ThreadID}
	})
	streamListenTurnScoped(on, notifyAgentMessageDelta, g, p.threadID, p.client.reportHandlerError, onEvent, dispatchTurnScoped, func(n AgentMessageDeltaNotification) string {
		return n.ThreadID
	}, func(n AgentMessageDeltaNotification) string {
		return n.TurnID
	}, func(n AgentMessageDeltaNotification) Event {
		return &TextDelta{Delta: n.Delta, ItemID: n.ItemID}
	})
	streamListenTurnScoped(on, notifyReasoningTextDelta, g, p.threadID, p.client.reportHandlerError, onEvent, dispatchTurnScoped, func(n ReasoningTextDeltaNotification) string {
		return n.ThreadID
	}, func(n ReasoningTextDeltaNotification) string {
		return n.TurnID
	}, func(n ReasoningTextDeltaNotification) Event {
		return &ReasoningDelta{Delta: n.Delta, ItemID: n.ItemID, ContentIndex: n.ContentIndex}
	})
	streamListenTurnScoped(on, notifyReasoningSummaryTextDelta, g, p.threadID, p.client.reportHandlerError, onEvent, dispatchTurnScoped, func(n ReasoningSummaryTextDeltaNotification) string {
		return n.ThreadID
	}, func(n ReasoningSummaryTextDeltaNotification) string {
		return n.TurnID
	}, func(n ReasoningSummaryTextDeltaNotification) Event {
		return &ReasoningSummaryDelta{Delta: n.Delta, ItemID: n.ItemID, SummaryIndex: n.SummaryIndex}
	})
	streamListenTurnScoped(on, notifyPlanDelta, g, p.threadID, p.client.reportHandlerError, onEvent, dispatchTurnScoped, func(n PlanDeltaNotification) string {
		return n.ThreadID
	}, func(n PlanDeltaNotification) string {
		return n.TurnID
	}, func(n PlanDeltaNotification) Event {
		return &PlanDelta{Delta: n.Delta, ItemID: n.ItemID}
	})
	streamListenTurnScoped(on, notifyFileChangeOutputDelta, g, p.threadID, p.client.reportHandlerError, onEvent, dispatchTurnScoped, func(n FileChangeOutputDeltaNotification) string {
		return n.ThreadID
	}, func(n FileChangeOutputDeltaNotification) string {
		return n.TurnID
	}, func(n FileChangeOutputDeltaNotification) Event {
		return &FileChangeDelta{Delta: n.Delta, ItemID: n.ItemID}
	})
}

func streamListenTurnScoped[N any](on func(string, NotificationHandler), method string, g *guardedChan, threadID string, reportErr func(string, error), onEvent func(Event), dispatchTurnScoped func(string, func()), threadIDOf func(N) string, turnIDOf func(N) string, convert func(N) Event) {
	on(method, func(_ context.Context, notif Notification) {
		var n N
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			reportErr(method, fmt.Errorf("unmarshal %s: %w", method, err))
			return
		}
		if threadIDOf(n) != threadID {
			return
		}

		ev := convert(n)
		dispatchTurnScoped(turnIDOf(n), func() {
			if onEvent != nil {
				onEvent(ev)
			}
			streamSendEvent(g, ev)
		})
	})
}

func registerItemListeners(ctx context.Context, p turnLifecycleParams, on func(string, NotificationHandler), emit func(Event), items *[]ThreadItemWrapper, itemsMu *sync.Mutex, dispatchTurnScoped func(string, func())) {
	on(notifyItemStarted, func(_ context.Context, notif Notification) {
		var n ItemStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			carrier, ok := unmarshalThreadIDCarrier(notif.Params)
			if !ok || carrier.ThreadID != p.threadID {
				return
			}
			p.client.reportHandlerError(notifyItemStarted, fmt.Errorf("unmarshal %s: %w", notifyItemStarted, err))
			return
		}
		if n.ThreadID != p.threadID {
			return
		}
		dispatchTurnScoped(n.TurnID, func() {
			if c, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
				emit(newCollabEvent(CollabToolCallStartedPhase, c))
			}
			emit(&ItemStarted{Item: n.Item})
		})
	})

	on(notifyItemCompleted, func(_ context.Context, notif Notification) {
		n, ok, err := parseItemCompletedForThread(notif.Params, p.threadID)
		if !ok {
			return
		}
		if err != nil {
			p.client.reportHandlerError(notifyItemCompleted, fmt.Errorf("unmarshal %s: %w", notifyItemCompleted, err))
		}
		dispatchTurnScoped(n.TurnID, func() {
			itemsMu.Lock()
			*items = append(*items, n.Item)
			itemsMu.Unlock()
			if c, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
				emit(newCollabEvent(CollabToolCallCompletedPhase, c))
			}
			emit(&ItemCompleted{Item: n.Item})
		})
	})

	_ = ctx // keeps signature consistent for future listener additions.
}

func registerTurnCompletedListener(p turnLifecycleParams, on func(string, NotificationHandler), queueTurnCompletion func(turnCompletionCandidate)) {
	on(notifyTurnCompleted, func(_ context.Context, notif Notification) {
		candidate, ok, err := parseTurnCompletedForThread(notif.Params, p.threadID)
		if !ok {
			return
		}
		if err != nil {
			if candidate.notification.Turn.Error != nil {
				p.client.reportHandlerError(notifyTurnCompleted, fmt.Errorf("unmarshal %s: %w", notifyTurnCompleted, err))
			} else {
				p.client.reportHandlerError(notifyTurnCompleted, fmt.Errorf("validate %s: %w", notifyTurnCompleted, err))
			}
		}
		queueTurnCompletion(candidate)
	})
}

func registerCollectorListeners(p turnLifecycleParams, on func(string, NotificationHandler), dispatchTurnScoped func(string, func())) {
	if p.collector == nil {
		return
	}

	on(notifyCommandExecutionOutputDelta, func(_ context.Context, notif Notification) {
		var n CommandExecutionOutputDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			carrier, ok := unmarshalThreadIDCarrier(notif.Params)
			if !ok || carrier.ThreadID != p.threadID {
				return
			}
			p.client.reportHandlerError(notifyCommandExecutionOutputDelta, fmt.Errorf("unmarshal %s: %w", notifyCommandExecutionOutputDelta, err))
			return
		}
		if n.ThreadID != p.threadID {
			return
		}
		dispatchTurnScoped(n.TurnID, func() {
			p.collector.processCommandExecutionOutputDelta(n)
		})
	})

	on(notifyThreadTokenUsageUpdated, func(_ context.Context, notif Notification) {
		var n ThreadTokenUsageUpdatedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			carrier, ok := unmarshalThreadIDCarrier(notif.Params)
			if !ok || carrier.ThreadID != p.threadID {
				return
			}
			p.client.reportHandlerError(notifyThreadTokenUsageUpdated, fmt.Errorf("unmarshal %s: %w", notifyThreadTokenUsageUpdated, err))
			return
		}
		if n.ThreadID != p.threadID {
			return
		}
		dispatchTurnScoped(n.TurnID, func() {
			p.collector.processThreadTokenUsageUpdated(n)
		})
	})

	on(notifyError, func(_ context.Context, notif Notification) {
		var n ErrorNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			carrier, ok := unmarshalThreadIDCarrier(notif.Params)
			if !ok || carrier.ThreadID != p.threadID {
				return
			}
			p.client.reportHandlerError(notifyError, fmt.Errorf("unmarshal %s: %w", notifyError, err))
			return
		}
		if n.ThreadID != p.threadID {
			return
		}
		dispatchTurnScoped(n.TurnID, func() {
			p.collector.processSystemError(n)
		})
	})

	on(notifyRealtimeError, func(_ context.Context, notif Notification) {
		var n ThreadRealtimeErrorNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			carrier, ok := unmarshalThreadIDCarrier(notif.Params)
			if !ok || carrier.ThreadID != p.threadID {
				return
			}
			p.client.reportHandlerError(notifyRealtimeError, fmt.Errorf("unmarshal %s: %w", notifyRealtimeError, err))
			return
		}
		if n.ThreadID != p.threadID {
			return
		}
		p.collector.processThreadRealtimeError(n)
	})
}
