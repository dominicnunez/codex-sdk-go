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

// TestNotificationListenerCoverage verifies all notification methods have listener registration.
// The compile-time listener registrations below are the real check: if any On*
// method or notification type is removed, this test fails to compile.
func TestNotificationListenerCoverage(t *testing.T) {
	// All 41 notification methods from ServerNotification.json:
	expectedNotifications := []string{
		"account/login/completed",                   // OnAccountLoginCompleted
		"account/rateLimits/updated",                // OnAccountRateLimitsUpdated
		"account/updated",                           // OnAccountUpdated
		"app/list/updated",                          // OnAppListUpdated
		"configWarning",                             // OnConfigWarning
		"deprecationNotice",                         // OnDeprecationNotice
		"error",                                     // OnError
		"fuzzyFileSearch/sessionCompleted",          // OnFuzzyFileSearchSessionCompleted
		"fuzzyFileSearch/sessionUpdated",            // OnFuzzyFileSearchSessionUpdated
		"item/agentMessage/delta",                   // OnAgentMessageDelta
		"item/commandExecution/outputDelta",         // OnCommandExecutionOutputDelta
		"item/commandExecution/terminalInteraction", // OnTerminalInteraction
		"item/completed",                            // OnItemCompleted
		"item/fileChange/outputDelta",               // OnFileChangeOutputDelta
		"item/mcpToolCall/progress",                 // OnMcpToolCallProgress
		"item/plan/delta",                           // OnPlanDelta
		"item/reasoning/summaryPartAdded",           // OnReasoningSummaryPartAdded
		"item/reasoning/summaryTextDelta",           // OnReasoningSummaryTextDelta
		"item/reasoning/textDelta",                  // OnReasoningTextDelta
		"item/started",                              // OnItemStarted
		"mcpServer/oauthLogin/completed",            // OnMcpServerOauthLoginCompleted
		"model/rerouted",                            // OnModelRerouted
		"thread/archived",                           // OnThreadArchived
		"thread/closed",                             // OnThreadClosed
		"thread/compacted",                          // OnContextCompacted
		"thread/name/updated",                       // OnThreadNameUpdated
		"thread/realtime/closed",                    // OnThreadRealtimeClosed
		"thread/realtime/error",                     // OnThreadRealtimeError
		"thread/realtime/itemAdded",                 // OnThreadRealtimeItemAdded
		"thread/realtime/outputAudio/delta",         // OnThreadRealtimeOutputAudioDelta
		"thread/realtime/started",                   // OnThreadRealtimeStarted
		"thread/started",                            // OnThreadStarted
		"thread/status/changed",                     // OnThreadStatusChanged
		"thread/tokenUsage/updated",                 // OnThreadTokenUsageUpdated
		"thread/unarchived",                         // OnThreadUnarchived
		"turn/completed",                            // OnTurnCompleted
		"turn/diff/updated",                         // OnTurnDiffUpdated
		"turn/plan/updated",                         // OnTurnPlanUpdated
		"turn/started",                              // OnTurnStarted
		"windowsSandbox/setupCompleted",             // OnWindowsSandboxSetupCompleted
		"windows/worldWritableWarning",              // OnWindowsWorldWritableWarning
	}

	_ = expectedNotifications // used for documentation; the compile-time block below is the real check

	client := codex.NewClient(NewMockTransport())

	// Register all 41 notification listeners
	client.OnAccountLoginCompleted(func(codex.AccountLoginCompletedNotification) {})
	client.OnAccountRateLimitsUpdated(func(codex.AccountRateLimitsUpdatedNotification) {})
	client.OnAccountUpdated(func(codex.AccountUpdatedNotification) {})
	client.OnAppListUpdated(func(codex.AppListUpdatedNotification) {})
	client.OnConfigWarning(func(codex.ConfigWarningNotification) {})
	client.OnDeprecationNotice(func(codex.DeprecationNoticeNotification) {})
	client.OnError(func(codex.ErrorNotification) {})
	client.OnFuzzyFileSearchSessionCompleted(func(codex.FuzzyFileSearchSessionCompletedNotification) {})
	client.OnFuzzyFileSearchSessionUpdated(func(codex.FuzzyFileSearchSessionUpdatedNotification) {})
	client.OnAgentMessageDelta(func(codex.AgentMessageDeltaNotification) {})
	client.OnCommandExecutionOutputDelta(func(codex.CommandExecutionOutputDeltaNotification) {})
	client.OnTerminalInteraction(func(codex.TerminalInteractionNotification) {})
	client.OnItemCompleted(func(codex.ItemCompletedNotification) {})
	client.OnFileChangeOutputDelta(func(codex.FileChangeOutputDeltaNotification) {})
	client.OnMcpToolCallProgress(func(codex.McpToolCallProgressNotification) {})
	client.OnPlanDelta(func(codex.PlanDeltaNotification) {})
	client.OnReasoningSummaryPartAdded(func(codex.ReasoningSummaryPartAddedNotification) {})
	client.OnReasoningSummaryTextDelta(func(codex.ReasoningSummaryTextDeltaNotification) {})
	client.OnReasoningTextDelta(func(codex.ReasoningTextDeltaNotification) {})
	client.OnItemStarted(func(codex.ItemStartedNotification) {})
	client.OnMcpServerOauthLoginCompleted(func(codex.McpServerOauthLoginCompletedNotification) {})
	client.OnModelRerouted(func(codex.ModelReroutedNotification) {})
	client.OnThreadArchived(func(codex.ThreadArchivedNotification) {})
	client.OnThreadClosed(func(codex.ThreadClosedNotification) {})
	client.OnContextCompacted(func(codex.ContextCompactedNotification) {})
	client.OnThreadNameUpdated(func(codex.ThreadNameUpdatedNotification) {})
	client.OnThreadRealtimeClosed(func(codex.ThreadRealtimeClosedNotification) {})
	client.OnThreadRealtimeError(func(codex.ThreadRealtimeErrorNotification) {})
	client.OnThreadRealtimeItemAdded(func(codex.ThreadRealtimeItemAddedNotification) {})
	client.OnThreadRealtimeOutputAudioDelta(func(codex.ThreadRealtimeOutputAudioDeltaNotification) {})
	client.OnThreadRealtimeStarted(func(codex.ThreadRealtimeStartedNotification) {})
	client.OnThreadStarted(func(codex.ThreadStartedNotification) {})
	client.OnThreadStatusChanged(func(codex.ThreadStatusChangedNotification) {})
	client.OnThreadTokenUsageUpdated(func(codex.ThreadTokenUsageUpdatedNotification) {})
	client.OnThreadUnarchived(func(codex.ThreadUnarchivedNotification) {})
	client.OnTurnCompleted(func(codex.TurnCompletedNotification) {})
	client.OnTurnDiffUpdated(func(codex.TurnDiffUpdatedNotification) {})
	client.OnTurnPlanUpdated(func(codex.TurnPlanUpdatedNotification) {})
	client.OnTurnStarted(func(codex.TurnStartedNotification) {})
	client.OnWindowsSandboxSetupCompleted(func(codex.WindowsSandboxSetupCompletedNotification) {})
	client.OnWindowsWorldWritableWarning(func(codex.WindowsWorldWritableWarningNotification) {})

	t.Logf("All 41 notification listener registration methods verified on Client")
}
