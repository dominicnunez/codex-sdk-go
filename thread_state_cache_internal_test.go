package codex

import (
	"fmt"
	"testing"
)

func TestCacheThreadStateEvictsLeastRecentlyUpdatedThread(t *testing.T) {
	client := &Client{
		threadStates: make(map[string]Thread),
	}

	for i := range maxCachedThreadStates {
		client.cacheThreadState(Thread{ID: fmt.Sprintf("thread-%02d", i)})
	}

	client.cacheThreadState(Thread{ID: "thread-00"})
	client.cacheThreadState(Thread{ID: "thread-new"})

	if got := len(client.threadStates); got != maxCachedThreadStates {
		t.Fatalf("len(threadStates) = %d, want %d", got, maxCachedThreadStates)
	}
	if _, ok := client.threadStateSnapshot("thread-00"); !ok {
		t.Fatal("expected refreshed thread to remain cached")
	}
	if _, ok := client.threadStateSnapshot("thread-01"); ok {
		t.Fatal("expected oldest untouched thread to be evicted")
	}
	if _, ok := client.threadStateSnapshot("thread-new"); !ok {
		t.Fatal("expected newest thread to be cached")
	}
}

func TestMutateThreadStateRefreshesThreadRecency(t *testing.T) {
	client := &Client{
		threadStates: make(map[string]Thread),
	}

	for i := range maxCachedThreadStates {
		client.cacheThreadState(Thread{ID: fmt.Sprintf("thread-%02d", i)})
	}

	name := "updated"
	client.mutateThreadState("thread-00", func(thread *Thread) {
		thread.Name = &name
	})
	client.cacheThreadState(Thread{ID: "thread-new"})

	snapshot, ok := client.threadStateSnapshot("thread-00")
	if !ok {
		t.Fatal("expected mutated thread to remain cached")
	}
	if snapshot.Name == nil || *snapshot.Name != name {
		t.Fatalf("snapshot.Name = %v, want %q", snapshot.Name, name)
	}
	if _, ok := client.threadStateSnapshot("thread-01"); ok {
		t.Fatal("expected oldest untouched thread to be evicted")
	}
}

func TestThreadStateListenersReceiveCacheAndMutationUpdates(t *testing.T) {
	client := &Client{
		threadStates:         make(map[string]Thread),
		threadStateListeners: make(map[string][]threadStateListener),
	}

	var snapshots []Thread
	unsubscribe := client.addThreadStateListener("thread-1", func(thread Thread) {
		snapshots = append(snapshots, thread)
	})
	defer unsubscribe()

	client.cacheThreadState(Thread{ID: "thread-1"})
	name := "renamed"
	client.mutateThreadState("thread-1", func(thread *Thread) {
		thread.Name = &name
	})

	if got := len(snapshots); got != 2 {
		t.Fatalf("listener snapshots = %d, want 2", got)
	}
	if snapshots[0].ID != "thread-1" {
		t.Fatalf("first snapshot ID = %q, want thread-1", snapshots[0].ID)
	}
	if snapshots[1].Name == nil || *snapshots[1].Name != name {
		t.Fatalf("second snapshot Name = %v, want %q", snapshots[1].Name, name)
	}
}

func TestObservedThreadStateSurvivesEvictionPressure(t *testing.T) {
	client := &Client{
		threadStates:         make(map[string]Thread),
		threadStateListeners: make(map[string][]threadStateListener),
	}

	client.cacheThreadState(Thread{ID: "thread-observed"})
	unsubscribe := client.addThreadStateListener("thread-observed", func(Thread) {})
	defer unsubscribe()

	for i := range maxCachedThreadStates + 10 {
		client.cacheThreadState(Thread{ID: fmt.Sprintf("thread-%02d", i)})
	}

	if _, ok := client.threadStateSnapshot("thread-observed"); !ok {
		t.Fatal("expected observed thread to remain cached")
	}
	if got := client.cachedThreadStatesWithoutListenersForTest(); got != maxCachedThreadStates {
		t.Fatalf("cached thread count without listeners = %d, want %d", got, maxCachedThreadStates)
	}
}

func TestRemovingThreadStateListenerAllowsDeferredEviction(t *testing.T) {
	client := &Client{
		threadStates:         make(map[string]Thread),
		threadStateListeners: make(map[string][]threadStateListener),
	}

	client.cacheThreadState(Thread{ID: "thread-observed"})
	unsubscribe := client.addThreadStateListener("thread-observed", func(Thread) {})

	for i := range maxCachedThreadStates + 1 {
		client.cacheThreadState(Thread{ID: fmt.Sprintf("thread-%02d", i)})
	}
	if _, ok := client.threadStateSnapshot("thread-observed"); !ok {
		t.Fatal("expected observed thread to remain cached before listener removal")
	}

	unsubscribe()

	if _, ok := client.threadStateSnapshot("thread-observed"); ok {
		t.Fatal("expected formerly observed thread to be evicted after listener removal")
	}
}

func (c *Client) cachedThreadStatesWithoutListenersForTest() int {
	c.threadStateMu.RLock()
	defer c.threadStateMu.RUnlock()
	return c.cachedThreadStatesWithoutListenersLocked()
}
