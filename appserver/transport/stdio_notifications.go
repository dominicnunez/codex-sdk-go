package transport

import (
	"errors"
	"sync"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type turnScopedNotificationQueue struct {
	mu        sync.Mutex
	threadKey string
	queue     []bufferedNotification
	scheduled bool
}

type streamingNotificationBacklog struct {
	mu       sync.Mutex
	queue    []bufferedNotification
	draining bool
}

func (t *StdioTransport) initTurnScopedScheduler() {
	t.turnNotifReadyOnce.Do(func() {
		if t.turnNotifQueues == nil {
			t.turnNotifQueues = make(map[string]*turnScopedNotificationQueue)
		}
		t.turnNotifReadyCond = sync.NewCond(&t.turnNotifReadyMu)
	})
}

func (t *StdioTransport) wakeTurnScopedNotificationWorkers() {
	if t.turnNotifReadyCond == nil {
		return
	}
	t.turnNotifReadyMu.Lock()
	t.turnNotifReadyCond.Broadcast()
	t.turnNotifReadyMu.Unlock()
}

func (t *StdioTransport) stopTurnScopedNotificationWorkers() {
	t.initTurnScopedScheduler()
	t.turnNotifStopped.Store(true)
	t.turnNotifQueuesMu.Lock()
	for _, queue := range t.turnNotifQueues {
		queue.mu.Lock()
		pending := queue.queue
		queue.queue = nil
		queue.threadKey = ""
		queue.scheduled = false
		releaseBufferedNotifications(pending)
		queue.mu.Unlock()
	}
	clear(t.turnNotifQueues)
	t.orderedNotifCount = 0
	t.turnNotifQueuesMu.Unlock()
	t.turnNotifReadyMu.Lock()
	t.turnNotifReady = nil
	t.turnNotifReadyCond.Broadcast()
	t.turnNotifReadyMu.Unlock()
}

func (t *StdioTransport) canDrainOrderedNotifications() bool {
	if t.turnNotifStopped.Load() {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ctx.Err() == nil || t.readerEOF
}

func (t *StdioTransport) notificationWorker() {
	t.handleNotificationQueue(t.notifQueue)
}

func (t *StdioTransport) streamingNotificationWorker() {
	t.handleNotificationQueue(t.streamingNotifQueue)
}

func (t *StdioTransport) protectedNotificationWorker() {
	t.handleNotificationQueue(t.protectedNotifQueue)
}

func (t *StdioTransport) criticalNotificationWorker() {
	t.handleNotificationQueue(t.criticalNotifQueue)
}

func (t *StdioTransport) handleNotificationQueue(queue <-chan bufferedNotification) {
	for {
		notif, ok := t.recvBufferedNotification(queue)
		if !ok {
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) recvBufferedNotification(queue <-chan bufferedNotification) (bufferedNotification, bool) {
	if t.ctx.Err() != nil {
		return bufferedNotification{}, false
	}
	select {
	case <-t.ctx.Done():
		return bufferedNotification{}, false
	case notif := <-queue:
		if t.ctx.Err() != nil && !t.canDrainOrderedNotifications() {
			// A consumed entry can no longer be found by stop's queue drain.
			// Release its reservation rather than discarding the value unseen.
			notif.release()
			return bufferedNotification{}, false
		}
		return notif, true
	}
}

func (t *StdioTransport) turnScopedNotificationWorker() {
	t.initTurnScopedScheduler()
	for {
		queue, ok := t.nextTurnScopedNotificationQueue()
		if !ok {
			return
		}
		t.handleTurnScopedNotificationQueue(queue)
	}
}

func (t *StdioTransport) nextTurnScopedNotificationQueue() (*turnScopedNotificationQueue, bool) {
	t.turnNotifReadyMu.Lock()
	defer t.turnNotifReadyMu.Unlock()

	for {
		if !t.canDrainOrderedNotifications() {
			return nil, false
		}
		t.mu.Lock()
		hasHandler := t.notifHandler != nil
		t.mu.Unlock()
		if len(t.turnNotifReady) > 0 && hasHandler {
			break
		}
		if len(t.turnNotifReady) == 0 && t.ctx.Err() != nil {
			return nil, false
		}
		t.turnNotifReadyCond.Wait()
	}

	queue := t.turnNotifReady[0]
	t.turnNotifReady[0] = nil
	t.turnNotifReady = t.turnNotifReady[1:]
	return queue, true
}

func (t *StdioTransport) scheduleTurnScopedNotificationQueue(queue *turnScopedNotificationQueue) {
	t.turnNotifReadyMu.Lock()
	if t.turnNotifStopped.Load() {
		t.turnNotifReadyMu.Unlock()
		return
	}
	t.turnNotifReady = append(t.turnNotifReady, queue)
	t.turnNotifReadyMu.Unlock()
	t.turnNotifReadyCond.Signal()
}

func (t *StdioTransport) enqueueNotification(notif Notification) {
	key := orderedNotificationKey(notif)
	// Params is owned by the frame decoder. Charge its backing capacity,
	// strings, and the separately allocated owner key. Queue bookkeeping
	// remains bounded by the existing count limits.
	size := cap(notif.Params) + len(notif.JSONRPC) + len(notif.Method) + len(key)
	reservation, err := t.notificationBudget.reserve(size)
	if err != nil {
		if errors.Is(err, errNotificationByteLimit) && (key != "" || isStreamingNotificationMethod(notif.Method) || isProtectedNotificationMethod(notif.Method) || isCriticalNotificationMethod(notif.Method)) {
			t.closeWithFailure(err, err)
		}
		return
	}
	t.enqueueBufferedNotification(bufferedNotification{Notification: notif, scopeKey: key, reservation: reservation})
}

func (t *StdioTransport) enqueueBufferedNotification(notif bufferedNotification) {
	if t.notificationBudget.stopped.Load() {
		notif.release()
		return
	}
	if notif.scopeKey != "" {
		t.enqueueTurnScopedNotification(notif, notif.scopeKey)
		return
	}
	if isStreamingNotificationMethod(notif.Method) {
		t.enqueueStreamingNotification(notif)
		return
	}
	if isProtectedNotificationMethod(notif.Method) {
		t.enqueueProtectedNotification(notif)
		return
	}
	if isCriticalNotificationMethod(notif.Method) {
		t.enqueueCriticalNotification(notif)
		return
	}

	// Unknown notifications remain best-effort to preserve read-loop liveness
	// without changing known SDK-visible behavior.
	t.enqueueBestEffortNotification(t.notifQueue, notif)
}

func (t *StdioTransport) enqueueTurnScopedNotification(notif bufferedNotification, threadKey string) {
	t.initTurnScopedScheduler()

	if threadKey == "" {
		// Non-attributable notifications remain best-effort.
		t.enqueueBestEffortNotification(t.notifQueue, notif)
		return
	}

	t.turnNotifQueuesMu.Lock()
	if t.turnNotifStopped.Load() {
		t.turnNotifQueuesMu.Unlock()
		notif.release()
		return
	}
	queue := t.turnNotifQueues[threadKey]
	if queue == nil {
		if len(t.turnNotifQueues) >= maxTurnScopedNotificationQueues {
			t.turnNotifQueuesMu.Unlock()
			notif.release()
			t.closeWithFailure(
				errTurnScopedNotificationQueueLimit,
				errTurnScopedNotificationQueueLimit,
			)
			return
		}
		queue = &turnScopedNotificationQueue{threadKey: threadKey}
		t.turnNotifQueues[threadKey] = queue
	}
	queue.mu.Lock()
	if len(queue.queue) >= maxTurnScopedNotificationQueueSize || t.orderedNotifCount >= maxOrderedNotificationBacklog {
		queue.mu.Unlock()
		t.turnNotifQueuesMu.Unlock()
		notif.release()
		t.closeWithFailure(
			errTurnScopedNotificationQueueOverflow,
			errTurnScopedNotificationQueueOverflow,
		)
		return
	}
	// Keep one canonical owner string alive. Each pending reservation charges
	// its length, so the key remains covered after earlier callbacks finish.
	notif.scopeKey = queue.threadKey
	queue.queue = append(queue.queue, notif)
	t.orderedNotifCount++
	if queue.scheduled {
		queue.mu.Unlock()
		t.turnNotifQueuesMu.Unlock()
		return
	}
	queue.scheduled = true
	queue.mu.Unlock()
	t.turnNotifQueuesMu.Unlock()

	if !t.canDrainOrderedNotifications() {
		t.clearTurnScopedNotificationQueue(queue)
		return
	}
	t.scheduleTurnScopedNotificationQueue(queue)
}

func (t *StdioTransport) handleTurnScopedNotificationQueue(queue *turnScopedNotificationQueue) {
	if !t.canDrainOrderedNotifications() {
		t.clearTurnScopedNotificationQueue(queue)
		return
	}
	// Capture the handler before taking an event. If registration changes to
	// nil, retain the event in its scope queue rather than moving it into a
	// replay buffer where it could be overtaken.
	t.mu.Lock()
	handler, panicFn := t.notifHandler, t.panicHandler
	t.mu.Unlock()
	if handler != nil {
		if notif, ok := t.dequeueTurnScopedNotification(queue); ok {
			// Release after finish retires an empty queue's owner key.
			defer notif.release()
			t.invokeNotificationHandler(notif.Notification, handler, panicFn)
		}
	}
	// Yield after each callback so a busy scope cannot monopolize a worker.
	t.finishTurnScopedNotification(queue)
}

func (t *StdioTransport) finishTurnScopedNotification(queue *turnScopedNotificationQueue) {
	t.turnNotifQueuesMu.Lock()
	queue.mu.Lock()
	if len(queue.queue) == 0 {
		queue.scheduled = false
		if t.turnNotifQueues[queue.threadKey] == queue {
			delete(t.turnNotifQueues, queue.threadKey)
		}
		queue.threadKey = ""
		queue.mu.Unlock()
		t.turnNotifQueuesMu.Unlock()
		return
	}
	queue.mu.Unlock()
	t.turnNotifQueuesMu.Unlock()
	t.scheduleTurnScopedNotificationQueue(queue)
}

func (t *StdioTransport) enqueueLosslessNotification(
	queue chan bufferedNotification,
	notif bufferedNotification,
) {
	// Linearize fallback admission with reader termination. Its workers stop
	// on cancellation; later replay must not enter a channel with no owner.
	t.mu.Lock()
	if t.closed || t.notificationBudget.stopped.Load() {
		t.mu.Unlock()
		notif.release()
		return
	}
	select {
	case <-t.ctx.Done():
		t.mu.Unlock()
		notif.release()
		return
	case queue <- notif:
		t.mu.Unlock()
		if t.notificationBudget.stopped.Load() {
			t.discardNotificationQueue(queue)
		}
		return
	default:
	}
	t.mu.Unlock()
	notif.release()
	t.closeWithFailure(errNotificationQueueOverflow, errNotificationQueueOverflow)
}

func (t *StdioTransport) enqueueBestEffortNotification(queue chan bufferedNotification, notif bufferedNotification) {
	t.mu.Lock()
	if t.closed || t.notificationBudget.stopped.Load() {
		t.mu.Unlock()
		notif.release()
		return
	}
	select {
	case <-t.ctx.Done():
		t.mu.Unlock()
		notif.release()
		return
	case queue <- notif:
		t.mu.Unlock()
		if t.notificationBudget.stopped.Load() {
			t.discardNotificationQueue(queue)
		}
	default:
		t.mu.Unlock()
		notif.release()
	}
}

func (t *StdioTransport) enqueueStreamingNotification(notif bufferedNotification) {
	var startDrainer bool

	t.streamingBacklog.mu.Lock()
	t.mu.Lock()
	if t.closed || t.notificationBudget.stopped.Load() {
		t.mu.Unlock()
		t.streamingBacklog.mu.Unlock()
		notif.release()
		return
	}
	if len(t.streamingBacklog.queue) == 0 && !t.streamingBacklog.draining {
		select {
		case <-t.ctx.Done():
			t.mu.Unlock()
			t.streamingBacklog.mu.Unlock()
			notif.release()
			return
		case t.streamingNotifQueue <- notif:
			t.mu.Unlock()
			t.streamingBacklog.mu.Unlock()
			if t.notificationBudget.stopped.Load() {
				t.discardNotificationQueue(t.streamingNotifQueue)
			}
			return
		default:
		}
	}
	if len(t.streamingBacklog.queue) >= maxStreamingNotificationBacklog {
		t.mu.Unlock()
		t.streamingBacklog.mu.Unlock()
		notif.release()
		t.closeWithFailure(
			errStreamingNotificationBacklogOverflow,
			errStreamingNotificationBacklogOverflow,
		)
		return
	}
	t.streamingBacklog.queue = append(t.streamingBacklog.queue, notif)
	if !t.streamingBacklog.draining {
		t.streamingBacklog.draining = true
		startDrainer = true
	}
	t.mu.Unlock()
	t.streamingBacklog.mu.Unlock()

	if startDrainer {
		go t.flushStreamingNotificationBacklog()
	}
}

func (t *StdioTransport) flushStreamingNotificationBacklog() {
	for {
		notif, ok := t.nextStreamingBacklogNotification()
		if !ok {
			return
		}

		select {
		case <-t.ctx.Done():
			if t.canDrainOrderedNotifications() {
				t.handleNotification(notif)
			} else {
				notif.release()
			}
			return
		case t.streamingNotifQueue <- notif:
			if t.notificationBudget.stopped.Load() {
				t.discardNotificationQueue(t.streamingNotifQueue)
			} else if t.ctx.Err() != nil {
				// EOF's drain may already have passed this channel while the
				// flusher owned an entry. Complete the transfer synchronously.
				t.drainNotificationQueue(t.streamingNotifQueue)
			}
		}
	}
}

func (t *StdioTransport) nextStreamingBacklogNotification() (bufferedNotification, bool) {
	t.streamingBacklog.mu.Lock()
	defer t.streamingBacklog.mu.Unlock()

	if len(t.streamingBacklog.queue) == 0 {
		t.streamingBacklog.draining = false
		return bufferedNotification{}, false
	}

	notif := t.streamingBacklog.queue[0]
	t.streamingBacklog.queue[0] = bufferedNotification{}
	t.streamingBacklog.queue = t.streamingBacklog.queue[1:]
	return notif, true
}

func (t *StdioTransport) enqueueProtectedNotification(notif bufferedNotification) {
	t.enqueueLosslessNotification(t.protectedNotifQueue, notif)
}

func (t *StdioTransport) enqueueCriticalNotification(notif bufferedNotification) {
	t.enqueueLosslessNotification(t.criticalNotifQueue, notif)
}

// Keep the notification bucket switches explicit. The lists define transport
// delivery priority, and hiding them behind shared data structures would make
// ordering changes harder to audit.
func isCriticalNotificationMethod(method string) bool {
	switch method {
	case protocol.NotifyError, protocol.NotifyRealtimeError:
		return true
	default:
		return false
	}
}

func isStreamingNotificationMethod(method string) bool {
	switch method {
	case protocol.NotifyAgentMessageDelta,
		protocol.NotifyFileChangeOutputDelta,
		protocol.NotifyPlanDelta,
		protocol.NotifyReasoningTextDelta,
		protocol.NotifyReasoningSummaryTextDelta,
		protocol.NotifyReasoningSummaryPartAdded,
		protocol.NotifyRealtimeOutputAudioDelta,
		protocol.NotifyCommandExecutionOutputDelta,
		protocol.NotifyCommandExecOutputDelta,
		protocol.NotifyProcessOutputDelta:
		return true
	case "thread/realtime/transcript/delta", "thread/realtime/item/transcript/delta":
		return true
	default:
		return false
	}
}

func isProtectedNotificationMethod(method string) bool {
	switch method {
	case protocol.NotifyItemStarted,
		protocol.NotifyThreadStarted,
		protocol.NotifyThreadClosed,
		protocol.NotifyThreadArchived,
		protocol.NotifyThreadUnarchived,
		protocol.NotifyThreadNameUpdated,
		protocol.NotifyThreadSettingsUpdated,
		protocol.NotifyThreadAttachmentUpdated,
		protocol.NotifyThreadPredictionUpdated,
		protocol.NotifyThreadStatusChanged,
		protocol.NotifyThreadTokenUsageUpdated,
		protocol.NotifyTurnStarted,
		protocol.NotifyTurnPlanUpdated,
		protocol.NotifyTurnDiffUpdated,
		protocol.NotifyAccountUpdated,
		protocol.NotifyAccountLoginCompleted,
		protocol.NotifyAccountRateLimitsUpdated,
		protocol.NotifyGatewayOAuthChanged,
		protocol.NotifyRealtimeStarted,
		protocol.NotifyRealtimeClosed,
		protocol.NotifyRealtimeItemAdded,
		protocol.NotifyWindowsSandboxSetupCompleted,
		protocol.NotifyWindowsWorldWritableWarning,
		protocol.NotifyThreadCompacted,
		protocol.NotifyDeprecationNotice,
		protocol.NotifyTerminalInteraction,
		protocol.NotifyMcpServerOauthLoginCompleted,
		protocol.NotifyMcpToolCallProgress,
		protocol.NotifyServerRequestResolved,
		protocol.NotifyModelRerouted,
		protocol.NotifyFuzzyFileSearchSessionCompleted,
		protocol.NotifyFuzzyFileSearchSessionUpdated,
		protocol.NotifyAppListUpdated,
		protocol.NotifyConfigWarning,
		protocol.NotifySkillsChanged,
		protocol.NotifyHookStarted,
		protocol.NotifyHookCompleted,
		protocol.NotifyProcessExited,
		protocol.NotifyItemGuardianApprovalReviewStarted,
		protocol.NotifyItemGuardianApprovalReviewCompleted:
		return true
	case protocol.NotifyFileChangePatchUpdated,
		"thread/deleted", "thread/reverted", "thread/goal/updated", "thread/goal/cleared",
		"thread/queue/changed", "thread/project/updated",
		"thread/environment/connected", "thread/environment/disconnected",
		"thread/realtime/sdp", "thread/realtime/transcript/done",
		"thread/realtime/item/started", "thread/realtime/item/completed",
		"autoApprovalReview/strictReviewRequired", "turn/moderationMetadata",
		"model/verification", "model/safetyBuffering/updated",
		"modelProvider/authRecoveryStarted", "modelProvider/authRecoveryCompleted",
		"mcpServer/startupStatus/updated", "warning", "guardianWarning":
		return true
	default:
		return false
	}
}

func (t *StdioTransport) drainPendingNotificationsAfterStop() {
	for {
		drained := false

		drained = t.drainNotificationQueue(t.criticalNotifQueue) || drained
		drained = t.drainNotificationQueue(t.protectedNotifQueue) || drained
		drained = t.drainNotificationQueue(t.streamingNotifQueue) || drained
		drained = t.drainStreamingNotificationBacklog() || drained
		drained = t.drainNotificationQueue(t.notifQueue) || drained
		// Ordered queues stay with their owning workers, including at EOF.

		if !drained {
			return
		}
	}
}

func (t *StdioTransport) drainNotificationQueue(queue chan bufferedNotification) bool {
	drained := false
	for {
		select {
		case notif := <-queue:
			t.handleNotification(notif)
			drained = true
		default:
			return drained
		}
	}
}

func (t *StdioTransport) drainStreamingNotificationBacklog() bool {
	drained := false
	for {
		notif, ok := t.nextStreamingBacklogNotification()
		if !ok {
			return drained
		}
		t.handleNotification(notif)
		drained = true
	}
}

func (t *StdioTransport) dequeueTurnScopedNotification(queue *turnScopedNotificationQueue) (bufferedNotification, bool) {
	t.turnNotifQueuesMu.Lock()
	defer t.turnNotifQueuesMu.Unlock()
	queue.mu.Lock()
	if len(queue.queue) == 0 {
		queue.mu.Unlock()
		return bufferedNotification{}, false
	}
	notif := queue.queue[0]
	queue.queue[0] = bufferedNotification{}
	queue.queue = queue.queue[1:]
	t.orderedNotifCount--
	queue.mu.Unlock()
	return notif, true
}

func (t *StdioTransport) clearTurnScopedNotificationQueue(queue *turnScopedNotificationQueue) {
	t.turnNotifQueuesMu.Lock()
	defer t.turnNotifQueuesMu.Unlock()
	queue.mu.Lock()
	if t.turnNotifQueues[queue.threadKey] == queue {
		t.orderedNotifCount -= len(queue.queue)
		delete(t.turnNotifQueues, queue.threadKey)
	}
	pending := queue.queue
	queue.queue = nil
	queue.threadKey = ""
	queue.scheduled = false
	releaseBufferedNotifications(pending)
	queue.mu.Unlock()
}

// handleNotification dispatches an incoming server→client notification to the handler
func (t *StdioTransport) handleNotification(notif bufferedNotification) {
	t.mu.Lock()
	if t.notificationBudget.stopped.Load() {
		t.mu.Unlock()
		notif.release()
		return
	}
	handler := t.notifHandler
	panicFn := t.panicHandler
	if handler == nil {
		if len(t.pendingNotifHandle) >= inboundNotifQueueSize {
			evicted := t.pendingNotifHandle[0]
			t.pendingNotifHandle[0] = bufferedNotification{}
			t.pendingNotifHandle = append(t.pendingNotifHandle[1:], notif)
			evicted.release()
		} else {
			t.pendingNotifHandle = append(t.pendingNotifHandle, notif)
		}
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()

	defer notif.release()
	t.invokeNotificationHandler(notif.Notification, handler, panicFn)
}

func (t *StdioTransport) invokeNotificationHandler(notif Notification, handler NotificationHandler, panicFn func(any)) {
	defer func() {
		if r := recover(); r != nil {
			if panicFn != nil {
				panicFn(r)
			}
		}
	}()
	handler(t.ctx, notif)
}
