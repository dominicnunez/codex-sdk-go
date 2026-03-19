package codex_test

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestAllRequestMethodsCovered verifies that every client→server request method
// defined in specs/ClientRequest.json has a corresponding service method on the Client,
// and that each service method sends the correct RPC method string on the wire.
func TestAllRequestMethodsCovered(t *testing.T) {
	requiredMethods := loadClientRequestMethods(t)

	// Create a mock transport and client to verify service methods exist
	transport := NewMockTransport()
	client := codex.NewClient(transport)

	// Track which methods are verified
	verified := make(map[string]bool)

	// Verify Initialize (v1)
	verified["initialize"] = verifyMethod(t, transport, "initialize", func() {
		_, _ = client.Initialize(context.Background(), codex.InitializeParams{})
	})

	// Verify Account service methods
	verified["account/read"] = verifyMethod(t, transport, "account/read", func() {
		_, _ = client.Account.Get(context.Background(), codex.GetAccountParams{})
	})
	verified["account/rateLimits/read"] = verifyMethod(t, transport, "account/rateLimits/read", func() {
		_, _ = client.Account.GetRateLimits(context.Background())
	})
	verified["account/login/start"] = verifyMethod(t, transport, "account/login/start", func() {
		_, _ = client.Account.Login(context.Background(), &codex.ChatgptLoginAccountParams{Type: "chatgpt"})
	})
	verified["account/login/cancel"] = verifyMethod(t, transport, "account/login/cancel", func() {
		_, _ = client.Account.CancelLogin(context.Background(), codex.CancelLoginAccountParams{})
	})
	verified["account/logout"] = verifyMethod(t, transport, "account/logout", func() {
		_, _ = client.Account.Logout(context.Background())
	})

	// Verify Apps service
	verified["app/list"] = verifyMethod(t, transport, "app/list", func() {
		_, _ = client.Apps.List(context.Background(), codex.AppsListParams{})
	})

	// Verify Command service
	verified["command/exec"] = verifyMethod(t, transport, "command/exec", func() {
		_, _ = client.Command.Exec(context.Background(), codex.CommandExecParams{
			Command: []string{"echo", "hello"},
		})
	})
	verified["command/exec/write"] = verifyMethod(t, transport, "command/exec/write", func() {
		_, _ = client.Command.Write(context.Background(), codex.CommandExecWriteParams{ProcessID: "proc-1"})
	})
	verified["command/exec/terminate"] = verifyMethod(t, transport, "command/exec/terminate", func() {
		_, _ = client.Command.Terminate(context.Background(), codex.CommandExecTerminateParams{ProcessID: "proc-1"})
	})
	verified["command/exec/resize"] = verifyMethod(t, transport, "command/exec/resize", func() {
		_, _ = client.Command.Resize(context.Background(), codex.CommandExecResizeParams{
			ProcessID: "proc-1",
			Size:      codex.CommandExecTerminalSize{Cols: 80, Rows: 24},
		})
	})

	// Verify Config service
	verified["config/read"] = verifyMethod(t, transport, "config/read", func() {
		_, _ = client.Config.Read(context.Background(), codex.ConfigReadParams{})
	})
	verified["configRequirements/read"] = verifyMethod(t, transport, "configRequirements/read", func() {
		_, _ = client.Config.ReadRequirements(context.Background())
	})
	verified["config/value/write"] = verifyMethod(t, transport, "config/value/write", func() {
		_, _ = client.Config.Write(context.Background(), codex.ConfigValueWriteParams{
			KeyPath:       "model",
			MergeStrategy: codex.MergeStrategyReplace,
			Value:         json.RawMessage(`"gpt-5"`),
		})
	})
	verified["config/batchWrite"] = verifyMethod(t, transport, "config/batchWrite", func() {
		_, _ = client.Config.BatchWrite(context.Background(), codex.ConfigBatchWriteParams{
			Edits: []codex.ConfigEdit{{
				KeyPath:       "model",
				MergeStrategy: codex.MergeStrategyReplace,
				Value:         json.RawMessage(`"gpt-5"`),
			}},
		})
	})
	verified["config/mcpServer/reload"] = verifyMethod(t, transport, "config/mcpServer/reload", func() {
		_, _ = client.Mcp.Refresh(context.Background())
	})

	// Verify Experimental service
	verified["experimentalFeature/list"] = verifyMethod(t, transport, "experimentalFeature/list", func() {
		_, _ = client.Experimental.FeatureList(context.Background(), codex.ExperimentalFeatureListParams{})
	})

	// Verify External Agent service
	verified["externalAgentConfig/detect"] = verifyMethod(t, transport, "externalAgentConfig/detect", func() {
		_, _ = client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{})
	})
	verified["externalAgentConfig/import"] = verifyMethod(t, transport, "externalAgentConfig/import", func() {
		_, _ = client.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{
			MigrationItems: []codex.ExternalAgentConfigMigrationItem{{
				Description: "Import repo config",
				ItemType:    codex.MigrationItemTypeConfig,
			}},
		})
	})

	// Verify Feedback service
	verified["feedback/upload"] = verifyMethod(t, transport, "feedback/upload", func() {
		_, _ = client.Feedback.Upload(context.Background(), codex.FeedbackUploadParams{})
	})

	// Verify Fs service
	verified["fs/readFile"] = verifyMethod(t, transport, "fs/readFile", func() {
		_, _ = client.Fs.ReadFile(context.Background(), codex.FsReadFileParams{Path: "/tmp/file"})
	})
	verified["fs/writeFile"] = verifyMethod(t, transport, "fs/writeFile", func() {
		_, _ = client.Fs.WriteFile(context.Background(), codex.FsWriteFileParams{Path: "/tmp/file", DataBase64: "ZGF0YQ=="})
	})
	verified["fs/createDirectory"] = verifyMethod(t, transport, "fs/createDirectory", func() {
		_, _ = client.Fs.CreateDirectory(context.Background(), codex.FsCreateDirectoryParams{Path: "/tmp/dir"})
	})
	verified["fs/getMetadata"] = verifyMethod(t, transport, "fs/getMetadata", func() {
		_, _ = client.Fs.GetMetadata(context.Background(), codex.FsGetMetadataParams{Path: "/tmp/file"})
	})
	verified["fs/readDirectory"] = verifyMethod(t, transport, "fs/readDirectory", func() {
		_, _ = client.Fs.ReadDirectory(context.Background(), codex.FsReadDirectoryParams{Path: "/tmp"})
	})
	verified["fs/remove"] = verifyMethod(t, transport, "fs/remove", func() {
		_, _ = client.Fs.Remove(context.Background(), codex.FsRemoveParams{Path: "/tmp/file"})
	})
	verified["fs/copy"] = verifyMethod(t, transport, "fs/copy", func() {
		_, _ = client.Fs.Copy(context.Background(), codex.FsCopyParams{SourcePath: "/tmp/src", DestinationPath: "/tmp/dst"})
	})

	// Verify FuzzyFileSearch service
	verified["fuzzyFileSearch"] = verifyMethod(t, transport, "fuzzyFileSearch", func() {
		_, _ = client.FuzzyFileSearch.Search(context.Background(), codex.FuzzyFileSearchParams{
			Query: "main.go",
			Roots: []string{"/tmp/project"},
		})
	})

	// Verify MCP service
	verified["mcpServerStatus/list"] = verifyMethod(t, transport, "mcpServerStatus/list", func() {
		_, _ = client.Mcp.ListServerStatus(context.Background(), codex.ListMcpServerStatusParams{})
	})
	verified["mcpServer/oauth/login"] = verifyMethod(t, transport, "mcpServer/oauth/login", func() {
		_, _ = client.Mcp.OauthLogin(context.Background(), codex.McpServerOauthLoginParams{})
	})

	// Verify Model service
	verified["model/list"] = verifyMethod(t, transport, "model/list", func() {
		_, _ = client.Model.List(context.Background(), codex.ModelListParams{})
	})

	// Verify Review service
	verified["review/start"] = verifyMethod(t, transport, "review/start", func() {
		_, _ = client.Review.Start(context.Background(), codex.ReviewStartParams{
			ThreadID: "thread-1",
			Target: codex.ReviewTargetWrapper{
				Value: &codex.UncommittedChangesReviewTarget{},
			},
		})
	})

	// Verify Plugin service
	verified["plugin/list"] = verifyMethod(t, transport, "plugin/list", func() {
		_, _ = client.Plugin.List(context.Background(), codex.PluginListParams{})
	})
	verified["plugin/read"] = verifyMethod(t, transport, "plugin/read", func() {
		_, _ = client.Plugin.Read(context.Background(), codex.PluginReadParams{MarketplacePath: "/tmp/market", PluginName: "plugin"})
	})
	verified["plugin/install"] = verifyMethod(t, transport, "plugin/install", func() {
		_, _ = client.Plugin.Install(context.Background(), codex.PluginInstallParams{MarketplacePath: "/tmp/market", PluginName: "plugin"})
	})
	verified["plugin/uninstall"] = verifyMethod(t, transport, "plugin/uninstall", func() {
		_, _ = client.Plugin.Uninstall(context.Background(), codex.PluginUninstallParams{PluginID: "plugin-1"})
	})

	// Verify Skills service
	verified["skills/list"] = verifyMethod(t, transport, "skills/list", func() {
		_, _ = client.Skills.List(context.Background(), codex.SkillsListParams{})
	})
	verified["skills/config/write"] = verifyMethod(t, transport, "skills/config/write", func() {
		_, _ = client.Skills.ConfigWrite(context.Background(), codex.SkillsConfigWriteParams{})
	})

	// Verify Thread service
	verified["thread/start"] = verifyMethod(t, transport, "thread/start", func() {
		_, _ = client.Thread.Start(context.Background(), codex.ThreadStartParams{})
	})
	verified["thread/read"] = verifyMethod(t, transport, "thread/read", func() {
		_, _ = client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "thread-1"})
	})
	verified["thread/list"] = verifyMethod(t, transport, "thread/list", func() {
		_, _ = client.Thread.List(context.Background(), codex.ThreadListParams{})
	})
	verified["thread/loaded/list"] = verifyMethod(t, transport, "thread/loaded/list", func() {
		_, _ = client.Thread.LoadedList(context.Background(), codex.ThreadLoadedListParams{})
	})
	verified["thread/resume"] = verifyMethod(t, transport, "thread/resume", func() {
		_, _ = client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "thread-1"})
	})
	verified["thread/fork"] = verifyMethod(t, transport, "thread/fork", func() {
		_, _ = client.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: "thread-1"})
	})
	verified["thread/rollback"] = verifyMethod(t, transport, "thread/rollback", func() {
		_, _ = client.Thread.Rollback(context.Background(), codex.ThreadRollbackParams{ThreadID: "thread-1", NumTurns: 1})
	})
	verified["thread/name/set"] = verifyMethod(t, transport, "thread/name/set", func() {
		_, _ = client.Thread.SetName(context.Background(), codex.ThreadSetNameParams{ThreadID: "thread-1", Name: "thread"})
	})
	verified["thread/metadata/update"] = verifyMethod(t, transport, "thread/metadata/update", func() {
		_, _ = client.Thread.MetadataUpdate(context.Background(), codex.ThreadMetadataUpdateParams{ThreadID: "thread-1"})
	})
	verified["thread/archive"] = verifyMethod(t, transport, "thread/archive", func() {
		_, _ = client.Thread.Archive(context.Background(), codex.ThreadArchiveParams{ThreadID: "thread-1"})
	})
	verified["thread/unarchive"] = verifyMethod(t, transport, "thread/unarchive", func() {
		_, _ = client.Thread.Unarchive(context.Background(), codex.ThreadUnarchiveParams{ThreadID: "thread-1"})
	})
	verified["thread/unsubscribe"] = verifyMethod(t, transport, "thread/unsubscribe", func() {
		_, _ = client.Thread.Unsubscribe(context.Background(), codex.ThreadUnsubscribeParams{ThreadID: "thread-1"})
	})
	verified["thread/compact/start"] = verifyMethod(t, transport, "thread/compact/start", func() {
		_, _ = client.Thread.CompactStart(context.Background(), codex.ThreadCompactStartParams{ThreadID: "thread-1"})
	})

	// Verify Turn service
	verified["turn/start"] = verifyMethod(t, transport, "turn/start", func() {
		_, _ = client.Turn.Start(context.Background(), codex.TurnStartParams{
			ThreadID: "thread-1",
			Input:    []codex.UserInput{&codex.TextUserInput{Text: "hello"}},
		})
	})
	verified["turn/interrupt"] = verifyMethod(t, transport, "turn/interrupt", func() {
		_, _ = client.Turn.Interrupt(context.Background(), codex.TurnInterruptParams{ThreadID: "thread-1", TurnID: "turn-1"})
	})
	verified["turn/steer"] = verifyMethod(t, transport, "turn/steer", func() {
		_, _ = client.Turn.Steer(context.Background(), codex.TurnSteerParams{
			ThreadID:       "thread-1",
			ExpectedTurnID: "turn-1",
			Input:          []codex.UserInput{&codex.TextUserInput{Text: "hello"}},
		})
	})

	// Verify System service
	verified["windowsSandbox/setupStart"] = verifyMethod(t, transport, "windowsSandbox/setupStart", func() {
		_, _ = client.System.WindowsSandboxSetupStart(context.Background(), codex.WindowsSandboxSetupStartParams{})
	})

	// Check that all methods were verified
	var missing []string
	for method := range requiredMethods {
		if !verified[method] {
			missing = append(missing, method)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("Missing service methods for the following protocol methods: %v", missing)
	}

	// Report summary
	t.Logf("Verified %d/%d client→server request methods have corresponding SDK methods", len(verified), len(requiredMethods))
}

func loadClientRequestMethods(t *testing.T) map[string]bool {
	t.Helper()

	data, err := os.ReadFile("specs/ClientRequest.json")
	if err != nil {
		t.Fatalf("ReadFile(specs/ClientRequest.json) failed: %v", err)
	}

	var schema struct {
		OneOf []json.RawMessage `json:"oneOf"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Unmarshal(specs/ClientRequest.json) failed: %v", err)
	}

	methods := make(map[string]bool)
	for _, variant := range schema.OneOf {
		var request struct {
			Properties struct {
				Method struct {
					Enum []string `json:"enum"`
				} `json:"method"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(variant, &request); err != nil {
			t.Fatalf("Unmarshal client request variant failed: %v", err)
		}
		if len(request.Properties.Method.Enum) != 1 {
			t.Fatalf("client request variant has %d method values, want 1", len(request.Properties.Method.Enum))
		}
		methods[request.Properties.Method.Enum[0]] = true
	}

	if len(methods) == 0 {
		t.Fatal("found 0 client request methods in specs/ClientRequest.json")
	}

	return methods
}

// verifyMethod calls the service method and asserts that the mock transport
// recorded a request with the expected RPC method string. This catches methods
// that exist but are wired to the wrong RPC method name.
func verifyMethod(t *testing.T, transport *MockTransport, expectedMethod string, fn func()) bool {
	t.Helper()
	before := transport.CallCount()
	fn()
	after := transport.CallCount()

	if after <= before {
		t.Errorf("%s: service method did not send a request", expectedMethod)
		return false
	}

	lastReq := transport.GetSentRequest(after - 1)
	if lastReq.Method != expectedMethod {
		t.Errorf("%s: sent wrong RPC method %q", expectedMethod, lastReq.Method)
		return false
	}
	return true
}

func TestRepresentativeRequestMethodOutcomes(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	ctx := context.Background()

	_ = transport.SetResponseData("initialize", validInitializeResponseData("codex-test/1.0"))
	_ = transport.SetResponseData("thread/start", validProcessThreadStartResponse(validProcessThreadPayload("thread-1")))
	transport.SetResponse("account/read", codex.Response{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`"not-an-object"`),
	})

	initResp, err := client.Initialize(ctx, codex.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize() unexpected error: %v", err)
	}
	if initResp.UserAgent != "codex-test/1.0" {
		t.Fatalf("Initialize() userAgent = %q, want %q", initResp.UserAgent, "codex-test/1.0")
	}

	threadResp, err := client.Thread.Start(ctx, codex.ThreadStartParams{Ephemeral: codex.Ptr(false)})
	if err != nil {
		t.Fatalf("Thread.Start() unexpected error: %v", err)
	}
	if threadResp.Thread.ID != "thread-1" {
		t.Fatalf("Thread.Start() thread.id = %q, want %q", threadResp.Thread.ID, "thread-1")
	}

	_, err = client.Account.Get(ctx, codex.GetAccountParams{})
	if err == nil {
		t.Fatal("Account.Get() expected decode error from invalid response payload")
	}
	if !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Fatalf("Account.Get() error = %q, want decode failure context", err)
	}
}

func TestRequestValidationRejectsMalformedParamsBeforeSending(t *testing.T) {
	tests := []struct {
		name string
		call func(*codex.Client) error
	}{
		{
			name: "review start zero value params",
			call: func(client *codex.Client) error {
				_, err := client.Review.Start(context.Background(), codex.ReviewStartParams{})
				return err
			},
		},
		{
			name: "fuzzy search zero value params",
			call: func(client *codex.Client) error {
				_, err := client.FuzzyFileSearch.Search(context.Background(), codex.FuzzyFileSearchParams{})
				return err
			},
		},
		{
			name: "external agent import zero value params",
			call: func(client *codex.Client) error {
				_, err := client.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{})
				return err
			},
		},
		{
			name: "turn start nil input",
			call: func(client *codex.Client) error {
				_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{
					ThreadID: "thread-1",
				})
				return err
			},
		},
		{
			name: "turn steer nil input",
			call: func(client *codex.Client) error {
				_, err := client.Turn.Steer(context.Background(), codex.TurnSteerParams{
					ThreadID:       "thread-1",
					ExpectedTurnID: "turn-1",
				})
				return err
			},
		},
		{
			name: "config batch write zero value params",
			call: func(client *codex.Client) error {
				_, err := client.Config.BatchWrite(context.Background(), codex.ConfigBatchWriteParams{})
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
			if got := transport.CallCount(); got != 0 {
				t.Fatalf("transport recorded %d requests, want 0", got)
			}
		})
	}
}
