# Protocol maintenance

This reference collects the schema and codec cases used when maintaining
`appserver/protocol/`. The [agent guidance](../AGENTS.md) states the repository
rules; the [protocol guide](protocol.md) describes the public APIs.

## Schema inventory

The [checked-in schemas](../appserver/protocol/schema/json/) are authoritative
for wire shapes. Their [upstream revision](../appserver/protocol/schema/json/.upstream-sha)
identifies the server source used when producer behavior needs investigation.
A schema field inventory includes properties, requiredness, nullability, enum
values, defaults and union branches, including nested definitions.

Go type-name coverage cannot detect fields lost in custom wire structs or
assignment lists. Trace those fields through public service calls, notification
handlers and other supported carriers. Shared Go types may have different enum
constraints in different carrier schemas. Named string types do not enforce an
enum unless the owning decoder actually validates them. Where admission rejects
invalid recognized occurrences, include earlier invalid duplicates followed by
valid values, escaped names and the owner's actual alias matching rules.
Legacy public fields can remain supported without making them required when
absent from the current schema.

## Unions and public representations

For untagged `anyOf` object unions, alternatives are evaluated independently in
schema order. A field constrained by one alternative may be an unconstrained
extra in another. Valid alternatives with malformed inactive properties belong
in the test cases alongside malformed shared properties.

Interface marker and JSON method sets need separate inspection. Value markers
admit values and pointers, while forwarding encoders and type switches may
handle only one form. Relevant cases include both forms, typed nils, unknown raw
payloads, nested carriers and application extensions. Private marker methods
also permit embedding; a root concrete-type switch does not seal the complete
encoded graph against application fields or nested `json.Number` values.

Plain public records retain ordinary JSON behavior when their carriers own
validation. Adding a JSON method to such a record can promote that method into
an anonymous application envelope and consume the entire envelope. Named and
anonymous wrappers therefore exercise different composition boundaries.

Constructed union branches need their own serialization cases. A pointer to a
nil slice can select a branch explicitly; nullability must not erase that choice.

## Presence, defaults and copying

Absent, null, empty and populated collections are distinct cases. JSON member
checks establish which values are present; decoding into a Go zero value alone
can lose that distinction. Include duplicate resets, aliases and reused
receivers, and preserve presence state through the shared deep-copy boundary.

Zero scalar values are separate from nullability. Defaults must not silently
omit counters, and nonnil optional pointers must retain explicit false or zero.
Trace omitted-field defaults to the owning reference codec, then verify both
omitted defaults and explicit values survive serialization. An omitting encoder
can otherwise change a round trip.

Map-valued fields can merge duplicate objects in wire order, reset on null and
reuse entries already held by the receiver. Error cases can also update retained
references. These behaviors need comparison as well as successful decoding.
The [runtime ownership reference](architecture.md#snapshots-and-retention) traces
where decoded references become independently owned views.

## Decoder compatibility

A shared decoder's public contract includes outer wrappers, error precedence,
field context, offsets, receiver updates and direct `UnmarshalJSON` calls with
invalid JSON. An independent reference model and the prior revision provide
comparison evidence; reproducing the implementation's own helper is not an
independent model.

The [default backend](json-backend-migration.md) can continue through enclosing
fields after a custom decoder fails and retains the first failure. Do not infer
atomicity or traversal from the presence of a JSON method. Compare named and
anonymous wrappers, mixed failures and receiver reuse. Existing owner checks
run before new restrictions so their established errors retain precedence.
New admission failures still need evidence for their own publication and
receiver guarantees; those guarantees are not implied by ordinary JSON decoding.

## Paths and approvals

The [public path guide](protocol.md#server-path-resolution) distinguishes opaque
strings from `AbsolutePathBuf`. An absolute value produced by the server does
not narrow a plain-string schema. Likewise, a server constructor that resolves
`PathBuf` does not establish `AbsolutePathBuf` JSON deserialization behavior.

For path changes, inventory production `normalize*AbsolutePath*` callers and
response readers, then check the schema and pinned server behavior at every
admission stage. Request selectors and returned grouping values belong to the
same analysis. Existing negative fixtures alone do not establish supported inputs.
Approval metadata retains opaque path evidence for the application; it is not
resolved or normalized locally before the approval handler sees it.

## Diagnostics and JSON selection

The [threat model](threat-model.md#json-rpc-framing-dispatch-and-resource-exhaustion)
describes implemented diagnostic bounds and their limits. Native error ownership
comes from the destination and caller, not an error type or message prefix.
The shared [decoder](../internal/jsondecode/unmarshal.go) distinguishes standard
JSON, text and streaming application codecs; the shared
[encoder](../internal/jsonencode/marshal.go) bounds native Number failures before
SDK wrappers retain them. Application error identity and chains remain owned
by the application.

Diagnostic cases include escaping growth, enclosing error chains, native versus
application codecs, and the original display spelling of an identifier as well
as its canonical matching key. A frame limit does not bound diagnostic allocation
or retained errors by itself.

Borrowed-field selection in [jsonobject](../internal/jsonobject/selection.go)
avoids decoding ignored metadata. Its behavior depends on duplicate selection,
aliases, nulls and malformed prefixes, not only valid final values. Relevant cost
cases vary ignored metadata, duplicate selected fields, identity length and depth.
Follow selected identities into typed callbacks and fallback attribution, with
malformed semantic fields before and after dense metadata and nested recovery.
The [transport benchmarks](transport-benchmarks.md) include complete transport
operations as well as isolated selection helpers.

## Test fixtures

Protocol tests use `MockTransport` for immediate responses, recorded requests
and incoming server messages, and `SlowMockTransport` for delayed responses.
`TestSpecCoverage` checks that schema names have corresponding Go types. Those
fixtures and type coverage complement public-call, serialization and receiver
checks; they do not replace them. [Spec-sync maintenance](spec-sync.md) records
schema snapshot evidence and the synchronization workflow.
