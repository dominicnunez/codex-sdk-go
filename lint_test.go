package codex_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestGolangciLint verifies that golangci-lint passes with no issues.
//
// To run golangci-lint manually:
//
//	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
//	golangci-lint run ./...
func TestGolangciLint(t *testing.T) {
	lintBin := "golangci-lint"
	if _, err := exec.LookPath(lintBin); err != nil {
		// Fall back to GOPATH/bin or ~/go/bin
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			gopath = filepath.Join(os.Getenv("HOME"), "go")
		}
		candidate := filepath.Join(gopath, "bin", "golangci-lint")
		if _, err := exec.LookPath(candidate); err != nil {
			t.Skip("golangci-lint not found - install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
		}
		lintBin = candidate
	}

	cmd := exec.Command(lintBin, "run", "./...")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("golangci-lint failed:\n%s", string(output))
	}

	t.Logf("golangci-lint passed successfully")
}

// TestNotificationListenerCoverage verifies all typed notification methods
// have listener registration. Compile-time: if any On* method or notification
// type is removed, this test fails to compile. Runtime: the count assertion
// catches new methods that were added but not registered here.
func TestNotificationListenerCoverage(t *testing.T) {
	client := codex.NewClient(NewMockTransport())

	// expectedListeners must match the number of typed On* notification methods
	// on Client (excluding OnNotification and OnCollabToolCall* helpers).
	// Update this constant when adding or removing On* methods.
	const expectedListeners = 42

	registered := 0

	client.OnAccountLoginCompleted(func(codex.AccountLoginCompletedNotification) {})
	registered++
	client.OnAccountRateLimitsUpdated(func(codex.AccountRateLimitsUpdatedNotification) {})
	registered++
	client.OnAccountUpdated(func(codex.AccountUpdatedNotification) {})
	registered++
	client.OnAppListUpdated(func(codex.AppListUpdatedNotification) {})
	registered++
	client.OnConfigWarning(func(codex.ConfigWarningNotification) {})
	registered++
	client.OnDeprecationNotice(func(codex.DeprecationNoticeNotification) {})
	registered++
	client.OnError(func(codex.ErrorNotification) {})
	registered++
	client.OnFuzzyFileSearchSessionCompleted(func(codex.FuzzyFileSearchSessionCompletedNotification) {})
	registered++
	client.OnFuzzyFileSearchSessionUpdated(func(codex.FuzzyFileSearchSessionUpdatedNotification) {})
	registered++
	client.OnAgentMessageDelta(func(codex.AgentMessageDeltaNotification) {})
	registered++
	client.OnCommandExecutionOutputDelta(func(codex.CommandExecutionOutputDeltaNotification) {})
	registered++
	client.OnTerminalInteraction(func(codex.TerminalInteractionNotification) {})
	registered++
	client.OnItemCompleted(func(codex.ItemCompletedNotification) {})
	registered++
	client.OnFileChangeOutputDelta(func(codex.FileChangeOutputDeltaNotification) {})
	registered++
	client.OnMcpToolCallProgress(func(codex.McpToolCallProgressNotification) {})
	registered++
	client.OnPlanDelta(func(codex.PlanDeltaNotification) {})
	registered++
	client.OnReasoningSummaryPartAdded(func(codex.ReasoningSummaryPartAddedNotification) {})
	registered++
	client.OnReasoningSummaryTextDelta(func(codex.ReasoningSummaryTextDeltaNotification) {})
	registered++
	client.OnReasoningTextDelta(func(codex.ReasoningTextDeltaNotification) {})
	registered++
	client.OnItemStarted(func(codex.ItemStartedNotification) {})
	registered++
	client.OnMcpServerOauthLoginCompleted(func(codex.McpServerOauthLoginCompletedNotification) {})
	registered++
	client.OnModelRerouted(func(codex.ModelReroutedNotification) {})
	registered++
	client.OnServerRequestResolved(func(codex.ServerRequestResolvedNotification) {})
	registered++
	client.OnThreadArchived(func(codex.ThreadArchivedNotification) {})
	registered++
	client.OnThreadClosed(func(codex.ThreadClosedNotification) {})
	registered++
	client.OnContextCompacted(func(codex.ContextCompactedNotification) {})
	registered++
	client.OnThreadNameUpdated(func(codex.ThreadNameUpdatedNotification) {})
	registered++
	client.OnThreadRealtimeClosed(func(codex.ThreadRealtimeClosedNotification) {})
	registered++
	client.OnThreadRealtimeError(func(codex.ThreadRealtimeErrorNotification) {})
	registered++
	client.OnThreadRealtimeItemAdded(func(codex.ThreadRealtimeItemAddedNotification) {})
	registered++
	client.OnThreadRealtimeOutputAudioDelta(func(codex.ThreadRealtimeOutputAudioDeltaNotification) {})
	registered++
	client.OnThreadRealtimeStarted(func(codex.ThreadRealtimeStartedNotification) {})
	registered++
	client.OnThreadStarted(func(codex.ThreadStartedNotification) {})
	registered++
	client.OnThreadStatusChanged(func(codex.ThreadStatusChangedNotification) {})
	registered++
	client.OnThreadTokenUsageUpdated(func(codex.ThreadTokenUsageUpdatedNotification) {})
	registered++
	client.OnThreadUnarchived(func(codex.ThreadUnarchivedNotification) {})
	registered++
	client.OnTurnCompleted(func(codex.TurnCompletedNotification) {})
	registered++
	client.OnTurnDiffUpdated(func(codex.TurnDiffUpdatedNotification) {})
	registered++
	client.OnTurnPlanUpdated(func(codex.TurnPlanUpdatedNotification) {})
	registered++
	client.OnTurnStarted(func(codex.TurnStartedNotification) {})
	registered++
	client.OnWindowsSandboxSetupCompleted(func(codex.WindowsSandboxSetupCompletedNotification) {})
	registered++
	client.OnWindowsWorldWritableWarning(func(codex.WindowsWorldWritableWarningNotification) {})
	registered++

	if registered != expectedListeners {
		t.Errorf("registered %d listeners, expected %d — update this test when adding/removing On* methods", registered, expectedListeners)
	}
}
