package codex_test

import (
	"context"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestAllRequestMethodsCovered verifies that every client→server request method
// defined in specs/ClientRequest.json has a corresponding service method on the Client,
// and that each service method sends the correct RPC method string on the wire.
func TestAllRequestMethodsCovered(t *testing.T) {
	// All 42 client→server request methods extracted from specs/ClientRequest.json
	// Format: "method/name" → description of what it does
	requiredMethods := map[string]string{
		// v1 handshake
		"initialize": "Client.Initialize()",

		// Account service (5 methods)
		"account/read":            "Client.Account.Get()",
		"account/rateLimits/read": "Client.Account.GetRateLimits()",
		"account/login/start":     "Client.Account.Login()",
		"account/login/cancel":    "Client.Account.CancelLogin()",
		"account/logout":          "Client.Account.Logout()",

		// Apps service (1 method)
		"app/list": "Client.Apps.List()",

		// Command service (1 method)
		"command/exec": "Client.Command.Exec()",

		// Config service (5 methods)
		"config/read":             "Client.Config.Read()",
		"configRequirements/read": "Client.Config.ReadRequirements()",
		"config/value/write":      "Client.Config.Write()",
		"config/batchWrite":       "Client.Config.BatchWrite()",
		"config/mcpServer/reload": "Client.Mcp.Refresh()",

		// Experimental service (1 method)
		"experimentalFeature/list": "Client.Experimental.FeatureList()",

		// External agent service (2 methods)
		"externalAgentConfig/detect": "Client.ExternalAgent.ConfigDetect()",
		"externalAgentConfig/import": "Client.ExternalAgent.ConfigImport()",

		// Feedback service (1 method)
		"feedback/upload": "Client.Feedback.Upload()",

		// Fuzzy search (client→server request, not an approval flow)
		"fuzzyFileSearch": "Client.FuzzyFileSearch.Search()",

		// MCP service (2 methods)
		"mcpServerStatus/list":  "Client.Mcp.ListServerStatus()",
		"mcpServer/oauth/login": "Client.Mcp.OauthLogin()",

		// Model service (1 method)
		"model/list": "Client.Model.List()",

		// Review service (1 method)
		"review/start": "Client.Review.Start()",

		// Skills service (4 methods)
		"skills/list":          "Client.Skills.List()",
		"skills/config/write":  "Client.Skills.ConfigWrite()",
		"skills/remote/list":   "Client.Skills.RemoteRead()",
		"skills/remote/export": "Client.Skills.RemoteWrite()",

		// Thread service (11 methods)
		"thread/start":         "Client.Thread.Start()",
		"thread/read":          "Client.Thread.Read()",
		"thread/list":          "Client.Thread.List()",
		"thread/loaded/list":   "Client.Thread.LoadedList()",
		"thread/resume":        "Client.Thread.Resume()",
		"thread/fork":          "Client.Thread.Fork()",
		"thread/rollback":      "Client.Thread.Rollback()",
		"thread/name/set":      "Client.Thread.SetName()",
		"thread/archive":       "Client.Thread.Archive()",
		"thread/unarchive":     "Client.Thread.Unarchive()",
		"thread/unsubscribe":   "Client.Thread.Unsubscribe()",
		"thread/compact/start": "Client.Thread.CompactStart()",

		// Turn service (3 methods)
		"turn/start":     "Client.Turn.Start()",
		"turn/interrupt": "Client.Turn.Interrupt()",
		"turn/steer":     "Client.Turn.Steer()",

		// System service (1 method)
		"windowsSandbox/setupStart": "Client.System.WindowsSandboxSetupStart()",
	}

	if len(requiredMethods) != 42 {
		t.Fatalf("Expected 42 methods in test map, got %d", len(requiredMethods))
	}

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
		_, _ = client.Command.Exec(context.Background(), codex.CommandExecParams{})
	})

	// Verify Config service
	verified["config/read"] = verifyMethod(t, transport, "config/read", func() {
		_, _ = client.Config.Read(context.Background(), codex.ConfigReadParams{})
	})
	verified["configRequirements/read"] = verifyMethod(t, transport, "configRequirements/read", func() {
		_, _ = client.Config.ReadRequirements(context.Background())
	})
	verified["config/value/write"] = verifyMethod(t, transport, "config/value/write", func() {
		_, _ = client.Config.Write(context.Background(), codex.ConfigValueWriteParams{})
	})
	verified["config/batchWrite"] = verifyMethod(t, transport, "config/batchWrite", func() {
		_, _ = client.Config.BatchWrite(context.Background(), codex.ConfigBatchWriteParams{})
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
		_, _ = client.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{})
	})

	// Verify Feedback service
	verified["feedback/upload"] = verifyMethod(t, transport, "feedback/upload", func() {
		_, _ = client.Feedback.Upload(context.Background(), codex.FeedbackUploadParams{})
	})

	// Verify FuzzyFileSearch service
	verified["fuzzyFileSearch"] = verifyMethod(t, transport, "fuzzyFileSearch", func() {
		_, _ = client.FuzzyFileSearch.Search(context.Background(), codex.FuzzyFileSearchParams{})
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
		_, _ = client.Review.Start(context.Background(), codex.ReviewStartParams{})
	})

	// Verify Skills service
	verified["skills/list"] = verifyMethod(t, transport, "skills/list", func() {
		_, _ = client.Skills.List(context.Background(), codex.SkillsListParams{})
	})
	verified["skills/config/write"] = verifyMethod(t, transport, "skills/config/write", func() {
		_, _ = client.Skills.ConfigWrite(context.Background(), codex.SkillsConfigWriteParams{})
	})
	verified["skills/remote/list"] = verifyMethod(t, transport, "skills/remote/list", func() {
		_, _ = client.Skills.RemoteRead(context.Background(), codex.SkillsRemoteReadParams{})
	})
	verified["skills/remote/export"] = verifyMethod(t, transport, "skills/remote/export", func() {
		_, _ = client.Skills.RemoteWrite(context.Background(), codex.SkillsRemoteWriteParams{})
	})

	// Verify Thread service
	verified["thread/start"] = verifyMethod(t, transport, "thread/start", func() {
		_, _ = client.Thread.Start(context.Background(), codex.ThreadStartParams{})
	})
	verified["thread/read"] = verifyMethod(t, transport, "thread/read", func() {
		_, _ = client.Thread.Read(context.Background(), codex.ThreadReadParams{})
	})
	verified["thread/list"] = verifyMethod(t, transport, "thread/list", func() {
		_, _ = client.Thread.List(context.Background(), codex.ThreadListParams{})
	})
	verified["thread/loaded/list"] = verifyMethod(t, transport, "thread/loaded/list", func() {
		_, _ = client.Thread.LoadedList(context.Background(), codex.ThreadLoadedListParams{})
	})
	verified["thread/resume"] = verifyMethod(t, transport, "thread/resume", func() {
		_, _ = client.Thread.Resume(context.Background(), codex.ThreadResumeParams{})
	})
	verified["thread/fork"] = verifyMethod(t, transport, "thread/fork", func() {
		_, _ = client.Thread.Fork(context.Background(), codex.ThreadForkParams{})
	})
	verified["thread/rollback"] = verifyMethod(t, transport, "thread/rollback", func() {
		_, _ = client.Thread.Rollback(context.Background(), codex.ThreadRollbackParams{})
	})
	verified["thread/name/set"] = verifyMethod(t, transport, "thread/name/set", func() {
		_, _ = client.Thread.SetName(context.Background(), codex.ThreadSetNameParams{})
	})
	verified["thread/archive"] = verifyMethod(t, transport, "thread/archive", func() {
		_, _ = client.Thread.Archive(context.Background(), codex.ThreadArchiveParams{})
	})
	verified["thread/unarchive"] = verifyMethod(t, transport, "thread/unarchive", func() {
		_, _ = client.Thread.Unarchive(context.Background(), codex.ThreadUnarchiveParams{})
	})
	verified["thread/unsubscribe"] = verifyMethod(t, transport, "thread/unsubscribe", func() {
		_, _ = client.Thread.Unsubscribe(context.Background(), codex.ThreadUnsubscribeParams{})
	})
	verified["thread/compact/start"] = verifyMethod(t, transport, "thread/compact/start", func() {
		_, _ = client.Thread.CompactStart(context.Background(), codex.ThreadCompactStartParams{})
	})

	// Verify Turn service
	verified["turn/start"] = verifyMethod(t, transport, "turn/start", func() {
		_, _ = client.Turn.Start(context.Background(), codex.TurnStartParams{})
	})
	verified["turn/interrupt"] = verifyMethod(t, transport, "turn/interrupt", func() {
		_, _ = client.Turn.Interrupt(context.Background(), codex.TurnInterruptParams{})
	})
	verified["turn/steer"] = verifyMethod(t, transport, "turn/steer", func() {
		_, _ = client.Turn.Steer(context.Background(), codex.TurnSteerParams{})
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

	if len(missing) > 0 {
		t.Errorf("Missing service methods for the following protocol methods: %v", missing)
	}

	// Report summary
	t.Logf("Verified %d/%d client→server request methods have corresponding SDK methods", len(verified), len(requiredMethods))
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
