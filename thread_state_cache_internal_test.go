package codex

import (
	"fmt"
	"testing"
)

func TestCacheThreadStateEvictsLeastRecentlyUpdatedThread(t *testing.T) {
	client := &Client{
		threadStates:    make(map[string]Thread),
		threadStatePins: make(map[string]int),
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
		threadStates:    make(map[string]Thread),
		threadStatePins: make(map[string]int),
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

func TestPinnedThreadStateSurvivesEvictionPressure(t *testing.T) {
	client := &Client{
		threadStates:    make(map[string]Thread),
		threadStatePins: make(map[string]int),
	}

	client.cacheThreadState(Thread{ID: "thread-pinned"})
	client.pinThreadState("thread-pinned")

	for i := range maxCachedThreadStates + 10 {
		client.cacheThreadState(Thread{ID: fmt.Sprintf("thread-%02d", i)})
	}

	if _, ok := client.threadStateSnapshot("thread-pinned"); !ok {
		t.Fatal("expected pinned thread to remain cached")
	}
	if got := client.cachedUnpinnedThreadStatesForTest(); got != maxCachedThreadStates {
		t.Fatalf("cached unpinned thread count = %d, want %d", got, maxCachedThreadStates)
	}
}

func TestUnpinThreadStateAllowsDeferredEviction(t *testing.T) {
	client := &Client{
		threadStates:    make(map[string]Thread),
		threadStatePins: make(map[string]int),
	}

	client.cacheThreadState(Thread{ID: "thread-pinned"})
	client.pinThreadState("thread-pinned")

	for i := range maxCachedThreadStates + 1 {
		client.cacheThreadState(Thread{ID: fmt.Sprintf("thread-%02d", i)})
	}
	if _, ok := client.threadStateSnapshot("thread-pinned"); !ok {
		t.Fatal("expected pinned thread to remain cached before unpin")
	}

	client.unpinThreadState("thread-pinned")

	if _, ok := client.threadStateSnapshot("thread-pinned"); ok {
		t.Fatal("expected formerly pinned thread to be evicted after unpin")
	}
}

func (c *Client) cachedUnpinnedThreadStatesForTest() int {
	c.threadStateMu.RLock()
	defer c.threadStateMu.RUnlock()
	return c.cachedUnpinnedThreadStatesLocked()
}
