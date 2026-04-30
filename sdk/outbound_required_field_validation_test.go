package codex_test

import (
	"context"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/sdk"
)

func TestIDScopedRequestsRejectEmptyIdentifiersBeforeSend(t *testing.T) {
	tests := []struct {
		name        string
		field       string
		call        func(*codex.Client) error
		wantMessage string
	}{
		{
			name:  "fs unwatch",
			field: "watchId",
			call: func(client *codex.Client) error {
				_, err := client.Fs.Unwatch(context.Background(), codex.FsUnwatchParams{})
				return err
			},
			wantMessage: "watchId must not be empty",
		},
		{
			name:  "plugin uninstall",
			field: "pluginId",
			call: func(client *codex.Client) error {
				_, err := client.Plugin.Uninstall(context.Background(), codex.PluginUninstallParams{})
				return err
			},
			wantMessage: "pluginId must not be empty",
		},
		{
			name:  "account login cancel",
			field: "loginId",
			call: func(client *codex.Client) error {
				_, err := client.Account.CancelLogin(context.Background(), codex.CancelLoginAccountParams{})
				return err
			},
			wantMessage: "loginId must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected invalid params error")
			}
			if !strings.Contains(err.Error(), "invalid params") {
				t.Fatalf("error = %v; want invalid params context", err)
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("error = %v; want %s context", err, tt.field)
			}
			if got := transport.CallCount(); got != 0 {
				t.Fatalf("CallCount() = %d; want 0", got)
			}
		})
	}
}
