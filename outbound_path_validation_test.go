package codex_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestOutboundAbsolutePathRequestsNormalizeBeforeSend(t *testing.T) {
	t.Run("filesystem paths", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readFile", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"dataBase64":"ZGF0YQ=="}`),
		})

		_, err := client.Fs.ReadFile(context.Background(), codex.FsReadFileParams{
			Path: "/tmp/../var//log.txt",
		})
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}

		var params codex.FsReadFileParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.Path != "/var/log.txt" {
			t.Fatalf("Path = %q, want /var/log.txt", params.Path)
		}
	})

	t.Run("plugin paths", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/install", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"appsNeedingAuth":[],"authPolicy":"ON_INSTALL"}`),
		})

		_, err := client.Plugin.Install(context.Background(), codex.PluginInstallParams{
			MarketplacePath: `C:\plugins\..\official`,
			PluginName:      "calendar",
		})
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		var params codex.PluginInstallParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.MarketplacePath != `C:\official` {
			t.Fatalf("MarketplacePath = %q, want %q", params.MarketplacePath, `C:\official`)
		}
	})

	t.Run("windows sandbox cwd", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("windowsSandbox/setupStart", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"started":true}`),
		})

		_, err := client.System.WindowsSandboxSetupStart(context.Background(), codex.WindowsSandboxSetupStartParams{
			Cwd:  ptr(`\\server\share\repo\..\project`),
			Mode: codex.WindowsSandboxSetupModeElevated,
		})
		if err != nil {
			t.Fatalf("WindowsSandboxSetupStart() error = %v", err)
		}

		var params codex.WindowsSandboxSetupStartParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.Cwd == nil || *params.Cwd != `\\server\share\project` {
			t.Fatalf("Cwd = %v, want %q", params.Cwd, `\\server\share\project`)
		}
	})

	t.Run("command sandbox roots", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("command/exec", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"exitCode":0,"stdout":"","stderr":""}`),
		})

		_, err := client.Command.Exec(context.Background(), codex.CommandExecParams{
			Command: []string{"cat", "/etc/hosts"},
			SandboxPolicy: &codex.SandboxPolicyWrapper{Value: codex.SandboxPolicyWorkspaceWrite{
				ReadOnlyAccess: &codex.ReadOnlyAccessWrapper{Value: codex.ReadOnlyAccessRestricted{
					ReadableRoots: []string{"/tmp/../readonly"},
				}},
				WritableRoots: []string{`C:\workspace\..\repo`},
			}},
		})
		if err != nil {
			t.Fatalf("Command.Exec() error = %v", err)
		}

		var params codex.CommandExecParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		policy, ok := params.SandboxPolicy.Value.(codex.SandboxPolicyWorkspaceWrite)
		if !ok {
			t.Fatalf("sandbox policy type = %T, want SandboxPolicyWorkspaceWrite", params.SandboxPolicy.Value)
		}
		restricted, ok := policy.ReadOnlyAccess.Value.(codex.ReadOnlyAccessRestricted)
		if !ok {
			t.Fatalf("read only access type = %T, want ReadOnlyAccessRestricted", policy.ReadOnlyAccess.Value)
		}
		if got := restricted.ReadableRoots[0]; got != "/readonly" {
			t.Fatalf("ReadableRoots[0] = %q, want /readonly", got)
		}
		if got := policy.WritableRoots[0]; got != `C:\repo` {
			t.Fatalf("WritableRoots[0] = %q, want %q", got, `C:\repo`)
		}
	})

	t.Run("turn sandbox roots", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("turn/start", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"turn":{"id":"turn-1","status":"inProgress","items":[]}}`),
		})

		sandboxPolicy := codex.SandboxPolicy(codex.SandboxPolicyReadOnly{
			Access: &codex.ReadOnlyAccessWrapper{Value: codex.ReadOnlyAccessRestricted{
				ReadableRoots: []string{"/workspace/../repo"},
			}},
		})
		_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{
			ThreadID:      "thread-1",
			Input:         []codex.UserInput{&codex.TextUserInput{Text: "hi"}},
			SandboxPolicy: &sandboxPolicy,
		})
		if err != nil {
			t.Fatalf("Turn.Start() error = %v", err)
		}

		var params codex.TurnStartParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		policy, ok := (*params.SandboxPolicy).(codex.SandboxPolicyReadOnly)
		if !ok {
			t.Fatalf("sandbox policy type = %T, want SandboxPolicyReadOnly", *params.SandboxPolicy)
		}
		restricted, ok := policy.Access.Value.(codex.ReadOnlyAccessRestricted)
		if !ok {
			t.Fatalf("read only access type = %T, want ReadOnlyAccessRestricted", policy.Access.Value)
		}
		if got := restricted.ReadableRoots[0]; got != "/repo" {
			t.Fatalf("ReadableRoots[0] = %q, want /repo", got)
		}
	})
}

func TestOutboundAbsolutePathRequestsRejectRelativePaths(t *testing.T) {
	tests := []struct {
		name string
		call func(*codex.Client) error
	}{
		{
			name: "filesystem request",
			call: func(client *codex.Client) error {
				_, err := client.Fs.ReadFile(context.Background(), codex.FsReadFileParams{Path: "relative/file.txt"})
				return err
			},
		},
		{
			name: "plugin list request",
			call: func(client *codex.Client) error {
				_, err := client.Plugin.List(context.Background(), codex.PluginListParams{Cwds: []string{"relative/repo"}})
				return err
			},
		},
		{
			name: "plugin read request",
			call: func(client *codex.Client) error {
				_, err := client.Plugin.Read(context.Background(), codex.PluginReadParams{
					MarketplacePath: "plugins",
					PluginName:      "calendar",
				})
				return err
			},
		},
		{
			name: "windows sandbox request",
			call: func(client *codex.Client) error {
				_, err := client.System.WindowsSandboxSetupStart(context.Background(), codex.WindowsSandboxSetupStartParams{
					Cwd:  ptr("repo"),
					Mode: codex.WindowsSandboxSetupModeElevated,
				})
				return err
			},
		},
		{
			name: "command sandbox roots",
			call: func(client *codex.Client) error {
				_, err := client.Command.Exec(context.Background(), codex.CommandExecParams{
					Command: []string{"pwd"},
					SandboxPolicy: &codex.SandboxPolicyWrapper{Value: codex.SandboxPolicyWorkspaceWrite{
						WritableRoots: []string{"repo"},
					}},
				})
				return err
			},
		},
		{
			name: "turn sandbox roots",
			call: func(client *codex.Client) error {
				sandboxPolicy := codex.SandboxPolicy(codex.SandboxPolicyReadOnly{
					Access: &codex.ReadOnlyAccessWrapper{Value: codex.ReadOnlyAccessRestricted{
						ReadableRoots: []string{"repo"},
					}},
				})
				_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{
					ThreadID:      "thread-1",
					Input:         []codex.UserInput{&codex.TextUserInput{Text: "hi"}},
					SandboxPolicy: &sandboxPolicy,
				})
				return err
			},
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
				t.Fatalf("error = %v, want invalid params context", err)
			}
			if transport.CallCount() != 0 {
				t.Fatalf("CallCount() = %d, want 0", transport.CallCount())
			}
		})
	}
}
