package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

type budgetStartTransport struct {
	*startStateTransport
	entered chan struct{}
}

func (tr *budgetStartTransport) Send(ctx context.Context, req Request) (Response, error) {
	var params struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}
	if params.Model == "hold" {
		tr.entered <- struct{}{}
		<-ctx.Done()
		return Response{}, ctx.Err()
	}
	if tr.before != nil {
		tr.before(ctx, req)
	}
	tr.started("target", "notification")
	return Response{ID: req.ID, Result: startStateResponse("target")}, nil
}

func TestPendingStartsShareClientRetentionBudget(t *testing.T) {
	tr := &budgetStartTransport{startStateTransport: &startStateTransport{}, entered: make(chan struct{}, 8)}
	client := NewClient(tr)
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	var pending sync.WaitGroup
	defer func() { cancel(); pending.Wait() }()
	for group := range 8 {
		pending.Go(func() {
			model := "hold"
			_, _, remove, err := client.Thread.StartWithStateListener(ctx, ThreadStartParams{Model: &model}, nil, nil)
			remove()
			if !errors.Is(err, context.Canceled) {
				t.Errorf("held start did not return cancellation: %v", err)
			}
		})
		waitStartSignal(t, tr.entered)
		for i := range 64 {
			tr.started(fmt.Sprintf("group-%d-%d", group, i), strings.Repeat("x", 64<<10))
		}
		client.threadStateMu.RLock()
		pins, retained := len(client.threadStartPins), len(client.threadStates)
		client.threadStateMu.RUnlock()
		if pins > 64 || retained > 128 {
			t.Errorf("aggregate startup budget exceeded: group=%d distinctPins=%d cacheEntries=%d", group, pins, retained)
		}
	}
	calls := 0
	response, generation, remove, err := client.Thread.StartWithStateListener(ctx, ThreadStartParams{}, func(Thread) { calls++ }, nil)
	remove()
	if !errors.Is(err, ErrThreadStartObservationLost) || response.Thread.ID != "target" || generation != 0 || calls != 0 {
		t.Fatalf("exhausted client budget admitted observation: response=%+v generation=%d calls=%d err=%v", response, generation, calls, err)
	}
	cancel()
	pending.Wait()
	assertNoStartResources(t, client)
	if len(client.threadStates) > 64 {
		t.Fatal("cancellation retained temporary cache entries")
	}
	response, generation, remove, err = client.Thread.StartWithStateListener(context.Background(), ThreadStartParams{}, func(Thread) { calls++ }, nil)
	defer remove()
	if err != nil || response.Thread.ID != "target" || generation == 0 || calls != 1 {
		t.Fatalf("released budget did not allow observation: generation=%d calls=%d err=%v", generation, calls, err)
	}
}

func TestPendingStartSharedPinsReleaseIndependently(t *testing.T) {
	tr := &budgetStartTransport{startStateTransport: &startStateTransport{}, entered: make(chan struct{}, 2)}
	client := NewClient(tr)
	defer client.Close()
	var cancels []context.CancelFunc
	var done []chan struct{}
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
		for _, signal := range done {
			waitStartSignal(t, signal)
		}
	}()
	for range 2 {
		ctx, cancel := context.WithCancel(context.Background())
		cancels = append(cancels, cancel)
		signal := make(chan struct{})
		done = append(done, signal)
		go func() {
			defer close(signal)
			model := "hold"
			_, _, remove, _ := client.Thread.StartWithStateListener(ctx, ThreadStartParams{Model: &model}, nil, nil)
			remove()
		}()
		waitStartSignal(t, tr.entered)
	}
	tr.started("shared", "notification")
	client.threadStateMu.RLock()
	pins := client.threadStartPins["shared"]
	client.threadStateMu.RUnlock()
	if pins != 2 {
		t.Fatalf("shared startup pin count=%d, want 2", pins)
	}
	cancels[0]()
	waitStartSignal(t, done[0])
	for i := range 100 {
		client.CacheThreadState(Thread{ID: fmt.Sprint("pressure-", i)})
	}
	if _, present := client.ThreadStateSnapshot("shared"); !present {
		t.Fatal("one cancellation released another request's snapshot")
	}
	cancels[1]()
	waitStartSignal(t, done[1])
	assertNoStartResources(t, client)
}

func TestPendingStartDoesNotRecaptureLostHistoryAfterBudgetRelease(t *testing.T) {
	tr := &budgetStartTransport{startStateTransport: &startStateTransport{}, entered: make(chan struct{}, 1)}
	client := NewClient(tr)
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	defer func() { cancel(); waitStartSignal(t, done) }()
	go func() {
		defer close(done)
		model := "hold"
		_, _, remove, _ := client.Thread.StartWithStateListener(ctx, ThreadStartParams{Model: &model}, nil, nil)
		remove()
	}()
	waitStartSignal(t, tr.entered)
	for i := range 64 {
		tr.started(fmt.Sprint("pinned-", i), "notification")
	}
	tr.before = func(context.Context, Request) {
		// These events exceed the budget while the first request owns all
		// slots. Releasing it cannot restore this request's missing history.
		tr.started("target", "original")
		tr.closed("target")
		cancel()
		waitStartSignal(t, done)
	}
	calls := 0
	response, generation, remove, err := client.Thread.StartWithStateListener(context.Background(), ThreadStartParams{}, func(Thread) { calls++ }, func() { calls++ })
	defer remove()
	if !errors.Is(err, ErrThreadStartObservationLost) || response.Thread.ID != "target" || generation != 0 || calls != 0 {
		t.Fatalf("lost closure adopted replacement after slot reuse: generation=%d calls=%d err=%v", generation, calls, err)
	}
	assertNoStartResources(t, client)
}
