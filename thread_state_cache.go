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

	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	c.threadStates[thread.ID] = cloneThreadState(thread)
	c.touchThreadStateLocked(thread.ID)
	c.evictThreadStatesLocked()
	c.threadStateMu.Unlock()
}

func (c *Client) pinThreadState(threadID string) {
	if threadID == "" {
		return
	}

	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	c.threadStatePins[threadID]++
	c.evictThreadStatesLocked()
	c.threadStateMu.Unlock()
}

func (c *Client) unpinThreadState(threadID string) {
	if threadID == "" {
		return
	}

	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	if pins := c.threadStatePins[threadID]; pins <= 1 {
		delete(c.threadStatePins, threadID)
	} else {
		c.threadStatePins[threadID] = pins - 1
	}
	c.evictThreadStatesLocked()
	c.threadStateMu.Unlock()
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

	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	thread, ok := c.threadStates[threadID]
	if ok {
		mutate(&thread)
		c.threadStates[threadID] = thread
		c.touchThreadStateLocked(threadID)
		c.evictThreadStatesLocked()
	}
	c.threadStateMu.Unlock()
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
	for c.cachedUnpinnedThreadStatesLocked() > maxCachedThreadStates {
		evictedIndex := -1
		for i, threadID := range c.threadStateOrder {
			if c.threadStatePins[threadID] == 0 {
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

func (c *Client) cachedUnpinnedThreadStatesLocked() int {
	count := 0
	for _, threadID := range c.threadStateOrder {
		if c.threadStatePins[threadID] == 0 {
			count++
		}
	}
	return count
}

func (c *Client) ensureThreadStateCacheMapsLocked() {
	if c.threadStates == nil {
		c.threadStates = make(map[string]Thread)
	}
	if c.threadStatePins == nil {
		c.threadStatePins = make(map[string]int)
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
