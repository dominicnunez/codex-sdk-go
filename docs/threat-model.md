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

5. **Conversation state to application consumers:** Responses and notifications update cached state. Snapshots and listeners cross an ownership boundary where the recipient may mutate returned values. Clone helpers isolate supported protocol values, including disabled plugin IDs, generated-image file references and input provenance, image detail preferences, and MCP app UI metadata. Generated-image paths and hashes are peer assertions; decoding does not read or verify local image files. Read-state notifications validate their required revision and unread-position branch before typed callbacks and use protected per-thread transport delivery. Opaque MCP tool-list metadata remains untrusted raw JSON. This protects state ownership, not the truthfulness of peer-provided content.

6. **Browser and network to OAuth login:** The normal login flow generates PKCE and state, publishes an authorization URL through a caller callback, receives a local or manual authorization code, and exchanges it at a configured token endpoint. The browser, local requests, pasted input, endpoint configuration, and HTTP client behavior are separate trust concerns.

7. **Credentials to filesystem and app-server:** Credential helpers persist plaintext JSON and construct token-bearing protocol payloads. The selected path, parent directories, operating-system access controls, transport peer, and logging choices determine confidentiality. Extracted JWT claims are metadata, not locally verified authentication evidence.

8. **Source and upstream schemas to build:** Maintainers review Go code, schemas, workflows, and automated schema-sync PRs. Upstream schema updates can change protocol semantics and require matching implementation and tests. CI actions and the Go toolchain are supply-chain dependencies despite the absence of external Go modules.

## Assumptions and exclusions

The application's account, operating system, Go runtime, selected Codex executable, and trusted configuration are assumed uncompromised. An attacker with equivalent process authority may read memory, alter configuration, or access the same credential files. Strong protection against the host administrator or a malicious embedding application is outside this library's claim.

Untrusted prompts, peers, and lower-privileged processes are not excluded merely because the SDK normally runs locally. They may exploit mistakes in application approval policy, output rendering, endpoint selection, transport implementation, or file ownership. A trusted Codex executable still receives untrusted content and can produce unsafe suggestions.

The SDK is not a hosted multi-tenant service, browser dashboard, durable audit ledger, encrypted credential vault, JWT verifier, or hostile-code containment mechanism. It does not provide quotas for total turn output, provider spend, workspace storage, or arbitrary application callbacks. External provider behavior and Codex's actual sandbox implementation require their own security assessment.

# 3. Attack surface, mitigations and attacker stories

The JSON admission and bounded-diagnostic controls below use Go 1.27.3's default
`encoding/json` implementation. No experiment override is required. Native error
paths and enclosing traversal follow that implementation; a failed direct decode
can leave partial receiver updates and later enclosing fields may be visited.
Typed SDK service calls reject failed admission before returning a response.

## Process startup and local execution

A malicious PATH entry could substitute a program for Codex. [Process startup](../appserver/process.go) requires an absolute `BinaryPath` and uses separate fixed arguments rather than a shell command string. It forwards a minimal environment allowlist unless the application supplies `Env` or opts into `InheritParentEnv`. These controls reduce executable-selection and unrelated-secret exposure risks; they do not prevent replacement of a writable binary, unsafe child configuration, or intentional forwarding of sensitive environment variables.

The application controls the working directory and stderr destination. Codex and invoked tools may access files with the process account's authority; stderr and explicit application logging may contain sensitive data. Applications must select trusted binaries and protected configuration, use suitable Codex sandbox/approval settings, and restrict filesystem authority for their deployment. The SDK does not check binary signatures or impose an operating-system sandbox.

Process shutdown uses Unix process groups or Windows Job Objects through [internal/process](../internal/process/). These support cleanup of an owned runtime and its tracked children. Unix graceful shutdown waits for both parent completion and group disappearance, polling surviving descendants at a fixed interval after disabling the completed parent signal. Grace expiry still triggers a group kill; OS termination and reaping may finish after that signal returns. These controls do not establish containment of a hostile program, termination of escaped Unix descendants, termination of remote work, or rollback of completed filesystem/network effects. A process wrapped with `NewProcessFromClient` is externally owned; closing that wrapper does not terminate an external runtime.

If cancellation prevents publication after process-tree attachment, startup
force-kills the attached tree and closes its handle before killing and reaping the
direct child. This prevents tracked descendants from surviving a parent-only
startup cleanup and lets kill-on-close act before a wait on inherited pipes.
Expected already-stopped outcomes preserve the cancellation error; material tree
termination or handle-release failures are retained alongside it. Startup failure
before attachment has no initialized tree owner, and attachment releases any
provisional handles before returning failure. Descendant reaping remains with its
process adopter where the SDK is not the direct parent. Arbitrary blocked
application stderr writers still require application cooperation.

## JSON-RPC framing, dispatch, and resource exhaustion

Inbound error-response admission requires non-null code and message members,
including recognized aliases and duplicate occurrences. Malformed records resolve
only the correlated pending request through the existing sanitized parse-error
path, discard peer error data and increment malformed-frame accounting. Valid
zero codes, empty messages and arbitrary data remain supported. Requiredness is
tracked during the existing borrowed-field pass before selected data is copied;
public error records retain ordinary application JSON behavior. This distinguishes
malformed envelopes from valid server errors without authenticating the peer or
changing the existing outer-envelope selection and code-width rules.

Collaboration response/settings admission rejects incomplete mode/settings/model
records and forbidden nested null occurrences before response, callback or cache
publication. Presence follows existing alias and duplicate-object decoding rules,
including allowed nullable resume resets. The private resume carrier applies
existing lifecycle validation first and releases borrowed input after admission.
Public collaboration records retain ordinary JSON behavior; these checks do not
change constructed request encoding or grant authority to reported settings.

Optional-member skills-changed and account-updated notifications still require
object roots before typed delivery. Null roots produce diagnostics under their
actual methods without invoking either replacement or additive callbacks. Valid
empty objects, open metadata and nullable account members remain accepted. The
skills registrations share a private admission handler, preserving the public
empty record's ordinary JSON composition and raw subscription behavior. This
validates payload shape without refreshing skills or authenticating an account.

Typed app label/icon maps, configuration feature-requirement and browser-origin
maps, and feature-enablement results reject null entries at their existing
admission owners before successful responses or typed callbacks. Duplicate
entries, later maps and nullable resets cannot hide a forbidden occurrence.
Whole-map nullability and native valid map merging remain owner-specific. Guards
run after established checks without retaining dynamic keys in diagnostics.
Fresh-value owners preserve prior receivers on new failures; standalone `Config`
decoding keeps its documented native partial updates, and callers must discard
that partial value after an error. Service admission publishes no partial result.
These shape checks do not interpret origin strings, enforce feature policy or
roll back a feature update performed by the server before its response.

Login-completion onboarding and MCP startup-failure metadata validate their
nullable enum values before typed callbacks. Configuration-requirements admission
also validates credential-store enums and login-method/sandbox-implementation
arrays, including invalid earlier occurrences and non-null elements. Native and
existing structured requirements checks retain priority, and failures do not
publish partial responses or overwrite prior notification receivers. SDK-owned
enum diagnostics bound rejected values before wrapping. These controls validate
reported metadata; they do not authenticate a login, select a credential store,
contact an MCP server or enforce managed policy locally.

Configuration-requirements admission validates the known nested shapes of six
raw object fields before response publication, including malformed earlier
duplicates. Existing typed-field errors keep their precedence. Nullable fields,
open extensions and arbitrary provider-map values remain accepted and retain
owned raw JSON. Nested schema properties use exact matching; differently cased
extras remain unconstrained. Diagnostics identify fixed schema fields and array
positions without retaining dynamic origin/provider keys or rejected values.
These checks do not authenticate policy origin, interpret origin strings, enforce
managed requirements locally or contact providers.

Known packaged-default and enterprise-managed config sources validate required
metadata before read or write results are published. Packaged file paths must
already be absolute and normalized, including earlier canonical occurrences;
validation does not resolve, open or canonicalize them. Enterprise identifiers and
display names may be empty, but not null. Unknown future sources remain opaque.
Decoded source fields use exact schema-name matching, so differently cased extras
cannot replace the metadata being validated.
These checks do not authenticate policy origin or undo a configuration write whose
returned override metadata fails validation.

The known hook-prompt item validates required fragment strings and array entries
at the item union before response, notification or cache publication. Fragment
presence follows native duplicate-array storage reuse, while forbidden null
occurrences remain visible. Empty arrays reset that presence history. Known
lifecycle fields retain native numeric validation, and snapshots and callbacks
own independent fragment and timestamp references. Future unknown item types
remain opaque; this shape validation does not establish the trustworthiness of
text supplied by a hook.

The known Bedrock account branch validates its optional managed-credentials flag
as a non-null boolean before account-read publication, including recognized
duplicate and escaped names. Omission means false. Other account branches keep
that extra field unconstrained, and unknown future account types retain raw JSON.
This validates reported metadata; it does not authenticate the server's claim,
grant credential access or change the credential exchange flow.

Required non-null fields in thread, plugin, sharing and initialization responses
cannot hide a canonical null behind a later valid duplicate. Nested granular and
thread-spawn checks inspect repeated objects without requiring each earlier
partial object to contain the final required set. Existing decoding and semantic
checks keep their precedence; new rejection paths preserve prior receiver
references and prevent response, cache and typed callback publication. The private
initialization carrier checks the response before latching a session and releases
its borrowed input after validation. After existing required-value and non-null
checks, it rejects a selected `codexHome` that is not an absolute, normalized
path. Rejected responses publish no handshake metadata or cached session and
permit a valid retry. This lexical check neither accesses the filesystem nor
resolves symlinks. These checks validate message shape, not the truth of metadata
supplied by the peer.

Marketplace upgrade admission requires both declared strings on each visible
error record and rejects null elements or required-member occurrences, including
ones hidden by later duplicates. Presence follows native array storage reuse;
empty strings and open extras remain valid. Existing path and string-array
failures keep priority, failed admission preserves the prior response, and the
public method returns no successful typed result. These are peer-reported
operation results; validation does not establish whether remote files changed.

Plugin-share target updates accept the declared `LISTED`, `UNLISTED` and
`PRIVATE` visibility values. Remote plugin and principal IDs, principal types
and target roles remain validated before dispatch; unknown visibility values
are rejected. This validates the requested representation, while the server
owns sharing authorization and the effect of changing discoverability.

Realtime startup admission accepts the declared `v1`, `v2` and `v3` versions
before either typed listener API receives a notification. Required-field, null
and unknown-version failures prevent callbacks and retain diagnostic origin.
The version and optional session ID are peer reports of startup acceptance;
they do not independently prove an authenticated or usable provider connection.

Async user-input questions reject undeclared properties before typed response,
notification or cache publication, including invalid questions hidden by later
duplicate arrays. Their existing required-title and option checks retain priority,
and new failures preserve prior receiver references. The check is limited to the
closed question object; surrounding messages and notification metadata remain
open. Rejected property names are not retained in diagnostics. Schema-valid
questions remain untrusted prompts and do not confer application authority.

Known source and approval object alternatives reject schema-forbidden outer
properties and competing discriminators before response, cache or typed callback
publication. Validation follows their established required/native checks and
leaves prior receivers intact on new failures. It checks borrowed property names
without decoding unknown values or retaining peer names in diagnostics. Open
nested granular/thread-spawn metadata and future root source objects retain
their existing acceptance; the latter preserve owned raw JSON. These shape
checks do not authenticate source identity or grant application authority.

Numeric request IDs retain signed 64-bit integer identity across serialization,
equality and pending-response correlation. Caller-provided floating values must
be finite, integral and in [-2^63, 2^63); the upper boundary is rejected before
conversion or pending-request admission. Inbound JSON integers are range checked
without floating conversion. String IDs remain a distinct correlation family.
These checks preserve correlation identity; an ID is not authentication.

A peer may send malformed envelopes, oversized frames, unexpected IDs, or notification floods. The built-in [stdio transport](../appserver/transport/stdio.go) limits inbound frames to 10 MiB, uses fixed worker pools and bounded queues, and gives outbound operations a default timeout. [Frame parsing](../appserver/transport/stdio_framing.go) distinguishes numeric and string request IDs, checks protocol versions and response shape, and classifies malformed input. Queue-overflow handling and notification classes are explicit in [notification dispatch](../appserver/transport/stdio_notifications.go). All 85 methods in the checked-in notification schema have explicit delivery classes. Thread events, including streaming deltas and completion, share an ordered queue per thread. Process handles, command process IDs, search sessions, import IDs, project IDs, filesystem watch IDs, and MCP subscription IDs have separate identity scopes; account updates share one account scope, and global methods share a queue per method. Exact schema properties determine the owner, including a present empty string when the schema permits it; optional missing or null thread identities use global method scope. Eight workers yield between callbacks, with at most 128 active scopes and 1,280 queued events per scope, and 34,048 queued ordered events in total. Overflow fails the connection explicitly rather than silently discarding an update. Unattributable events retain their class's fallback policy; unknown methods remain best-effort.

All notification delivery classes share a 64 MiB retained-data budget, charged before routing. It covers payload backing capacity, method/version strings and owner keys while events are queued, awaiting handler registration, transferring between buffers, or executing callbacks. A reservation stays with an event through replay and panic recovery until delivery finishes or the event is discarded. Classified delivery fails the connection on byte exhaustion; best-effort delivery may drop an event. Count limits separately bound queue bookkeeping. Accepted ordered events retain their queue ownership and reservation when the reader terminates, including EOF; registering a handler later can drain them. Explicit transport close stops queued notification dispatch and releases buffered payloads. A callback already admitted for execution may finish and keeps its reservation until it returns. Notification and request workers recover application handler panics and suppress secondary panics from the configured panic-reporting callback. Deferred ordered cleanup retires callback ownership before releasing its reservation, allowing subsequent events to proceed after recovery.

A peer can place invalid requests before a valid response while the external
writer is held. Reader-owned rejection replies use bounded admission to the
existing 256-envelope serialized write queue, allowing response correlation to
continue without waiting for each reply's physical write. A separate 64 MiB budget
charges marshaled rejection backing capacity through pending admission, queueing
and active writing, including peer-controlled IDs. Exhaustion terminates the
connection. Writer errors remain terminal, and a queued reply is not reported as
written. Terminal cleanup discards unclaimed replies, including at EOF; a claimed
external write retains its reservation until it returns. Saturated queues may
still apply admission backpressure, and this control cannot force a noncooperative
external writer to return. Ordinary application-request replies and notifications
retain their actual-write completion contracts.

Transport identity and envelope selection scan raw JSON without materializing ignored properties or decoding overwritten identities. Selected fields are decoded once, including nested thread IDs and RPC error fields; malformed and oversized recovery inspect bounded prefixes while preserving usable earlier correlation. The shared raw scanner also serves runtime lifecycle fallback attribution and fixed-property response-shape and union-discriminator selection. For syntactically valid objects, protocol object validation skips ignored properties and checks every recognized occurrence in wire order. Plain string fields retain only the final usable spelling for decoding, while required/non-null/type failures remain visible even when followed by a valid duplicate; custom semantic fields still decode each occurrence. Direct calls with syntactically invalid JSON preserve the earlier prefix/error behavior through a compatibility decoder, which built-in wire unmarshaling does not enter. Full JSON validation and recovery impose a nesting limit of 10,000 containers. Scalar request-ID decoding rejects composite values before allocating their graphs and rejects oversized numeric spellings before diagnostic formatting. These controls keep discarded fields and overwritten selector identities out of decoded object graphs; they do not bound arbitrary semantic payload decoding. Allocation measurements for recovery use standard builds, since race instrumentation deliberately changes allocation and pool behavior.

Lifecycle attribution preserves the identity contract of each carrier. Top-level notification identity properties use exact names; nested Thread and Turn ID selection retains the existing typed decoder's folded spelling, duplicate and null behavior. Malformed turn completion may recover an ID that the complete schema validator rejects, but produces a failed completion only for the matching active turn. An unrelated semantic failure must not change which turn receives completion. Discriminator selection preserves standard type-error field and object-offset diagnostics. Direct malformed request-ID decoding preserves the prior syntax/EOF and trailing-input errors through a decoder that discards values without constructing their graphs; its temporary input buffer remains proportional to input size.

These limits do not constitute a total transport or process heap budget. Frame parsing still copies raw data and allocates transient storage proportional to frame size and bounded nesting. Typed payload decoding, request buffers, outbound writes, client caches, long conversations, and data retained by application callbacks have separate ownership and limits. Slow or noncooperative callbacks can consume workers and notification capacity, and a peer can make the connection unusable within the stated bounds. Applications must close transports, handle transport failure, and apply their own admission, concurrency, output-retention, and retry policy. Custom transports do not automatically inherit these limits.

Typed decoders validate required fields, selected non-null fields, enums, unions, and request-specific constraints. [Object validation](../appserver/protocol/json_validation.go) and [schema coverage tests](../appserver/protocol/spec_coverage_test.go) help preserve the wire contract. Session-source, nested sub-agent-source, and approval-policy unions reject null before trying string decoding, including null policy-list elements. A later duplicate sub-agent field cannot erase an earlier null violation. Nullable outer policy pointers and policy lists remain supported. The known custom session-source branch requires a non-null string before typed response or notification admission; unknown source objects remain available for forward compatibility. Invalid source responses fail before thread-cache publication or listener delivery; these shape checks do not authenticate provenance or enforce application approval authority. Validation is not universal semantic authorization or a guarantee of canonical JSON: some semantic leaf types use ordinary `encoding/json`, which can allocate when unquoting escaped field names or decoding known duplicate values, and duplicate-member rejection is not a general SDK control. Selected fallback item/turn payloads, unknown variants, open JSON metadata, application configuration maps, and attachments intentionally retain their semantic data. Consumers must validate any additional security meaning before acting on it.

Shared account metadata checks every recognized plan, authentication-mode and
rate-limit-reason enum occurrence before typed response or callback publication.
The owning ChatGPT account, rate-limit snapshot and account-update records share
these checks across account reads, rate-limit buckets, and On/Add notifications.
Required plans remain non-null; optional enum pointers preserve absent/null
states. A later duplicate or map reset cannot hide an earlier invalid value.
Existing type/final-enum error precedence and receiver atomicity remain intact.
Unknown account union branches retain raw metadata, and ordinary public scalar
decoding remains available outside these owners. These metadata checks do not
establish authentication, billing authority or permission to issue requests.

The rate-limit response owner validates known reset-credit summary and detail
metadata after existing bucket checks and before successful publication. It
requires declared non-null fields, kinds and enum values across recognized
occurrences, retaining signed integer ranges, nullable details and open extras.
Inner schema names match exactly; differently cased extras remain opaque.
The complete selected raw value is retained, including null/empty distinctions
and numeric spelling. Rejection preserves the prior response and returns no
public result. These checks do not consume a credit, authenticate its issuer,
enforce a budget or prove authorization to redeem it.

Managed OpenTelemetry requirements expose exporter choices, logging limits and
server-reported TLS strings as metadata. Admission checks exporter union shapes,
required members, protocol enums and unsigned byte limits before publishing a
successful requirements response. The SDK does not configure exporters, connect
to these endpoints or read certificate/key paths. Feedback saving sends a
caller-selected server path to the app server; the server writes the ZIP and the
caller owns its cleanup even if the RPC disconnects. The SDK does not resolve
these opaque paths or gain local filesystem authority from them.

Plugin summary admission checks every recognized authentication-policy,
installation-policy and availability enum occurrence across list, installed,
read and share-list responses. Later duplicates, folded field aliases and outer
array replacements cannot hide invalid enum values. These checks preserve
existing final-value errors and partial receiver updates; newly rejected hidden
enum values restore the prior summary. Omitted availability defaults to
`AVAILABLE`, and recognized null occurrences cannot produce successful
admission. Failures from the new null check restore the prior summary;
higher-priority existing errors retain their established partial updates.
Its public representation is a non-null enum value. These checks validate
metadata and do not authorize plugin installation or execution.

Reset-credit consumption, plugin-install, thread-unsubscribe, login-cancel,
add-credits-email and configuration-write response records also check every
recognized enum occurrence before returning typed results. Later duplicates
cannot hide an invalid earlier enum value.
The checks preserve existing required/type/path/final-enum error precedence and
receiver atomicity. Reset-credit records recognize exact decoded keys; the other
five records retain their ordinary case-folded JSON field matching. These are
response-shape controls, not proof of a completed server-side action or authority
to perform one; malformed responses do not roll back remote side effects.

The process and command output notification decoders reject an invalid recognized
`stream` occurrence even when a later duplicate supplies `stdout` or `stderr`.
Checks run before typed callback delivery and preserve existing object, stream
and base64 error precedence. They match exact decoded field names, including
escaped spellings; unrelated nested fields and unknown case variants remain
ignored by these record decoders. This prevents duplicate-field repair from
hiding a malformed stream without treating stream labels as execution authority.

[Thread-list filters](../appserver/protocol/thread_service.go) preserve an empty
provider list as a request for all providers. The additive `ListJSON` parameters
also preserve cwd string/array choices and explicit null selecting unsectioned
threads. The shared request path validates raw cwd shapes before transport and
uses the same response admission and cache publication as the legacy method.
Selected collection values backed by nil slices remain empty arrays during
request preparation. Explicit null for the nonnullable useStateDbOnly filter
fails before transport rather than becoming omission.
Service tiers and thread analytics sources follow their open string schemas,
including custom and empty strings. Request preparation, lifecycle admission,
typed notifications and thread caches preserve those peer-controlled values.
Optional carriers retain nullability, standalone codecs reject non-string
values, and the separate thread-source-kind filter remains a closed enum.
These metadata strings do not establish origin, authorization or service-tier
availability; their interpretation belongs to the server and application.
Request overrides preserve explicit null independently of omission for service
tiers, goal token budgets and each Git metadata update field. Omission remains
the default, so an unset Go field does not accidentally request clearing stored
state. Constructed values, decoded requests and Turn.Start's custom wire record
retain the three states, including zero budgets and null-reset histories.
These controls preserve update intent; the server owns authorization and the
actual state change. Goal budgets remain signed 64-bit protocol integers rather
than local duration or spending controls.
Plain string paths in thread, turn, command, config, permission-profile, skills,
hooks, fuzzy-search and migration requests pass unchanged to the server, which
owns relative-path resolution and operation-specific restrictions. Skill grouping
and migration-item response cwd strings, skill error paths and Thread.path retain
the peer's value. A running resume keeps the server's active-configuration
consistency check, and config writes still require the server's allowed user-config
target. Genuine `AbsolutePathBuf`
fields, including Windows sandbox setup's cwd, keep their SDK absolute-path
validation. These controls prevent request representation loss; they do not
make a listing filter an authorization boundary. An application that relies on
section, provider or path filters to enforce user access still needs an
independent authorization policy and a trusted peer.

Typed service parameters retain
ordinary Go JSON decoding semantics when an application decodes into them. The
SDK cannot recover a null array element after that decoder converts it to an
empty string. Validating externally supplied raw request JSON belongs at the
application's decoder boundary; typed preparation does not supply that control.

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

[Rate-limit response admission](../appserver/protocol/account.go) permits a null
outer multi-bucket map but requires each supplied bucket to be an object. A
malformed peer could otherwise expose a nil bucket to an application expecting
a usable snapshot. The response owner rejects null bucket occurrences after its
existing required-field, native type and enum checks, before publishing a
successful value; later duplicates or map resets do not hide that failure.
Empty object buckets remain valid, and a present empty map retains its object
shape when encoded. The legacy public map represents both omitted and null
roots as nil and omits them during encoding; this is not exact null/absence
preservation. The guard does not authenticate the account, enforce usage policy,
or validate every programmatically constructed response. Newly rejected values
participate in the default decoder's first-error selection; enclosing fields may
still be visited, so callers must discard failed decodes.

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
strict field decoder's diagnostics. The default decoder may continue through
later fields in a containing application wrapper after a record fails. [Scratch-state reuse](../appserver/protocol/synced_object_decode.go)
retains only schema/type metadata between decodes and clears payload references
and admission flags before reuse; it does not bound the number of valid records
in an admitted payload. Generic raw notification handlers remain responsible
for their own decoding and validation.

Fuzzy-search result admission rejects null `indices` elements before a search
response or either typed session-update listener publishes the result. The
shared result owner checks every recognized occurrence, including folded and
escaped aliases, so later duplicate arrays or null resets cannot hide a rejected
element. Existing numeric type/range checks retain priority. Omitted, null and
empty arrays and valid uint32 values remain accepted; string and path semantics
are unchanged. Rejection preserves the prior record and containing response or
notification. Generic raw handlers retain responsibility for their own validation.

[String and optional array admission](../appserver/protocol/optional_array_validation.go)
rejects forbidden null arrays and null string elements in plugin, app, model,
thread lifecycle, reasoning, text-input and workspace-sandbox carriers, as well
as loaded thread IDs, hook warnings, plugin reconciliation IDs, citation thread
IDs, collaboration receiver IDs, Windows warning paths and filesystem changes.
Nullable plugin default prompts, app categories, question options and web-search
queries accept whole-array null but reject null string elements. Model
verification arrays admit only their schema enum values. Guards follow each
existing owner's exact or folded property matching, including escaped names;
invalid earlier occurrences remain failures after valid duplicates or resets.
Absolute path arrays retain their final-value path validation and also reject
overwritten null elements; opaque strings do not gain path restrictions.
Marketplace error records require their canonical schema
members, reject nulls through recognized aliases, and validate the same final
absolute path that the wire decoder publishes. Validation uses existing response,
Config and union owners after their established decoding and semantic checks.
The formerly plain concrete representations retain ordinary Go JSON decoding,
receiver merging, error context and named or anonymous envelope traversal;
decoding those representations directly does not perform SDK carrier admission.
Fresh response owners retain replacement semantics. Config retains its existing
merge and partial-update behavior; callers must not consume a failed decode.
Plugin interface and detail path failures retain their existing partial receiver updates;
new string-array admission failures preserve the prior receiver.
New admission errors propagate through existing custom decoders and can stop
outer traversal or precede later service validation. This does not establish
error parity for payloads that newly fail the array contract.
These admission checks neither authorize the reported data nor bound the number
of valid elements or callback-owned copies.

[Memory-citation admission](../appserver/protocol/memory_citation_admission.go)
requires the citation's thread IDs and entries, plus every visible entry's
schema-required fields, before a typed agent message is published. The optional
whole citation may be omitted or reset with null. Null entries arrays, entry
elements and required scalar occurrences remain invalid even when a later
duplicate would otherwise reset or repair the native Go value. Ordinary object
merges, folded and escaped field aliases, and same-index array-entry reuse
retain the wire decoder's semantics; private per-call presence masks track
those merges in proportion to historical entry indices and validate required
fields in the final visible record after split-field duplicates. Errors identify
fixed field names and numeric indices without quoting peer values. The exported
citation records remain plain Go representations: direct caller
decoding, caller-owned raw handlers, and outbound construction do not acquire
this carrier admission rule. This validates record shape, not citation truth or
a total payload/heap budget.

[Hook list and run admission](../appserver/protocol/hook_error_admission.go)
requires each visible `hooks/list` error record to contain non-null `message`
and `path` strings. Private presence masks follow ordinary same-index struct
reuse across duplicate arrays, preserve hidden slots across shrink and reextend,
reset when an empty array replaces the slice, and check required fields only on
the final visible records. Recognized null items or scalar occurrences remain
invalid after later repair or reset; existing root, native type and warning
errors keep their precedence. Hook metadata requires a non-null valid `source`;
an omitted command-handler `async` decodes to an owned `false`, while other
handler variants leave it unset. Explicit command booleans survive decoding.
Run summaries materialize omitted optional `source` as an owned `unknown`, and
both carriers reject explicit null or unrecognized source values while validating
the 11 schema enum values. Constructed nil `source` and `async` pointers remain
omitted by encoding. Hook metadata `sourcePath` remains a normalized absolute
path; run-summary `sourcePath` and `HookErrorInfo.path` are opaque server path
strings. Run summaries preserve relative, empty and non-normalized spellings. `HookSource` and
`HookErrorInfo` stay plain Go representations, so direct decoding and anonymous
envelopes retain standard JSON behavior. These checks admit record shape; they
do not authenticate hook provenance, authorize or execute hooks, or set a total
payload or heap limit.

[Plugin.Read hook-summary admission](../appserver/protocol/plugin_hook_admission.go)
requires each visible plugin hook summary to include a valid `eventName` and a
string `key`; empty keys and empty hook arrays remain valid. After existing
native decode, required-field, marketplace-path and `mcpServers` checks, a pass
over borrowed JSON spans tracks required-field presence across duplicate-array
slot reuse, shrink/reextend and empty-array reset, and checks only the final
visible records. Recognized null `hooks` arrays, items and keys remain invalid
after later duplicates; event-name type, null and enum errors remain owned by
the native decoder. `PluginHookSummary` remains plain and embeddable, and this
carrier check does not authenticate hook provenance or execute hooks; existing
path failures retain their established partial receiver update behavior.

[Plugin source decoding](../appserver/protocol/plugin.go) admits the checked-in
`npm` source branch and retains its required package string plus optional
registry and version states (absent, null or string). Branch-specific native
decoding for npm ignores legacy fields that its schema leaves unrestricted,
while the git branch and invalid-selector fallback retain their existing
decoding and validation path. The local and remote source branches also leave
their legacy extra string fields unrestricted. For those selected branches, a
pass through a private native string decoder skips only direct wrong-type
errors for these extras, keeps the last successfully decoded string, and lets
null clear it. Required local path, the selected type, and all git-declared
fields retain their native validation; the git branch keeps its established
wire path. Successful decoding replaces the whole source, while failures
preserve a seeded receiver and its references. The public source representation
uses ordinary JSON encoding and adds no outbound validation. This validates
response shape only; it does not verify packages, authenticate a registry,
install or execute code, or authorize a plugin.

[MCP resource-read admission](../appserver/protocol/mcp_resource_read_admission.go)
requires every visible resource item to have a non-null string URI and at least
one string `text` or `blob` branch. A pass over borrowed JSON spans uses small
presence masks to follow native duplicate-array reuse, shrink/reextend and
empty-array reset behavior. Nullable branch members clear their state, while
recognized null contents, null items and null URIs remain invalid after later
duplicates. The normal wire decode keeps ownership of common field types and
native errors. On a failed decode, a cold recovery accepts only schema-permitted
incompatible opposite-branch extras when the counterpart is usable by the end
of the same source object; it projects those values to null in a temporary copy
at their original member positions and retries the plain wire decode. If the
recovery proof fails, the original native error, including its type and field
context, is returned. `_meta` remains opaque raw JSON with its original numeric
precision. `ResourceContent` stays a plain Go representation, so direct
decoding, serialization and caller envelopes keep ordinary `encoding/json`
behavior. This validates the typed response carrier only; it does not impose
URI, MIME, base64 or metadata policy, or bound valid response size and
application-owned copies.

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
boundaries, native JSON conversion errors retain at most 2,048 bytes in each of
`json.UnmarshalTypeError.Value` and `Field`. Oversized number/string descriptions
and dynamic field paths keep a quoted 256-byte source preview and omitted-byte
count before SDK wrapping or runtime fallback copies. Normalization copies
metadata without retaining the discarded full error; it preserves target type,
offset, root struct and native causes. Short diagnostics and standard decoding
behavior remain unchanged. Parsing may allocate temporary full-token copies;
accepted raw or `json.Number` values remain complete.

Unrestricted application decoding and callback errors retain their ownership
and identity. `OptionalNullable[T]` only normalizes native diagnostics when the
entire static destination graph lacks application JSON, text or streaming JSON
decoders and interface slots. The standard library's `json.Number` codec is
explicitly recognized as native. Only the exact `json:"-"` tag excludes a field
from this ownership check; dash-named fields with tag options remain active.
Extension-bearing graphs pass errors through, including native failures in
another field. Application-selected envelope labels and structured RPC data
are outside these bounds.

Malformed `json.Number` serialization has a separate control at SDK-owned
protocol JSON serialization calls, including request preparation, results,
inner codecs and wire-only representations. The default backend reports this
through its built-in Number streaming marshaler. Only a direct
`json.MarshalerError` owned by that standard-library type is normalized;
application codec wrappers retain their original chains. Native messages over
2,048 bytes keep a quoted preview of at most 256 bytes of the formatted literal
and an omitted-byte count. The replacement preserves wrapper metadata and the
native syntax sentinel without retaining the full formatter error. Inner SDK
marshalers apply the policy before enclosing wrappers are added. Valid Numbers,
including empty constructed Numbers encoding as zero, are unchanged.
Union interface embedding may supply application fields inside accepted concrete
variants; the same inner serialization policy applies before SDK publication.

Native malformed `,string` fields use the typed conversion-error policy,
including boolean and Number string values. Short syntax diagnostics remain
short and do not acquire a truncation marker. Standard admission, first-error
precedence, partial updates, duplicate handling and null behavior follow the
default backend. Current typed SDK schema and provider destinations do not use
`,string`; the demonstrated route is a caller-selected generic destination.

Application JSON, text and streaming marshaler error chains pass through unchanged. Generic
decoding with interface slots or application codecs also passes errors through,
including reused interfaces containing a Number, because error text alone cannot
distinguish a native decoder failure from an application-owned error. Plain
caller-owned `encoding/json` operations, application handler errors, structured
RPC data, application-defined type/envelope labels and unrelated parser or
unknown-field diagnostics remain outside these preview controls. Standard
parsing, unquoting and native formatting still allocate temporary full input or
quoted-error copies; this bounds retained native literal diagnostics and
subsequent wrapping, not total operation allocation.

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

Public raw client notification callbacks receive independent `Params` bytes on
each invocation. A callback may retain or modify them without changing the
transport input, later raw or typed callbacks, or SDK runtime/cache decoding.
Private protocol listeners borrow the original input for read-only decoding. Copies
add allocation proportional to payload size and raw callback count; application
retention remains outside the transport's retained-byte budget and does not
establish a total client heap limit.

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

Conversation startup captures admitted startup identities and closure evidence
before sending the request, then couples response publication, listener
registration and cache-incarnation transfer before callbacks or eviction. Each
pending start retains at most 64 candidate references. A client-wide admission
budget limits temporary pins to 64 distinct shared cache entries, so staged
requests cannot accumulate disjoint pinned snapshots without limit. Existing
application listener pins are separate. Complete Thread graphs and the number
of application-controlled pending requests remain unbounded in bytes. Every
exit releases temporary candidates and pins without releasing other owners'
references.
Failed requests create no new subscription or publication. Capacity loss or
ambiguous same-ID startup history fails conservatively with
`ErrThreadStartObservationLost`, returning a valid owned initial response to
direct callers without a listener. This error does not undo a successful remote
start and must not trigger an automatic retry. Cache pressure cannot evict a
captured thread before its Conversation pins observation. If closure coalesces
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

Attestation request admission requires an object root before invoking the
application's token-generation callback. Null parameters retain invalid-params
classification, and the stdio transport returns a sanitized error under the
request ID without a token result. Empty/open objects remain valid; this is shape
validation rather than proof of the requesting peer's authority. The application
still owns attestation generation and authorization. A private carrier preserves
the public empty record's ordinary JSON behavior and reuses existing handler-error
and non-empty-token response validation.

Current and legacy command-approval admission rejects null string elements in
proposed execpolicy amendments and command argument arrays before application
callbacks. A valid later duplicate cannot hide a malformed occurrence. Existing
owner checks keep their precedence; the optional current amendment retains its
nullable-array contract, and empty arrays and empty arguments remain valid.
These checks prevent malformed evidence reaching handlers without interpreting
paths or making an authorization decision on the application's behalf.

The retained `ApprovalPolicyOnFailure` constant cannot be encoded as a typed
approval policy because the checked-in schemas removed that known value. The
shared literal encoder and wrapper reject it before request dispatch, including
pointer forms. This prevents accidental obsolete requests rather than enforcing
an application authorization policy: current policies, deliberately compatible
future/empty strings and explicit raw unknown variants retain their behavior.
Inbound string decoding remains permissive, including the legacy value; encoding
that decoded known literal rejects it at the same outbound boundary.

User-input approval admission distinguishes omitted question flags from explicit
nulls. Exact `isSecret` and `isOther` occurrences must be non-null booleans;
omission retains their false defaults. A malformed question rejects the whole
request before the application handler or a successful reply is published.
Existing required, native and question-identity errors retain precedence, and
rejected values do not replace receiver references. Applications own prompt
rendering and answer handling; these flags do not establish authorization to
collect input.

The deprecated user-input `autoResolutionMs` field accepts nullable unsigned
64-bit integers. Negative or overflowing recognized occurrences reject the
request before callback dispatch, even if a later duplicate is valid or null.
Absent/null values and present zero remain distinct in the typed representation,
and large values retain integer precision. This validates reported metadata;
the SDK does not run a timeout or convert this value into a duration.

MCP elicitation request admission validates every recognized `mode` occurrence
against `form`, `url`, `openaiForm` and `openai/form` before invoking the
application handler. Later valid duplicates cannot hide an unsupported mode.
Field matching remains exact, including escaped spellings; supported mode
transitions retain the final branch selection. Existing identity, variant and
form-schema errors keep their precedence, and rejected parameters do not replace
a seeded receiver. These checks validate request shape; they do not authorize
input collection, choose a trusted URL or fetch external resources.

Standard `form` requests also validate the typed `requestedSchema` before
callback publication: its root must be an object schema, and property definitions
must match a supported string, numeric, boolean or select alternative. Closed
schema objects reject unknown members; required members, nullable metadata,
array elements, string-format literals and nonnegative integer limits follow the
checked-in definitions. String lengths are bounded by uint32 and select-array
item counts by uint64. Their admitted property-map values retain exact numeric
tokens instead of rounded float64 values; numeric-property double fields keep
their existing representation. Limits are checked from decimal length and digits
without expanding exponent-sized integers. Property definitions and their nested options retain final-member map decoding;
repeated root property dictionaries retain their merging behavior. Existing
native and required-field errors take precedence over these added checks, and
failed validation leaves the receiver unchanged. Both OpenAI form modes retain
arbitrary JSON schemas. This checks the schema's representation, not submitted
answers, cross-field consistency or whether collecting the input is authorized.

The selected form branches leave `url` and `elicitationId` unrestricted. Their
existing Go string fields retain string values and null resets; other JSON kinds
are ignored without building their object graphs. The URL branch keeps its
required, non-null string checks. Branch selection reads the final exact mode
from borrowed JSON and leaves invalid-selector errors with the original decoder.
The shared string-extra decoder also preserves plugin-source behavior for its
unrestricted fields. These rules do not add URL trust or navigation policy.

A repository file, retrieved resource, MCP tool response, attachment, or model message may try to persuade an application to authorize commands, disclose secrets, or render executable content. The SDK exposes these values and protocol operations; it does not classify their trustworthiness or implement prompt-injection prevention. Treat text and structured metadata as data, including image identifiers, MCP UI resource URIs, prediction text, and plugin metadata.

[Approval handlers](../appserver/protocol/approval_handlers.go) are application-owned callbacks. Without a matching handler, the client returns a JSON-RPC method-not-found error rather than granting approval. A permissive handler or permissive Codex approval policy can deliberately remove that protection. The application must bind decisions to its actual user, account, workspace, operation, and permitted consequences. Dynamic tool callbacks and token-refresh callbacks are privileged code and must not derive authority solely from model output or a claimed thread ID.

Filesystem approval scope retains structured read, write and deny entries, path/glob/special-location variants, scan depth and legacy path strings. Explicit empty entries remain distinct from absent entries, because the runtime gives present entries precedence over legacy lists. Typed approval requests and guardian permission-review metadata validate the known scope before callback dispatch; invalid programmatic grants fail before a successful response is produced. Normalization clones mutable filesystem scope without resolving paths or expanding patterns. Guardian review record decoders validate the declared user-authorization and decision-source enums before typed delivery; these descriptive values do not grant application authority. Guardian actions retain their generic map shape, with the known `globScanMaxDepth` represented as `json.Number` to preserve integers beyond float64 precision. These controls preserve the requested and granted scope; Codex and the application remain responsible for interpreting paths, authorizing decisions and enforcing access.

Approval cwd, command-action and legacy parsed-command paths, grantRoot, and
file-change path strings remain opaque request evidence. The SDK does not resolve
these values against cwd or substitute normalized paths before callbacks. Actual
AbsolutePathBuf fields retain their separate typed validation. An application
must distinguish the schema contracts when deriving display paths or applying
policy; preserved strings do not authenticate a target or grant filesystem access.

Dynamic-tool callback admission and MCP/dynamic-tool item admission preserve
arbitrary argument numbers as `json.Number` before callbacks or typed history
publication, including nested numbers and values outside float64 range. This
prevents silent numeric rounding at those three owners. Nullable arguments and
existing field admission remain unchanged; numeric conversion and interpreting
tool arguments remain the application's responsibility. The policy does not
change direct ordinary decoding of plain item records, unrelated metadata or
application-selected codecs. Exact numeric tokens are data, not tool authority.

The shared MCP result owner preserves numeric tokens in arbitrary content
array elements and structured content before history and typed item-notification
publication. It keeps the required non-null content array, nullable elements,
existing error/receiver behavior and raw `_meta` ownership. Cached snapshots own
the complete nested graph. This prevents numeric rounding at result admission;
it does not validate tool claims or assign authority to returned data. Other
metadata and application-selected decoder policies remain separate.

Image input and function-call image output codecs evaluate their URL/file-ID
alternatives independently, matching the open schema alternatives. Invalid
inactive properties are discarded; a valid shared image detail is still required
when supplied. Missing or invalid alternatives reject the record without
publishing a partial receiver. Empty strings retain explicit presence, and the
shared copy boundary owns URL pointers in snapshots. Admission establishes wire
shape only: URLs and file IDs remain opaque peer data, and the SDK does not fetch
or verify their contents.

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

App metadata admission distinguishes omitted defaulted flags from explicit
nulls. Recognized `isEnabled` and `isAccessible` fields, including escaped,
folded and Unicode simple-fold aliases accepted by the native decoder, must be
non-null booleans; omission retains their true/false defaults. Invalid app metadata rejects the
whole public listing and is reported before either typed app-list notification
API invokes its callback. Existing metadata errors retain precedence, and
failed decoding preserves the prior receiver. These peer-provided flags do not
establish local authorization or whether a connector is safe to invoke.

The native admission owners also reject null defaulted flags in model
`supportsPersonality`, workspace-write configuration, granular approval
`request_permissions`/`skill_approval`, and `workspaceWrite` and `readOnly`
sandbox policies. Read-only network access retains explicit true/false values
through response, notification and request carriers; omission retains the
schema's false default. Its supported legacy access metadata stays optional.
Checks follow native field matching and preserve earlier required, native,
metadata and path errors. Invalid policy metadata cannot enter successful
thread-start responses, typed settings callbacks or the thread cache. Plain
workspace configuration records keep their normal JSON method sets; the
containing `Config` owns validation. Direct `Config` decoding retains native
partial updates on errors, while a failed config-read returns no public value.
These checks validate the supplied representation, not whether a policy should
be granted or how the server enforces it.

Structured web-search tool settings retain optional domain lists, a declared
context-size enum and location metadata before config-read publication. The
existing `Config` admission owner checks closed tool/location objects and
nullable arrays with non-null string elements, including overwritten invalid
occurrences. Its earlier sandbox and browser-origin checks keep priority, and
direct Config decoding retains native partial updates and references on errors.
Tool and location records remain method-free for ordinary application JSON.
Root web-search mode and the legacy view-image flag remain separate. These are
peer-supplied settings; decoding neither performs searches nor validates domain
ownership, location accuracy or permission to use a provider.

Permission profile listings require an explicit non-null `allowed` flag for
every summary before a successful response is returned. Missing or null flags
cannot silently become false, and one invalid entry rejects the whole public
response. This validates peer-provided catalog metadata; the flag does not
authorize profile selection or replace server-side requirements enforcement.

Skill catalog and bundled-plugin skill paths follow the schema's opaque legacy
string contract. They may be relative or non-normalized and are preserved as
returned; genuine `AbsolutePathBuf` fields retain their separate validation.
The skill metadata and bundled-skill admission owners reject non-null local
`iconSmall` and `iconLarge` values that are not absolute, normalized paths,
including icons on a plugin's onboarding skill. Failure preserves each skill
receiver and prevents public response success. Nullable icons and opaque URL
siblings remain supported. These lexical checks neither read icon files nor
fetch URLs; the plain `SkillInterface` record keeps ordinary JSON behavior.
A malicious peer could supply a path that looks safe in a catalog but resolves
elsewhere in the consuming application. Applications must bind any filesystem
access to their own trusted base and policy rather than treating a catalog path
as an approved location.

Remote-control status notifications validate the declared connection-state enum
before typed delivery. A `connected` status and its remote identity fields are
peer-provided metadata, not proof of an authenticated remote connection.

Thread-item admission validates agent delivery, command source, and sub-agent
activity-kind enums before typed notification delivery or thread-cache
publication. Omitted command sources default to `agent`, as specified for older
records; this default does not authenticate who initiated a command or authorize
execution.

Browser extension request-header requirements, independent speed-mode support,
turn lineage, sub-agent model metadata, attachment-owner lookup results, and
misalignment review targets are also protocol data. The SDK does not apply these
headers to browser requests, enforce the described browser policy, authenticate
attachment ownership, or grant continuation authority from a review target.
Consumers must
avoid logging sensitive header values and must authorize any resulting action
at their own boundary. A partial-answer phase is display metadata; turn
completion remains a separate lifecycle event.

`Resource.icons` and `Tool.icons` catalog values are admitted only when absent,
null, or arrays; the schema leaves array elements unrestricted. The check runs
after each owner's existing required-field and native type decoding, before a
successful value is assigned to its receiver. A rejected icon root returns from
the custom item decoder, so it can replace an earlier outer sibling error or
prevent a later sibling from being decoded. This is limited to catalog response
shape; it does not validate icon contents or establish that a peer is trusted.

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

An accepted turn-start response does not complete a runtime turn. Blocking and
streaming runtime helpers observe the client's notification-delivery boundary.
When delivery ends without a matching completion, they return a transport error;
an accepted matching completion retains priority. Stdio EOF drains accepted
notifications before signaling that boundary, while explicit close discards
unclaimed notifications and lets claimed callbacks retire. Custom transports
without `TransportLifecycle` expose only client-close notification entry and
retirement; autonomous termination and their internal queues remain outside the
SDK's observation. No synthetic remote thread closure is produced.

A matching terminal turn with `failed` status produces a runtime error even if
the peer omits diagnostic details or supplies null. Missing diagnostics cannot
turn a known failed outcome into success. Terminal history bookkeeping precedes
classification, and streamed completion remains visible before the failure.
Detailed peer errors retain their existing wrapping; absent details remain absent
in protocol snapshots. This is outcome reporting, not proof that remote side
effects were rolled back or that a failed turn is safe to retry.

Transport termination does not discard runtime-owned startup replay or final
stream events. An attached, stalled stream consumer therefore retains ordinary
backpressure even after EOF; consumer progress, detachment or caller cancellation
is still needed. An application notification callback that never returns can
likewise delay delivery retirement. These application-cooperation limits preserve
accepted data and bounded memory rather than treating reader exit as delivery
completion.

Caller cancellation releases blocked streaming event publication during startup
replay and terminal-event delivery, even when the lifecycle itself is applying
backpressure. This stops further queue admission without choosing a terminal
error or replacing an already selected completion's result. The lifecycle keeps
ownership of its result, error and listener cleanup. The queue remains bounded;
cancellation does not force an application iterator callback to return.

If an application iterator callback panics or exits its goroutine, deferred
consumer detachment releases queued events and blocked producers. This prevents
an exited consumer from indefinitely blocking completion delivery. The original
panic propagates to the application; a callback that never returns still requires
application cooperation. Detachment does not cancel remote execution.

A stale or unrelated notification could contaminate a turn result. [Turn lifecycle handling](../appserver/turn_lifecycle.go) registers listeners before starting a turn, filters thread/turn identities (including streamed turn-start events), bounds pending start notifications, and checks terminal status. Live turn-owned publication waits for startup replay to finish, keeping collector updates and event publication ordered across that handoff without holding the startup-state mutex during backpressure. Its ordering assumptions must also hold for custom transports. Matching IDs prevent accidental cross-talk; a malicious connected peer can still claim those IDs, so correlation does not authenticate completion or independently prove an external effect.

[Conversation snapshots](../appserver/conversation.go), the [protocol cache](../appserver/protocol/clone_helpers.go), collector item ingestion and summaries use shared [deep-copy helpers](../internal/deepcopy/deepcopy.go) for complete reference-valued protocol shapes. Streamed completion retains an owned item before exposing public events; derived collaboration events and turn-completion events have separate references. Result item/turn/thread views and repeated `Stream.Result()` snapshots remain independent. These copies preserve nil versus present empty values, dynamic union types and raw metadata without a serialization round trip. Lifecycle responses and settings notifications supply cached disabled plugin IDs separately from the Thread wire object; later Thread-only refreshes preserve those saved settings. Settings admission rejects supplied null ID arrays and null elements, including earlier duplicate occurrences, before typed callbacks or cache updates; omitted lists retain their established empty-default behavior. Cloning the IDs prevents mutation of a returned snapshot from changing cached metadata. The schema describes them as a saved list that does not yet filter plugin capabilities; they are not an SDK enforcement control. Snapshot isolation covers supported protocol shapes; arbitrary application-provided values containing unexported internals, channels, functions, or other opaque references are not a general isolation mechanism. The cache is best-effort state, not durable evidence or an authorization source.

Context cancellation and RPC timeouts stop local waiting or request cooperative interruption. They do not prove that Codex or a remote provider stopped, nor that a command, file write, or network action was undone. Applications must retain uncertainty and reconcile consequential operations before retrying. The SDK does not provide general exactly-once execution, transactional rollback, or durable recovery of application approvals and effects.

## OAuth callbacks, provider routing, and account identity

The [normal login flow](../login/login.go) generates a random state and an S256 PKCE challenge using [crypto/rand](../login/pkce.go). The [callback server](../login/callback.go) defaults to IPv4 loopback, accepts only the callback route, checks the expected state when supplied, and has a header-read timeout. Wrong-state requests do not complete the normal flow. A caller invoking the lower-level callback API with an empty expected state opts out of that comparison.

`CallbackServer.Close` attempts graceful HTTP shutdown with its fixed deadline,
then force-closes the owned server if graceful shutdown fails. This releases
active callback TCP connections after a timeout while preserving the shutdown
error; if forced close also errors, both errors are retained. `Close` does not
cancel `Wait` or promise to join arbitrary HTTP handler goroutines. Login still
uses the same callback response, authorization-code and token-exchange behavior;
the cleanup change affects only callback server lifetime.
Close also explicitly releases the owned listener when shutdown races ahead of
Serve registration. Immediate terminal input cannot leave a listening socket
owned only by a goroutine that has not run yet; already-closed listener errors
are harmless, and other listener-close failures remain visible.

Callback host, authorization/token endpoints, redirect URI, and HTTP client are [configurable](../login/config.go), including a callback-host environment override. Non-loopback binding and nonstandard endpoints are caller-controlled exposure decisions. The SDK does not require every configured endpoint to use HTTPS, pin endpoint identity, enforce exact browser Origin/Host, or limit callback connections per user. It must not be exposed as a general authenticated HTTP API. The [manual-input parser](../login/parse.go) supports a bare code without state; supplying a manual code relies on the application's trusted user interaction and the flow's PKCE verifier. A provided mismatched state is rejected.

Recognized inbound authorization queries are parsed strictly for pasted redirect URLs
and browser callbacks. A malformed escape or unescaped semicolon in any pair
rejects the whole query before authorization-code publication or token exchange,
even when other code/state pairs are valid. This prevents a malformed supplied
state from being silently treated as absent. Trusted bare-code input and valid
lower-level callbacks without an expected state retain their existing policy.
Configured authorization endpoints are caller-controlled configuration, separate
from inbound redirect validation.
The manual parser recognizes raw query input through its existing literal
`code=` spelling or literal `error=` query keys before a manual code/state
separator. Other non-URL input retains its trusted bare-code or code/state
grammar.

OAuth authorization errors such as access denial or provider failure terminate
the flow with `ErrAuthorizationFailed`, zero credentials and no token exchange.
Callbacks validate the expected state before publishing a terminal error; wrong
or missing state cannot terminate a normal login. Pasted error URLs and queries
use the manual input's existing state policy. A provider error takes precedence
over a simultaneously supplied code after state checks.
The authorization waiter cancels competing input contexts and Login closes the
callback listener on return. Error code and description previews are bounded
before wrapping, and the callback page HTML-escapes diagnostic text. Such text is
still peer-provided information, not a verified account or a universal secret
redaction boundary; provider error URIs are not followed.

The manual-input state comparison uses the complete received value. A rejected
state is displayed through a bounded diagnostic preview before error formatting,
so an unusually large pasted value is not copied into the retained error in
full. The same parser serves redirect URLs, query strings and code/state pairs;
a mismatch does not publish credentials or initiate token exchange. This is an
error-retention control, not a manual-input size limit or universal redaction.

[Token requests](../login/token.go) use the caller's context, bound token-response reads to 1 MiB, and omit non-success response bodies from errors because they may echo credentials. The configured HTTP client's TLS, redirect, and timeout behavior still matters. A malicious configured endpoint, permissive custom HTTP transport, or unsafe URL logger can disclose codes, verifiers, or refresh tokens.

A successful refresh response may omit or null the replacement refresh token.
The refresh flow retains the submitted token in that case, uses a nonblank new
token when supplied, and rejects an explicitly blank replacement. Authorization-
code exchange still requires an issued refresh token. The selected token is
published only with the validated access token, account metadata and lifetime;
failed validation returns no partial credentials. This preserves the endpoint's
rotation contract, not proof that a retained token remains valid or unrevoked.

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
