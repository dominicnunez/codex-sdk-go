package transport

import (
	"errors"
	"sync"
	"sync/atomic"
)

// Count limits bound queue bookkeeping. This shared byte limit separately
// bounds retained notification data, including callbacks and pending replay.
const maxRetainedNotificationBytes = 64 * 1024 * 1024

var errNotificationByteLimit = errors.New("retained notification byte limit exceeded")

type notificationBudget struct {
	mu      sync.Mutex
	bytes   int
	stopped atomic.Bool
}

type notificationReservation struct {
	budget *notificationBudget
	bytes  int
	once   sync.Once
}

type bufferedNotification struct {
	Notification
	scopeKey    string
	reservation *notificationReservation
}

func (b *notificationBudget) reserve(size int) (*notificationReservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped.Load() {
		return nil, errTransportClosed
	}
	if size > maxRetainedNotificationBytes-b.bytes {
		return nil, errNotificationByteLimit
	}
	b.bytes += size
	return &notificationReservation{budget: b, bytes: size}, nil
}

func (n bufferedNotification) release() {
	if r := n.reservation; r != nil {
		r.once.Do(func() {
			r.budget.mu.Lock()
			r.budget.bytes -= r.bytes
			r.budget.mu.Unlock()
		})
	}
}

func (t *StdioTransport) stopNotificationWorkers() {
	t.notificationBudget.mu.Lock()
	t.notificationBudget.stopped.Store(true)
	t.notificationBudget.mu.Unlock()
	t.stopTurnScopedNotificationWorkers()
	t.mu.Lock()
	pending := t.pendingNotifHandle
	t.pendingNotifHandle = nil
	t.mu.Unlock()
	releaseBufferedNotifications(pending)
	t.streamingBacklog.mu.Lock()
	backlog := t.streamingBacklog.queue
	t.streamingBacklog.queue = nil
	t.streamingBacklog.mu.Unlock()
	releaseBufferedNotifications(backlog)
	t.discardNotificationQueue(t.notifQueue)
	t.discardNotificationQueue(t.streamingNotifQueue)
	t.discardNotificationQueue(t.protectedNotifQueue)
	t.discardNotificationQueue(t.criticalNotifQueue)
}

func releaseBufferedNotifications(queue []bufferedNotification) {
	for i := range queue {
		notif := queue[i]
		queue[i] = bufferedNotification{}
		notif.release()
	}
}

func (t *StdioTransport) discardNotificationQueue(queue chan bufferedNotification) {
	for {
		select {
		case notif := <-queue:
			notif.release()
		default:
			return
		}
	}
}
