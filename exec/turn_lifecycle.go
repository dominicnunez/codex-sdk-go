package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const maxPendingTurnStartNotifications = 1024

var errPendingTurnStartQueueOverflow = errors.New("pending turn/start notification queue overflow")

// threadIDCarrier extracts thread-scoping fields from raw notification JSON for filtering.
type threadIDCarrier struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type rawTurnCompletedCarrier struct {
	ThreadID string          `json:"threadId"`
	Turn     json.RawMessage `json:"turn"`
}

type rawItemCompletedCarrier struct {
	ThreadID string          `json:"threadId"`
	TurnID   string          `json:"turnId"`
	Item     json.RawMessage `json:"item"`
}

func unmarshalThreadIDCarrier(params json.RawMessage) (threadIDCarrier, bool) {
	var carrier threadIDCarrier
	if err := json.Unmarshal(params, &carrier); err != nil {
		return threadIDCarrier{}, false
	}
	return carrier, true
}

func unmarshalItemCompletedCarrier(params json.RawMessage) (rawItemCompletedCarrier, bool) {
	var carrier rawItemCompletedCarrier
	if err := json.Unmarshal(params, &carrier); err != nil {
		return rawItemCompletedCarrier{}, false
	}
	return carrier, true
}

func unmarshalTurnCompletedCarrier(params json.RawMessage) (rawTurnCompletedCarrier, bool) {
	var carrier rawTurnCompletedCarrier
	if err := json.Unmarshal(params, &carrier); err != nil {
		return rawTurnCompletedCarrier{}, false
	}
	return carrier, true
}

func extractRawTurnCompletedID(turn json.RawMessage) string {
	if len(turn) == 0 {
		return ""
	}

	var carrier struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(turn, &carrier); err != nil {
		return ""
	}
	return carrier.ID
}

func parseItemCompletedForThread(params json.RawMessage, threadID string) (ItemCompletedNotification, bool, error) {
	var n ItemCompletedNotification
	if err := json.Unmarshal(params, &n); err != nil {
		carrier, ok := unmarshalItemCompletedCarrier(params)
		if !ok || carrier.ThreadID != threadID {
			return ItemCompletedNotification{}, false, nil
		}
		n.ThreadID = carrier.ThreadID
		n.TurnID = carrier.TurnID
		n.Item = ThreadItemWrapper{Value: &UnknownThreadItem{
			Type: UnmarshalErrorItemType,
			Raw:  append(json.RawMessage(nil), carrier.Item...),
		}}
		return n, true, err
	}
	if n.ThreadID != threadID {
		return ItemCompletedNotification{}, false, nil
	}
	return n, true, nil
}

func parseTurnCompletedForThread(params json.RawMessage, threadID string, allowMissingTurnID bool) (turnCompletionCandidate, bool, error) {
	var n TurnCompletedNotification
	if err := json.Unmarshal(params, &n); err != nil {
		carrier, ok := unmarshalTurnCompletedCarrier(params)
		if !ok || carrier.ThreadID != threadID {
			return turnCompletionCandidate{}, false, nil
		}
		rawTurnID := extractRawTurnCompletedID(carrier.Turn)
		return turnCompletionCandidate{
			notification: TurnCompletedNotification{
				ThreadID: carrier.ThreadID,
				Turn: Turn{
					ID:     rawTurnID,
					Status: TurnStatusFailed,
					Error:  &TurnError{Message: "failed to unmarshal turn/completed: " + err.Error()},
				},
			},
			turnID:             rawTurnID,
			allowMissingTurnID: allowMissingTurnID && rawTurnID == "",
		}, true, err
	}
	if n.ThreadID == "" {
		// Without threadId this completion cannot be attributed to a specific
		// lifecycle, so ignore it rather than failing all active turns.
		return turnCompletionCandidate{}, false, nil
	}
	if n.ThreadID != threadID {
		return turnCompletionCandidate{}, false, nil
	}
	if err := validateTurnCompletedNotification(n); err != nil {
		return turnCompletionCandidate{
			notification:       invalidTurnCompletedNotification(n.ThreadID, n.Turn.ID, err),
			turnID:             n.Turn.ID,
			allowMissingTurnID: allowMissingTurnID && n.Turn.ID == "",
		}, true, err
	}
	return turnCompletionCandidate{
		notification: n,
		turnID:       n.Turn.ID,
	}, true, nil
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
	client                    *Client
	turnParams                TurnStartParams
	thread                    Thread
	threadID                  string
	allowMissingInitialTurnID bool
	onStart                   func()       // called after turn/start returns a valid turn ID; nil = no-op
	onComplete                func(Thread) // called with the completed thread snapshot; nil = no-op
	collector                 *StreamCollector
}

type turnCompletionCandidate struct {
	notification       TurnCompletedNotification
	turnID             string
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

func invalidTurnCompletedNotification(threadID, turnID string, err error) TurnCompletedNotification {
	return TurnCompletedNotification{
		ThreadID: threadID,
		Turn: Turn{
			ID:     turnID,
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
	return (candidate.turnID != "" && candidate.turnID == activeTurnID) || (candidate.allowMissingTurnID && candidate.turnID == "")
}

type blockingTurnState struct {
	mu                 sync.Mutex
	ready              bool
	turnID             string
	pendingItems       []ItemCompletedNotification
	pendingCompletions []turnCompletionCandidate
	pendingCount       int
	overflowErr        error
}

func newPendingTurnStartOverflowError() error {
	return fmt.Errorf("%w: queued notification limit %d reached", errPendingTurnStartQueueOverflow, maxPendingTurnStartNotifications)
}

func (s *blockingTurnState) queuePendingLocked() error {
	if s.pendingCount >= maxPendingTurnStartNotifications {
		s.overflowErr = newPendingTurnStartOverflowError()
		return s.overflowErr
	}
	s.pendingCount++
	return nil
}

func (s *blockingTurnState) queueItem(n ItemCompletedNotification) (ThreadItemWrapper, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		if s.overflowErr != nil {
			return ThreadItemWrapper{}, false, nil
		}
		if err := s.queuePendingLocked(); err != nil {
			return ThreadItemWrapper{}, false, err
		}
		s.pendingItems = append(s.pendingItems, n)
		return ThreadItemWrapper{}, false, nil
	}
	if n.TurnID != s.turnID {
		return ThreadItemWrapper{}, false, nil
	}
	return n.Item, true, nil
}

func (s *blockingTurnState) queueCompletion(n turnCompletionCandidate) (TurnCompletedNotification, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		if s.overflowErr != nil {
			return TurnCompletedNotification{}, false, nil
		}
		if err := s.queuePendingLocked(); err != nil {
			return TurnCompletedNotification{}, false, err
		}
		s.pendingCompletions = append(s.pendingCompletions, n)
		return TurnCompletedNotification{}, false, nil
	}
	if !matchesActiveTurn(s.turnID, n) {
		return TurnCompletedNotification{}, false, nil
	}
	return n.notification, true, nil
}

func (s *blockingTurnState) start(turnID string) ([]ItemCompletedNotification, []turnCompletionCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ready = true
	s.turnID = turnID
	items := s.pendingItems
	completions := s.pendingCompletions
	err := s.overflowErr
	s.pendingItems = nil
	s.pendingCompletions = nil
	s.pendingCount = 0
	return items, completions, err
}

type streamedTurnState struct {
	mu                 sync.Mutex
	ready              bool
	turnID             string
	pendingEvents      []func(string)
	pendingCompletions []turnCompletionCandidate
	pendingCount       int
	overflowErr        error
}

func (s *streamedTurnState) queuePendingLocked() error {
	if s.pendingCount >= maxPendingTurnStartNotifications {
		s.overflowErr = newPendingTurnStartOverflowError()
		return s.overflowErr
	}
	s.pendingCount++
	return nil
}

func (s *streamedTurnState) queueEvent(turnID string, fn func()) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		if s.overflowErr != nil {
			return "", false, nil
		}
		if err := s.queuePendingLocked(); err != nil {
			return "", false, err
		}
		capturedTurnID := turnID
		s.pendingEvents = append(s.pendingEvents, func(activeTurnID string) {
			if capturedTurnID == activeTurnID {
				fn()
			}
		})
		return "", false, nil
	}

	return s.turnID, true, nil
}

func (s *streamedTurnState) queueCompletion(n turnCompletionCandidate) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		if s.overflowErr != nil {
			return "", false, nil
		}
		if err := s.queuePendingLocked(); err != nil {
			return "", false, err
		}
		s.pendingCompletions = append(s.pendingCompletions, n)
		return "", false, nil
	}

	return s.turnID, true, nil
}

func (s *streamedTurnState) start(turnID string) ([]func(string), []turnCompletionCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ready = true
	s.turnID = turnID
	events := s.pendingEvents
	completions := s.pendingCompletions
	err := s.overflowErr
	s.pendingEvents = nil
	s.pendingCompletions = nil
	s.pendingCount = 0
	return events, completions, err
}

func waitForTurnCompletion(ctx context.Context, done <-chan TurnCompletedNotification) (TurnCompletedNotification, error) {
	select {
	case completed := <-done:
		return completed, nil
	case <-ctx.Done():
		return TurnCompletedNotification{}, ctx.Err()
	}
}

func completeTurnLifecycle(p turnLifecycleParams, completed Turn, items []ThreadItemWrapper) *RunResult {
	thread := p.thread
	if p.client != nil {
		if snapshot, ok := p.client.ThreadStateSnapshot(p.threadID); ok {
			thread = snapshot
		}
	}

	result := buildRunResult(thread, completed, items)
	if p.client != nil {
		p.client.CacheThreadState(result.Thread)
	}
	if p.onComplete != nil {
		p.onComplete(result.Thread)
	}
	return result
}

// executeTurn runs a blocking turn: registers listeners, starts the turn,
// collects items, and waits for completion or context cancellation.
// Listeners are filtered by threadID and active turnID to avoid cross-turn contamination.
func executeTurn(ctx context.Context, p turnLifecycleParams) (*RunResult, error) {
	var (
		items              []ThreadItemWrapper
		itemsMu            sync.Mutex
		state              blockingTurnState
		allowMissingTurnID = p.allowMissingInitialTurnID
		done               = make(chan TurnCompletedNotification, 1)
		sendDone           = func(n TurnCompletedNotification) {
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

	unsubItem := p.client.AddNotificationListener(protocol.NotifyItemCompleted, func(_ context.Context, notif Notification) {
		n, ok, err := parseItemCompletedForThread(notif.Params, p.threadID)
		if !ok {
			return
		}
		if err != nil {
			p.client.ReportHandlerError(protocol.NotifyItemCompleted, fmt.Errorf("unmarshal %s: %w", protocol.NotifyItemCompleted, err))
		}
		item, ok, err := state.queueItem(n)
		if err != nil {
			p.client.ReportHandlerError(protocol.NotifyItemCompleted, err)
			return
		}
		if !ok {
			return
		}
		appendItem(item)
	})

	unsubTurn := p.client.AddNotificationListener(protocol.NotifyTurnCompleted, func(_ context.Context, notif Notification) {
		candidate, ok, err := parseTurnCompletedForThread(notif.Params, p.threadID, allowMissingTurnID)
		if !ok {
			return
		}
		if err != nil {
			if candidate.notification.Turn.Error != nil {
				p.client.ReportHandlerError(protocol.NotifyTurnCompleted, fmt.Errorf("unmarshal %s: %w", protocol.NotifyTurnCompleted, err))
			} else {
				p.client.ReportHandlerError(protocol.NotifyTurnCompleted, fmt.Errorf("validate %s: %w", protocol.NotifyTurnCompleted, err))
			}
		}
		completed, ok, err := state.queueCompletion(candidate)
		if err != nil {
			p.client.ReportHandlerError(protocol.NotifyTurnCompleted, err)
			return
		}
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
	if p.onStart != nil {
		p.onStart()
	}

	bufferedItems, bufferedCompletions, err := state.start(startResp.Turn.ID)
	if err != nil {
		return nil, err
	}

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

	itemsMu.Lock()
	collectedItems := make([]ThreadItemWrapper, len(items))
	copy(collectedItems, items)
	itemsMu.Unlock()

	result := completeTurnLifecycle(p, completed.Turn, collectedItems)
	if completed.Turn.Error != nil {
		return nil, fmt.Errorf("turn error: %w", completed.Turn.Error)
	}
	return result, nil
}

// executeStreamedTurn runs the streaming lifecycle: registers filtered listeners,
// starts the turn, and sends events on ch until completion or context cancellation.
func executeStreamedTurn(ctx context.Context, p turnLifecycleParams, g *guardedChan, s *Stream) {
	var (
		items              []ThreadItemWrapper
		itemsMu            sync.Mutex
		turnState          streamedTurnState
		allowMissingTurnID = p.allowMissingInitialTurnID
		startedTurnID      string
		unsubFuncs         []func()
	)
	defer func() {
		for _, unsub := range unsubFuncs {
			unsub()
		}
	}()

	on := func(method string, handler NotificationHandler) {
		unsub := p.client.AddNotificationListener(method, handler)
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
		activeTurnID, ready, err := turnState.queueEvent(turnID, fn)
		if err != nil {
			emitErr(err)
			return
		}
		if !ready || turnID != activeTurnID {
			return
		}
		fn()
	}

	queueTurnCompletionCandidate := func(n turnCompletionCandidate) {
		activeTurnID, ready, err := turnState.queueCompletion(n)
		if err != nil {
			emitErr(err)
			return
		}
		if !ready || !matchesActiveTurn(activeTurnID, n) {
			return
		}
		select {
		case turnDone <- n.notification:
		default:
		}
	}

	registerStreamDeltaListeners(p, g, on, onEvent, dispatchTurnScoped)
	registerItemListeners(p, on, emit, &items, &itemsMu, dispatchTurnScoped)
	registerTurnCompletedListener(p, on, allowMissingTurnID, queueTurnCompletionCandidate)
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
	if p.onStart != nil {
		p.onStart()
	}

	startedTurnID = startResp.Turn.ID
	pendingEvents, pendingCompletions, err := turnState.start(startedTurnID)
	if err != nil {
		emitErr(err)
		return
	}

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
		emit(&TurnCompleted{Turn: completed.Turn, ThreadID: completed.ThreadID})

		itemsMu.Lock()
		collectedItems := make([]ThreadItemWrapper, len(items))
		copy(collectedItems, items)
		itemsMu.Unlock()

		result := completeTurnLifecycle(p, completed.Turn, collectedItems)
		if completed.Turn.Error != nil {
			emitErr(fmt.Errorf("turn error: %w", completed.Turn.Error))
			return
		}
		s.mu.Lock()
		s.result = result
		s.mu.Unlock()

	case <-ctx.Done():
		emitErr(ctx.Err())
	}
}

func registerStreamDeltaListeners(p turnLifecycleParams, g *guardedChan, on func(string, NotificationHandler), onEvent func(Event), dispatchTurnScoped func(string, func())) {
	streamListen(on, protocol.NotifyTurnStarted, g, p.threadID, p.client.ReportHandlerError, onEvent, func(n TurnStartedNotification) string {
		return n.ThreadID
	}, func(n TurnStartedNotification) Event {
		return &TurnStarted{Turn: n.Turn, ThreadID: n.ThreadID}
	})
	streamListenTurnScoped(on, protocol.NotifyAgentMessageDelta, g, p.threadID, p.client.ReportHandlerError, onEvent, dispatchTurnScoped, func(n AgentMessageDeltaNotification) string {
		return n.ThreadID
	}, func(n AgentMessageDeltaNotification) string {
		return n.TurnID
	}, func(n AgentMessageDeltaNotification) Event {
		return &TextDelta{Delta: n.Delta, ItemID: n.ItemID}
	})
	streamListenTurnScoped(on, protocol.NotifyReasoningTextDelta, g, p.threadID, p.client.ReportHandlerError, onEvent, dispatchTurnScoped, func(n ReasoningTextDeltaNotification) string {
		return n.ThreadID
	}, func(n ReasoningTextDeltaNotification) string {
		return n.TurnID
	}, func(n ReasoningTextDeltaNotification) Event {
		return &ReasoningDelta{Delta: n.Delta, ItemID: n.ItemID, ContentIndex: n.ContentIndex}
	})
	streamListenTurnScoped(on, protocol.NotifyReasoningSummaryTextDelta, g, p.threadID, p.client.ReportHandlerError, onEvent, dispatchTurnScoped, func(n ReasoningSummaryTextDeltaNotification) string {
		return n.ThreadID
	}, func(n ReasoningSummaryTextDeltaNotification) string {
		return n.TurnID
	}, func(n ReasoningSummaryTextDeltaNotification) Event {
		return &ReasoningSummaryDelta{Delta: n.Delta, ItemID: n.ItemID, SummaryIndex: n.SummaryIndex}
	})
	streamListenTurnScoped(on, protocol.NotifyPlanDelta, g, p.threadID, p.client.ReportHandlerError, onEvent, dispatchTurnScoped, func(n PlanDeltaNotification) string {
		return n.ThreadID
	}, func(n PlanDeltaNotification) string {
		return n.TurnID
	}, func(n PlanDeltaNotification) Event {
		return &PlanDelta{Delta: n.Delta, ItemID: n.ItemID, ThreadID: n.ThreadID, TurnID: n.TurnID}
	})
	streamListenTurnScoped(on, protocol.NotifyFileChangeOutputDelta, g, p.threadID, p.client.ReportHandlerError, onEvent, dispatchTurnScoped, func(n FileChangeOutputDeltaNotification) string {
		return n.ThreadID
	}, func(n FileChangeOutputDeltaNotification) string {
		return n.TurnID
	}, func(n FileChangeOutputDeltaNotification) Event {
		return &FileChangeDelta{Delta: n.Delta, ItemID: n.ItemID, ThreadID: n.ThreadID, TurnID: n.TurnID}
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

func registerItemListeners(p turnLifecycleParams, on func(string, NotificationHandler), emit func(Event), items *[]ThreadItemWrapper, itemsMu *sync.Mutex, dispatchTurnScoped func(string, func())) {
	on(protocol.NotifyItemStarted, func(_ context.Context, notif Notification) {
		var n ItemStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			carrier, ok := unmarshalThreadIDCarrier(notif.Params)
			if !ok || carrier.ThreadID != p.threadID {
				return
			}
			p.client.ReportHandlerError(protocol.NotifyItemStarted, fmt.Errorf("unmarshal %s: %w", protocol.NotifyItemStarted, err))
			return
		}
		if n.ThreadID != p.threadID {
			return
		}
		dispatchTurnScoped(n.TurnID, func() {
			if c, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
				emit(newCollabEvent(CollabToolCallStartedPhase, c))
			}
			emit(&ItemStarted{Item: n.Item, ThreadID: n.ThreadID, TurnID: n.TurnID})
		})
	})

	on(protocol.NotifyItemCompleted, func(_ context.Context, notif Notification) {
		n, ok, err := parseItemCompletedForThread(notif.Params, p.threadID)
		if !ok {
			return
		}
		if err != nil {
			p.client.ReportHandlerError(protocol.NotifyItemCompleted, fmt.Errorf("unmarshal %s: %w", protocol.NotifyItemCompleted, err))
		}
		dispatchTurnScoped(n.TurnID, func() {
			itemsMu.Lock()
			*items = append(*items, n.Item)
			itemsMu.Unlock()
			if c, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
				emit(newCollabEvent(CollabToolCallCompletedPhase, c))
			}
			emit(&ItemCompleted{Item: n.Item, ThreadID: n.ThreadID, TurnID: n.TurnID})
		})
	})
}

func registerTurnCompletedListener(p turnLifecycleParams, on func(string, NotificationHandler), allowMissingTurnID bool, queueTurnCompletion func(turnCompletionCandidate)) {
	on(protocol.NotifyTurnCompleted, func(_ context.Context, notif Notification) {
		candidate, ok, err := parseTurnCompletedForThread(notif.Params, p.threadID, allowMissingTurnID)
		if !ok {
			return
		}
		if err != nil {
			if candidate.notification.Turn.Error != nil {
				p.client.ReportHandlerError(protocol.NotifyTurnCompleted, fmt.Errorf("unmarshal %s: %w", protocol.NotifyTurnCompleted, err))
			} else {
				p.client.ReportHandlerError(protocol.NotifyTurnCompleted, fmt.Errorf("validate %s: %w", protocol.NotifyTurnCompleted, err))
			}
		}
		queueTurnCompletion(candidate)
	})
}

func registerCollectorListeners(p turnLifecycleParams, on func(string, NotificationHandler), dispatchTurnScoped func(string, func())) {
	if p.collector == nil {
		return
	}

	on(protocol.NotifyCommandExecutionOutputDelta, func(_ context.Context, notif Notification) {
		var n CommandExecutionOutputDeltaNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			carrier, ok := unmarshalThreadIDCarrier(notif.Params)
			if !ok || carrier.ThreadID != p.threadID {
				return
			}
			p.client.ReportHandlerError(protocol.NotifyCommandExecutionOutputDelta, fmt.Errorf("unmarshal %s: %w", protocol.NotifyCommandExecutionOutputDelta, err))
			return
		}
		if n.ThreadID != p.threadID {
			return
		}
		dispatchTurnScoped(n.TurnID, func() {
			p.collector.processCommandExecutionOutputDelta(n)
		})
	})

	on(protocol.NotifyThreadTokenUsageUpdated, func(_ context.Context, notif Notification) {
		var n ThreadTokenUsageUpdatedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			carrier, ok := unmarshalThreadIDCarrier(notif.Params)
			if !ok || carrier.ThreadID != p.threadID {
				return
			}
			p.client.ReportHandlerError(protocol.NotifyThreadTokenUsageUpdated, fmt.Errorf("unmarshal %s: %w", protocol.NotifyThreadTokenUsageUpdated, err))
			return
		}
		if n.ThreadID != p.threadID {
			return
		}
		dispatchTurnScoped(n.TurnID, func() {
			p.collector.processThreadTokenUsageUpdated(n)
		})
	})

	on(protocol.NotifyError, func(_ context.Context, notif Notification) {
		var n ErrorNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			carrier, ok := unmarshalThreadIDCarrier(notif.Params)
			if !ok || carrier.ThreadID != p.threadID {
				return
			}
			p.client.ReportHandlerError(protocol.NotifyError, fmt.Errorf("unmarshal %s: %w", protocol.NotifyError, err))
			return
		}
		if n.ThreadID != p.threadID {
			return
		}
		dispatchTurnScoped(n.TurnID, func() {
			p.collector.processSystemError(n)
		})
	})

	on(protocol.NotifyRealtimeError, func(_ context.Context, notif Notification) {
		var n ThreadRealtimeErrorNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			carrier, ok := unmarshalThreadIDCarrier(notif.Params)
			if !ok || carrier.ThreadID != p.threadID {
				return
			}
			p.client.ReportHandlerError(protocol.NotifyRealtimeError, fmt.Errorf("unmarshal %s: %w", protocol.NotifyRealtimeError, err))
			return
		}
		if n.ThreadID != p.threadID {
			return
		}
		p.collector.processThreadRealtimeError(n)
	})
}
