package codex

import "sync"

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

func (t *StdioTransport) notificationWorker() {
	for {
		notif, ok := recvWhileRunning(t.ctx, t.notifQueue)
		if !ok {
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) streamingNotificationWorker() {
	for {
		notif, ok := recvWhileRunning(t.ctx, t.streamingNotifQueue)
		if !ok {
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) protectedNotificationWorker() {
	for {
		notif, ok := recvWhileRunning(t.ctx, t.protectedNotifQueue)
		if !ok {
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) criticalNotificationWorker() {
	for {
		notif, ok := recvWhileRunning(t.ctx, t.criticalNotifQueue)
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

	for len(t.turnNotifReady) == 0 && t.ctx.Err() == nil {
		t.turnNotifReadyCond.Wait()
	}
	if len(t.turnNotifReady) == 0 {
		return nil, false
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
	t.annotateTurnScopedNotification(&notif)
	if isTurnScopedNotification(notif) {
		t.enqueueTurnScopedNotification(notif)
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

	select {
	case <-t.ctx.Done():
		return
	case t.notifQueue <- notif:
	default:
		// Unknown notifications remain best-effort to preserve read-loop
		// liveness without changing known SDK-visible behavior.
	}
}

func (t *StdioTransport) enqueueTurnScopedNotification(notif Notification) {
	t.initTurnScopedScheduler()

	if notif.threadKey == "" {
		select {
		case <-t.ctx.Done():
			return
		case t.notifQueue <- notif:
		default:
			// Non-attributable notifications remain best-effort.
		}
		return
	}

	t.turnNotifQueuesMu.Lock()
	queue := t.turnNotifQueues[notif.threadKey]
	if queue == nil {
		if len(t.turnNotifQueues) >= maxTurnScopedNotificationQueues {
			t.turnNotifQueuesMu.Unlock()
			t.closeWithFailure(
				errTurnScopedNotificationQueueLimit,
				errTurnScopedNotificationQueueLimit,
			)
			return
		}
		queue = &turnScopedNotificationQueue{threadKey: notif.threadKey}
		t.turnNotifQueues[notif.threadKey] = queue
	}
	t.turnNotifQueuesMu.Unlock()

	queue.mu.Lock()
	if len(queue.queue) >= maxTurnScopedNotificationQueueSize {
		queue.mu.Unlock()
		t.closeWithFailure(
			errTurnScopedNotificationQueueOverflow,
			errTurnScopedNotificationQueueOverflow,
		)
		return
	}
	queue.queue = append(queue.queue, notif)
	if queue.scheduled {
		queue.mu.Unlock()
		return
	}
	queue.scheduled = true
	queue.mu.Unlock()

	if t.ctx.Err() != nil {
		queue.mu.Lock()
		queue.queue = nil
		queue.scheduled = false
		queue.mu.Unlock()
		t.removeTurnScopedNotificationQueue(queue.threadKey, queue)
		return
	}
	t.scheduleTurnScopedNotificationQueue(queue)
}

func (t *StdioTransport) handleTurnScopedNotificationQueue(queue *turnScopedNotificationQueue) {
	for {
		queue.mu.Lock()
		if len(queue.queue) == 0 {
			queue.scheduled = false
			queue.mu.Unlock()
			t.removeTurnScopedNotificationQueue(queue.threadKey, queue)
			return
		}
		notif := queue.queue[0]
		queue.queue[0] = Notification{}
		queue.queue = queue.queue[1:]
		queue.mu.Unlock()

		if t.ctx.Err() != nil {
			queue.mu.Lock()
			queue.queue = nil
			queue.scheduled = false
			queue.mu.Unlock()
			t.removeTurnScopedNotificationQueue(queue.threadKey, queue)
			return
		}
		t.handleNotification(notif)
	}
}

func (t *StdioTransport) removeTurnScopedNotificationQueue(threadKey string, queue *turnScopedNotificationQueue) {
	t.turnNotifQueuesMu.Lock()
	defer t.turnNotifQueuesMu.Unlock()

	current, ok := t.turnNotifQueues[threadKey]
	if !ok || current != queue {
		return
	}

	queue.mu.Lock()
	empty := len(queue.queue) == 0 && !queue.scheduled
	queue.mu.Unlock()
	if empty {
		delete(t.turnNotifQueues, threadKey)
	}
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

func isCriticalNotificationMethod(method string) bool {
	switch method {
	case notifyError, notifyRealtimeError:
		return true
	default:
		return false
	}
}

func isStreamingNotificationMethod(method string) bool {
	switch method {
	case notifyAgentMessageDelta,
		notifyFileChangeOutputDelta,
		notifyPlanDelta,
		notifyReasoningTextDelta,
		notifyReasoningSummaryTextDelta,
		notifyReasoningSummaryPartAdded,
		notifyRealtimeOutputAudioDelta,
		notifyCommandExecutionOutputDelta,
		notifyCommandExecOutputDelta:
		return true
	default:
		return false
	}
}

func isProtectedNotificationMethod(method string) bool {
	switch method {
	case notifyItemStarted,
		notifyThreadStarted,
		notifyThreadClosed,
		notifyThreadArchived,
		notifyThreadUnarchived,
		notifyThreadNameUpdated,
		notifyThreadStatusChanged,
		notifyThreadTokenUsageUpdated,
		notifyTurnStarted,
		notifyTurnPlanUpdated,
		notifyTurnDiffUpdated,
		notifyAccountUpdated,
		notifyAccountLoginCompleted,
		notifyAccountRateLimitsUpdated,
		notifyRealtimeStarted,
		notifyRealtimeClosed,
		notifyRealtimeItemAdded,
		notifyWindowsSandboxSetupCompleted,
		notifyWindowsWorldWritableWarning,
		notifyThreadCompacted,
		notifyDeprecationNotice,
		notifyTerminalInteraction,
		notifyMcpServerOauthLoginCompleted,
		notifyMcpToolCallProgress,
		notifyServerRequestResolved,
		notifyModelRerouted,
		notifyFuzzyFileSearchSessionCompleted,
		notifyFuzzyFileSearchSessionUpdated,
		notifyAppListUpdated,
		notifyConfigWarning,
		notifySkillsChanged,
		notifyHookStarted,
		notifyHookCompleted,
		notifyItemGuardianApprovalReviewStarted,
		notifyItemGuardianApprovalReviewCompleted:
		return true
	default:
		return false
	}
}

func isTurnScopedNotification(notif Notification) bool {
	switch notif.Method {
	case notifyItemCompleted, notifyTurnCompleted:
		return notif.threadKey != ""
	default:
		return false
	}
}

func (t *StdioTransport) annotateTurnScopedNotification(notif *Notification) {
	switch notif.Method {
	case notifyItemCompleted:
		notif.threadKey = itemCompletedThreadKey(notif.Params)
	case notifyTurnCompleted:
		notif.threadKey = turnCompletedThreadKey(notif.Params)
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
		drained = t.drainTurnScopedNotificationQueues() || drained

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

func (t *StdioTransport) drainTurnScopedNotificationQueues() bool {
	t.turnNotifQueuesMu.Lock()
	queues := make([]*turnScopedNotificationQueue, 0, len(t.turnNotifQueues))
	for _, queue := range t.turnNotifQueues {
		queues = append(queues, queue)
	}
	t.turnNotifQueuesMu.Unlock()

	drained := false
	for _, queue := range queues {
		for {
			queue.mu.Lock()
			if len(queue.queue) == 0 {
				queue.scheduled = false
				queue.mu.Unlock()
				t.removeTurnScopedNotificationQueue(queue.threadKey, queue)
				break
			}
			notif := queue.queue[0]
			queue.queue[0] = Notification{}
			queue.queue = queue.queue[1:]
			queue.mu.Unlock()

			t.handleNotification(notif)
			drained = true
		}
	}

	return drained
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

	defer func() {
		if r := recover(); r != nil {
			if panicFn != nil {
				panicFn(r)
			}
		}
	}()
	handler(t.ctx, notif)
}
