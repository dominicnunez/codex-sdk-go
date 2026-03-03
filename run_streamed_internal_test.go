package codex

import (
	"errors"
	"sync"
	"testing"
)

func TestGuardedChanRetainsTerminalErrorUnderBackpressure(t *testing.T) {
	const iterations = 200
	wantErr := errors.New("terminal failure")

	for i := range iterations {
		g := newGuardedChan(1)
		streamSendEvent(g, &TextDelta{Delta: "seed", ItemID: "seed"})

		stop := make(chan struct{})
		var wg sync.WaitGroup
		const producers = 6
		wg.Add(producers)
		for range producers {
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						streamSendEvent(g, &TextDelta{Delta: "noise", ItemID: "n"})
					}
				}
			}()
		}

		streamSendErr(g, wantErr)
		close(stop)
		wg.Wait()
		g.closeOnce()

		var gotErr error
		for _, err := range streamIterator(g) {
			if err != nil {
				gotErr = err
			}
		}
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("iteration %d: got terminal err %v, want %v", i, gotErr, wantErr)
		}
	}
}

func TestGuardedChanKeepsFirstTerminalError(t *testing.T) {
	g := newGuardedChan(1)

	first := errors.New("first")
	second := errors.New("second")
	streamSendErr(g, first)
	streamSendErr(g, second)
	g.closeOnce()

	var gotErr error
	for _, err := range streamIterator(g) {
		if err != nil {
			gotErr = err
		}
	}
	if !errors.Is(gotErr, first) {
		t.Fatalf("got terminal err %v, want %v", gotErr, first)
	}
}
