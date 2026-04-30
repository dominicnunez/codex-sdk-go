package codex_test

import (
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/sdk"
)

func TestProtocolEnumsRejectInvalidMarshalAndUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		marshal   func() ([]byte, error)
		unmarshal func([]byte) error
		wantErr   string
	}{
		{
			name: "personality",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.Personality("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.Personality
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid personality "totally-invalid"`,
		},
		{
			name: "approvals reviewer",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ApprovalsReviewer("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ApprovalsReviewer
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid approvalsReviewer "totally-invalid"`,
		},
		{
			name: "service tier",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ServiceTier("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ServiceTier
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid serviceTier "totally-invalid"`,
		},
		{
			name: "mode kind",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ModeKind("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ModeKind
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid mode "totally-invalid"`,
		},
		{
			name: "merge strategy",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.MergeStrategy("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.MergeStrategy
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid mergeStrategy "totally-invalid"`,
		},
		{
			name: "verbosity",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.Verbosity("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.Verbosity
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid verbosity "totally-invalid"`,
		},
		{
			name: "sandbox mode",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.SandboxMode("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.SandboxMode
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid sandboxMode "totally-invalid"`,
		},
		{
			name: "web search mode",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.WebSearchMode("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.WebSearchMode
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid webSearchMode "totally-invalid"`,
		},
		{
			name: "thread start source",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ThreadStartSource("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ThreadStartSource
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid sessionStartSource "totally-invalid"`,
		},
		{
			name: "sort direction",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.SortDirection("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.SortDirection
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid sortDirection "totally-invalid"`,
		},
		{
			name: "thread sort key",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ThreadSortKey("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ThreadSortKey
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid sortKey "totally-invalid"`,
		},
		{
			name: "thread source kind",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ThreadSourceKind("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ThreadSourceKind
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid sourceKinds "totally-invalid"`,
		},
		{
			name: "windows sandbox setup mode",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.WindowsSandboxSetupMode("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.WindowsSandboxSetupMode
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid windowsSandbox.mode "totally-invalid"`,
		},
		{
			name: "forced login method",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ForcedLoginMethod("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ForcedLoginMethod
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid forcedLoginMethod "totally-invalid"`,
		},
		{
			name: "reasoning effort",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ReasoningEffort("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ReasoningEffort
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid reasoningEffort "totally-invalid"`,
		},
		{
			name: "reasoning summary mode",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ReasoningSummaryMode("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ReasoningSummaryMode
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid reasoningSummary "totally-invalid"`,
		},
		{
			name: "review delivery",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ReviewDelivery("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ReviewDelivery
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid delivery "totally-invalid"`,
		},
		{
			name: "residency requirement",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.ResidencyRequirement("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.ResidencyRequirement
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid residencyRequirement "totally-invalid"`,
		},
		{
			name: "hook event name",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.HookEventName("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.HookEventName
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid hook.eventName "totally-invalid"`,
		},
		{
			name: "hook execution mode",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.HookExecutionMode("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.HookExecutionMode
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid hook.executionMode "totally-invalid"`,
		},
		{
			name: "hook handler type",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.HookHandlerType("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.HookHandlerType
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid hook.handlerType "totally-invalid"`,
		},
		{
			name: "hook output entry kind",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.HookOutputEntryKind("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.HookOutputEntryKind
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid hook.output.kind "totally-invalid"`,
		},
		{
			name: "hook run status",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.HookRunStatus("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.HookRunStatus
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid hook.run.status "totally-invalid"`,
		},
		{
			name: "hook scope",
			marshal: func() ([]byte, error) {
				return json.Marshal(codex.HookScope("totally-invalid"))
			},
			unmarshal: func(data []byte) error {
				var value codex.HookScope
				return json.Unmarshal(data, &value)
			},
			wantErr: `invalid hook.scope "totally-invalid"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/marshal", func(t *testing.T) {
			_, err := tt.marshal()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})

		t.Run(tt.name+"/unmarshal", func(t *testing.T) {
			err := tt.unmarshal([]byte(`"totally-invalid"`))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
