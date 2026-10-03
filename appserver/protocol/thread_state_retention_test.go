package protocol

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestThreadStatePendingDeliveryRetention(t *testing.T) {
	c := NewClient(&mockInternalTransport{})
	defer c.Close()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var received []string
	remove := c.AddThreadStateListener("thread", func(thread Thread) {
		if thread.Preview == "executing" {
			close(entered)
			<-release
		}
		received = append(received, thread.Preview)
	}, func() { received = append(received, "closed") })
	c.threadStateMu.Lock()
	backing := c.threadStateListeners["thread"]
	delivery := backing[0].delivery
	c.threadStateMu.Unlock()
	go func() { c.CacheThreadState(Thread{ID: "thread", Preview: "executing"}); close(done) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("callback did not start")
	}
	for i := range 50 {
		c.CacheThreadState(Thread{ID: "thread", Preview: fmt.Sprintf("before-close-%d", i)})
		c.closeThreadState("thread")
		c.CacheThreadState(Thread{ID: "thread", Preview: fmt.Sprintf("reopened-%d", i)})
	}
	c.threadStateMu.Lock()
	pending := delivery.active && delivery.update != nil && delivery.update.thread.Preview == "reopened-49" && delivery.closeContext == notifyThreadClosed
	c.threadStateMu.Unlock()
	if !pending {
		t.Fatal("pending delivery did not retain only latest update and sticky closure")
	}
	remove()
	c.CacheThreadState(Thread{ID: "thread", Preview: "after removal"})
	c.threadStateMu.Lock()
	retained := delivery.update != nil && delivery.update.thread.Preview == "reopened-49"
	cleared := reflect.DeepEqual(backing[0], threadStateListener{}) && len(c.threadStateListeners["thread"]) == 0
	c.threadStateMu.Unlock()
	if !retained || !cleared {
		t.Fatal("unsubscribe changed admitted work or retained registry callback")
	}
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("selected delivery did not drain")
	}
	if !reflect.DeepEqual(received, []string{"executing", "closed", "reopened-49"}) {
		t.Fatalf("bounded delivery=%v", received)
	}
	c.threadStateMu.Lock()
	released := !delivery.active && delivery.update == nil && delivery.closeContext == ""
	c.threadStateMu.Unlock()
	if !released {
		t.Fatal("drained delivery retained pending state")
	}
}
