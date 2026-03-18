package codex

import (
	"errors"
	"testing"
)

func TestGuardedChanOverflowFailsStream(t *testing.T) {
	g := newGuardedChan(1)

	streamSendEvent(g, &TextDelta{Delta: "seed", ItemID: "seed"})
	streamSendEvent(g, &TextDelta{Delta: "overflow", ItemID: "overflow"})
	streamSendErr(g, errors.New("later error"))

	var (
		events []Event
		gotErr error
	)
	for event, err := range streamIterator(g) {
		if event != nil {
			events = append(events, event)
		}
		if err != nil {
			gotErr = err
		}
	}

	if len(events) != 1 {
		t.Fatalf("event count = %d; want 1", len(events))
	}
	if delta, ok := events[0].(*TextDelta); !ok || delta.Delta != "seed" {
		t.Fatalf("first event = %#v; want seed TextDelta", events[0])
	}
	if !errors.Is(gotErr, ErrStreamOverflow) {
		t.Fatalf("terminal err = %v; want %v", gotErr, ErrStreamOverflow)
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
