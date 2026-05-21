package appserver

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

func rawCarrierForThread[T any](params json.RawMessage, threadID string, unmarshal func(json.RawMessage) (T, bool), threadIDOf func(T) string) (T, bool) {
	carrier, ok := unmarshal(params)
	if !ok || threadIDOf(carrier) != threadID {
		var zero T
		return zero, false
	}
	return carrier, true
}

func unmarshalOrCarrierForThread[N any, C any](params json.RawMessage, threadID string, threadIDOf func(N) string, unmarshalCarrier func(json.RawMessage) (C, bool), carrierThreadIDOf func(C) string) (N, C, bool, error) {
	var n N
	if err := json.Unmarshal(params, &n); err != nil {
		carrier, ok := rawCarrierForThread(params, threadID, unmarshalCarrier, carrierThreadIDOf)
		return n, carrier, ok, err
	}
	if threadIDOf(n) != threadID {
		var carrier C
		return n, carrier, false, nil
	}
	var carrier C
	return n, carrier, true, nil
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
	n, carrier, ok, err := unmarshalOrCarrierForThread(params, threadID, func(n ItemCompletedNotification) string {
		return n.ThreadID
	}, unmarshalItemCompletedCarrier, func(carrier rawItemCompletedCarrier) string {
		return carrier.ThreadID
	})
	if !ok {
		return ItemCompletedNotification{}, false, nil
	}
	if err != nil {
		n.ThreadID = carrier.ThreadID
		n.TurnID = carrier.TurnID
		n.Item = ThreadItemWrapper{Value: &UnknownThreadItem{
			Type: UnmarshalErrorItemType,
			Raw:  append(json.RawMessage(nil), carrier.Item...),
		}}
		return n, true, err
	}
	return n, true, nil
}

func parseTurnCompletedForThread(params json.RawMessage, threadID string, allowMissingTurnID bool) (turnCompletionCandidate, bool, error) {
	n, carrier, ok, err := unmarshalOrCarrierForThread(params, threadID, func(n TurnCompletedNotification) string {
		return n.ThreadID
	}, unmarshalTurnCompletedCarrier, func(carrier rawTurnCompletedCarrier) string {
		return carrier.ThreadID
	})
	if !ok {
		return turnCompletionCandidate{}, false, nil
	}
	if err != nil {
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

func decodeTurnLifecycleThreadNotification[N any](p turnLifecycleParams, method string, params json.RawMessage, threadIDOf func(N) string) (N, bool) {
	var n N
	if err := json.Unmarshal(params, &n); err != nil {
		carrier, ok := unmarshalThreadIDCarrier(params)
		if !ok || carrier.ThreadID != p.threadID {
			return n, false
		}
		p.client.ReportHandlerError(method, fmt.Errorf("unmarshal %s: %w", method, err))
		return n, false
	}
	if threadIDOf(n) != p.threadID {
		return n, false
	}
	return n, true
}

func parseItemCompletedNotification(p turnLifecycleParams, notif Notification) (ItemCompletedNotification, bool) {
	n, ok, err := parseItemCompletedForThread(notif.Params, p.threadID)
	if !ok {
		return ItemCompletedNotification{}, false
	}
	if err != nil {
		p.client.ReportHandlerError(protocol.NotifyItemCompleted, fmt.Errorf("unmarshal %s: %w", protocol.NotifyItemCompleted, err))
	}
	return n, true
}

func parseTurnCompletionNotification(p turnLifecycleParams, notif Notification, allowMissingTurnID bool) (turnCompletionCandidate, bool) {
	candidate, ok, err := parseTurnCompletedForThread(notif.Params, p.threadID, allowMissingTurnID)
	if !ok {
		return turnCompletionCandidate{}, false
	}
	if err != nil {
		reportTurnCompletionError(p.client, candidate, err)
	}
	return candidate, true
}

func reportTurnCompletionError(client *Client, candidate turnCompletionCandidate, err error) {
	if candidate.notification.Turn.Error != nil {
		client.ReportHandlerError(protocol.NotifyTurnCompleted, fmt.Errorf("unmarshal %s: %w", protocol.NotifyTurnCompleted, err))
		return
	}
	client.ReportHandlerError(protocol.NotifyTurnCompleted, fmt.Errorf("validate %s: %w", protocol.NotifyTurnCompleted, err))
}

func matchesActiveTurn(activeTurnID string, candidate turnCompletionCandidate) bool {
	return (candidate.turnID != "" && candidate.turnID == activeTurnID) || (candidate.allowMissingTurnID && candidate.turnID == "")
}

type blockingTurnState struct {
	turnStartState

	pendingItems       []ItemCompletedNotification
	pendingCompletions []turnCompletionCandidate
}

func newPendingTurnStartOverflowError() error {
	return fmt.Errorf("%w: queued notification limit %d reached", errPendingTurnStartQueueOverflow, maxPendingTurnStartNotifications)
}

type turnStartState struct {
	mu           sync.Mutex
	ready        bool
	turnID       string
	pendingCount int
	overflowErr  error
}

func (s *turnStartState) queuePendingLocked() error {
	if s.pendingCount >= maxPendingTurnStartNotifications {
		s.overflowErr = newPendingTurnStartOverflowError()
		return s.overflowErr
	}
	s.pendingCount++
	return nil
}

func (s *turnStartState) queueBeforeReadyLocked(addPending func()) (bool, error) {
	if s.ready {
		return false, nil
	}
	if s.pendingOverflowedLocked() {
		return true, nil
	}
	if err := s.queuePendingLocked(); err != nil {
		return true, err
	}
	addPending()
	return true, nil
}

func (s *turnStartState) pendingOverflowedLocked() bool {
	return s.overflowErr != nil
}

func (s *turnStartState) startLocked(turnID string) error {
	s.ready = true
	s.turnID = turnID
	err := s.overflowErr
	s.pendingCount = 0
	return err
}

func startTurnStateWithPending[T any](state *turnStartState, turnID string, pending *[]T, completions *[]turnCompletionCandidate) ([]T, []turnCompletionCandidate, error) {
	state.mu.Lock()
	defer state.mu.Unlock()

	err := state.startLocked(turnID)
	queued := *pending
	queuedCompletions := *completions
	*pending = nil
	*completions = nil
	return queued, queuedCompletions, err
}

func (s *blockingTurnState) queueItem(n ItemCompletedNotification) (ThreadItemWrapper, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if queued, err := s.queueBeforeReadyLocked(func() {
		s.pendingItems = append(s.pendingItems, n)
	}); queued {
		return ThreadItemWrapper{}, false, err
	}
	if n.TurnID != s.turnID {
		return ThreadItemWrapper{}, false, nil
	}
	return n.Item, true, nil
}

func (s *blockingTurnState) queueCompletion(n turnCompletionCandidate) (TurnCompletedNotification, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if queued, err := s.queueBeforeReadyLocked(func() {
		s.pendingCompletions = append(s.pendingCompletions, n)
	}); queued {
		return TurnCompletedNotification{}, false, err
	}
	if !matchesActiveTurn(s.turnID, n) {
		return TurnCompletedNotification{}, false, nil
	}
	return n.notification, true, nil
}

func (s *blockingTurnState) start(turnID string) ([]ItemCompletedNotification, []turnCompletionCandidate, error) {
	return startTurnStateWithPending(&s.turnStartState, turnID, &s.pendingItems, &s.pendingCompletions)
}

type streamedTurnState struct {
	turnStartState

	pendingEvents      []func(string)
	pendingCompletions []turnCompletionCandidate
}

func (s *streamedTurnState) queueEvent(turnID string, fn func()) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if queued, err := s.queueBeforeReadyLocked(func() {
		capturedTurnID := turnID
		s.pendingEvents = append(s.pendingEvents, func(activeTurnID string) {
			if capturedTurnID == activeTurnID {
				fn()
			}
		})
	}); queued {
		return "", false, err
	}

	return s.turnID, true, nil
}

func (s *streamedTurnState) queueCompletion(n turnCompletionCandidate) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if queued, err := s.queueBeforeReadyLocked(func() {
		s.pendingCompletions = append(s.pendingCompletions, n)
	}); queued {
		return "", false, err
	}

	return s.turnID, true, nil
}

func (s *streamedTurnState) start(turnID string) ([]func(string), []turnCompletionCandidate, error) {
	return startTurnStateWithPending(&s.turnStartState, turnID, &s.pendingEvents, &s.pendingCompletions)
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

func sendTurnCompletion(done chan<- TurnCompletedNotification, n TurnCompletedNotification) {
	select {
	case done <- n:
	default:
	}
}

func snapshotCollectedItems(itemsMu *sync.Mutex, items *[]ThreadItemWrapper) []ThreadItemWrapper {
	itemsMu.Lock()
	defer itemsMu.Unlock()

	collectedItems := make([]ThreadItemWrapper, len(*items))
	copy(collectedItems, *items)
	return collectedItems
}

func finishCompletedTurnLifecycle(p turnLifecycleParams, completed TurnCompletedNotification, items []ThreadItemWrapper) (*RunResult, error) {
	result := completeTurnLifecycle(p, completed.Turn, items)
	if completed.Turn.Error != nil {
		return nil, fmt.Errorf("turn error: %w", completed.Turn.Error)
	}
	return result, nil
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
		appendItem         = func(item ThreadItemWrapper) {
			itemsMu.Lock()
			items = append(items, item)
			itemsMu.Unlock()
		}
	)

	unsubItem := p.client.AddNotificationListener(protocol.NotifyItemCompleted, func(_ context.Context, notif Notification) {
		n, ok := parseItemCompletedNotification(p, notif)
		if !ok {
			return
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
		candidate, ok := parseTurnCompletionNotification(p, notif, allowMissingTurnID)
		if !ok {
			return
		}
		completed, ok, err := state.queueCompletion(candidate)
		if err != nil {
			p.client.ReportHandlerError(protocol.NotifyTurnCompleted, err)
			return
		}
		if !ok {
			return
		}
		sendTurnCompletion(done, completed)
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
		sendTurnCompletion(done, n.notification)
	}

	completed, err := waitForTurnCompletion(ctx, done)
	if err != nil {
		return nil, err
	}

	return finishCompletedTurnLifecycle(p, completed, snapshotCollectedItems(&itemsMu, &items))
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
		sendTurnCompletion(turnDone, n.notification)
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
		sendTurnCompletion(turnDone, n.notification)
	}

	// Wait for turn completion or context cancellation.
	select {
	case completed := <-turnDone:
		emit(&TurnCompleted{Turn: completed.Turn, ThreadID: completed.ThreadID})

		result, err := finishCompletedTurnLifecycle(p, completed, snapshotCollectedItems(&itemsMu, &items))
		if err != nil {
			emitErr(err)
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
	streamListenDecoded(on, method, threadID, reportErr, threadIDOf, convert, func(n N, ev Event) {
		dispatchTurnScoped(turnIDOf(n), func() {
			emitStreamEvent(g, onEvent, ev)
		})
	})
}

func registerItemListeners(p turnLifecycleParams, on func(string, NotificationHandler), emit func(Event), items *[]ThreadItemWrapper, itemsMu *sync.Mutex, dispatchTurnScoped func(string, func())) {
	on(protocol.NotifyItemStarted, func(_ context.Context, notif Notification) {
		n, ok := decodeTurnLifecycleThreadNotification(p, protocol.NotifyItemStarted, notif.Params, func(n ItemStartedNotification) string {
			return n.ThreadID
		})
		if !ok {
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
		n, ok := parseItemCompletedNotification(p, notif)
		if !ok {
			return
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
		candidate, ok := parseTurnCompletionNotification(p, notif, allowMissingTurnID)
		if !ok {
			return
		}
		queueTurnCompletion(candidate)
	})
}

func registerCollectorTurnScopedListener[N any](p turnLifecycleParams, on func(string, NotificationHandler), method string, dispatchTurnScoped func(string, func()), threadIDOf func(N) string, turnIDOf func(N) string, handle func(N)) {
	on(method, func(_ context.Context, notif Notification) {
		n, ok := decodeTurnLifecycleThreadNotification(p, method, notif.Params, threadIDOf)
		if !ok {
			return
		}
		dispatchTurnScoped(turnIDOf(n), func() {
			handle(n)
		})
	})
}

func registerCollectorThreadScopedListener[N any](p turnLifecycleParams, on func(string, NotificationHandler), method string, threadIDOf func(N) string, handle func(N)) {
	on(method, func(_ context.Context, notif Notification) {
		n, ok := decodeTurnLifecycleThreadNotification(p, method, notif.Params, threadIDOf)
		if !ok {
			return
		}
		handle(n)
	})
}

func registerCollectorListeners(p turnLifecycleParams, on func(string, NotificationHandler), dispatchTurnScoped func(string, func())) {
	if p.collector == nil {
		return
	}

	registerCollectorTurnScopedListener(p, on, protocol.NotifyCommandExecutionOutputDelta, dispatchTurnScoped, func(n CommandExecutionOutputDeltaNotification) string {
		return n.ThreadID
	}, func(n CommandExecutionOutputDeltaNotification) string {
		return n.TurnID
	}, p.collector.processCommandExecutionOutputDelta)
	registerCollectorTurnScopedListener(p, on, protocol.NotifyThreadTokenUsageUpdated, dispatchTurnScoped, func(n ThreadTokenUsageUpdatedNotification) string {
		return n.ThreadID
	}, func(n ThreadTokenUsageUpdatedNotification) string {
		return n.TurnID
	}, p.collector.processThreadTokenUsageUpdated)
	registerCollectorTurnScopedListener(p, on, protocol.NotifyError, dispatchTurnScoped, func(n ErrorNotification) string {
		return n.ThreadID
	}, func(n ErrorNotification) string {
		return n.TurnID
	}, p.collector.processSystemError)
	registerCollectorThreadScopedListener(p, on, protocol.NotifyRealtimeError, func(n ThreadRealtimeErrorNotification) string {
		return n.ThreadID
	}, p.collector.processThreadRealtimeError)
}
