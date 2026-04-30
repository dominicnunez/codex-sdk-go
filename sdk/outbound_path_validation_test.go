package codex_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/sdk"
)

func TestOutboundAbsolutePathRequestsNormalizeBeforeSend(t *testing.T) {
	t.Run("filesystem paths", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readFile", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"dataBase64":"ZGF0YQ=="}`),
		})
		transport.SetResponse("fs/watch", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"path":"/var/log"}`),
		})

		_, err := client.Fs.ReadFile(context.Background(), codex.FsReadFileParams{
			Path: "/tmp/../var//log.txt",
		})
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		_, err = client.Fs.Watch(context.Background(), codex.FsWatchParams{
			Path:    "/tmp/../var//log",
			WatchID: "watch-1",
		})
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}

		var params codex.FsReadFileParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.Path != "/var/log.txt" {
			t.Fatalf("Path = %q, want /var/log.txt", params.Path)
		}

		var watchParams codex.FsWatchParams
		if err := json.Unmarshal(transport.GetSentRequest(1).Params, &watchParams); err != nil {
			t.Fatalf("unmarshal watch params: %v", err)
		}
		if watchParams.Path != "/var/log" {
			t.Fatalf("Watch Path = %q, want /var/log", watchParams.Path)
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

	t.Run("command cwd", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("command/exec", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"exitCode":0,"stdout":"","stderr":""}`),
		})

		_, err := client.Command.Exec(context.Background(), codex.CommandExecParams{
			Command: []string{"pwd"},
			Cwd:     ptr("/tmp/../workspace//repo"),
		})
		if err != nil {
			t.Fatalf("Command.Exec() error = %v", err)
		}

		var params codex.CommandExecParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.Cwd == nil || *params.Cwd != "/workspace/repo" {
			t.Fatalf("Cwd = %v, want /workspace/repo", params.Cwd)
		}
	})

	t.Run("config request paths", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("config/read", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"config":{},"origins":{}}`),
		})
		transport.SetResponse("config/value/write", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"filePath":"/tmp/config.toml","status":"ok","version":"v1"}`),
		})
		transport.SetResponse("config/batchWrite", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"filePath":"/tmp/config.toml","status":"ok","version":"v1"}`),
		})

		_, err := client.Config.Read(context.Background(), codex.ConfigReadParams{
			Cwd: ptr("/tmp/../repo//project"),
		})
		if err != nil {
			t.Fatalf("Config.Read() error = %v", err)
		}
		_, err = client.Config.Write(context.Background(), codex.ConfigValueWriteParams{
			KeyPath:       "model",
			MergeStrategy: codex.MergeStrategyReplace,
			Value:         json.RawMessage(`"gpt-5"`),
			FilePath:      ptr("/tmp/../repo/config.toml"),
		})
		if err != nil {
			t.Fatalf("Config.Write() error = %v", err)
		}
		_, err = client.Config.BatchWrite(context.Background(), codex.ConfigBatchWriteParams{
			Edits: []codex.ConfigEdit{{
				KeyPath:       "model",
				MergeStrategy: codex.MergeStrategyReplace,
				Value:         json.RawMessage(`"gpt-5"`),
			}},
			FilePath: ptr("/tmp/../repo/config.toml"),
		})
		if err != nil {
			t.Fatalf("Config.BatchWrite() error = %v", err)
		}

		var readParams codex.ConfigReadParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &readParams); err != nil {
			t.Fatalf("unmarshal read params: %v", err)
		}
		if readParams.Cwd == nil || *readParams.Cwd != "/repo/project" {
			t.Fatalf("Read Cwd = %v, want /repo/project", readParams.Cwd)
		}

		var writeParams codex.ConfigValueWriteParams
		if err := json.Unmarshal(transport.GetSentRequest(1).Params, &writeParams); err != nil {
			t.Fatalf("unmarshal write params: %v", err)
		}
		if writeParams.FilePath == nil || *writeParams.FilePath != "/repo/config.toml" {
			t.Fatalf("Write FilePath = %v, want /repo/config.toml", writeParams.FilePath)
		}

		var batchParams codex.ConfigBatchWriteParams
		if err := json.Unmarshal(transport.GetSentRequest(2).Params, &batchParams); err != nil {
			t.Fatalf("unmarshal batch params: %v", err)
		}
		if batchParams.FilePath == nil || *batchParams.FilePath != "/repo/config.toml" {
			t.Fatalf("Batch FilePath = %v, want /repo/config.toml", batchParams.FilePath)
		}
	})

	t.Run("skills request paths", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("skills/list", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"data":[]}`),
		})
		transport.SetResponse("skills/config/write", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"effectiveEnabled":true}`),
		})

		_, err := client.Skills.List(context.Background(), codex.SkillsListParams{
			Cwds: []string{"/tmp/../repo"},
			PerCwdExtraUserRoots: []codex.SkillsListExtraRootsForCwd{{
				Cwd:            "/workspace/../project",
				ExtraUserRoots: []string{"/Users/demo/.codex/../skills"},
			}},
		})
		if err != nil {
			t.Fatalf("Skills.List() error = %v", err)
		}
		_, err = client.Skills.ConfigWrite(context.Background(), codex.SkillsConfigWriteParams{
			Path:    "/tmp/../repo/.codex/skills/test",
			Enabled: true,
		})
		if err != nil {
			t.Fatalf("Skills.ConfigWrite() error = %v", err)
		}

		var listParams codex.SkillsListParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &listParams); err != nil {
			t.Fatalf("unmarshal list params: %v", err)
		}
		if got := listParams.Cwds[0]; got != "/repo" {
			t.Fatalf("Cwds[0] = %q, want /repo", got)
		}
		if got := listParams.PerCwdExtraUserRoots[0].Cwd; got != "/project" {
			t.Fatalf("PerCwdExtraUserRoots[0].Cwd = %q, want /project", got)
		}
		if got := listParams.PerCwdExtraUserRoots[0].ExtraUserRoots[0]; got != "/Users/demo/skills" {
			t.Fatalf("PerCwdExtraUserRoots[0].ExtraUserRoots[0] = %q, want /Users/demo/skills", got)
		}

		var writeParams codex.SkillsConfigWriteParams
		if err := json.Unmarshal(transport.GetSentRequest(1).Params, &writeParams); err != nil {
			t.Fatalf("unmarshal config write params: %v", err)
		}
		if writeParams.Path != "/repo/.codex/skills/test" {
			t.Fatalf("Path = %q, want /repo/.codex/skills/test", writeParams.Path)
		}
	})

	t.Run("external agent detect cwds", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("externalAgentConfig/detect", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"items":[]}`),
		})

		_, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{
			Cwds: ptr([]string{"/tmp/../repo", `C:\work\..\project`}),
		})
		if err != nil {
			t.Fatalf("ConfigDetect() error = %v", err)
		}

		var params codex.ExternalAgentConfigDetectParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.Cwds == nil {
			t.Fatal("Cwds = nil, want normalized values")
		}
		if got := (*params.Cwds)[0]; got != "/repo" {
			t.Fatalf("Cwds[0] = %q, want /repo", got)
		}
		if got := (*params.Cwds)[1]; got != `C:\project` {
			t.Fatalf("Cwds[1] = %q, want %q", got, `C:\project`)
		}
	})

	t.Run("thread lifecycle cwd", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("thread/start", codex.Response{
			JSONRPC: "2.0",
			Result:  mustJSONRawMessage(t, validThreadLifecycleResponse(validThreadPayload("thread-start"))),
		})
		transport.SetResponse("thread/list", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"data":[]}`),
		})
		transport.SetResponse("thread/resume", codex.Response{
			JSONRPC: "2.0",
			Result:  mustJSONRawMessage(t, validThreadLifecycleResponse(validThreadPayload("thread-resume"))),
		})
		transport.SetResponse("thread/fork", codex.Response{
			JSONRPC: "2.0",
			Result:  mustJSONRawMessage(t, validThreadLifecycleResponse(validThreadPayload("thread-fork"))),
		})

		_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{
			Cwd: ptr("/tmp/../repo"),
		})
		if err != nil {
			t.Fatalf("Thread.Start() error = %v", err)
		}
		_, err = client.Thread.List(context.Background(), codex.ThreadListParams{
			Cwd: ptr("/workspace/../project"),
		})
		if err != nil {
			t.Fatalf("Thread.List() error = %v", err)
		}
		_, err = client.Thread.Resume(context.Background(), codex.ThreadResumeParams{
			ThreadID: "thread-resume",
			Cwd:      ptr("/tmp/../repo"),
		})
		if err != nil {
			t.Fatalf("Thread.Resume() error = %v", err)
		}
		_, err = client.Thread.Fork(context.Background(), codex.ThreadForkParams{
			ThreadID: "thread-fork",
			Cwd:      ptr(`C:\repo\..\fork`),
		})
		if err != nil {
			t.Fatalf("Thread.Fork() error = %v", err)
		}

		var startParams codex.ThreadStartParams
		if err := json.Unmarshal(transport.GetSentRequest(0).Params, &startParams); err != nil {
			t.Fatalf("unmarshal start params: %v", err)
		}
		if startParams.Cwd == nil || *startParams.Cwd != "/repo" {
			t.Fatalf("ThreadStart Cwd = %v, want /repo", startParams.Cwd)
		}

		var listParams codex.ThreadListParams
		if err := json.Unmarshal(transport.GetSentRequest(1).Params, &listParams); err != nil {
			t.Fatalf("unmarshal list params: %v", err)
		}
		if listParams.Cwd == nil || *listParams.Cwd != "/project" {
			t.Fatalf("ThreadList Cwd = %v, want /project", listParams.Cwd)
		}

		var resumeParams codex.ThreadResumeParams
		if err := json.Unmarshal(transport.GetSentRequest(2).Params, &resumeParams); err != nil {
			t.Fatalf("unmarshal resume params: %v", err)
		}
		if resumeParams.Cwd == nil || *resumeParams.Cwd != "/repo" {
			t.Fatalf("ThreadResume Cwd = %v, want /repo", resumeParams.Cwd)
		}

		var forkParams codex.ThreadForkParams
		if err := json.Unmarshal(transport.GetSentRequest(3).Params, &forkParams); err != nil {
			t.Fatalf("unmarshal fork params: %v", err)
		}
		if forkParams.Cwd == nil || *forkParams.Cwd != `C:\fork` {
			t.Fatalf("ThreadFork Cwd = %v, want %q", forkParams.Cwd, `C:\fork`)
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
			name: "filesystem watch request",
			call: func(client *codex.Client) error {
				_, err := client.Fs.Watch(context.Background(), codex.FsWatchParams{
					Path:    "relative/file.txt",
					WatchID: "watch-1",
				})
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
			name: "command cwd",
			call: func(client *codex.Client) error {
				_, err := client.Command.Exec(context.Background(), codex.CommandExecParams{
					Command: []string{"pwd"},
					Cwd:     ptr("repo"),
				})
				return err
			},
		},
		{
			name: "config read cwd",
			call: func(client *codex.Client) error {
				_, err := client.Config.Read(context.Background(), codex.ConfigReadParams{
					Cwd: ptr("repo"),
				})
				return err
			},
		},
		{
			name: "config write file path",
			call: func(client *codex.Client) error {
				_, err := client.Config.Write(context.Background(), codex.ConfigValueWriteParams{
					KeyPath:       "model",
					MergeStrategy: codex.MergeStrategyReplace,
					Value:         json.RawMessage(`"gpt-5"`),
					FilePath:      ptr("config.toml"),
				})
				return err
			},
		},
		{
			name: "config batch write file path",
			call: func(client *codex.Client) error {
				_, err := client.Config.BatchWrite(context.Background(), codex.ConfigBatchWriteParams{
					Edits: []codex.ConfigEdit{{
						KeyPath:       "model",
						MergeStrategy: codex.MergeStrategyReplace,
						Value:         json.RawMessage(`"gpt-5"`),
					}},
					FilePath: ptr("config.toml"),
				})
				return err
			},
		},
		{
			name: "skills list cwd",
			call: func(client *codex.Client) error {
				_, err := client.Skills.List(context.Background(), codex.SkillsListParams{
					Cwds: []string{"repo"},
				})
				return err
			},
		},
		{
			name: "skills list extra roots",
			call: func(client *codex.Client) error {
				_, err := client.Skills.List(context.Background(), codex.SkillsListParams{
					PerCwdExtraUserRoots: []codex.SkillsListExtraRootsForCwd{{
						Cwd:            "/repo",
						ExtraUserRoots: []string{"skills"},
					}},
				})
				return err
			},
		},
		{
			name: "skills config write path",
			call: func(client *codex.Client) error {
				_, err := client.Skills.ConfigWrite(context.Background(), codex.SkillsConfigWriteParams{
					Path:    "skill-path",
					Enabled: true,
				})
				return err
			},
		},
		{
			name: "external agent detect cwd",
			call: func(client *codex.Client) error {
				_, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{
					Cwds: ptr([]string{"repo"}),
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
			name: "thread start cwd",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{
					Cwd: ptr("repo"),
				})
				return err
			},
		},
		{
			name: "thread list cwd",
			call: func(client *codex.Client) error {
				_, err := client.Thread.List(context.Background(), codex.ThreadListParams{
					Cwd: ptr("repo"),
				})
				return err
			},
		},
		{
			name: "thread resume cwd",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{
					ThreadID: "thread-1",
					Cwd:      ptr("repo"),
				})
				return err
			},
		},
		{
			name: "thread fork cwd",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{
					ThreadID: "thread-1",
					Cwd:      ptr("repo"),
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

func TestThreadLifecycleRequestsRejectNonObjectConfigBeforeSend(t *testing.T) {
	tests := []struct {
		name string
		call func(*codex.Client) error
	}{
		{
			name: "thread start array config",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{
					Config: json.RawMessage(`[]`),
				})
				return err
			},
		},
		{
			name: "thread resume string config",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{
					ThreadID: "thread-1",
					Config:   json.RawMessage(`"bad"`),
				})
				return err
			},
		},
		{
			name: "thread fork malformed config",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{
					ThreadID: "thread-1",
					Config:   json.RawMessage(`{`),
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

func mustJSONRawMessage(t *testing.T, value interface{}) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal response fixture: %v", err)
	}
	return data
}
