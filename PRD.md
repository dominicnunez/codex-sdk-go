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

- [ ] Create `go.mod` with module path `github.com/dominicnunez/codex-sdk-go`, Go 1.22, zero external dependencies
- [ ] Create `jsonrpc.go` with core JSON-RPC 2.0 types: `Request` (id + method + params), `Response` (id + result + error), `Notification` (method + params), `Error` (code + message + data), and `RequestID` (string | int64 union type). All types must implement JSON marshal/unmarshal. Include JSON-RPC error codes as constants (-32700 parse error, -32600 invalid request, -32601 method not found, -32602 invalid params, -32603 internal error).
- [ ] Create `transport.go` with a `Transport` interface: `Send(ctx, Request) (Response, error)`, `Notify(ctx, Notification) error`, `OnRequest(handler)`, `OnNotify(handler)`, `Close() error`. This abstracts stdio vs WebSocket vs any future transport.
- [ ] Create `stdio.go` implementing `Transport` over stdin/stdout using newline-delimited JSON. Must handle concurrent reads (server notifications/requests arriving while waiting for a response) by dispatching to handlers in goroutines and matching responses to pending requests by ID.
- [ ] Create `client.go` with `Client` struct using functional options pattern: `NewClient(transport Transport, opts ...ClientOption) *Client`. Options: `WithRequestTimeout(time.Duration)`. Client must track pending requests by ID, route incoming server requests to registered handlers, and route incoming notifications to registered listeners.
- [ ] Create `errors.go` with typed SDK errors: `RPCError` (wraps JSON-RPC error response), `TransportError` (connection/IO failures), `TimeoutError`. All must work with `errors.Is` and `errors.As`.

### Phase 2: V1 Handshake

- [ ] Create `initialize.go` with `InitializeParams` struct (ClientInfo with Name string + Version string + optional Title *string, optional InitializeCapabilities with ExperimentalApi bool + OptOutNotificationMethods []string). Add `Client.Initialize(ctx, InitializeParams) (InitializeResponse, error)` method that sends JSON-RPC request with method `initialize`.
- [ ] Parse `specs/v1/InitializeResponse.json` and create the response type in `initialize.go`. Include all fields from the schema with proper Go types.

### Phase 3: Thread Types & Service

- [ ] Create `thread.go` with all thread-related param and response types parsed from specs: `ThreadStartParams`, `ThreadStartResponse`, `ThreadReadParams`, `ThreadReadResponse`, `ThreadListParams`, `ThreadListResponse`, `ThreadLoadedListParams`, `ThreadLoadedListResponse`, `ThreadResumeParams`, `ThreadResumeResponse`, `ThreadForkParams`, `ThreadForkResponse`, `ThreadRollbackParams`, `ThreadRollbackResponse`, `ThreadSetNameParams`, `ThreadSetNameResponse`, `ThreadArchiveParams`, `ThreadArchiveResponse`, `ThreadUnarchiveParams`, `ThreadUnarchiveResponse`, `ThreadUnsubscribeParams`, `ThreadUnsubscribeResponse`, `ThreadCompactStartParams`, `ThreadCompactStartResponse`. Required fields use direct types, optional fields use pointer types.
- [ ] Create `ThreadService` struct in `thread.go` with methods: `Start(ctx, ThreadStartParams) (ThreadStartResponse, error)`, `Read(ctx, ThreadReadParams) (ThreadReadResponse, error)`, `List(ctx, ThreadListParams) (ThreadListResponse, error)`, `LoadedList(ctx, ThreadLoadedListParams) (ThreadLoadedListResponse, error)`, `Resume(ctx, ThreadResumeParams) (ThreadResumeResponse, error)`, `Fork(ctx, ThreadForkParams) (ThreadForkResponse, error)`, `Rollback(ctx, ThreadRollbackParams) (ThreadRollbackResponse, error)`, `SetName(ctx, ThreadSetNameParams) (ThreadSetNameResponse, error)`, `Archive(ctx, ThreadArchiveParams) (ThreadArchiveResponse, error)`, `Unarchive(ctx, ThreadUnarchiveParams) (ThreadUnarchiveResponse, error)`, `Unsubscribe(ctx, ThreadUnsubscribeParams) (ThreadUnsubscribeResponse, error)`, `CompactStart(ctx, ThreadCompactStartParams) (ThreadCompactStartResponse, error)`. Wire into Client as `Client.Thread`.
- [ ] Create `thread_notifications.go` with notification types: `ThreadStartedNotification`, `ThreadClosedNotification`, `ThreadArchivedNotification`, `ThreadUnarchivedNotification`, `ThreadNameUpdatedNotification`, `ThreadStatusChangedNotification`, `ThreadTokenUsageUpdatedNotification`. Add `Client.OnThreadStarted(func(ThreadStartedNotification))` and equivalent for each notification type.

### Phase 4: Turn Types & Service

- [ ] Create `turn.go` with param/response types from specs: `TurnStartParams`, `TurnStartResponse`, `TurnInterruptParams`, `TurnInterruptResponse`, `TurnSteerParams`, `TurnSteerResponse`. Create `TurnService` struct with methods: `Start(ctx, TurnStartParams) (TurnStartResponse, error)`, `Interrupt(ctx, TurnInterruptParams) (TurnInterruptResponse, error)`, `Steer(ctx, TurnSteerParams) (TurnSteerResponse, error)`. Wire into Client as `Client.Turn`.
- [ ] Create `turn_notifications.go` with notification types: `TurnStartedNotification`, `TurnCompletedNotification`, `TurnPlanUpdatedNotification`, `TurnDiffUpdatedNotification`. Add listener registration methods on Client for each.

### Phase 5: Account & Auth Service

- [ ] Create `account.go` with types from specs: `GetAccountParams`, `GetAccountResponse`, `GetAccountRateLimitsResponse`, `LoginAccountParams`, `LoginAccountResponse`, `CancelLoginAccountParams`, `CancelLoginAccountResponse`, `LogoutAccountResponse`. Create `AccountService` with methods: `Get(ctx, GetAccountParams) (GetAccountResponse, error)`, `GetRateLimits(ctx) (GetAccountRateLimitsResponse, error)`, `Login(ctx, LoginAccountParams) (LoginAccountResponse, error)`, `CancelLogin(ctx, CancelLoginAccountParams) (CancelLoginAccountResponse, error)`, `Logout(ctx) (LogoutAccountResponse, error)`. Wire as `Client.Account`.
- [ ] Create `account_notifications.go` with: `AccountUpdatedNotification`, `AccountLoginCompletedNotification`, `AccountRateLimitsUpdatedNotification`. Add listener registration methods on Client.

### Phase 6: Config Service

- [ ] Create `config.go` with types from specs: `ConfigReadParams`, `ConfigReadResponse`, `ConfigRequirementsReadResponse`, `ConfigValueWriteParams`, `ConfigBatchWriteParams`, `ConfigWriteResponse`. Create `ConfigService` with methods: `Read(ctx, ConfigReadParams) (ConfigReadResponse, error)`, `ReadRequirements(ctx) (ConfigRequirementsReadResponse, error)`, `Write(ctx, ConfigValueWriteParams) (ConfigWriteResponse, error)`, `BatchWrite(ctx, ConfigBatchWriteParams) (ConfigWriteResponse, error)`. Wire as `Client.Config`. Add `ConfigWarningNotification` type and listener.

### Phase 7: Model & Skills Services

- [ ] Create `model.go` with types: `ModelListParams`, `ModelListResponse`, `ModelReroutedNotification`. Create `ModelService` with `List(ctx, ModelListParams) (ModelListResponse, error)`. Wire as `Client.Model`. Add `ModelReroutedNotification` listener.
- [ ] Create `skills.go` with types: `SkillsListParams`, `SkillsListResponse`, `SkillsConfigWriteParams`, `SkillsConfigWriteResponse`, `SkillsRemoteReadParams`, `SkillsRemoteReadResponse`, `SkillsRemoteWriteParams`, `SkillsRemoteWriteResponse`. Create `SkillsService` with methods: `List`, `ConfigWrite`, `RemoteRead`, `RemoteWrite`. Wire as `Client.Skills`.

### Phase 8: Apps, MCP, Command Services

- [ ] Create `apps.go` with types: `AppsListParams`, `AppsListResponse`, `AppListUpdatedNotification`. Create `AppsService` with `List(ctx, AppsListParams) (AppsListResponse, error)`. Wire as `Client.Apps`. Add notification listener.
- [ ] Create `mcp.go` with types: `ListMcpServerStatusParams`, `ListMcpServerStatusResponse`, `McpServerOauthLoginParams`, `McpServerOauthLoginResponse`, `McpServerRefreshResponse`, `McpServerOauthLoginCompletedNotification`, `McpToolCallProgressNotification`. Create `McpService` with methods: `ListServerStatus`, `OauthLogin`, `Refresh`. Wire as `Client.Mcp`. Add notification listeners.
- [ ] Create `command.go` with types: `CommandExecParams`, `CommandExecResponse`, `CommandExecutionOutputDeltaNotification`. Create `CommandService` with `Exec(ctx, CommandExecParams) (CommandExecResponse, error)`. Wire as `Client.Command`. Add notification listener.

### Phase 9: Remaining Services

- [ ] Create `review.go` with types: `ReviewStartParams`, `ReviewStartResponse`. Create `ReviewService` with `Start(ctx, ReviewStartParams) (ReviewStartResponse, error)`. Wire as `Client.Review`.
- [ ] Create `feedback.go` with types: `FeedbackUploadParams`, `FeedbackUploadResponse`. Create `FeedbackService` with `Upload(ctx, FeedbackUploadParams) (FeedbackUploadResponse, error)`. Wire as `Client.Feedback`.
- [ ] Create `external_agent.go` with types: `ExternalAgentConfigDetectParams`, `ExternalAgentConfigDetectResponse`, `ExternalAgentConfigImportParams`, `ExternalAgentConfigImportResponse`. Create `ExternalAgentService` with methods: `ConfigDetect`, `ConfigImport`. Wire as `Client.ExternalAgent`.
- [ ] Create `experimental.go` with types: `ExperimentalFeatureListParams`, `ExperimentalFeatureListResponse`. Create `ExperimentalService` with `FeatureList(ctx, ExperimentalFeatureListParams) (ExperimentalFeatureListResponse, error)`. Wire as `Client.Experimental`.

### Phase 10: Approval Handlers (Server → Client Requests)

- [ ] Create `approval.go` with request/response types for all server-to-client approval flows: `ApplyPatchApprovalParams`/`Response`, `CommandExecutionRequestApprovalParams`/`Response`, `ExecCommandApprovalParams`/`Response`, `FileChangeRequestApprovalParams`/`Response`, `SkillRequestApprovalParams`/`Response`, `DynamicToolCallParams`/`Response`, `ToolRequestUserInputParams`/`Response`, `FuzzyFileSearchParams`/`Response`. Types parsed from corresponding spec files.
- [ ] Create `ApprovalHandler` interface pattern in `approval.go`: define `type ApprovalHandlers struct` with optional function fields for each approval type (e.g. `OnApplyPatch func(context.Context, ApplyPatchApprovalParams) (ApplyPatchApprovalResponse, error)`). Add `Client.SetApprovalHandlers(ApprovalHandlers)` method. When the server sends a request, dispatch to the matching handler; if no handler is set, return a JSON-RPC method-not-found error.
- [ ] Create `fuzzy_search.go` with `FuzzyFileSearchParams`, `FuzzyFileSearchResponse`, `FuzzyFileSearchSessionCompletedNotification`, `FuzzyFileSearchSessionUpdatedNotification`. Wire search approval into ApprovalHandlers and add notification listeners.

### Phase 11: Streaming Notifications

- [ ] Create `streaming.go` with all streaming notification types: `AgentMessageDeltaNotification`, `ItemStartedNotification`, `ItemCompletedNotification`, `RawResponseItemCompletedNotification`, `FileChangeOutputDeltaNotification`, `PlanDeltaNotification`, `ReasoningTextDeltaNotification`, `ReasoningSummaryTextDeltaNotification`, `ReasoningSummaryPartAddedNotification`. Add listener registration methods on Client for each.

### Phase 12: Realtime & System Notifications

- [ ] Create `realtime.go` with notification types: `ThreadRealtimeStartedNotification`, `ThreadRealtimeClosedNotification`, `ThreadRealtimeErrorNotification`, `ThreadRealtimeItemAddedNotification`, `ThreadRealtimeOutputAudioDeltaNotification`. Add listener registration methods on Client.
- [ ] Create `system.go` with notification types: `WindowsSandboxSetupCompletedNotification`, `WindowsWorldWritableWarningNotification`, `ContextCompactedNotification`, `DeprecationNoticeNotification`, `ErrorNotification`, `TerminalInteractionNotification`. Add listener registration methods on Client.

### Phase 13: Helpers & Polish

- [ ] Create `ptr.go` with `Ptr[T](v T) *T` helper function for optional field construction (same pattern as opencode-sdk-go).
- [ ] Create `dispatch.go` that wires the Client's internal message router: incoming JSON-RPC messages are parsed, then dispatched to the correct notification listener or approval handler based on the `method` field. Must handle unknown methods gracefully (log and ignore for notifications, return method-not-found for requests).
- [ ] Create `README.md` with: module path, installation, requirements (Go 1.22+), usage example showing Initialize → ThreadStart → TurnStart → listen for notifications, explanation of the approval handler pattern, link to api.md. Note this is for OpenAI Codex CLI (not OpenCode by Anomaly Co).
- [ ] Create `SECURITY.md` with vulnerability reporting instructions pointing to GitHub issues.
- [ ] Create `LICENSE.md` with MIT license, copyright 2025 Dominic Nunez.

### Phase 14: Testing

- [ ] Create `jsonrpc_test.go` with tests for: Request/Response/Notification JSON marshal/unmarshal round-trips, RequestID string vs int64 handling, Error type serialization, JSON-RPC error code constants.
- [ ] Create `transport_test.go` with a `MockTransport` implementation of the Transport interface that records sent messages and allows injecting responses/notifications. Test concurrent request/response matching.
- [ ] Create `client_test.go` with tests using MockTransport: Initialize handshake, sending a request and receiving a response, notification listener dispatch, approval handler dispatch, unknown method handling, timeout behavior.
- [ ] Create `thread_test.go` with tests for all ThreadService methods using MockTransport: verify correct JSON-RPC method names are sent, params are serialized correctly, responses are deserialized correctly.
- [ ] Create `turn_test.go` with tests for all TurnService methods using MockTransport.
- [ ] Create `approval_test.go` with tests for: approval handler dispatch, missing handler returns method-not-found error, each approval type round-trip.
