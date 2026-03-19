package codex_test

import (
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestProtocolEnumsRejectInvalidMarshalAndUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		marshal   func() ([]byte, error)
		unmarshal func([]byte) error
		wantErr   string
	}{
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
