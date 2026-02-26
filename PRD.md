# codex-sdk-go — Product Requirements Document

**Module:** `github.com/dominicnunez/codex-sdk-go`
**Protocol:** JSON-RPC 2.0 (not REST — bidirectional client↔server over stdio/WebSocket)
**Specs:** 150 JSON schemas in `specs/` (v1 handshake + v2 full protocol)
**Reference:** Modeled after `opencode-sdk-go` patterns (functional options, typed errors, stdlib only)

---

## Architecture Overview

Codex uses JSON-RPC 2.0 with two message directions:
- **Client → Server:** Requests (expect response) and Notifications (fire-and-forget)
- **Server → Client:** Requests (approval flows) and Notifications (streaming events)

The SDK must handle both directions — it's not just a request/response HTTP client.

### Domain Groupings

| Domain | Requests (client→server) | Notifications (server→client) |
|--------|-------------------------|------------------------------|
| **Lifecycle** | Initialize (v1) | — |
| **Thread** | Start, Read, List, LoadedList, Resume, Fork, Rollback, SetName, Archive, Unarchive, Unsubscribe, CompactStart | Started, Closed, Archived, Unarchived, NameUpdated, StatusChanged, TokenUsageUpdated |
| **Turn** | Start, Interrupt, Steer | Started, Completed, PlanUpdated, DiffUpdated |
| **Account** | GetAccount, LoginAccount, CancelLoginAccount | Updated, LoginCompleted, RateLimitsUpdated |
| **Config** | Read, ValueWrite, BatchWrite | Warning |
| **Model** | List | Rerouted |
| **Skills** | List, ConfigWrite, RemoteRead, RemoteWrite | — |
| **Apps** | List | ListUpdated |
| **MCP** | ListServerStatus, OauthLogin | OauthLoginCompleted, ToolCallProgress |
| **Command** | Exec | ExecutionOutputDelta |
| **Review** | Start | — |
| **Feedback** | Upload | — |
| **ExternalAgent** | ConfigDetect, ConfigImport | — |
| **Experimental** | FeatureList | — |
| **Streaming** | — | AgentMessageDelta, ItemStarted, ItemCompleted, RawResponseItemCompleted, FileChangeOutputDelta, PlanDelta, ReasoningTextDelta, ReasoningSummaryTextDelta, ReasoningSummaryPartAdded |
| **Approval** (server→client requests) | — | ApplyPatchApproval, CommandExecutionRequestApproval, ExecCommandApproval, FileChangeRequestApproval, SkillRequestApproval, DynamicToolCall, ToolRequestUserInput, FuzzyFileSearch |
| **Realtime** | — | Started, Closed, Error, ItemAdded, OutputAudioDelta |
| **System** | WindowsSandboxSetupStart | WindowsSandboxSetupCompleted, WindowsWorldWritableWarning, ContextCompacted, DeprecationNotice, Error, TerminalInteraction |

### Approval Flow

Server sends requests TO the client for approval (patch, command exec, file change, skill, tool calls). The SDK needs a handler/callback pattern so users can respond to these.

---

## Tasks

### Phase 1: Foundation

- [x] Create `go.mod` with module path `github.com/dominicnunez/codex-sdk-go`, Go 1.25, zero external dependencies
- [x] Create `jsonrpc_test.go` with tests for: Request/Response/Notification JSON marshal/unmarshal round-trips, RequestID string vs int64 handling, Error type serialization, JSON-RPC error code constants. Tests reference types from `jsonrpc.go` that don't exist yet — they define the contract.
- [x] Create `jsonrpc.go` with core JSON-RPC 2.0 types: `Request` (id + method + params), `Response` (id + result + error), `Notification` (method + params), `Error` (code + message + data), and `RequestID` (string | int64 union type). All types must implement JSON marshal/unmarshal. Include JSON-RPC error codes as constants (-32700 parse error, -32600 invalid request, -32601 method not found, -32602 invalid params, -32603 internal error). All tests in `jsonrpc_test.go` must pass.
- [x] Create `errors_test.go` with tests for: `RPCError` wrapping a JSON-RPC error response, `TransportError` wrapping IO failures, `TimeoutError`. Test `errors.Is` and `errors.As` behavior for each.
- [x] Create `errors.go` with typed SDK errors: `RPCError` (wraps JSON-RPC error response), `TransportError` (connection/IO failures), `TimeoutError`. All must work with `errors.Is` and `errors.As`. All tests in `errors_test.go` must pass.
- [x] Create `transport.go` with a `Transport` interface: `Send(ctx, Request) (Response, error)`, `Notify(ctx, Notification) error`, `OnRequest(handler)`, `OnNotify(handler)`, `Close() error`. This abstracts stdio vs WebSocket vs any future transport.
- [x] Create `mock_transport_test.go` with a `MockTransport` implementation of the Transport interface that records sent messages and allows injecting responses/notifications. Include helpers for setting up expected request→response pairs and verifying call counts. This is the shared test infrastructure for all subsequent phases.
- [x] Create `transport_test.go` testing concurrent request/response matching using MockTransport.
- [x] Create `stdio_test.go` with tests for the stdio transport: newline-delimited JSON encoding/decoding, concurrent read dispatching, response-to-request ID matching. Use `io.Pipe` to simulate stdin/stdout.
- [x] Create `stdio.go` implementing `Transport` over stdin/stdout using newline-delimited JSON. Must handle concurrent reads (server notifications/requests arriving while waiting for a response) by dispatching to handlers in goroutines and matching responses to pending requests by ID. All tests in `stdio_test.go` must pass.
- [x] Create `client_test.go` with tests using MockTransport: sending a request and receiving a response, notification listener dispatch, unknown method handling, timeout behavior.
- [x] Create `client.go` with `Client` struct using functional options pattern: `NewClient(transport Transport, opts ...ClientOption) *Client`. Options: `WithRequestTimeout(time.Duration)`. Client must track pending requests by ID, route incoming server requests to registered handlers, and route incoming notifications to registered listeners. All tests in `client_test.go` must pass.

### Phase 2: V1 Handshake

- [x] Create `initialize_test.go` with tests for: `InitializeParams` serialization matching `specs/v1/InitializeParams.json` schema, `InitializeResponse` deserialization matching `specs/v1/InitializeResponse.json` schema, `Client.Initialize` round-trip using MockTransport verifying correct JSON-RPC method name `initialize`.
- [x] Create `initialize.go` with `InitializeParams` struct (ClientInfo with Name string + Version string + optional Title *string, optional InitializeCapabilities with ExperimentalApi bool + OptOutNotificationMethods []string). Add `Client.Initialize(ctx, InitializeParams) (InitializeResponse, error)` method that sends JSON-RPC request with method `initialize`. Parse `specs/v1/InitializeResponse.json` and create the response type. All tests in `initialize_test.go` must pass.

### Phase 3: Thread Types & Service

- [x] Create `thread_test.go` with tests for all ThreadService methods using MockTransport: verify correct JSON-RPC method names are sent for each of Start/Read/List/LoadedList/Resume/Fork/Rollback/SetName/Archive/Unarchive/Unsubscribe/CompactStart, params are serialized correctly per their spec schemas, responses are deserialized correctly per their spec schemas.
- [x] Create `thread.go` with all thread-related param and response types parsed from specs: `ThreadStartParams`, `ThreadStartResponse`, `ThreadReadParams`, `ThreadReadResponse`, `ThreadListParams`, `ThreadListResponse`, `ThreadLoadedListParams`, `ThreadLoadedListResponse`, `ThreadResumeParams`, `ThreadResumeResponse`, `ThreadForkParams`, `ThreadForkResponse`, `ThreadRollbackParams`, `ThreadRollbackResponse`, `ThreadSetNameParams`, `ThreadSetNameResponse`, `ThreadArchiveParams`, `ThreadArchiveResponse`, `ThreadUnarchiveParams`, `ThreadUnarchiveResponse`, `ThreadUnsubscribeParams`, `ThreadUnsubscribeResponse`, `ThreadCompactStartParams`, `ThreadCompactStartResponse`. Required fields use direct types, optional fields use pointer types. Create `ThreadService` struct with all methods. Wire into Client as `Client.Thread`. All tests in `thread_test.go` must pass.
- [x] Create `thread_notifications_test.go` with tests for: each notification type deserializes correctly from JSON matching its spec schema, listener registration and dispatch works via MockTransport.
- [x] Create `thread_notifications.go` with notification types: `ThreadStartedNotification`, `ThreadClosedNotification`, `ThreadArchivedNotification`, `ThreadUnarchivedNotification`, `ThreadNameUpdatedNotification`, `ThreadStatusChangedNotification`, `ThreadTokenUsageUpdatedNotification`. Add `Client.OnThreadStarted(func(ThreadStartedNotification))` and equivalent for each notification type. All tests in `thread_notifications_test.go` must pass.

### Phase 4: Turn Types & Service

- [x] Create `turn_test.go` with tests for all TurnService methods using MockTransport: verify correct JSON-RPC method names, param serialization, response deserialization for Start/Interrupt/Steer. Include notification dispatch tests for TurnStarted/TurnCompleted/TurnPlanUpdated/TurnDiffUpdated.
- [x] Create `turn.go` with param/response types from specs: `TurnStartParams`, `TurnStartResponse`, `TurnInterruptParams`, `TurnInterruptResponse`, `TurnSteerParams`, `TurnSteerResponse`. Create `TurnService` struct with methods: `Start(ctx, TurnStartParams) (TurnStartResponse, error)`, `Interrupt(ctx, TurnInterruptParams) (TurnInterruptResponse, error)`, `Steer(ctx, TurnSteerParams) (TurnSteerResponse, error)`. Wire into Client as `Client.Turn`. All tests in `turn_test.go` must pass.
- [x] Create `turn_notifications.go` with notification types: `TurnStartedNotification`, `TurnCompletedNotification`, `TurnPlanUpdatedNotification`, `TurnDiffUpdatedNotification`. Add listener registration methods on Client for each. All notification dispatch tests must pass.

### Phase 5: Account & Auth Service

- [ ] Create `account_test.go` with tests for all AccountService methods using MockTransport: Get/GetRateLimits/Login/CancelLogin/Logout round-trips, plus notification dispatch tests for AccountUpdated/AccountLoginCompleted/AccountRateLimitsUpdated.
- [ ] Create `account.go` with types from specs: `GetAccountParams`, `GetAccountResponse`, `GetAccountRateLimitsResponse`, `LoginAccountParams`, `LoginAccountResponse`, `CancelLoginAccountParams`, `CancelLoginAccountResponse`, `LogoutAccountResponse`. Create `AccountService` with methods: `Get(ctx, GetAccountParams) (GetAccountResponse, error)`, `GetRateLimits(ctx) (GetAccountRateLimitsResponse, error)`, `Login(ctx, LoginAccountParams) (LoginAccountResponse, error)`, `CancelLogin(ctx, CancelLoginAccountParams) (CancelLoginAccountResponse, error)`, `Logout(ctx) (LogoutAccountResponse, error)`. Wire as `Client.Account`. All tests in `account_test.go` must pass.
- [ ] Create `account_notifications.go` with: `AccountUpdatedNotification`, `AccountLoginCompletedNotification`, `AccountRateLimitsUpdatedNotification`. Add listener registration methods on Client. All notification tests must pass.

### Phase 6: Config Service

- [ ] Create `config_test.go` with tests for all ConfigService methods using MockTransport: Read/ReadRequirements/Write/BatchWrite round-trips, plus ConfigWarningNotification dispatch test.
- [ ] Create `config.go` with types from specs: `ConfigReadParams`, `ConfigReadResponse`, `ConfigRequirementsReadResponse`, `ConfigValueWriteParams`, `ConfigBatchWriteParams`, `ConfigWriteResponse`. Create `ConfigService` with methods: `Read(ctx, ConfigReadParams) (ConfigReadResponse, error)`, `ReadRequirements(ctx) (ConfigRequirementsReadResponse, error)`, `Write(ctx, ConfigValueWriteParams) (ConfigWriteResponse, error)`, `BatchWrite(ctx, ConfigBatchWriteParams) (ConfigWriteResponse, error)`. Wire as `Client.Config`. Add `ConfigWarningNotification` type and listener. All tests in `config_test.go` must pass.

### Phase 7: Model & Skills Services

- [ ] Create `model_test.go` with tests for ModelService.List round-trip and ModelReroutedNotification dispatch.
- [ ] Create `model.go` with types: `ModelListParams`, `ModelListResponse`, `ModelReroutedNotification`. Create `ModelService` with `List(ctx, ModelListParams) (ModelListResponse, error)`. Wire as `Client.Model`. Add `ModelReroutedNotification` listener. All tests in `model_test.go` must pass.
- [ ] Create `skills_test.go` with tests for all SkillsService methods: List/ConfigWrite/RemoteRead/RemoteWrite round-trips.
- [ ] Create `skills.go` with types: `SkillsListParams`, `SkillsListResponse`, `SkillsConfigWriteParams`, `SkillsConfigWriteResponse`, `SkillsRemoteReadParams`, `SkillsRemoteReadResponse`, `SkillsRemoteWriteParams`, `SkillsRemoteWriteResponse`. Create `SkillsService` with methods: `List`, `ConfigWrite`, `RemoteRead`, `RemoteWrite`. Wire as `Client.Skills`. All tests in `skills_test.go` must pass.

### Phase 8: Apps, MCP, Command Services

- [ ] Create `apps_test.go` with tests for AppsService.List round-trip and AppListUpdatedNotification dispatch.
- [ ] Create `apps.go` with types: `AppsListParams`, `AppsListResponse`, `AppListUpdatedNotification`. Create `AppsService` with `List(ctx, AppsListParams) (AppsListResponse, error)`. Wire as `Client.Apps`. Add notification listener. All tests in `apps_test.go` must pass.
- [ ] Create `mcp_test.go` with tests for McpService methods: ListServerStatus/OauthLogin/Refresh round-trips, plus OauthLoginCompleted and ToolCallProgress notification dispatch.
- [ ] Create `mcp.go` with types: `ListMcpServerStatusParams`, `ListMcpServerStatusResponse`, `McpServerOauthLoginParams`, `McpServerOauthLoginResponse`, `McpServerRefreshResponse`, `McpServerOauthLoginCompletedNotification`, `McpToolCallProgressNotification`. Create `McpService` with methods: `ListServerStatus`, `OauthLogin`, `Refresh`. Wire as `Client.Mcp`. Add notification listeners. All tests in `mcp_test.go` must pass.
- [ ] Create `command_test.go` with tests for CommandService.Exec round-trip and CommandExecutionOutputDeltaNotification dispatch.
- [ ] Create `command.go` with types: `CommandExecParams`, `CommandExecResponse`, `CommandExecutionOutputDeltaNotification`. Create `CommandService` with `Exec(ctx, CommandExecParams) (CommandExecResponse, error)`. Wire as `Client.Command`. Add notification listener. All tests in `command_test.go` must pass.

### Phase 9: Remaining Services

- [ ] Create `review_test.go` with test for ReviewService.Start round-trip.
- [ ] Create `review.go` with types: `ReviewStartParams`, `ReviewStartResponse`. Create `ReviewService` with `Start(ctx, ReviewStartParams) (ReviewStartResponse, error)`. Wire as `Client.Review`. All tests must pass.
- [ ] Create `feedback_test.go` with test for FeedbackService.Upload round-trip.
- [ ] Create `feedback.go` with types: `FeedbackUploadParams`, `FeedbackUploadResponse`. Create `FeedbackService` with `Upload(ctx, FeedbackUploadParams) (FeedbackUploadResponse, error)`. Wire as `Client.Feedback`. All tests must pass.
- [ ] Create `external_agent_test.go` with tests for ExternalAgentService methods: ConfigDetect/ConfigImport round-trips.
- [ ] Create `external_agent.go` with types: `ExternalAgentConfigDetectParams`, `ExternalAgentConfigDetectResponse`, `ExternalAgentConfigImportParams`, `ExternalAgentConfigImportResponse`. Create `ExternalAgentService` with methods: `ConfigDetect`, `ConfigImport`. Wire as `Client.ExternalAgent`. All tests must pass.
- [ ] Create `experimental_test.go` with test for ExperimentalService.FeatureList round-trip.
- [ ] Create `experimental.go` with types: `ExperimentalFeatureListParams`, `ExperimentalFeatureListResponse`. Create `ExperimentalService` with `FeatureList(ctx, ExperimentalFeatureListParams) (ExperimentalFeatureListResponse, error)`. Wire as `Client.Experimental`. All tests must pass.

### Phase 10: Approval Handlers (Server → Client Requests)

- [ ] Create `approval_test.go` with tests for: each approval type's params/response JSON round-trip matching spec schemas, approval handler dispatch via MockTransport, missing handler returns JSON-RPC method-not-found error, each approval type end-to-end (server sends request → handler called → response sent back). Include ChatgptAuthTokensRefresh as a server→client request.
- [ ] Create `approval.go` with request/response types for all server-to-client approval flows: `ApplyPatchApprovalParams`/`Response`, `CommandExecutionRequestApprovalParams`/`Response`, `ExecCommandApprovalParams`/`Response`, `FileChangeRequestApprovalParams`/`Response`, `SkillRequestApprovalParams`/`Response`, `DynamicToolCallParams`/`Response`, `ToolRequestUserInputParams`/`Response`, `FuzzyFileSearchParams`/`Response`, `ChatgptAuthTokensRefreshParams`/`Response`. Types parsed from corresponding spec files. Create `ApprovalHandlers` struct with optional function fields for each approval type (including `OnChatgptAuthTokensRefresh`). Add `Client.SetApprovalHandlers(ApprovalHandlers)` method. When the server sends a request, dispatch to the matching handler; if no handler is set, return a JSON-RPC method-not-found error. All tests in `approval_test.go` must pass.
- [ ] Create `fuzzy_search_test.go` with tests for FuzzyFileSearch params/response round-trip and notification dispatch for SessionCompleted/SessionUpdated.
- [ ] Create `fuzzy_search.go` with `FuzzyFileSearchParams`, `FuzzyFileSearchResponse`, `FuzzyFileSearchSessionCompletedNotification`, `FuzzyFileSearchSessionUpdatedNotification`. Wire search approval into ApprovalHandlers and add notification listeners. All tests must pass.

### Phase 11: Shared Event Types & Streaming Notifications

- [ ] Create `event_types_test.go` with tests for shared event types from `specs/EventMsg.json`: JSON deserialization of `EventMsg` and its nested definitions (AgentMessageContent variants, AbsolutePathBuf, etc.). These are the base types that streaming notifications embed.
- [ ] Create `event_types.go` with shared types parsed from `specs/EventMsg.json`. These are referenced by multiple streaming notification types and must be defined before the notifications that embed them.
- [ ] Create `streaming_test.go` with tests for each streaming notification type: JSON deserialization matching spec schemas, listener registration and dispatch via MockTransport for all 9 types (AgentMessageDelta, ItemStarted, ItemCompleted, RawResponseItemCompleted, FileChangeOutputDelta, PlanDelta, ReasoningTextDelta, ReasoningSummaryTextDelta, ReasoningSummaryPartAdded).
- [ ] Create `streaming.go` with all streaming notification types: `AgentMessageDeltaNotification`, `ItemStartedNotification`, `ItemCompletedNotification`, `RawResponseItemCompletedNotification`, `FileChangeOutputDeltaNotification`, `PlanDeltaNotification`, `ReasoningTextDeltaNotification`, `ReasoningSummaryTextDeltaNotification`, `ReasoningSummaryPartAddedNotification`. Add listener registration methods on Client for each. All tests in `streaming_test.go` must pass.

### Phase 12: Realtime & System Notifications

- [ ] Create `realtime_test.go` with tests for each realtime notification type: JSON deserialization and listener dispatch for Started/Closed/Error/ItemAdded/OutputAudioDelta.
- [ ] Create `realtime.go` with notification types: `ThreadRealtimeStartedNotification`, `ThreadRealtimeClosedNotification`, `ThreadRealtimeErrorNotification`, `ThreadRealtimeItemAddedNotification`, `ThreadRealtimeOutputAudioDeltaNotification`. Add listener registration methods on Client. All tests must pass.
- [ ] Create `system_test.go` with tests for each system notification type: JSON deserialization and listener dispatch for WindowsSandboxSetupCompleted/WindowsWorldWritableWarning/ContextCompacted/DeprecationNotice/Error/TerminalInteraction. Include tests for WindowsSandboxSetupStart request round-trip.
- [ ] Create `system.go` with notification types: `WindowsSandboxSetupCompletedNotification`, `WindowsWorldWritableWarningNotification`, `ContextCompactedNotification`, `DeprecationNoticeNotification`, `ErrorNotification`, `TerminalInteractionNotification`. Create `SystemService` with `WindowsSandboxSetupStart(ctx, WindowsSandboxSetupStartParams) (WindowsSandboxSetupStartResponse, error)` (client→server request from specs). Wire as `Client.System`. Add listener registration methods on Client. All tests must pass.

### Phase 13: Helpers & Polish

- [ ] Create `ptr.go` with `Ptr[T](v T) *T` helper function for optional field construction (same pattern as opencode-sdk-go).
- [ ] Create `dispatch_test.go` with tests for the message router: known notification methods dispatch to correct listeners, known request methods dispatch to correct approval handlers, unknown notification methods are ignored without error, unknown request methods return method-not-found JSON-RPC error.
- [ ] Create `dispatch.go` that wires the Client's internal message router: incoming JSON-RPC messages are parsed, then dispatched to the correct notification listener or approval handler based on the `method` field. Must handle unknown methods gracefully (log and ignore for notifications, return method-not-found for requests). All tests in `dispatch_test.go` must pass.

### Phase 14: Docs

- [ ] Create `README.md` with: module path, installation, requirements (Go 1.25+), usage example showing Initialize → ThreadStart → TurnStart → listen for notifications, explanation of the approval handler pattern. Note this is for OpenAI Codex CLI (not OpenCode by Anomaly Co).
- [ ] Create `SECURITY.md` with vulnerability reporting instructions pointing to GitHub issues.
- [ ] Create `LICENSE.md` with MIT license, copyright 2025 Dominic Nunez.

### Phase 15: Final Verification & Cleanup

- [ ] Run `go mod tidy` — verify no unexpected dependencies, confirm zero external deps outside stdlib
- [ ] Run `go vet ./...` — fix any issues
- [ ] Run `go build ./...` — verify clean compilation with no errors or warnings
- [ ] Run `go test ./...` — all tests pass
- [ ] Run `go test -race ./...` — no data races detected (critical for the concurrent transport/dispatch code)
- [ ] Run `golangci-lint run ./...` — fix any lint issues (add `.golangci.yml` if needed)
- [ ] Run `govulncheck ./...` — verify no known vulnerabilities in dependencies or stdlib usage
- [ ] Verify every JSON schema in `specs/` has a corresponding Go type — no spec coverage gaps
- [ ] Verify all 38 request methods (1 v1 + 37 v2) have service methods on Client
- [ ] Verify all 40 notification types have listener registration methods on Client
- [ ] Verify all 9 server→client request types have handler fields in ApprovalHandlers (including ChatgptAuthTokensRefresh)
