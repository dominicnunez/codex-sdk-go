package protocol

import (
	"context"
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
)

const maxCachedThreadStates = 64

const threadStateUpdateContext = "CacheThreadState"
const threadStateListenerContext = "AddThreadStateListener"

func (c *Client) cacheThreadState(thread Thread) {
	c.cacheThreadStateForMethod(threadStateUpdateContext, thread)
}

func (c *Client) cacheThreadStateForMethod(method string, thread Thread) {
	c.cacheThreadStateWithListener(method, thread, nil, nil)
}

// Publication and optional registration share the same admission lock, so a
// start response cannot be evicted between its publication and observation.
func (c *Client) cacheThreadStateWithListener(method string, thread Thread, onUpdate func(Thread), onClose func()) (uint64, func()) {
	threadID := thread.ID
	if threadID == "" {
		return 0, func() {}
	}

	snapshot := cloneThreadState(thread)
	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	entry := c.threadStates[thread.ID]
	if !entry.hasSnapshot || entry.closed {
		c.threadStateGeneration++
		entry.generation = c.threadStateGeneration
	}
	if snapshot.DisabledPluginIDs == nil {
		// Thread-only reads and notifications do not carry lifecycle settings.
		// Preserve the last settings without sharing ownership with the old entry.
		snapshot.DisabledPluginIDs = cloneArbitraryValue(c.threadStates[thread.ID].thread.DisabledPluginIDs)
	}
	c.threadStates[thread.ID] = threadStateEntry{
		thread:      snapshot,
		hasSnapshot: true,
		generation:  entry.generation,
	}
	var id uint64
	if onUpdate != nil || onClose != nil {
		id = c.newThreadStateListenerLocked(thread.ID, onUpdate, onClose).id
	}
	c.touchThreadStateLocked(thread.ID)
	listeners := c.queueThreadUpdateLocked(thread.ID, method, snapshot)
	c.evictThreadStatesLocked()
	c.threadStateMu.Unlock()

	c.drainThreadStateListeners(listeners)
	if id == 0 {
		return entry.generation, func() {}
	}
	return entry.generation, func() { c.removeThreadStateListener(threadID, id) }
}

// CacheThreadState stores a best-effort latest snapshot for a thread.
// Listener panics are reported with "CacheThreadState" as the error context.
func (c *Client) CacheThreadState(thread Thread) {
	c.cacheThreadState(thread)
}

func (c *Client) threadStateSnapshot(threadID string) (Thread, bool) {
	c.threadStateMu.RLock()
	entry, ok := c.threadStates[threadID]
	c.threadStateMu.RUnlock()
	if !ok || !entry.hasSnapshot || entry.closed {
		return Thread{}, false
	}
	return cloneThreadState(entry.thread), true
}

// ThreadStateSnapshot returns the latest cached thread snapshot, if one exists.
func (c *Client) ThreadStateSnapshot(threadID string) (Thread, bool) {
	return c.threadStateSnapshot(threadID)
}

// ThreadStateGeneration returns the current open cache incarnation, or zero if
// absent or closed. It is local to this Client, remains stable across updates,
// and changes after eviction or closure/reopening. It is not a remote revision,
// durable identity, or authorization token.
func (c *Client) ThreadStateGeneration(threadID string) uint64 {
	c.threadStateMu.RLock()
	defer c.threadStateMu.RUnlock()
	entry := c.threadStates[threadID]
	if !entry.hasSnapshot || entry.closed {
		return 0
	}
	return entry.generation
}

// CompleteThreadTurn appends an owned turn to the latest open snapshot only if
// its cache incarnation still matches generation. It atomically preserves
// intervening metadata updates and admits listener delivery. Absence, eviction,
// closure or an incarnation mismatch returns false without publishing state.
// The returned snapshot describes this commit, even if reentrant listeners
// subsequently publish newer state. Each call appends once; callers must not
// retry the same completion expecting deduplication by turn ID.
func (c *Client) CompleteThreadTurn(threadID string, generation uint64, turn Turn) (Thread, bool) {
	if generation == 0 {
		return Thread{}, false
	}
	c.threadStateMu.Lock()
	entry, ok := c.threadStates[threadID]
	if !ok || !entry.hasSnapshot || entry.closed || entry.generation != generation {
		c.threadStateMu.Unlock()
		return Thread{}, false
	}
	snapshot := cloneThreadState(entry.thread)
	snapshot.Turns = append(snapshot.Turns, cloneArbitraryValue(turn))
	entry.thread = snapshot
	c.threadStates[threadID] = entry
	c.touchThreadStateLocked(threadID)
	c.evictThreadStatesLocked()
	listeners := c.queueThreadUpdateLocked(threadID, threadStateUpdateContext, snapshot)
	c.threadStateMu.Unlock()
	c.drainThreadStateListeners(listeners)
	return cloneThreadState(snapshot), true
}

func (c *Client) mutateThreadState(threadID string, mutate func(*Thread)) {
	c.mutateThreadStateForMethod(threadStateUpdateContext, threadID, mutate)
}

func (c *Client) mutateThreadStateForMethod(method string, threadID string, mutate func(*Thread)) {
	if threadID == "" {
		return
	}

	var (
		snapshot  Thread
		updated   bool
		listeners []threadStateListener
	)

	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	entry, ok := c.threadStates[threadID]
	if ok && entry.hasSnapshot && !entry.closed {
		// Pending deliveries may still refer to the previous immutable snapshot.
		entry.thread = cloneThreadState(entry.thread)
		mutate(&entry.thread)
		snapshot = entry.thread
		entry.thread = snapshot
		c.threadStates[threadID] = entry
		c.touchThreadStateLocked(threadID)
		c.evictThreadStatesLocked()
		listeners = c.queueThreadUpdateLocked(threadID, method, snapshot)
		updated = true
	}
	c.threadStateMu.Unlock()

	if updated {
		c.drainThreadStateListeners(listeners)
	}
}

func (c *Client) closeThreadState(threadID string) {
	if threadID == "" {
		return
	}

	var listeners []threadStateListener

	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	entry := c.threadStates[threadID]
	if entry.closed {
		c.threadStateMu.Unlock()
		return
	}
	entry.thread = Thread{}
	entry.hasSnapshot = false
	entry.closed = true
	c.threadStates[threadID] = entry
	c.touchThreadStateLocked(threadID)
	c.evictThreadStatesLocked()
	listeners = c.queueThreadCloseLocked(threadID, notifyThreadClosed)
	c.threadStateMu.Unlock()

	c.drainThreadStateListeners(listeners)
}

func (c *Client) addThreadStateListener(threadID string, onUpdate func(Thread), onClose func()) func() {
	if threadID == "" || (onUpdate == nil && onClose == nil) {
		return func() {}
	}

	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	listener := c.newThreadStateListenerLocked(threadID, onUpdate, onClose)
	if entry, ok := c.threadStates[threadID]; ok {
		switch {
		case entry.closed && onClose != nil:
			listener.delivery.closeContext = threadStateListenerContext
		case entry.hasSnapshot && onUpdate != nil:
			listener.delivery.update = &threadStateUpdate{method: threadStateListenerContext, thread: entry.thread}
		}
	}
	active := c.claimThreadDeliveryLocked(listener)
	c.evictThreadStatesLocked()
	c.threadStateMu.Unlock()

	if active {
		c.drainThreadStateListener(listener)
	}

	id := listener.id
	return func() { c.removeThreadStateListener(threadID, id) }
}

func (c *Client) newThreadStateListenerLocked(threadID string, onUpdate func(Thread), onClose func()) threadStateListener {
	c.threadStateListenerSeq++
	listener := threadStateListener{
		id: c.threadStateListenerSeq, onUpdate: onUpdate, onClose: onClose,
		delivery: &threadStateDelivery{},
	}
	c.threadStateListeners[threadID] = append(c.threadStateListeners[threadID], listener)
	return listener
}

func (c *Client) removeThreadStateListener(threadID string, id uint64) {
	c.threadStateMu.Lock()
	defer c.threadStateMu.Unlock()
	listeners := c.threadStateListeners[threadID]
	for i, listener := range listeners {
		if listener.id != id {
			continue
		}
		copy(listeners[i:], listeners[i+1:])
		listeners[len(listeners)-1] = threadStateListener{}
		c.threadStateListeners[threadID] = listeners[:len(listeners)-1]
		if len(c.threadStateListeners[threadID]) == 0 {
			delete(c.threadStateListeners, threadID)
		}
		c.evictThreadStatesLocked()
		break
	}
}

// AddThreadStateListener subscribes to cached thread-state updates for threadID.
// Each listener runs serially outside the cache lock. Reentrant or concurrent
// writes can return before a busy listener receives them; pending updates are
// coalesced to the latest snapshot. Closure supersedes earlier pending updates
// and is delivered before a subsequent reopened snapshot. Repeated close/reopen
// transitions while busy may coalesce to one closure and the latest update.
// Unsubscribe prevents new admission; already admitted callbacks may finish.
// Listener panics are reported through WithHandlerErrorCallback without stopping
// other listeners. Initial delivery uses "AddThreadStateListener" as its context.
func (c *Client) AddThreadStateListener(threadID string, onUpdate func(Thread), onClose func()) func() {
	return c.addThreadStateListener(threadID, onUpdate, onClose)
}

func (c *Client) touchThreadStateLocked(threadID string) {
	for i, id := range c.threadStateOrder {
		if id != threadID {
			continue
		}
		copy(c.threadStateOrder[i:], c.threadStateOrder[i+1:])
		c.threadStateOrder = c.threadStateOrder[:len(c.threadStateOrder)-1]
		break
	}
	c.threadStateOrder = append(c.threadStateOrder, threadID)
}

func (c *Client) evictThreadStatesLocked() {
	for c.cachedThreadStatesWithoutListenersLocked() > maxCachedThreadStates {
		evictedIndex := -1
		for i, threadID := range c.threadStateOrder {
			if len(c.threadStateListeners[threadID]) == 0 {
				evictedIndex = i
				break
			}
		}
		if evictedIndex < 0 {
			return
		}
		evictedID := c.threadStateOrder[evictedIndex]
		copy(c.threadStateOrder[evictedIndex:], c.threadStateOrder[evictedIndex+1:])
		c.threadStateOrder = c.threadStateOrder[:len(c.threadStateOrder)-1]
		delete(c.threadStates, evictedID)
	}
}

func (c *Client) cachedThreadStatesWithoutListenersLocked() int {
	count := 0
	for _, threadID := range c.threadStateOrder {
		if len(c.threadStateListeners[threadID]) == 0 {
			count++
		}
	}
	return count
}

func (c *Client) ensureThreadStateCacheMapsLocked() {
	if c.threadStates == nil {
		c.threadStates = make(map[string]threadStateEntry)
	}
	if c.threadStateListeners == nil {
		c.threadStateListeners = make(map[string][]threadStateListener)
	}
}

func (c *Client) queueThreadUpdateLocked(threadID, method string, thread Thread) []threadStateListener {
	var drainers []threadStateListener
	for _, listener := range c.threadStateListeners[threadID] {
		if listener.onUpdate != nil {
			listener.delivery.update = &threadStateUpdate{method: method, thread: thread}
		}
		if c.claimThreadDeliveryLocked(listener) {
			drainers = append(drainers, listener)
		}
	}
	return drainers
}

func (c *Client) queueThreadCloseLocked(threadID, method string) []threadStateListener {
	var drainers []threadStateListener
	for _, listener := range c.threadStateListeners[threadID] {
		listener.delivery.update = nil
		if listener.onClose != nil {
			listener.delivery.closeContext = method
		}
		if c.claimThreadDeliveryLocked(listener) {
			drainers = append(drainers, listener)
		}
	}
	return drainers
}

func (c *Client) claimThreadDeliveryLocked(listener threadStateListener) bool {
	delivery := listener.delivery
	if delivery.active || (delivery.update == nil && delivery.closeContext == "") {
		return false
	}
	delivery.active = true
	return true
}

func (c *Client) drainThreadStateListeners(listeners []threadStateListener) {
	for _, listener := range listeners {
		c.drainThreadStateListener(listener)
	}
}

func (c *Client) drainThreadStateListener(listener threadStateListener) {
	for {
		c.threadStateMu.Lock()
		delivery := listener.delivery
		method := delivery.closeContext
		var update *threadStateUpdate
		if method != "" {
			delivery.closeContext = ""
		} else {
			update = delivery.update
			delivery.update = nil
		}
		if method == "" && update == nil {
			delivery.active = false
			c.threadStateMu.Unlock()
			return
		}
		c.threadStateMu.Unlock()

		if method != "" {
			c.safeCallNotificationHandler(method, listener.onClose)
		} else {
			c.safeCallNotificationHandler(update.method, func() { listener.onUpdate(cloneThreadState(update.thread)) })
		}
	}
}

func (c *Client) installThreadStateCache() {
	c.addNotificationListener(notifyThreadSettingsUpdated, func(_ context.Context, notif Notification) {
		var n ThreadSettingsUpdatedNotification
		if err := jsondecode.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyThreadSettingsUpdated, fmt.Errorf("unmarshal %s: %w", notifyThreadSettingsUpdated, err))
			return
		}
		c.mutateThreadStateForMethod(notif.Method, n.ThreadID, func(thread *Thread) {
			*thread = threadWithDisabledPlugins(*thread, n.ThreadSettings.DisabledPluginIDs)
		})
	})

	c.addNotificationListener(notifyThreadProjectUpdated, func(_ context.Context, notif Notification) {
		var n ThreadProjectUpdatedNotification
		if err := jsondecode.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyThreadProjectUpdated, fmt.Errorf("unmarshal %s: %w", notifyThreadProjectUpdated, err))
			return
		}
		c.mutateThreadStateForMethod(notif.Method, n.ThreadID, func(thread *Thread) { thread.ProjectID = cloneStringPtr(n.ProjectID) })
	})

	c.addNotificationListener(notifyThreadStarted, func(_ context.Context, notif Notification) {
		var n ThreadStartedNotification
		if err := jsondecode.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyThreadStarted, fmt.Errorf("unmarshal %s: %w", notifyThreadStarted, err))
			return
		}
		c.cacheThreadStateForMethod(notif.Method, n.Thread)
	})

	c.addNotificationListener(notifyThreadNameUpdated, func(_ context.Context, notif Notification) {
		var n ThreadNameUpdatedNotification
		if err := jsondecode.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyThreadNameUpdated, fmt.Errorf("unmarshal %s: %w", notifyThreadNameUpdated, err))
			return
		}
		c.mutateThreadStateForMethod(notif.Method, n.ThreadID, func(thread *Thread) {
			thread.Name = cloneStringPtr(n.ThreadName)
		})
	})

	c.addNotificationListener(notifyThreadStatusChanged, func(_ context.Context, notif Notification) {
		var n ThreadStatusChangedNotification
		if err := jsondecode.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyThreadStatusChanged, fmt.Errorf("unmarshal %s: %w", notifyThreadStatusChanged, err))
			return
		}
		c.mutateThreadStateForMethod(notif.Method, n.ThreadID, func(thread *Thread) {
			thread.Status = cloneThreadStatusWrapper(n.Status)
		})
	})

	c.addNotificationListener(notifyThreadClosed, func(_ context.Context, notif Notification) {
		var n ThreadClosedNotification
		if err := jsondecode.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyThreadClosed, fmt.Errorf("unmarshal %s: %w", notifyThreadClosed, err))
			return
		}
		c.closeThreadState(n.ThreadID)
	})
}
