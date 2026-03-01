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
type turnLifecycleParams struct {
	client     *Client
	turnParams TurnStartParams
	thread     Thread
	threadID   string
	onComplete func(Turn) // called on successful turn completion; nil = no-op
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
			return
		}
		mu.Lock()
		items = append(items, n.Item)
		mu.Unlock()
	})

	unsubTurn := p.client.addNotificationListener(notifyTurnCompleted, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n TurnCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			n = TurnCompletedNotification{
				Turn: Turn{Error: &TurnError{Message: "failed to unmarshal turn/completed: " + err.Error()}},
			}
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
func executeStreamedTurn(ctx context.Context, p turnLifecycleParams, ch chan<- eventOrErr, s *Stream) {
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

	turnDone := make(chan TurnCompletedNotification, 1)

	streamListen(ctx, on, notifyTurnStarted, ch, p.threadID, func(n TurnStartedNotification) Event {
		return &TurnStarted{Turn: n.Turn, ThreadID: n.ThreadID}
	})

	streamListen(ctx, on, notifyAgentMessageDelta, ch, p.threadID, func(n AgentMessageDeltaNotification) Event {
		return &TextDelta{Delta: n.Delta, ItemID: n.ItemID}
	})

	streamListen(ctx, on, notifyReasoningTextDelta, ch, p.threadID, func(n ReasoningTextDeltaNotification) Event {
		return &ReasoningDelta{Delta: n.Delta, ItemID: n.ItemID, ContentIndex: n.ContentIndex}
	})

	streamListen(ctx, on, notifyReasoningSummaryTextDelta, ch, p.threadID, func(n ReasoningSummaryTextDeltaNotification) Event {
		return &ReasoningSummaryDelta{Delta: n.Delta, ItemID: n.ItemID, SummaryIndex: n.SummaryIndex}
	})

	streamListen(ctx, on, notifyPlanDelta, ch, p.threadID, func(n PlanDeltaNotification) Event {
		return &PlanDelta{Delta: n.Delta, ItemID: n.ItemID}
	})

	streamListen(ctx, on, notifyFileChangeOutputDelta, ch, p.threadID, func(n FileChangeOutputDeltaNotification) Event {
		return &FileChangeDelta{Delta: n.Delta, ItemID: n.ItemID}
	})

	// item/started: emit collab event before generic event when applicable.
	on(notifyItemStarted, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n ItemStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		if c, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
			streamSendEvent(ctx, ch, newCollabEvent(CollabToolCallStartedPhase, c))
		}
		streamSendEvent(ctx, ch, &ItemStarted{Item: n.Item})
	})

	// item/completed: emit collab event before generic event, and append to collected items.
	on(notifyItemCompleted, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n ItemCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		itemsMu.Lock()
		items = append(items, n.Item)
		itemsMu.Unlock()
		if c, ok := n.Item.Value.(*CollabAgentToolCallThreadItem); ok {
			streamSendEvent(ctx, ch, newCollabEvent(CollabToolCallCompletedPhase, c))
		}
		streamSendEvent(ctx, ch, &ItemCompleted{Item: n.Item})
	})

	// turn/completed signals turnDone; synthesizes on unmarshal failure.
	on(notifyTurnCompleted, func(_ context.Context, notif Notification) {
		var carrier threadIDCarrier
		if err := json.Unmarshal(notif.Params, &carrier); err != nil || carrier.ThreadID != p.threadID {
			return
		}
		var n TurnCompletedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			n = TurnCompletedNotification{
				Turn: Turn{Error: &TurnError{Message: "failed to unmarshal turn/completed: " + err.Error()}},
			}
		}
		select {
		case turnDone <- n:
		default:
		}
	})

	if _, err := p.client.Turn.Start(ctx, p.turnParams); err != nil {
		streamSendErr(ctx, ch, fmt.Errorf("turn/start: %w", err))
		return
	}

	// Wait for turn completion or context cancellation.
	select {
	case completed := <-turnDone:
		streamSendEvent(ctx, ch, &TurnCompleted{Turn: completed.Turn})

		if completed.Turn.Error != nil {
			streamSendErr(ctx, ch, fmt.Errorf("turn error: %w", completed.Turn.Error))
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
		streamSendErr(ctx, ch, ctx.Err())
	}
}
