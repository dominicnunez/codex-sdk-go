//go:build windows

package codex

import "testing"

func TestDefaultProcessShutdownModeWindows(t *testing.T) {
	if got := defaultProcessShutdownMode(); got != processShutdownModeNoSignal {
		t.Fatalf("defaultProcessShutdownMode() = %v, want %v", got, processShutdownModeNoSignal)
	}
}

func TestRequestProcessShutdownWindowsIsNoOp(t *testing.T) {
	if err := requestProcessShutdown(nil); err != nil {
		t.Fatalf("requestProcessShutdown(nil) error = %v, want nil", err)
	}
}
