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

5. **Conversation state to application consumers:** Responses and notifications update cached state. Snapshots and listeners cross an ownership boundary where the recipient may mutate returned values. Clone helpers isolate supported protocol values, including disabled plugin IDs, image file IDs, and MCP app UI metadata. This protects state ownership, not the truthfulness of peer-provided content.

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

Process shutdown uses Unix process groups or Windows Job Objects through [internal/process](../internal/process/). These support cleanup of an owned runtime and its tracked children. They do not establish containment of a hostile program, termination of escaped Unix descendants, termination of remote work, or rollback of completed filesystem/network effects. A process wrapped with `NewProcessFromClient` is externally owned; closing that wrapper does not terminate an external runtime.

## JSON-RPC framing, dispatch, and resource exhaustion

A peer may send malformed envelopes, oversized frames, unexpected IDs, or notification floods. The built-in [stdio transport](../appserver/transport/stdio.go) limits inbound frames to 10 MiB, uses fixed worker pools and bounded queues, and gives outbound operations a default timeout. [Frame parsing](../appserver/transport/stdio_framing.go) distinguishes numeric and string request IDs, checks protocol versions and response shape, and classifies malformed input. Queue-overflow handling and notification classes are explicit in [notification dispatch](../appserver/transport/stdio_notifications.go). All 84 methods in the checked-in notification schema have explicit delivery classes. Thread events, including streaming deltas and completion, share an ordered queue per thread. Process handles, command process IDs, search sessions, import IDs, project IDs, filesystem watch IDs, and MCP subscription IDs have separate identity scopes; account updates share one account scope, and global methods share a queue per method. Exact schema properties determine the owner, including a present empty string when the schema permits it; optional missing or null thread identities use global method scope. Eight workers yield between callbacks, with at most 128 active scopes and 1,280 queued events per scope, and 34,048 queued ordered events in total. Overflow fails the connection explicitly rather than silently discarding an update. Unattributable events retain their class's fallback policy; unknown methods remain best-effort.

All notification delivery classes share a 64 MiB retained-data budget, charged before routing. It covers payload backing capacity, method/version strings and owner keys while events are queued, awaiting handler registration, transferring between buffers, or executing callbacks. A reservation stays with an event through replay and panic recovery until delivery finishes or the event is discarded. Classified delivery fails the connection on byte exhaustion; best-effort delivery may drop an event. Count limits separately bound queue bookkeeping. Accepted ordered events retain their queue ownership and reservation when the reader terminates, including EOF; registering a handler later can drain them. Explicit transport close stops queued notification dispatch and releases buffered payloads. A callback already admitted for execution may finish and keeps its reservation until it returns. Notification and request workers recover application handler panics and suppress secondary panics from the configured panic-reporting callback. Deferred ordered cleanup retires callback ownership before releasing its reservation, allowing subsequent events to proceed after recovery.

These limits do not constitute a total transport or process heap budget. Frame parsing allocates transient data within the frame limit; request buffers, outbound writes, client caches, long conversations, and data retained by application callbacks have separate ownership and limits. Slow or noncooperative callbacks can consume workers and notification capacity, and a peer can make the connection unusable within the stated bounds. Applications must close transports, handle transport failure, and apply their own admission, concurrency, output-retention, and retry policy. Custom transports do not automatically inherit these limits.

Typed decoders validate required fields, selected non-null fields, enums, unions, and request-specific constraints. [Object validation](../appserver/protocol/json_validation.go) and [schema coverage tests](../appserver/protocol/spec_coverage_test.go) help preserve the wire contract. Validation is not universal semantic authorization or a guarantee of canonical JSON: some types use ordinary `encoding/json`, unknown fields may be ignored, and duplicate-member rejection is not a general SDK control. Open JSON metadata and attachment payloads intentionally retain caller/server data. Consumers must validate any additional security meaning before acting on it.

## Prompt injection, approvals, and tool results

A repository file, retrieved resource, MCP tool response, attachment, or model message may try to persuade an application to authorize commands, disclose secrets, or render executable content. The SDK exposes these values and protocol operations; it does not classify their trustworthiness or implement prompt-injection prevention. Treat text and structured metadata as data, including image identifiers, MCP UI resource URIs, prediction text, and plugin metadata.

[Approval handlers](../appserver/protocol/approval_handlers.go) are application-owned callbacks. Without a matching handler, the client returns a JSON-RPC method-not-found error rather than granting approval. A permissive handler or permissive Codex approval policy can deliberately remove that protection. The application must bind decisions to its actual user, account, workspace, operation, and permitted consequences. Dynamic tool callbacks and token-refresh callbacks are privileged code and must not derive authority solely from model output or a claimed thread ID.

The SDK does not fetch or render MCP app UI metadata, execute attachment payloads, or provide an HTML sanitizer. An application that renders model text, links, images, or tool metadata must use appropriate escaping and destination checks. Exposing SDK service methods through HTTP or another remote interface requires a separate authorization boundary.

[Apps configuration decoding](../appserver/protocol/config_apps.go) retains connector defaults, per-tool and per-account approval settings, and omitted model-facing tool surfaces returned by `config/read`. It validates enum values and preserves explicit disabled settings. These values describe server configuration; the SDK does not apply them as an authorization decision, enforce tool exposure, or authenticate the account-link IDs. Codex and the embedding application remain responsible for actual enforcement.

## Thread state, completion, and cancellation

A stale or unrelated notification could contaminate a turn result. [Turn lifecycle handling](../appserver/turn_lifecycle.go) registers listeners before starting a turn, filters thread/turn identities, bounds pending start notifications, and checks terminal status. Its ordering assumptions must also hold for custom transports. Matching IDs prevent accidental cross-talk; a malicious connected peer can still claim those IDs, so correlation does not authenticate completion or independently prove an external effect.

[Conversation snapshots](../appserver/conversation.go) explicitly clone reference-valued protocol fields, while the [protocol cache](../appserver/protocol/clone_helpers.go) uses shared [deep-copy helpers](../internal/deepcopy/deepcopy.go). Lifecycle responses and settings notifications supply cached disabled plugin IDs separately from the Thread wire object; later Thread-only refreshes preserve those saved settings. Cloning the IDs prevents mutation of a returned snapshot from changing cached metadata. The schema describes them as a saved list that does not yet filter plugin capabilities; they are not an SDK enforcement control. Snapshot isolation covers supported protocol shapes; arbitrary application-provided values containing unexported internals, channels, functions, or other opaque references are not a general isolation mechanism. The cache is best-effort state, not durable evidence or an authorization source.

Context cancellation and RPC timeouts stop local waiting or request cooperative interruption. They do not prove that Codex or a remote provider stopped, nor that a command, file write, or network action was undone. Applications must retain uncertainty and reconcile consequential operations before retrying. The SDK does not provide general exactly-once execution, transactional rollback, or durable recovery of application approvals and effects.

## OAuth callbacks, provider routing, and account identity

The [normal login flow](../login/login.go) generates a random state and an S256 PKCE challenge using [crypto/rand](../login/pkce.go). The [callback server](../login/callback.go) defaults to IPv4 loopback, accepts only the callback route, checks the expected state when supplied, and has a header-read timeout. Wrong-state requests do not complete the normal flow. A caller invoking the lower-level callback API with an empty expected state opts out of that comparison.

Callback host, authorization/token endpoints, redirect URI, and HTTP client are [configurable](../login/config.go), including a callback-host environment override. Non-loopback binding and nonstandard endpoints are caller-controlled exposure decisions. The SDK does not require every configured endpoint to use HTTPS, pin endpoint identity, enforce exact browser Origin/Host, or limit callback connections per user. It must not be exposed as a general authenticated HTTP API. The [manual-input parser](../login/parse.go) supports a bare code without state; supplying a manual code relies on the application's trusted user interaction and the flow's PKCE verifier. A provided mismatched state is rejected.

[Token requests](../login/token.go) use the caller's context, bound token-response reads to 1 MiB, and omit non-success response bodies from errors because they may echo credentials. The configured HTTP client's TLS, redirect, and timeout behavior still matters. A malicious configured endpoint, permissive custom HTTP transport, or unsafe URL logger can disclose codes, verifiers, or refresh tokens.

[JWT claim extraction](../login/auth/jwt.go) decodes the payload without signature, issuer, audience, or expiry verification. Account/plan fields are useful metadata from a trusted token source; they must not authorize a user, tenant, routing choice, or backend action on their own. Gateway OAuth methods and explicit-login capability settings in [gateway_oauth.go](../appserver/protocol/gateway_oauth.go) and [initialize.go](../appserver/protocol/initialize.go) convey app-server protocol choices. They do not independently establish consent, account ownership, or secure provider routing.

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
