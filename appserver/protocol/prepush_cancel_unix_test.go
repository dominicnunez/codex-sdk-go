//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package protocol_test

import "testing"

func TestPrePushCancellationPreservesState(t *testing.T) {
	for _, phase := range []string{"capture", "bootstrap", "ordinary", "race", "version", "install", "analysis", "tidy", "complete"} {
		t.Run(phase, func(t *testing.T) { testHookCancellation(t, "pre-push", phase) })
	}
}
