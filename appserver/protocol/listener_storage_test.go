package protocol

import (
	"context"
	"reflect"
	"sync"
	"testing"
)

func TestListenerUnsubscribeReleasesStorage(t *testing.T) {
	for _, owner := range []string{"notification", "thread-state"} {
		for _, removed := range []int{0, 1, 2} {
			t.Run(owner+string(rune('0'+removed)), func(t *testing.T) {
				c := NewClient(&mockInternalTransport{})
				var calls []int
				var unsub []func()
				for i := range 3 {
					if owner == "notification" {
						unsub = append(unsub, c.AddNotificationListener("storage", func(context.Context, Notification) { calls = append(calls, i) }))
					} else {
						unsub = append(unsub, c.AddThreadStateListener("storage", func(Thread) { calls = append(calls, i) }, func() { calls = append(calls, i) }))
					}
				}
				// Keep the original backing array to inspect references beyond len.
				notifications := c.internalListeners["storage"]
				states := c.threadStateListeners["storage"]
				checkTail := func(length int) {
					t.Helper()
					if owner == "notification" {
						for _, listener := range notifications[length:] {
							if listener.handler != nil || listener.id != 0 {
								t.Errorf("removed notification reference retained: %+v", listener)
							}
						}
					} else {
						for _, listener := range states[length:] {
							if listener.onUpdate != nil || listener.onClose != nil || listener.id != 0 {
								t.Errorf("removed state reference retained: %+v", listener)
							}
						}
					}
				}
				unsub[removed]()
				unsub[removed]()
				checkTail(2)
				if owner == "notification" {
					c.handleNotification(context.Background(), Notification{Method: "storage"})
				} else {
					c.CacheThreadState(Thread{ID: "storage"})
				}
				var want []int
				for i := range 3 {
					if i != removed {
						want = append(want, i)
					}
				}
				if !reflect.DeepEqual(calls, want) {
					t.Fatalf("survivors = %v, want %v", calls, want)
				}
				for _, remove := range unsub {
					remove()
					remove()
				}
				checkTail(0)
				if _, ok := c.internalListeners["storage"]; ok {
					t.Error("empty notification registry retained")
				}
				if _, ok := c.threadStateListeners["storage"]; ok {
					t.Error("empty state registry retained")
				}
			})
		}
	}
}

func TestThreadListenerConcurrentUnsubscribe(t *testing.T) {
	c := NewClient(&mockInternalTransport{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 50 {
				remove := c.AddThreadStateListener("concurrent", func(Thread) {}, func() {})
				c.CacheThreadState(Thread{ID: "concurrent"})
				remove()
				remove()
			}
		})
	}
	wg.Wait()
	if _, ok := c.threadStateListeners["concurrent"]; ok {
		t.Fatal("empty state registry retained after concurrent removal")
	}
}

func TestListenerUnsubscribePreservesAdmittedSnapshot(t *testing.T) {
	for _, owner := range []string{"notification", "thread-update", "thread-close"} {
		t.Run(owner, func(t *testing.T) {
			c := NewClient(&mockInternalTransport{})
			var calls []int
			var removeSecond func()
			first := func() { calls = append(calls, 1); removeSecond() }
			second := func() { calls = append(calls, 2) }
			switch owner {
			case "notification":
				c.AddNotificationListener("snapshot", func(context.Context, Notification) { first() })
				removeSecond = c.AddNotificationListener("snapshot", func(context.Context, Notification) { second() })
				c.handleNotification(context.Background(), Notification{Method: "snapshot"})
				c.handleNotification(context.Background(), Notification{Method: "snapshot"})
			case "thread-update":
				c.AddThreadStateListener("snapshot", func(Thread) { first() }, nil)
				removeSecond = c.AddThreadStateListener("snapshot", func(Thread) { second() }, nil)
				c.CacheThreadState(Thread{ID: "snapshot"})
				c.CacheThreadState(Thread{ID: "snapshot"})
			case "thread-close":
				c.AddThreadStateListener("snapshot", nil, first)
				removeSecond = c.AddThreadStateListener("snapshot", nil, second)
				c.closeThreadState("snapshot")
				c.CacheThreadState(Thread{ID: "snapshot"})
				c.closeThreadState("snapshot")
			}
			if !reflect.DeepEqual(calls, []int{1, 2, 1}) {
				t.Fatalf("admitted snapshot calls = %v", calls)
			}
		})
	}
}
