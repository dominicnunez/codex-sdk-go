//go:build windows

package process

import "testing"

func TestDefaultShutdownModeWindows(t *testing.T) {
	if got := DefaultShutdownMode(); got != ShutdownModeNoSignal {
		t.Fatalf("DefaultShutdownMode() = %v, want %v", got, ShutdownModeNoSignal)
	}
}

func TestRequestShutdownWindowsIsNoOp(t *testing.T) {
	if err := requestShutdown(nil); err != nil {
		t.Fatalf("requestShutdown(nil) error = %v, want nil", err)
	}
}
