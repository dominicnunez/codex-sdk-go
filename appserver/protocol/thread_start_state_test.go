package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
)

func assertNoStartResources(t *testing.T, c *Client) {
	t.Helper()
	c.threadStateMu.RLock()
	defer c.threadStateMu.RUnlock()
	if c.threadStarts != nil || c.threadStartPins != nil {
		t.Fatalf("temporary startup resources remain: starts=%d pins=%d", len(c.threadStarts), len(c.threadStartPins))
	}
}

func TestPendingStartPreservesSettingsAndOwnership(t *testing.T) {
	for _, settings := range []string{"absent", "latest", "empty"} {
		for _, observed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/observed=%v", settings, observed), func(t *testing.T) {
				tr := &startStateTransport{result: startStateResponse("thread")}
				client := NewClient(tr)
				defer client.Close()
				wantPlugins := []string{"initial"}
				tr.before = func(context.Context, Request) {
					tr.started("thread", "notification")
					switch settings {
					case "latest":
						wantPlugins = []string{"latest"}
						tr.settings("thread", wantPlugins)
					case "empty":
						wantPlugins = []string{}
						tr.settings("thread", wantPlugins)
					}
					for i := range 100 {
						client.CacheThreadState(Thread{ID: fmt.Sprintf("pressure-%d", i)})
					}
				}
				calls := 0
				var onUpdate func(Thread)
				if observed {
					onUpdate = func(thread Thread) {
						calls++
						if thread.Name == nil || *thread.Name != "notification" || !slices.Equal(thread.DisabledPluginIDs, wantPlugins) || thread.DisabledPluginIDs == nil {
							t.Errorf("observer received stale initial state: %+v", thread)
						}
						*thread.Name = "callback"
						if len(thread.DisabledPluginIDs) > 0 {
							thread.DisabledPluginIDs[0] = "callback"
						}
					}
				}
				response, generation, remove, err := client.Thread.StartWithStateListener(context.Background(), ThreadStartParams{}, onUpdate, nil)
				defer remove()
				if err != nil || generation == 0 || response.Thread.Name == nil || *response.Thread.Name != "initial" || !slices.Equal(response.Thread.DisabledPluginIDs, []string{"initial"}) {
					t.Fatalf("initial response or generation lost: response=%+v generation=%d err=%v", response, generation, err)
				}
				*response.Thread.Name = "result"
				response.Thread.DisabledPluginIDs[0] = "result"
				snapshot, present := client.ThreadStateSnapshot("thread")
				if !present || snapshot.Name == nil || *snapshot.Name != "notification" || !slices.Equal(snapshot.DisabledPluginIDs, wantPlugins) {
					t.Fatalf("response/callback changed cache: %+v", snapshot)
				}
				*snapshot.Name = "snapshot"
				again, _ := client.ThreadStateSnapshot("thread")
				if *again.Name != "notification" || (observed && calls != 1) || (!observed && calls != 0) {
					t.Fatal("snapshot isolation or initial callback count changed")
				}
				assertNoStartResources(t, client)
				remove()
				for i := range 65 {
					client.CacheThreadState(Thread{ID: fmt.Sprintf("after-%d", i)})
				}
				if _, present := client.ThreadStateSnapshot("thread"); present {
					t.Fatal("finished startup or removed listener retained a pin")
				}
			})
		}
	}
}

func TestPendingStartKeepsClosedIncarnation(t *testing.T) {
	for _, reopen := range []bool{false, true} {
		t.Run(fmt.Sprint("reopen=", reopen), func(t *testing.T) {
			tr := &startStateTransport{result: startStateResponse("thread")}
			client := NewClient(tr)
			defer client.Close()
			var original uint64
			tr.before = func(context.Context, Request) {
				tr.started("thread", "notification")
				original = client.ThreadStateGeneration("thread")
				tr.closed("thread")
				if reopen {
					name := "replacement"
					client.CacheThreadState(Thread{ID: "thread", Name: &name})
				}
				for i := range 100 {
					client.CacheThreadState(Thread{ID: fmt.Sprintf("pressure-%d", i)})
				}
			}
			var events []string
			response, generation, remove, err := client.Thread.StartWithStateListener(context.Background(), ThreadStartParams{}, func(thread Thread) { events = append(events, *thread.Name) }, func() { events = append(events, "closed") })
			defer remove()
			if err != nil || generation != original || original == 0 || *response.Thread.Name != "initial" || len(events) == 0 || events[0] != "closed" {
				t.Fatalf("original startup closure lost: generation=%d original=%d events=%v err=%v", generation, original, events, err)
			}
			if reopen {
				if !slices.Equal(events, []string{"closed", "replacement"}) || generation == client.ThreadStateGeneration("thread") {
					t.Fatalf("replacement adopted by original observer: events=%v", events)
				}
				if _, published := client.CompleteThreadTurn("thread", generation, Turn{ID: "stale", Status: TurnStatusCompleted}); published {
					t.Fatal("original generation wrote into replacement")
				}
			} else if _, present := client.ThreadStateSnapshot("thread"); present {
				t.Fatal("initial response reopened closed cache")
			}
			assertNoStartResources(t, client)
		})
	}
}

func TestPendingStartCandidateCapacity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		before int
		after  int
		lost   bool
	}{
		{name: "below"},
		{name: "at", before: 63},
		{name: "captured-overflow", after: 65},
		{name: "uncaptured-overflow", before: 65, lost: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := &startStateTransport{result: startStateResponse("thread")}
			client := NewClient(tr)
			defer client.Close()
			tr.before = func(context.Context, Request) {
				for i := range tc.before {
					tr.started(fmt.Sprintf("before-%d", i), "other")
				}
				tr.started("thread", "notification")
				for i := range tc.after {
					tr.started(fmt.Sprintf("after-%d", i), "other")
				}
				if tc.lost {
					tr.closed("thread")
					for i := range 100 {
						client.CacheThreadState(Thread{ID: fmt.Sprintf("eviction-%d", i)})
					}
				}
				client.threadStateMu.RLock()
				defer client.threadStateMu.RUnlock()
				for observation := range client.threadStarts {
					if len(observation.candidates) > 64 {
						t.Error("startup candidate bound exceeded")
					}
				}
			}
			calls := 0
			response, generation, remove, err := client.Thread.StartWithStateListener(context.Background(), ThreadStartParams{}, func(Thread) { calls++ }, func() { calls++ })
			defer remove()
			if tc.lost {
				if !errors.Is(err, ErrThreadStartObservationLost) || generation != 0 || calls != 0 || response.Thread.ID != "thread" || *response.Thread.Name != "initial" {
					t.Fatalf("lost history accepted or response lost: generation=%d calls=%d response=%+v err=%v", generation, calls, response, err)
				}
				if _, present := client.ThreadStateSnapshot("thread"); present {
					t.Fatal("missing cache state was treated as permission to reopen")
				}
			} else if err != nil || generation == 0 || calls != 1 {
				t.Fatalf("captured target failed: generation=%d calls=%d err=%v", generation, calls, err)
			}
			assertNoStartResources(t, client)
		})
	}
}

func TestPendingStartFailureReleasesCapturedPins(t *testing.T) {
	for _, failure := range []string{"transport", "provider", "decode", "validation", "canceled", "params", "pre-canceled"} {
		t.Run(failure, func(t *testing.T) {
			tr := &startStateTransport{result: startStateResponse("thread")}
			client := NewClient(tr)
			defer client.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			params := ThreadStartParams{}
			if failure == "params" {
				params.Config = json.RawMessage{}
			}
			if failure == "pre-canceled" {
				cancel()
			}
			tr.before = func(context.Context, Request) {
				tr.started("thread", "notification")
				switch failure {
				case "transport":
					tr.failure = errors.New("fixture transport failure")
				case "provider":
					tr.provider = &Error{Code: ErrCodeInternalError, Message: "fixture"}
				case "decode":
					tr.result = json.RawMessage(`{`)
				case "validation":
					tr.result = json.RawMessage(`{"thread":{}}`)
				case "canceled":
					cancel()
					tr.failure = ctx.Err()
				}
			}
			calls := 0
			response, generation, remove, err := client.Thread.StartWithStateListener(ctx, params, func(Thread) { calls++ }, func() { calls++ })
			remove()
			if err == nil || response.Thread.ID != "" || generation != 0 || calls != 0 {
				t.Fatalf("failed request published observation: response=%+v generation=%d calls=%d err=%v", response, generation, calls, err)
			}
			assertNoStartResources(t, client)
			for i := range 100 {
				client.CacheThreadState(Thread{ID: fmt.Sprintf("pressure-%d", i)})
			}
			if _, present := client.ThreadStateSnapshot("thread"); present || len(client.threadStateListeners) != 0 {
				t.Fatal("failed startup leaked a listener or pin")
			}
		})
	}
}

func TestPlainThreadStartReturnsOwnedResponseOnObservationLoss(t *testing.T) {
	tr := &startStateTransport{result: startStateResponse("thread")}
	client := NewClient(tr)
	defer client.Close()
	tr.before = func(context.Context, Request) {
		for i := range 65 {
			tr.started(fmt.Sprintf("other-%d", i), "other")
		}
		tr.started("thread", "notification")
	}
	response, err := client.Thread.Start(context.Background(), ThreadStartParams{})
	if !errors.Is(err, ErrThreadStartObservationLost) || response.Thread.ID != "thread" || *response.Thread.Name != "initial" {
		t.Fatalf("plain start lost valid response or hid observation failure: response=%+v err=%v", response, err)
	}
	*response.Thread.Name = "result"
	snapshot, present := client.ThreadStateSnapshot("thread")
	if !present || snapshot.Name == nil || *snapshot.Name != "notification" {
		t.Fatal("returned error response aliases admitted cache state")
	}
	assertNoStartResources(t, client)
	if len(client.threadStateListeners) != 0 {
		t.Fatal("plain start created a subscription")
	}
}

func TestPendingStartPanicReturnsCleanupAndOrigin(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(fmt.Sprint("closed=", closed), func(t *testing.T) {
			tr := &startStateTransport{result: startStateResponse("thread")}
			var contexts []string
			client := NewClient(tr, WithHandlerErrorCallback(func(method string, _ error) { contexts = append(contexts, method); panic("reporter") }))
			defer client.Close()
			tr.before = func(context.Context, Request) {
				tr.started("thread", "notification")
				if closed {
					tr.closed("thread")
				}
			}
			response, generation, remove, err := client.Thread.StartWithStateListener(context.Background(), ThreadStartParams{}, func(Thread) { panic("update") }, func() { panic("close") })
			if err != nil || response.Thread.ID != "thread" || generation == 0 || remove == nil {
				t.Fatalf("panic lost response or cleanup: generation=%d err=%v", generation, err)
			}
			want := methodThreadStart
			if closed {
				want = notifyThreadClosed
			}
			if !slices.Equal(contexts, []string{want}) {
				t.Fatalf("callback origin = %v, want%v", contexts, want)
			}
			assertNoStartResources(t, client)
			remove()
			for i := range 65 {
				client.CacheThreadState(Thread{ID: fmt.Sprintf("pressure-%d", i)})
			}
			if _, present := client.ThreadStateSnapshot("thread"); present {
				t.Fatal("panic cleanup retained a pin")
			}
		})
	}
}
