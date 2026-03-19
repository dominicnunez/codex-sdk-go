package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

const maxCachedThreadStates = 64

func (c *Client) cacheThreadState(thread Thread) {
	if thread.ID == "" {
		return
	}

	snapshot := cloneThreadState(thread)
	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	c.threadStates[thread.ID] = snapshot
	c.touchThreadStateLocked(thread.ID)
	c.evictThreadStatesLocked()
	listeners := c.threadStateListenersLocked(thread.ID)
	c.threadStateMu.Unlock()

	c.notifyThreadStateListeners(snapshot, listeners)
}

func (c *Client) threadStateSnapshot(threadID string) (Thread, bool) {
	c.threadStateMu.RLock()
	thread, ok := c.threadStates[threadID]
	c.threadStateMu.RUnlock()
	if !ok {
		return Thread{}, false
	}
	return cloneThreadState(thread), true
}

func (c *Client) mutateThreadState(threadID string, mutate func(*Thread)) {
	if threadID == "" {
		return
	}

	var (
		snapshot  Thread
		ok        bool
		listeners []threadStateListener
	)

	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	thread, ok := c.threadStates[threadID]
	if ok {
		mutate(&thread)
		snapshot = cloneThreadState(thread)
		c.threadStates[threadID] = snapshot
		c.touchThreadStateLocked(threadID)
		c.evictThreadStatesLocked()
		listeners = c.threadStateListenersLocked(threadID)
	}
	c.threadStateMu.Unlock()

	if ok {
		c.notifyThreadStateListeners(snapshot, listeners)
	}
}

func (c *Client) addThreadStateListener(threadID string, handler func(Thread)) func() {
	if threadID == "" || handler == nil {
		return func() {}
	}

	var snapshot *Thread

	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	c.threadStateListenerSeq++
	id := c.threadStateListenerSeq
	c.threadStateListeners[threadID] = append(c.threadStateListeners[threadID], threadStateListener{
		id:      id,
		handler: handler,
	})
	if thread, ok := c.threadStates[threadID]; ok {
		cp := cloneThreadState(thread)
		snapshot = &cp
	}
	c.evictThreadStatesLocked()
	c.threadStateMu.Unlock()

	if snapshot != nil {
		handler(*snapshot)
	}

	return func() {
		c.threadStateMu.Lock()
		defer c.threadStateMu.Unlock()
		listeners := c.threadStateListeners[threadID]
		for i, listener := range listeners {
			if listener.id != id {
				continue
			}
			c.threadStateListeners[threadID] = append(listeners[:i], listeners[i+1:]...)
			if len(c.threadStateListeners[threadID]) == 0 {
				delete(c.threadStateListeners, threadID)
			}
			c.evictThreadStatesLocked()
			break
		}
	}
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
		c.threadStates = make(map[string]Thread)
	}
	if c.threadStateListeners == nil {
		c.threadStateListeners = make(map[string][]threadStateListener)
	}
}

func (c *Client) threadStateListenersLocked(threadID string) []threadStateListener {
	src := c.threadStateListeners[threadID]
	if len(src) == 0 {
		return nil
	}
	listeners := make([]threadStateListener, len(src))
	copy(listeners, src)
	return listeners
}

func (c *Client) notifyThreadStateListeners(thread Thread, listeners []threadStateListener) {
	for _, listener := range listeners {
		listener.handler(cloneThreadState(thread))
	}
}

func (c *Client) installThreadStateCache() {
	c.addNotificationListener(notifyThreadStarted, func(_ context.Context, notif Notification) {
		var n ThreadStartedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyThreadStarted, fmt.Errorf("unmarshal %s: %w", notifyThreadStarted, err))
			return
		}
		c.cacheThreadState(n.Thread)
	})

	c.addNotificationListener(notifyThreadNameUpdated, func(_ context.Context, notif Notification) {
		var n ThreadNameUpdatedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyThreadNameUpdated, fmt.Errorf("unmarshal %s: %w", notifyThreadNameUpdated, err))
			return
		}
		c.mutateThreadState(n.ThreadID, func(thread *Thread) {
			thread.Name = cloneStringPtr(n.ThreadName)
		})
	})

	c.addNotificationListener(notifyThreadStatusChanged, func(_ context.Context, notif Notification) {
		var n ThreadStatusChangedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyThreadStatusChanged, fmt.Errorf("unmarshal %s: %w", notifyThreadStatusChanged, err))
			return
		}
		c.mutateThreadState(n.ThreadID, func(thread *Thread) {
			thread.Status = cloneThreadStatusWrapper(n.Status)
		})
	})
}
