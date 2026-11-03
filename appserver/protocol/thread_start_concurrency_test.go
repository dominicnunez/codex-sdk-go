package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func waitStartSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("startup observation fixture did not progress")
	}
}

func TestPendingStartRejectsAmbiguousHistory(t *testing.T) {
	for _, history := range []string{"close-only", "close-before-start", "remote-reopen"} {
		t.Run(history, func(t *testing.T) {
			tr := &startStateTransport{result: startStateResponse("thread")}
			client := NewClient(tr)
			defer client.Close()
			tr.before = func(context.Context, Request) {
				if history == "remote-reopen" {
					tr.started("thread", "first")
				}
				tr.closed("thread")
				if history != "close-only" {
					tr.started("thread", "replacement")
				}
			}
			calls := 0
			response, generation, remove, err := client.Thread.StartWithStateListener(context.Background(), ThreadStartParams{}, func(Thread) { calls++ }, func() { calls++ })
			remove()
			if !errors.Is(err, ErrThreadStartObservationLost) || generation != 0 || calls != 0 || response.Thread.ID != "thread" || *response.Thread.Name != "initial" {
				t.Fatalf("ambiguous history was adopted: generation=%d calls=%d err=%v", generation, calls, err)
			}
			if snapshot, present := client.ThreadStateSnapshot("thread"); history == "close-only" && present || history != "close-only" && (!present || snapshot.Name == nil || *snapshot.Name != "replacement") {
				t.Fatalf("initial response overwrote admitted state: %+v present=%v", snapshot, present)
			}
			assertNoStartResources(t, client)
		})
	}
}

func TestPendingStartCapturesBeforeReentrantCallbacks(t *testing.T) {
	tr := &startStateTransport{result: startStateResponse("thread")}
	client := NewClient(tr)
	defer client.Close()
	var original uint64
	var removeEarlier func()
	removeEarlier = client.AddThreadStateListener("thread", func(Thread) {
		removeEarlier()
		original = client.ThreadStateGeneration("thread")
		tr.closed("thread")
		name := "replacement"
		client.CacheThreadState(Thread{ID: "thread", Name: &name})
	}, nil)
	defer removeEarlier()
	tr.before = func(context.Context, Request) { tr.started("thread", "notification") }
	var events []string
	_, generation, remove, err := client.Thread.StartWithStateListener(context.Background(), ThreadStartParams{}, func(thread Thread) { events = append(events, *thread.Name) }, func() { events = append(events, "closed") })
	defer remove()
	if err != nil || original == 0 || generation != original || generation == client.ThreadStateGeneration("thread") || !slices.Equal(events, []string{"closed", "replacement"}) {
		t.Fatalf("reentrant capture lost original ownership: original=%d generation=%d events=%v err=%v", original, generation, events, err)
	}
	assertNoStartResources(t, client)
}

func TestPendingStartHandoffKeepsSerialDelivery(t *testing.T) {
	tr := &startStateTransport{result: startStateResponse("thread")}
	client := NewClient(tr)
	defer client.Close()
	tr.before = func(context.Context, Request) { tr.started("thread", "notification") }
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer func() { unblock(); waitStartSignal(t, done) }()
	var active atomic.Int32
	var mu sync.Mutex
	var events []string
	record := func(event string) {
		if active.Add(1) != 1 {
			t.Error("startup recipient callbacks overlapped")
		}
		defer active.Add(-1)
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
		if event == "notification" {
			close(entered)
			<-release
		}
	}
	var generation uint64
	var result ThreadStartResponse
	var startErr error
	var remove func()
	go func() {
		defer close(done)
		result, generation, remove, startErr = client.Thread.StartWithStateListener(context.Background(), ThreadStartParams{}, func(thread Thread) { record(*thread.Name) }, func() { record("closed") })
	}()
	waitStartSignal(t, entered)
	tr.name("thread", "coalesced")
	tr.closed("thread")
	name := "replacement"
	client.CacheThreadState(Thread{ID: "thread", Name: &name})
	unblock()
	waitStartSignal(t, done)
	defer remove()
	if startErr != nil || *result.Thread.Name != "initial" || generation == 0 || generation == client.ThreadStateGeneration("thread") || !slices.Equal(events, []string{"notification", "closed", "replacement"}) {
		t.Fatalf("startup replay order/ownership changed: events=%v generation=%d err=%v", events, generation, startErr)
	}
	assertNoStartResources(t, client)
}

type parallelStartTransport struct {
	*startStateTransport
	entered chan struct{}
	release chan struct{}
	failA   bool
}

func (tr *parallelStartTransport) Send(ctx context.Context, req Request) (Response, error) {
	var params struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}
	tr.entered <- struct{}{}
	select {
	case <-tr.release:
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}
	tr.started(params.Model, "notification")
	if params.Model == "a" {
		tr.closed("a")
		if tr.failA {
			return Response{}, errors.New("fixture start failure")
		}
	}
	return Response{ID: req.ID, Result: startStateResponse(params.Model)}, nil
}

func TestConcurrentPendingStartsTransferOnlyMatchingState(t *testing.T) {
	for _, failA := range []bool{false, true} {
		t.Run(fmt.Sprint("oneFails=", failA), func(t *testing.T) {
			tr := &parallelStartTransport{startStateTransport: &startStateTransport{}, entered: make(chan struct{}, 2), release: make(chan struct{}), failA: failA}
			client := NewClient(tr)
			defer client.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type outcome struct {
				id         string
				response   ThreadStartResponse
				generation uint64
				remove     func()
				err        error
				events     []string
			}
			results := make(chan outcome, 2)
			for _, id := range []string{"a", "b"} {
				go func() {
					var events []string
					response, generation, remove, err := client.Thread.StartWithStateListener(ctx, ThreadStartParams{Model: &id}, func(thread Thread) { events = append(events, thread.ID) }, func() { events = append(events, "closed") })
					results <- outcome{id, response, generation, remove, err, events}
				}()
			}
			for range 2 {
				select {
				case <-tr.entered:
				case <-time.After(3 * time.Second):
					t.Fatal("concurrent starts did not enter the request window")
				}
			}
			close(tr.release)
			for range 2 {
				select {
				case result := <-results:
					result.remove()
					if result.id == "a" && failA {
						if result.err == nil || result.response.Thread.ID != "" || result.generation != 0 || len(result.events) != 0 {
							t.Errorf("failed concurrent start acquired state: %+v", result)
						}
					} else {
						want := []string{result.id}
						if result.id == "a" {
							want = []string{"closed"}
						}
						if result.err != nil || result.response.Thread.ID != result.id || result.generation == 0 || !slices.Equal(result.events, want) {
							t.Errorf("concurrent start adopted unrelated state: %+v", result)
						}
					}
				case <-time.After(3 * time.Second):
					t.Fatal("concurrent start did not finish")
				}
			}
			assertNoStartResources(t, client)
			if len(client.threadStateListeners) != 0 {
				t.Fatal("concurrent starts leaked listeners")
			}
		})
	}
}
