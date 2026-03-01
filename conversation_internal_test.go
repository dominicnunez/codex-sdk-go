package codex

import "testing"

func TestThreadCloneAdditionalDetails(t *testing.T) {
	details := "retry after 30s"
	conv := &Conversation{
		thread: Thread{
			Turns: []Turn{{
				ID:     "t1",
				Status: "completed",
				Error: &TurnError{
					Message:           "rate limited",
					AdditionalDetails: &details,
				},
			}},
		},
	}

	snap := conv.Thread()
	if snap.Turns[0].Error.AdditionalDetails == nil {
		t.Fatal("expected AdditionalDetails in snapshot")
	}
	if *snap.Turns[0].Error.AdditionalDetails != "retry after 30s" {
		t.Fatalf("got %q, want %q", *snap.Turns[0].Error.AdditionalDetails, "retry after 30s")
	}

	// Mutate the snapshot.
	*snap.Turns[0].Error.AdditionalDetails = "mutated"

	// Original must be unchanged.
	if *conv.thread.Turns[0].Error.AdditionalDetails != "retry after 30s" {
		t.Errorf("AdditionalDetails = %q, want %q — mutation leaked through shallow copy",
			*conv.thread.Turns[0].Error.AdditionalDetails, "retry after 30s")
	}
}
