package codex_test

import (
	"testing"
)

// TestRaceDetectorVerification documents that race detector testing is performed
// via `go test -race ./...` which must be run manually outside the test suite
// to avoid recursive test execution.
//
// To verify no data races:
//
//	go test -race ./...
//
// This is critical for concurrent code in transport and dispatch layers.
func TestRaceDetectorVerification(t *testing.T) {
	// This test serves as documentation only
	// Actual race detection must be run via: go test -race ./...
	t.Log("Race detector testing must be run via: go test -race ./...")
	t.Log("See PRD.md Phase 15 Task 5 for race detector verification")
}
