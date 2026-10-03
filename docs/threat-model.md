# 1. Overview

codex-sdk-go is a Go library for the Codex app-server JSON-RPC protocol, managed app-server processes, conversations and streamed turns, and OAuth login and credential helpers. It uses the Go standard library without external module dependencies. The protocol package accepts a caller-provided transport; the runtime package can launch a caller-selected Codex executable over stdio. The login package also opens a local HTTP callback listener and sends token requests to configured endpoints.

The principal assets are access and refresh tokens, account identities, private prompts and responses, thread state, tool arguments and results, approval decisions, workspace files accessible to Codex, and the host application's availability. Codex can execute commands and modify files according to its own configuration and the application's approval choices. This SDK does not independently implement a sandbox, tenant authorization, tool capability policy, or inference-spend budget.

The intended deployment is an application integrating a trusted Codex runtime or a separately secured transport. The managed process runs with the application's operating-system authority. A thread ID or account ID is a protocol identifier, not proof of an authenticated user or permission to access that user's data. Applications exposing the SDK through a network service must establish their own authentication, authorization, account separation, and request limits.

This document describes the codebase at the revision containing it. Implemented controls, deployment assumptions, and remaining limitations are distinguished below. Tests support particular controls; schema coverage and passing CI do not establish comprehensive security. Package responsibilities and protocol contracts are described in [README.md](../README.md), the [protocol guide](protocol.md), and the checked-in [schemas](../appserver/protocol/schema/json/).

# 2. Threat model, Trust boundaries and assumptions

## Assets and security objectives

- Protect tokens from accidental logging, unintended child-process inheritance, insecure storage, and transmission to an unintended endpoint or peer.
- Preserve request/response correlation and attribute turn events to the correct thread and turn.
- Keep returned conversation and client-cache snapshots isolated from internal state and other consumers.
- Prevent malformed protocol data from silently becoming a valid approval decision or successful turn result.
- Require an explicit application handler for supported server requests; absence of a handler must not grant approval.
- Bound transport frames, dispatch queues, and selected network responses while exposing failures to the application.
- Preserve the distinction between local cancellation, protocol interruption, process termination, and reversal of remote effects.
- Keep dependency, schema, and workflow changes reviewable without treating upstream content as trusted executable authority.

## Trust boundaries

1. **Application to SDK:** The embedding application selects credentials, executable, environment, working directory, transport, models, approval policies, endpoints, and handlers. These are trusted configuration inputs. Prompts, retrieved content, tool results, and server-provided metadata remain untrusted data even when submitted by an authenticated application user.

2. **SDK to Codex process:** The managed runtime launches an absolute executable path with fixed app-server arguments and connects private stdio pipes. The child receives an environment allowlist by default. The executable, its dependencies, configuration, and inherited filesystem authority remain trusted prerequisites. An absolute path prevents PATH selection attacks but does not authenticate the binary or make it safe.

3. **Protocol client to transport and peer:** JSON-RPC responses, notifications, and server requests cross the transport boundary. The built-in stdio transport validates framing and correlation. A custom transport owns its channel security, peer identity, ordering, cancellation, and equivalent resource limits; merely satisfying the Go interface does not establish those properties.

4. **Server request to application authority:** Command, file-change, permission, dynamic-tool, elicitation, attestation, and token-refresh requests invoke application callbacks. Schema-valid requests are not automatically authorized. The application must decide which user, workspace, account, command, or resource is permitted, and Codex must enforce its execution and sandbox configuration.

5. **Conversation state to application consumers:** Responses and notifications update cached state. Snapshots and listeners cross an ownership boundary where the recipient may mutate returned values. Clone helpers isolate supported protocol values, including disabled plugin IDs, image file IDs and detail preferences, and MCP app UI metadata. This protects state ownership, not the truthfulness of peer-provided content.

6. **Browser and network to OAuth login:** The normal login flow generates PKCE and state, publishes an authorization URL through a caller callback, receives a local or manual authorization code, and exchanges it at a configured token endpoint. The browser, local requests, pasted input, endpoint configuration, and HTTP client behavior are separate trust concerns.

7. **Credentials to filesystem and app-server:** Credential helpers persist plaintext JSON and construct token-bearing protocol payloads. The selected path, parent directories, operating-system access controls, transport peer, and logging choices determine confidentiality. Extracted JWT claims are metadata, not locally verified authentication evidence.

8. **Source and upstream schemas to build:** Maintainers review Go code, schemas, workflows, and automated schema-sync PRs. Upstream schema updates can change protocol semantics and require matching implementation and tests. CI actions and the Go toolchain are supply-chain dependencies despite the absence of external Go modules.

## Assumptions and exclusions

The application's account, operating system, Go runtime, selected Codex executable, and trusted configuration are assumed uncompromised. An attacker with equivalent process authority may read memory, alter configuration, or access the same credential files. Strong protection against the host administrator or a malicious embedding application is outside this library's claim.

Untrusted prompts, peers, and lower-privileged processes are not excluded merely because the SDK normally runs locally. They may exploit mistakes in application approval policy, output rendering, endpoint selection, transport implementation, or file ownership. A trusted Codex executable still receives untrusted content and can produce unsafe suggestions.

The SDK is not a hosted multi-tenant service, browser dashboard, durable audit ledger, encrypted credential vault, JWT verifier, or hostile-code containment mechanism. It does not provide quotas for total turn output, provider spend, workspace storage, or arbitrary application callbacks. External provider behavior and Codex's actual sandbox implementation require their own security assessment.

# 3. Attack surface, mitigations and attacker stories

## Process startup and local execution

A malicious PATH entry could substitute a program for Codex. [Process startup](../appserver/process.go) requires an absolute `BinaryPath` and uses separate fixed arguments rather than a shell command string. It forwards a minimal environment allowlist unless the application supplies `Env` or opts into `InheritParentEnv`. These controls reduce executable-selection and unrelated-secret exposure risks; they do not prevent replacement of a writable binary, unsafe child configuration, or intentional forwarding of sensitive environment variables.

The application controls the working directory and stderr destination. Codex and invoked tools may access files with the process account's authority; stderr and explicit application logging may contain sensitive data. Applications must select trusted binaries and protected configuration, use suitable Codex sandbox/approval settings, and restrict filesystem authority for their deployment. The SDK does not check binary signatures or impose an operating-system sandbox.

Process shutdown uses Unix process groups or Windows Job Objects through [internal/process](../internal/process/). These support cleanup of an owned runtime and its tracked children. Unix graceful shutdown waits for both parent completion and group disappearance, polling surviving descendants at a fixed interval after disabling the completed parent signal. Grace expiry still triggers a group kill; OS termination and reaping may finish after that signal returns. These controls do not establish containment of a hostile program, termination of escaped Unix descendants, termination of remote work, or rollback of completed filesystem/network effects. A process wrapped with `NewProcessFromClient` is externally owned; closing that wrapper does not terminate an external runtime.

## JSON-RPC framing, dispatch, and resource exhaustion

Numeric request IDs retain signed 64-bit integer identity across serialization,
equality and pending-response correlation. Caller-provided floating values must
be finite, integral and in [-2^63, 2^63); the upper boundary is rejected before
conversion or pending-request admission. Inbound JSON integers are range checked
without floating conversion. String IDs remain a distinct correlation family.
These checks preserve correlation identity; an ID is not authentication.

A peer may send malformed envelopes, oversized frames, unexpected IDs, or notification floods. The built-in [stdio transport](../appserver/transport/stdio.go) limits inbound frames to 10 MiB, uses fixed worker pools and bounded queues, and gives outbound operations a default timeout. [Frame parsing](../appserver/transport/stdio_framing.go) distinguishes numeric and string request IDs, checks protocol versions and response shape, and classifies malformed input. Queue-overflow handling and notification classes are explicit in [notification dispatch](../appserver/transport/stdio_notifications.go). All 84 methods in the checked-in notification schema have explicit delivery classes. Thread events, including streaming deltas and completion, share an ordered queue per thread. Process handles, command process IDs, search sessions, import IDs, project IDs, filesystem watch IDs, and MCP subscription IDs have separate identity scopes; account updates share one account scope, and global methods share a queue per method. Exact schema properties determine the owner, including a present empty string when the schema permits it; optional missing or null thread identities use global method scope. Eight workers yield between callbacks, with at most 128 active scopes and 1,280 queued events per scope, and 34,048 queued ordered events in total. Overflow fails the connection explicitly rather than silently discarding an update. Unattributable events retain their class's fallback policy; unknown methods remain best-effort.

All notification delivery classes share a 64 MiB retained-data budget, charged before routing. It covers payload backing capacity, method/version strings and owner keys while events are queued, awaiting handler registration, transferring between buffers, or executing callbacks. A reservation stays with an event through replay and panic recovery until delivery finishes or the event is discarded. Classified delivery fails the connection on byte exhaustion; best-effort delivery may drop an event. Count limits separately bound queue bookkeeping. Accepted ordered events retain their queue ownership and reservation when the reader terminates, including EOF; registering a handler later can drain them. Explicit transport close stops queued notification dispatch and releases buffered payloads. A callback already admitted for execution may finish and keeps its reservation until it returns. Notification and request workers recover application handler panics and suppress secondary panics from the configured panic-reporting callback. Deferred ordered cleanup retires callback ownership before releasing its reservation, allowing subsequent events to proceed after recovery.

Transport identity and envelope selection scan raw JSON without materializing ignored properties or decoding overwritten identities. Selected fields are decoded once, including nested thread IDs and RPC error fields; malformed and oversized recovery inspect bounded prefixes while preserving usable earlier correlation. The shared raw scanner also serves runtime lifecycle fallback attribution and fixed-property response-shape and union-discriminator selection. For syntactically valid objects, protocol object validation skips ignored properties and checks every recognized occurrence in wire order. Plain string fields retain only the final usable spelling for decoding, while required/non-null/type failures remain visible even when followed by a valid duplicate; custom semantic fields still decode each occurrence. Direct calls with syntactically invalid JSON preserve the earlier prefix/error behavior through a compatibility decoder, which built-in wire unmarshaling does not enter. Full JSON validation and recovery impose a nesting limit of 10,000 containers. Scalar request-ID decoding rejects composite values before allocating their graphs and rejects oversized numeric spellings before diagnostic formatting. These controls keep discarded fields and overwritten selector identities out of decoded object graphs; they do not bound arbitrary semantic payload decoding. Allocation measurements for recovery use standard builds, since race instrumentation deliberately changes allocation and pool behavior.

Lifecycle attribution preserves the identity contract of each carrier. Top-level notification identity properties use exact names; nested Thread and Turn ID selection retains the existing typed decoder's folded spelling, duplicate and null behavior. Malformed turn completion may recover an ID that the complete schema validator rejects, but produces a failed completion only for the matching active turn. An unrelated semantic failure must not change which turn receives completion. Discriminator selection preserves standard type-error field and object-offset diagnostics. Direct malformed request-ID decoding preserves the prior syntax/EOF and trailing-input errors through a decoder that discards values without constructing their graphs; its temporary input buffer remains proportional to input size.

These limits do not constitute a total transport or process heap budget. Frame parsing still copies raw data and allocates transient storage proportional to frame size and bounded nesting. Typed payload decoding, request buffers, outbound writes, client caches, long conversations, and data retained by application callbacks have separate ownership and limits. Slow or noncooperative callbacks can consume workers and notification capacity, and a peer can make the connection unusable within the stated bounds. Applications must close transports, handle transport failure, and apply their own admission, concurrency, output-retention, and retry policy. Custom transports do not automatically inherit these limits.

Typed decoders validate required fields, selected non-null fields, enums, unions, and request-specific constraints. [Object validation](../appserver/protocol/json_validation.go) and [schema coverage tests](../appserver/protocol/spec_coverage_test.go) help preserve the wire contract. Validation is not universal semantic authorization or a guarantee of canonical JSON: some semantic leaf types use ordinary `encoding/json`, which can allocate when unquoting escaped field names or decoding known duplicate values, and duplicate-member rejection is not a general SDK control. Selected fallback item/turn payloads, unknown variants, open JSON metadata, application configuration maps, and attachments intentionally retain their semantic data. Consumers must validate any additional security meaning before acting on it.

[Moderation metadata admission](../appserver/protocol/notification_owner.go)
requires the schema's metadata member but accepts every present JSON value,
including null and numeric tokens outside float64 precision or range. The
new `TurnModerationMetadataJSONNotification` field owns raw JSON. The original
notification retains its object-map API and float64 conversion behavior; its
legacy handlers can reject nonobject metadata or large numbers. Applications
requiring the complete schema use the JSON replacement/append methods, which
share the same wire method and isolate their decoding from legacy listeners.
Consecutive duplicate
objects merge their top-level members, while another value kind resets that
accumulation. The decoder accumulates duplicate objects before one final
serialization; it does not repeatedly serialize the growing object. Each typed
replacement or append recipient independently decodes and owns its metadata
bytes. This prevents representation loss and recipient mutation from changing
another recipient's data; it does not interpret moderation evidence or grant
authority. Payload copying and duplicate-object accumulation remain proportional
to admitted semantic data, with no additional metadata byte quota. Application
retention and any later decoding or numeric conversion are separate contracts.

[Synced record admission](../appserver/protocol/synced_record_validation.go)
validates app, workspace-message, token-usage, goal, section and import records
at their shared decoding owners before typed service success or callback
publication. Required arrays reject null elements, and schema enums distinguish
detected connector sources from imported sources even though the detection
response retains its legacy public field type. App-tool enabled state defaults
to true when omitted and retains explicit false during serialization; this
metadata does not grant tool authority. These owners use exact schema property
names, accept valid empty and zero values, and retain failures from invalid
recognized duplicates. A failed decode leaves the record receiver unchanged;
a successful decode replaces it and resets omitted optional fields. These
rules differ from ordinary Go JSON receiver merging. Errors use the existing
strict field decoder's diagnostics and can stop traversal of a containing
application wrapper. [Scratch-state reuse](../appserver/protocol/synced_object_decode.go)
retains only schema/type metadata between decodes and clears payload references
and admission flags before reuse; it does not bound the number of valid records
in an admitted payload. Generic raw notification handlers remain responsible
for their own decoding and validation.

[Optional array admission](../appserver/protocol/optional_array_validation.go)
rejects forbidden null arrays and null string elements in plugin, app, model,
thread lifecycle, reasoning, text-input and workspace-sandbox carriers. Guards
cover the folded and escaped names accepted by their existing wire decoders;
invalid earlier occurrences remain failures after valid duplicates. Nullable
arrays retain their separate schema contracts, and opaque strings do not gain
path restrictions. Marketplace error records require their canonical schema
members, reject nulls through recognized aliases, and validate the same final
absolute path that the wire decoder publishes. Validation uses existing response,
Config and union owners after their established decoding and semantic checks.
The formerly plain concrete representations retain ordinary Go JSON decoding,
receiver merging, error context and named or anonymous envelope traversal;
decoding those representations directly does not perform SDK carrier admission.
Fresh response owners retain replacement semantics. Config retains its existing
merge and partial-update behavior; callers must not consume a failed decode.
New admission errors propagate through existing custom decoders and can stop
outer traversal or precede later service validation. This does not establish
error parity for payloads that newly fail the array contract.
These admission checks neither authorize the reported data nor bound the number
of valid elements or callback-owned copies.

SDK-generated previews of rejected paths, string enums, discriminators, app and
answer-map keys, and request identifiers quote at most 256 source bytes and
report the omitted byte count. The bound applies before escaping and wrapping,
across protocol validation and transport duplicate-ID errors. Short diagnostics,
field context, error classification and validation precedence remain intact;
duplicate-ID errors retain their existing unquoted short display. Validation
and decoding may still allocate for the complete input. Accepted long values,
unknown raw variants and failed receivers' existing partial updates are not
truncated. Structured RPC messages remain semantic error data and can retain
full values. This is not a total response-size, diagnostic-size or heap budget.

At audited SDK-owned protocol, runtime, token-response and JWT-claim decoding
boundaries, native JSON numeric conversion errors retain at most 2,048 bytes in
`json.UnmarshalTypeError.Value`. Descriptions exceeding that threshold keep a
quoted 256-byte source preview and omitted-byte count before SDK wrapping or
runtime fallback copies. The bounded error keeps the concrete type, target type,
offset, struct and field context; short diagnostics, error precedence and partial
receiver updates remain unchanged. Normalization copies metadata without keeping
the discarded full error as a cause. Standard JSON decoding and numeric parsing
can still allocate temporary full-token copies; accepted raw or `json.Number`
values remain complete.

Unrestricted application decoding and callback errors retain their ownership
and identity. `OptionalNullable[T]` only normalizes native type errors when the
entire static destination graph lacks custom JSON/text decoders and interface
slots. Extension-bearing graphs pass errors through, even if a native numeric
failure occurs in another field. Application-selected envelope labels and
structured RPC data are also outside these bounds.

Native malformed `json.Number` formatter errors have a separate control. At
audited SDK decoding owners and codec-free `OptionalNullable[T]` destinations,
and at SDK request preparation, request/result serialization, generic nullable
serialization, unrestricted union and user-message input serialization,
elicitation requests, turn output-schema and MCP/dynamic thread-item serialization
owners, formatter messages exceeding 2,048 bytes retain a quoted preview of at
most 256 bytes of the already formatted literal plus an omitted-byte count.
Inner SDK marshalers apply this before Go adds its `json.MarshalerError` wrapper.
The replacement retains no original error as a cause; ordinary short errors,
JSON admission, partial receiver updates and valid long numeric tokens are
unchanged, including empty constructed Numbers encoding as zero.
Union interface embedding can supply application fields without a custom JSON
codec; unrestricted SDK wrappers apply the native formatter policy to these
values too. Wrappers that accept only explicit concrete variants retain their
existing admission rules.

Application JSON/Text marshaler error chains pass through unchanged. Generic
decoding with interface slots or application codecs also passes errors through,
including reused interfaces containing a Number, because error text alone cannot
distinguish a native decoder failure from an application-owned error. Plain
caller-owned `encoding/json` operations, application handler errors, structured
RPC data and other formatter families such as invalid numeric `,string` values
remain outside this control. Standard parsing and native number formatting still
allocate temporary full input or quoted-error copies; this bounds retained native
diagnostics and subsequent SDK wrapping, not total operation allocation.

Terminal transport cleanup clears requests awaiting handler registration and
drains abandoned inbound requests and outbound envelopes. Queue admission is
coordinated with cleanup, including blocked producers and registration replay,
so late sends cannot refill stopped queues. Dequeued work claims execution
against terminal state before entering a handler or external writer. A callback
or write already claimed may finish and retains its in-flight payload until it
returns. Accepted response outcomes and the separate notification EOF-drain
policy remain intact. Releasing transport references is not secure erasure or
a guarantee that application-owned or actively executing data has been freed.

Client listener unsubscription clears vacated callback slots in both notification
and thread-state registries and deletes empty registration entries. Completed or
canceled turn lifecycles release their registrations; conversation handles require
Close or eventual runtime cleanup. A dispatch snapshot admitted before removal
may still invoke its callbacks and retain captured data until fanout finishes.
This is reference release, not secure erasure or cancellation of admitted callbacks.

The stream collector copies a retained suffix when trimming oversized plan text
or command delta history and clears evicted delta/raw-chunk backing slots. Its
existing byte/count limits therefore release the discarded text references in
those owners while preserving suffix and aggregation semantics. These limits do
not bound full semantic lifecycle items, the number of lifecycle records, or
snapshots retained by the application. Untrimmed immutable strings may share
caller storage, and ingestion can temporarily allocate for the complete input;
this is not a total collector heap or peak-allocation budget.

Collector lifecycle storage uses distinct thread, turn and item components as a
tuple, including command output and byte accounting. Summary keys preserve unique
bare item IDs and unambiguous scoped spellings, with collision-free opaque
fallbacks when arbitrary ID strings conflict. Identity remains available in each
lifecycle's fields; presentation keys must not be parsed as authority. Correlation
separates records but does not authenticate peer-provided IDs or content.
Latest plan deltas append only within the same full tuple; changing thread, turn
or item identity replaces that latest text and resets its dropped-byte accounting.

For an outbound request admitted to the serialized write queue, accepting its
correlated result and abandoning it on cancellation or transport failure share
one synchronized ownership boundary. An accepted response survives later EOF,
read/write failure or explicit close, including when the writer has transmitted
the delimiter but has not returned. Writer completion without a usable response
does not establish success. Invalid serialization and failed queue admission
remain failures. Peer errors and correlated malformed-response errors still
reach the protocol client's error handling. Notifications and internal replies
have no correlated inbound outcome and continue to wait for actual write
completion. An arbitrary caller-provided writer may block inside Write; the SDK
bounds caller waiting and queued work but cannot forcibly stop that writer.

Thread-state cache fanout also recovers each listener panic independently,
including initial replay and direct service or cache updates. Committed state
and closure continue to later listeners, so an earlier application callback
cannot suppress a Conversation's closure update. Error reporting preserves the
originating method or direct operation context and suppresses reporter panics.
Initial replay still returns registration cleanup after recovery. Callbacks run
outside the cache lock with independent snapshots. Each recipient runs serially
in cache commit order. Idle delivery runs on the publishing call; concurrent or
reentrant writes to a busy recipient commit and return with delivery deferred.
Pending updates coalesce to the latest snapshot. A pending close supersedes
older updates and survives reopening, preceding the latest reopened update;
repeated close/reopen transitions while busy may coalesce to one closure and the
latest update. Already admitted delivery may finish after unsubscribe. Each
recipient retains at most one executing snapshot, one pending latest snapshot
and one pending closure; complete semantic Thread graphs are not byte bounded.
Callbacks must return promptly: panic recovery does not stop blocked callbacks
or undo their external effects, and an earlier blocked recipient can delay
drainers already selected for later recipients.

Conversation startup couples successful response publication, listener
registration and cache-incarnation capture before callbacks or eviction. Failed
requests create no new subscription or publication. Cache pressure cannot evict
the new thread before its Conversation pins observation. If closure coalesces
away the initial pending update, the closed handle retains its owned initial
response without adopting a reopened incarnation or restoring the cache. Generic
listener replay follows the same per-recipient delivery admission. New turns
check that incarnation against committed cache
state, including closure before the selected Conversation callback executes.
An already admitted turn may finish after closure or local Close and return its
owned historical result. Completion atomically appends to the latest metadata
only within the captured open incarnation; it cannot reopen a closed entry or
publish into an entry recreated after eviction or reopening. Ordered listeners
own the Conversation's current snapshot, which freezes on local or delivered
closure. Cache generations are local best-effort state, not remote revisions,
durable identities or authorization. Transport EOF does not synthesize a thread
closure, and historical results may differ from subsequently published state.

## Prompt injection, approvals, and tool results

A repository file, retrieved resource, MCP tool response, attachment, or model message may try to persuade an application to authorize commands, disclose secrets, or render executable content. The SDK exposes these values and protocol operations; it does not classify their trustworthiness or implement prompt-injection prevention. Treat text and structured metadata as data, including image identifiers, MCP UI resource URIs, prediction text, and plugin metadata.

[Approval handlers](../appserver/protocol/approval_handlers.go) are application-owned callbacks. Without a matching handler, the client returns a JSON-RPC method-not-found error rather than granting approval. A permissive handler or permissive Codex approval policy can deliberately remove that protection. The application must bind decisions to its actual user, account, workspace, operation, and permitted consequences. Dynamic tool callbacks and token-refresh callbacks are privileged code and must not derive authority solely from model output or a claimed thread ID.

Filesystem approval scope retains structured read, write and deny entries, path/glob/special-location variants, scan depth and legacy path strings. Explicit empty entries remain distinct from absent entries, because the runtime gives present entries precedence over legacy lists. Typed approval requests and guardian permission-review metadata validate the known scope before callback dispatch; invalid programmatic grants fail before a successful response is produced. Normalization clones mutable filesystem scope without resolving paths or expanding patterns. Guardian actions retain their generic map shape, with the known `globScanMaxDepth` represented as `json.Number` to preserve integers beyond float64 precision. These controls preserve the requested and granted scope; Codex and the application remain responsible for interpreting paths, authorizing decisions and enforcing access.

Approval cwd, command-action and legacy parsed-command paths, grantRoot, and
file-change path strings remain opaque request evidence. The SDK does not resolve
these values against cwd or substitute normalized paths before callbacks. Actual
AbsolutePathBuf fields retain their separate typed validation. An application
must distinguish the schema contracts when deriving display paths or applying
policy; preserved strings do not authenticate a target or grant filesystem access.

The SDK does not fetch or render MCP app UI metadata, execute attachment payloads, or provide an HTML sanitizer. An application that renders model text, links, images, or tool metadata must use appropriate escaping and destination checks. Exposing SDK service methods through HTTP or another remote interface requires a separate authorization boundary.

Typed MCP results preserve arbitrary `_meta` JSON, including explicit null and
precise numbers, through item decoding and owned runtime snapshots. Config and
catalog responses retain approval-review routing, desktop settings, model service
tiers, plugin availability and sharing metadata, hook handler details, and token
cache-write counts. These are peer-provided descriptions, not local authorization
or billing evidence. Plugin schedules are decoded as data; the SDK does not run
their prompts. External config imports return the required import ID so callers
can correlate progress and completion; that ID does not authenticate the peer or
prove that an import completed.
New nullable collection fields retain absence, explicit null and present empty
values separately. Their exported presence and value state can be copied without
sharing collection storage. Nullable scalar and struct-pointer fields retain the
SDK's existing nil-as-unspecified convention; exact null/absence preservation is
not a general guarantee for all optional protocol fields.

[Apps configuration decoding](../appserver/protocol/config_apps.go) retains connector defaults, per-tool and per-account approval settings, and omitted model-facing tool surfaces returned by `config/read`. It validates enum values and preserves explicit disabled settings. These values describe server configuration; the SDK does not apply them as an authorization decision, enforce tool exposure, or authenticate the account-link IDs. Codex and the embedding application remain responsible for actual enforcement.

[Workspace restriction decoding](../appserver/protocol/config_workspace.go)
retains a single workspace ID or a list without narrowing a list to its first ID.
Null and absence use the existing unspecified-value convention; an explicit empty
list retains its array shape. The SDK exposes these peer-provided login
restrictions as configuration data. It does not authenticate the IDs or enforce
workspace membership in its independent OAuth login helpers. Codex and the
embedding application own enforcement and the meaning of an empty restriction.

## Thread state, completion, and cancellation

An accepted turn-start response does not complete a runtime turn. If the peer
terminates without a matching terminal notification, Run and RunStreamed still
wait for the caller's context to end; callers must bound that context as needed.

A stale or unrelated notification could contaminate a turn result. [Turn lifecycle handling](../appserver/turn_lifecycle.go) registers listeners before starting a turn, filters thread/turn identities, bounds pending start notifications, and checks terminal status. Its ordering assumptions must also hold for custom transports. Matching IDs prevent accidental cross-talk; a malicious connected peer can still claim those IDs, so correlation does not authenticate completion or independently prove an external effect.

[Conversation snapshots](../appserver/conversation.go), the [protocol cache](../appserver/protocol/clone_helpers.go), collector item ingestion and summaries use shared [deep-copy helpers](../internal/deepcopy/deepcopy.go) for complete reference-valued protocol shapes. Streamed completion retains an owned item before exposing public events; derived collaboration events and turn-completion events have separate references. Result item/turn/thread views and repeated `Stream.Result()` snapshots remain independent. These copies preserve nil versus present empty values, dynamic union types and raw metadata without a serialization round trip. Lifecycle responses and settings notifications supply cached disabled plugin IDs separately from the Thread wire object; later Thread-only refreshes preserve those saved settings. Cloning the IDs prevents mutation of a returned snapshot from changing cached metadata. The schema describes them as a saved list that does not yet filter plugin capabilities; they are not an SDK enforcement control. Snapshot isolation covers supported protocol shapes; arbitrary application-provided values containing unexported internals, channels, functions, or other opaque references are not a general isolation mechanism. The cache is best-effort state, not durable evidence or an authorization source.

Context cancellation and RPC timeouts stop local waiting or request cooperative interruption. They do not prove that Codex or a remote provider stopped, nor that a command, file write, or network action was undone. Applications must retain uncertainty and reconcile consequential operations before retrying. The SDK does not provide general exactly-once execution, transactional rollback, or durable recovery of application approvals and effects.

## OAuth callbacks, provider routing, and account identity

The [normal login flow](../login/login.go) generates a random state and an S256 PKCE challenge using [crypto/rand](../login/pkce.go). The [callback server](../login/callback.go) defaults to IPv4 loopback, accepts only the callback route, checks the expected state when supplied, and has a header-read timeout. Wrong-state requests do not complete the normal flow. A caller invoking the lower-level callback API with an empty expected state opts out of that comparison.

Callback host, authorization/token endpoints, redirect URI, and HTTP client are [configurable](../login/config.go), including a callback-host environment override. Non-loopback binding and nonstandard endpoints are caller-controlled exposure decisions. The SDK does not require every configured endpoint to use HTTPS, pin endpoint identity, enforce exact browser Origin/Host, or limit callback connections per user. It must not be exposed as a general authenticated HTTP API. The [manual-input parser](../login/parse.go) supports a bare code without state; supplying a manual code relies on the application's trusted user interaction and the flow's PKCE verifier. A provided mismatched state is rejected.

[Token requests](../login/token.go) use the caller's context, bound token-response reads to 1 MiB, and omit non-success response bodies from errors because they may echo credentials. The configured HTTP client's TLS, redirect, and timeout behavior still matters. A malicious configured endpoint, permissive custom HTTP transport, or unsafe URL logger can disclose codes, verifiers, or refresh tokens.

Both authorization-code exchange and refresh reject nonpositive lifetimes and
`expires_in` values above 9,223,372,036 seconds before multiplying into a signed
nanosecond duration or publishing credentials. Accepted lifetimes preserve their
intended expiration from the local clock; unsupported values return no partial
credentials, and lifetime errors omit token fields. This prevents arithmetic
overflow, not false lifetime claims from a trusted endpoint or verification of
token validity. Stored credentials may already be expired and still need refresh.

[JWT claim extraction](../login/auth/jwt.go) decodes the payload without signature, issuer, audience, or expiry verification. Account/plan fields are useful metadata from a trusted token source; they must not authorize a user, tenant, routing choice, or backend action on their own. Gateway OAuth methods and explicit-login capability settings in [gateway_oauth.go](../appserver/protocol/gateway_oauth.go) and [initialize.go](../appserver/protocol/initialize.go) convey app-server protocol choices. They do not independently establish consent, account ownership, or secure provider routing.

Initialization retains attestation, MCP extension and legacy form opt-ins as part
of the connection's handshake identity, independently of experimental API and
gateway OAuth choices. The client owns the serialized extension settings before
waiting on another initialization; ProcessOptions resolution also owns those
settings. Returned initialized parameters and mismatch diagnostics are separate
snapshots. Snapshot errors fail admission before dispatch or child launch;
process startup rechecks cancellation after parameter preparation. Extension
values use raw JSON, so exact numbers are preserved while capabilities,
initialize parameters and caller-defined wrappers retain ordinary Go decoding,
including partial updates and retained references when a type error occurs.
JSON numbers retain their exact spelling, object key order is ignored,
and array order remains significant when checking handshake identity. An absent,
null or empty root extension object declares no extensions; a named null setting
or empty settings object still declares that extension. Failed requests and
invalid responses do not latch the handshake, and failure of the subsequent
`initialized` notification permits a notification retry without renegotiating
the latched session. These declarations convey choices to the app-server; the
SDK does not independently enforce the server's attestation or elicitation policy.

## Secrets, persistence, and disclosure

[Credential storage](../login/auth/store.go) validates required credential fields, writes a temporary file in the destination directory, and replaces the target by rename. It requests directory mode `0700` and file mode `0600`, with a directory-mode check on non-Windows systems. Credentials remain plaintext. Windows mode bits do not establish a restrictive ACL, and loading does not independently validate ownership, permissions, symlinks, or a maximum file size. The application must choose a protected storage location and appropriate platform access controls; this is not a keychain or encrypted vault.

[Credential values](../login/auth/credentials.go) and [auth payload helpers](../login/auth/params.go) redact tokens in their custom formatting methods, while their JSON serialization intentionally includes secrets for storage and protocol exchange. The protocol package's login and refresh credential types redact both ordinary formatting and `json.Marshal`; dedicated wire serialization sends the real tokens. These distinctions have coverage in [credential_redact_test.go](../appserver/protocol/credential_redact_test.go). Redaction does not cover every raw request, error, copied string, application logger, or transport recorder. Prompts, account IDs, paths, stderr, and tool results may also be sensitive even when tokens are removed.

Applications must avoid logging serialized auth requests, protect backups and traces, and minimize credential lifetime and authority. The SDK does not promise encrypted memory, secure deletion, crash-dump protection, or complete audit logging. Returned attachment payloads and conversation data must retain their original trust level when stored or reused.

## Setup and supply chain

An upstream schema change can alter required fields, unions, routing metadata, or approval semantics. [Spec-sync maintenance](spec-sync.md) and the [sync workflow](../.github/workflows/spec-sync.yml) create reviewable changes; imported schemas are the protocol contract, not proof that an implementation is safe. Review must include custom marshalers, dispatch registration, snapshot ownership, and callers of newly added fields.

The [CI workflow](../.github/workflows/ci.yml) runs formatting, module tidiness, builds, vet, Linux and Windows tests, race tests, and lint. These detect selected implementation defects without establishing trustworthy runtime binaries or release provenance. The absence of external Go modules reduces one dependency surface; action version tags, the Go toolchain, Codex, and upstream schemas remain dependencies. No signed-release or hostile-build isolation guarantee is made here.

# 4. Criticality calibration

Severity depends on demonstrated reachability, attacker prerequisites, deployment authority, affected data, and consequences. The examples below calibrate possible findings; they are not a list of confirmed vulnerabilities. A malicious administrator, intentionally permissive application policy, or unsupported deployment assumption does not by itself establish an SDK defect.

## Critical

- A remotely reachable SDK flaw that enables arbitrary code execution with broad application authority without an authorized approval or trusted configuration change.
- A protocol or account-isolation bypass that exposes credentials or consequential execution across many independently authorized users in a supported deployment.

## High

- Disclosure of an access/refresh token to an attacker-controlled peer, endpoint, log, or file through a reachable violation of the SDK's documented controls.
- An approval or permission-handling defect that turns denial or an absent handler into authorization for consequential commands or file changes.
- Cross-thread result or state corruption that demonstrably causes unauthorized disclosure or execution in an application relying on supported SDK contracts.

## Medium

- A bounded peer-triggered denial of service, incorrect cancellation/completion attribution, or state-aliasing defect with meaningful availability or integrity impact and narrower consequences.
- Sensitive-data exposure requiring additional deployment conditions or application behavior, with a demonstrated SDK contribution to the violated boundary.

## Low

- Limited metadata disclosure, recoverable protocol rejection, or a defense-in-depth gap without a demonstrated credential, execution, or authorization consequence.
- Documentation inaccuracies or compatibility defects with low security impact; raise severity only when an actual caller and consequence establish it.
