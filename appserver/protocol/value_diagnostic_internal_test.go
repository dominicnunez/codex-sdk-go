package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func diagnosticJSON(field, value string, dest any, extra map[string]any) error {
	fields := make(map[string]any, len(extra)+1)
	for key, item := range extra {
		fields[key] = item
	}
	fields[field] = value
	data, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func TestValueDiagnosticOwners(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(string) error
		want func(string) string
	}{
		{"rate-limit", func(v string) error { return diagnosticJSON("rateLimitReachedType", v, new(RateLimitSnapshot), nil) }, func(q string) string { return "invalid rateLimits.rateLimitReachedType " + q }},
		{"cancel-login", func(v string) error { return diagnosticJSON("status", v, new(CancelLoginAccountResponse), nil) }, func(q string) string { return "invalid status " + q }},
		{"unsubscribe", func(v string) error { return diagnosticJSON("status", v, new(ThreadUnsubscribeResponse), nil) }, func(q string) string { return "invalid status " + q }},
		{"reset-credit", func(v string) error {
			return diagnosticJSON("outcome", v, new(ConsumeAccountRateLimitResetCreditResponse), nil)
		}, func(q string) string { return "invalid reset credit outcome " + q }},
		{"prediction", func(v string) error { return diagnosticJSON("type", v, new(ThreadPredictionResultWrapper), nil) }, func(q string) string { return "unknown ThreadPredictionResult type " + q }},
		{"schedule-concrete", func(v string) error { return diagnosticJSON("type", v, new(DailyScheduledTaskSchedule), nil) }, func(q string) string { return "invalid scheduled task schedule type " + q + `; want "daily"` }},
		{"review-concrete", func(v string) error { return diagnosticJSON("type", v, new(UncommittedChangesReviewTarget), nil) }, func(q string) string { return "review target: type " + q + ` does not match "uncommittedChanges"` }},
		{"tool-text", func(v string) error {
			return diagnosticJSON("type", v, new(InputTextDynamicToolCallOutputContentItem), map[string]any{"text": "t"})
		}, func(q string) string { return "invalid dynamic tool output content item type " + q }},
		{"tool-image", func(v string) error {
			return diagnosticJSON("type", v, new(InputImageDynamicToolCallOutputContentItem), map[string]any{"imageUrl": "u"})
		}, func(q string) string { return "invalid dynamic tool output content item type " + q }},
		{"tool-audio", func(v string) error {
			return diagnosticJSON("type", v, new(InputAudioDynamicToolCallOutputContentItem), map[string]any{"audioUrl": "u"})
		}, func(q string) string { return "invalid dynamic tool output content item type " + q }},
		{"filesystem-type", func(v string) error { return diagnosticJSON("type", v, new(FileSystemPathWrapper), nil) }, func(q string) string { return "invalid filesystem path type " + q }},
		{"filesystem-kind", func(v string) error { return diagnosticJSON("kind", v, new(FileSystemSpecialPathWrapper), nil) }, func(q string) string { return "invalid filesystem special path kind " + q }},
		{"network-policy", func(v string) error { return validateNetworkPolicyRuleAction(NetworkPolicyRuleAction(v)) }, func(q string) string { return "invalid network policy action " + q }},
		{"approval-scope", func(v string) error {
			scope := PermissionGrantScope(v)
			return (PermissionsRequestApprovalResponse{Scope: &scope}).validate()
		}, func(q string) string { return "invalid scope " + q }},
		{"approval-action", func(v string) error {
			return (McpServerElicitationRequestResponse{Action: McpServerElicitationAction(v)}).validate()
		}, func(q string) string { return "invalid action " + q }},
		{"command-decision", func(v string) error {
			return validateCommandExecutionApprovalDecisionWrapper(CommandExecutionApprovalDecisionWrapper{Value: v})
		}, func(q string) string { return "invalid decision " + q }},
		{"review-decision", func(v string) error { return validateReviewDecisionWrapper(ReviewDecisionWrapper{Value: v}) }, func(q string) string { return "invalid decision " + q }},
		{"answer-key", func(v string) error {
			return (ToolRequestUserInputResponse{Answers: map[string]ToolRequestUserInputAnswer{v: {}}}).validate()
		}, func(q string) string { return "answers[" + q + "].answers: missing answers" }},
		{"unknown-tool-output", func(v string) error {
			return (DynamicToolCallOutputContentItemWrapper{Value: &UnknownDynamicToolCallOutputContentItem{Type: v}}).validateForResponse()
		}, func(q string) string { return "unsupported content item type " + q }},
		{"login-brand", func(v string) error {
			brand := LoginAppBrand(v)
			_, err := (&ChatgptLoginAccountParams{AppBrand: &brand}).marshalWire()
			return err
		}, func(q string) string { return "invalid appBrand " + q }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			short := "bad\"\n日本語"
			err := tc.call(short)
			if err == nil || err.Error() != tc.want(fmt.Sprintf("%q", short)) {
				t.Fatalf("short diagnostic or fixture changed: %v", err)
			}
			for _, value := range []string{strings.Repeat("x", 1<<20), strings.Repeat("\x00", 1<<18), strings.Repeat("日本語", 1<<18)} {
				err := tc.call(value)
				if err == nil || len(err.Error()) > 1800 || !strings.Contains(err.Error(), "bytes omitted") {
					t.Error("custom owner diagnostic is not bounded")
				}
			}
		})
	}
}
