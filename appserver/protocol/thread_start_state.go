package protocol

import (
	"errors"
	"fmt"
)

// ErrThreadStartObservationLost means a valid start response was received, but
// bounded local history could not establish its observation ownership. The
// response remains available; callers must not assume the remote start failed.
var ErrThreadStartObservationLost = errors.New("thread/start observation history unavailable")

const maxPendingStartThreads = maxCachedThreadStates

type threadStartCandidate struct {
	generation uint64
	started    bool
	closed     bool
	ambiguous  bool
}

type threadStartObservation struct {
	candidates map[string]threadStartCandidate
	overflow   bool
}

func (c *Client) beginThreadStartObservation() *threadStartObservation {
	c.threadStateMu.Lock()
	defer c.threadStateMu.Unlock()
	if c.threadStarts == nil {
		c.threadStarts = make(map[*threadStartObservation]struct{})
	}
	observation := &threadStartObservation{candidates: make(map[string]threadStartCandidate)}
	c.threadStarts[observation] = struct{}{}
	return observation
}

// Capture happens before cache callbacks and eviction. Only startup identities
// and closures need history; shared cache pins retain their latest snapshots.
func (c *Client) captureThreadStartLocked(threadID string, generation uint64, closed bool) {
	for observation := range c.threadStarts {
		candidate, exists := observation.candidates[threadID]
		if !exists {
			if observation.overflow || len(observation.candidates) == maxPendingStartThreads ||
				(c.threadStartPins[threadID] == 0 && len(c.threadStartPins) == maxPendingStartThreads) {
				// Bound distinct retained snapshots across the client, including
				// staggered requests observing disjoint notification histories.
				// Once history is lost, freed slots cannot make it complete again.
				observation.overflow = true
				continue
			}
			candidate = threadStartCandidate{generation: generation, started: !closed, ambiguous: closed}
			if c.threadStartPins == nil {
				c.threadStartPins = make(map[string]int)
			}
			c.threadStartPins[threadID]++
		} else if !closed && (!candidate.started || candidate.generation != generation) {
			// Repeated remote starts using one opaque ID cannot be attributed to
			// this request without a correlation token in the notification.
			candidate.ambiguous = true
		}
		candidate.closed = candidate.closed || closed
		observation.candidates[threadID] = candidate
	}
}

func (c *Client) releaseThreadStartLocked(observation *threadStartObservation) {
	if _, active := c.threadStarts[observation]; !active {
		return
	}
	delete(c.threadStarts, observation)
	for threadID := range observation.candidates {
		c.threadStartPins[threadID]--
		if c.threadStartPins[threadID] == 0 {
			delete(c.threadStartPins, threadID)
		}
	}
	observation.candidates = nil
	if len(c.threadStarts) == 0 {
		c.threadStarts = nil
	}
	if len(c.threadStartPins) == 0 {
		c.threadStartPins = nil
	}
}

func (c *Client) abortThreadStartObservation(observation *threadStartObservation) {
	c.threadStateMu.Lock()
	defer c.threadStateMu.Unlock()
	c.releaseThreadStartLocked(observation)
	c.evictThreadStatesLocked()
	c.compactThreadStartOrderLocked()
}

func (c *Client) compactThreadStartOrderLocked() {
	if len(c.threadStarts) == 0 && cap(c.threadStateOrder) > 2*maxCachedThreadStates && cap(c.threadStateOrder) > 2*len(c.threadStateOrder) {
		c.threadStateOrder = append([]string(nil), c.threadStateOrder...)
	}
}

func (c *Client) finishThreadStartObservation(observation *threadStartObservation, thread Thread, onUpdate func(Thread), onClose func()) (uint64, func(), error) {
	snapshot := cloneThreadState(thread)
	c.threadStateMu.Lock()
	c.ensureThreadStateCacheMapsLocked()
	candidate, captured := observation.candidates[thread.ID]
	_, active := c.threadStarts[observation]
	if !active || (captured && candidate.ambiguous) || (!captured && observation.overflow) {
		c.releaseThreadStartLocked(observation)
		c.evictThreadStatesLocked()
		c.compactThreadStartOrderLocked()
		c.threadStateMu.Unlock()
		return 0, func() {}, fmt.Errorf("%w: missing or ambiguous startup incarnation", ErrThreadStartObservationLost)
	}
	var generation, id uint64
	var listeners []threadStateListener
	if !captured {
		generation, id, listeners = c.publishThreadStateLocked(methodThreadStart, snapshot, onUpdate, onClose)
	} else {
		generation = candidate.generation
		id, listeners = c.observeCapturedStartLocked(snapshot, candidate, onUpdate, onClose)
	}
	c.releaseThreadStartLocked(observation)
	c.evictThreadStatesLocked()
	c.compactThreadStartOrderLocked()
	c.threadStateMu.Unlock()
	c.drainThreadStateListeners(listeners)
	if id == 0 {
		return generation, func() {}, nil
	}
	return generation, func() { c.removeThreadStateListener(thread.ID, id) }, nil
}

func (c *Client) observeCapturedStartLocked(snapshot Thread, candidate threadStartCandidate, onUpdate func(Thread), onClose func()) (uint64, []threadStateListener) {
	var id uint64
	var listeners []threadStateListener
	entry := c.threadStates[snapshot.ID]
	if entry.hasSnapshot && !entry.closed && !candidate.closed && entry.generation == candidate.generation && entry.thread.DisabledPluginIDs == nil {
		// Started notifications lack lifecycle settings. Fill only unknown
		// settings; an admitted update (including []) takes precedence.
		entry.thread = cloneThreadState(entry.thread)
		entry.thread.DisabledPluginIDs = snapshot.DisabledPluginIDs
		c.threadStates[snapshot.ID] = entry
		listeners = c.queueThreadUpdateLocked(snapshot.ID, methodThreadStart, entry.thread)
	}
	if onUpdate != nil || onClose != nil {
		listener := c.newThreadStateListenerLocked(snapshot.ID, onUpdate, onClose)
		id = listener.id
		if candidate.closed && onClose != nil {
			listener.delivery.closeContext = notifyThreadClosed
		}
		if entry.hasSnapshot && !entry.closed && onUpdate != nil {
			listener.delivery.update = &threadStateUpdate{method: methodThreadStart, thread: entry.thread}
		}
		if c.claimThreadDeliveryLocked(listener) {
			listeners = append(listeners, listener)
		}
	}
	c.touchThreadStateLocked(snapshot.ID)
	return id, listeners
}
