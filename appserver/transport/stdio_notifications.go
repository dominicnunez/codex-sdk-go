package transport

import (
	"sync"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type turnScopedNotificationQueue struct {
	mu        sync.Mutex
	threadKey string
	queue     []Notification
	scheduled bool
}

type streamingNotificationBacklog struct {
	mu       sync.Mutex
	queue    []Notification
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
	t.turnNotifReadyMu.Lock()
	t.turnNotifStopped.Store(true)
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

func (t *StdioTransport) handleNotificationQueue(queue <-chan Notification) {
	for {
		notif, ok := recvWhileRunning(t.ctx, queue)
		if !ok {
			return
		}
		t.handleNotification(notif)
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
	t.turnNotifReady = append(t.turnNotifReady, queue)
	t.turnNotifReadyMu.Unlock()
	t.turnNotifReadyCond.Signal()
}

func (t *StdioTransport) enqueueNotification(notif Notification) {
	if threadKey := orderedNotificationKey(notif); threadKey != "" {
		t.enqueueTurnScopedNotification(notif, threadKey)
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

func (t *StdioTransport) enqueueTurnScopedNotification(notif Notification, threadKey string) {
	t.initTurnScopedScheduler()

	if threadKey == "" {
		// Non-attributable notifications remain best-effort.
		t.enqueueBestEffortNotification(t.notifQueue, notif)
		return
	}

	t.turnNotifQueuesMu.Lock()
	queue := t.turnNotifQueues[threadKey]
	if queue == nil {
		if len(t.turnNotifQueues) >= maxTurnScopedNotificationQueues {
			t.turnNotifQueuesMu.Unlock()
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
		t.closeWithFailure(
			errTurnScopedNotificationQueueOverflow,
			errTurnScopedNotificationQueueOverflow,
		)
		return
	}
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
			t.invokeNotificationHandler(notif, handler, panicFn)
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
		queue.mu.Unlock()
		t.turnNotifQueuesMu.Unlock()
		return
	}
	queue.mu.Unlock()
	t.turnNotifQueuesMu.Unlock()
	t.scheduleTurnScopedNotificationQueue(queue)
}

func (t *StdioTransport) enqueueLosslessNotification(
	queue chan Notification,
	notif Notification,
) {
	select {
	case <-t.ctx.Done():
		return
	case queue <- notif:
		return
	default:
	}
	t.closeWithFailure(errNotificationQueueOverflow, errNotificationQueueOverflow)
}

func (t *StdioTransport) enqueueBestEffortNotification(queue chan Notification, notif Notification) {
	select {
	case <-t.ctx.Done():
		return
	case queue <- notif:
	default:
	}
}

func (t *StdioTransport) enqueueStreamingNotification(notif Notification) {
	var startDrainer bool

	t.streamingBacklog.mu.Lock()
	if len(t.streamingBacklog.queue) == 0 && !t.streamingBacklog.draining {
		select {
		case <-t.ctx.Done():
			t.streamingBacklog.mu.Unlock()
			return
		case t.streamingNotifQueue <- notif:
			t.streamingBacklog.mu.Unlock()
			return
		default:
		}
	}
	if len(t.streamingBacklog.queue) >= maxStreamingNotificationBacklog {
		t.streamingBacklog.mu.Unlock()
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
			return
		case t.streamingNotifQueue <- notif:
		}
	}
}

func (t *StdioTransport) nextStreamingBacklogNotification() (Notification, bool) {
	t.streamingBacklog.mu.Lock()
	defer t.streamingBacklog.mu.Unlock()

	if len(t.streamingBacklog.queue) == 0 {
		t.streamingBacklog.draining = false
		return Notification{}, false
	}

	notif := t.streamingBacklog.queue[0]
	t.streamingBacklog.queue[0] = Notification{}
	t.streamingBacklog.queue = t.streamingBacklog.queue[1:]
	return notif, true
}

func (t *StdioTransport) enqueueProtectedNotification(notif Notification) {
	t.enqueueLosslessNotification(t.protectedNotifQueue, notif)
}

func (t *StdioTransport) enqueueCriticalNotification(notif Notification) {
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

func (t *StdioTransport) drainNotificationQueue(queue chan Notification) bool {
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
	t.streamingBacklog.mu.Lock()
	if len(t.streamingBacklog.queue) == 0 {
		t.streamingBacklog.draining = false
		t.streamingBacklog.mu.Unlock()
		return false
	}
	queue := append([]Notification(nil), t.streamingBacklog.queue...)
	t.streamingBacklog.queue = nil
	t.streamingBacklog.draining = false
	t.streamingBacklog.mu.Unlock()

	for _, notif := range queue {
		t.handleNotification(notif)
	}
	return true
}

func (t *StdioTransport) dequeueTurnScopedNotification(queue *turnScopedNotificationQueue) (Notification, bool) {
	t.turnNotifQueuesMu.Lock()
	defer t.turnNotifQueuesMu.Unlock()
	queue.mu.Lock()
	if len(queue.queue) == 0 {
		queue.mu.Unlock()
		return Notification{}, false
	}
	notif := queue.queue[0]
	queue.queue[0] = Notification{}
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
	queue.queue = nil
	queue.scheduled = false
	queue.mu.Unlock()
}

// handleNotification dispatches an incoming server→client notification to the handler
func (t *StdioTransport) handleNotification(notif Notification) {
	t.mu.Lock()
	handler := t.notifHandler
	panicFn := t.panicHandler
	t.mu.Unlock()

	if handler == nil {
		t.mu.Lock()
		if t.notifHandler == nil {
			if len(t.pendingNotifHandle) >= inboundNotifQueueSize {
				t.pendingNotifHandle = append(t.pendingNotifHandle[1:], notif)
			} else {
				t.pendingNotifHandle = append(t.pendingNotifHandle, notif)
			}
			t.mu.Unlock()
			return
		}
		handler = t.notifHandler
		panicFn = t.panicHandler
		t.mu.Unlock()
	}

	t.invokeNotificationHandler(notif, handler, panicFn)
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
