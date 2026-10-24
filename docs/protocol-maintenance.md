# Protocol maintenance

This reference collects the schema and codec cases used when maintaining
`appserver/protocol/`. The [agent guidance](../AGENTS.md) states the repository
rules; the [protocol guide](protocol.md) describes the public APIs. For changes
in these areas, apply the relevant contracts and verification cases below.
Explanations and examples supplement those requirements.

## Schema inventory

The [checked-in schemas](../appserver/protocol/schema/json/) are authoritative
for wire shapes. Their [upstream
revision](../appserver/protocol/schema/json/.upstream-sha) identifies the server
source used when producer behavior needs investigation. A schema field inventory
includes properties, requiredness, nullability, enum values, defaults and union
branches, including nested definitions.

Go type-name coverage cannot detect fields lost in custom wire structs or
assignment lists. Trace those fields through public service calls, notification
handlers and other supported carriers. Shared Go types may have different enum
constraints in different carrier schemas. Named string types do not enforce an
enum unless the owning decoder actually validates them. Where admission rejects
invalid recognized occurrences, include earlier invalid duplicates followed by
valid values, escaped names and the owner's actual alias matching rules. Legacy
public fields can remain supported without making them required when absent from
the current schema.

## Unions and public representations

For untagged `anyOf` object unions, alternatives are evaluated independently in
schema order. A field constrained by one alternative may be an unconstrained
extra in another. Test valid alternatives with malformed inactive properties
alongside malformed shared properties.

Interface marker and JSON method sets need separate inspection. Value markers
admit values and pointers, while forwarding encoders and type switches may
handle only one form. Exercise both forms, typed nils, unknown raw payloads,
nested carriers and application extensions. Private marker methods also permit
embedding; a root concrete-type switch does not seal the complete encoded graph
against application fields or nested `json.Number` values.

Keep public records method-free when their carriers own validation, preserving
ordinary JSON behavior. Adding a JSON method to such a record can promote that
method into an anonymous application envelope and consume the entire envelope.
Named and anonymous wrappers therefore exercise different composition
boundaries.

Test constructed union branches separately during serialization. A pointer to a
nil slice can select a branch explicitly; nullability must not erase that
choice.

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
references. Compare these behaviors on errors as well as successful decoding.
The [runtime ownership reference](architecture.md#snapshots-and-retention)
traces where decoded references become independently owned views.

## Decoder compatibility

A shared decoder's public contract includes outer wrappers, error precedence,
field context, offsets, receiver updates and direct `UnmarshalJSON` calls with
invalid JSON. An independent reference model and the prior revision provide
comparison evidence; reproducing the implementation's own helper is not an
independent model.

The default `encoding/json` backend can continue through enclosing
fields after a custom decoder fails and retains the first failure. Do not infer
atomicity or traversal from the presence of a JSON method. Compare named and
anonymous wrappers, mixed failures and receiver reuse. Run established owner
checks before new restrictions and verify mixed failures retain their error
precedence. New admission failures still need evidence for their own publication
and receiver guarantees; those guarantees are not implied by ordinary JSON
decoding.

Native `json.UnmarshalTypeError` paths can contain array indices and the actual
spelling of aliased JSON keys. Preserve native type, offset and field context;
the Config decoder restores its public root name after decoding its private
wire representation. Native error messages and causes follow the supported Go
runtime. Use structured error checks rather than complete message comparisons.

## Paths and approvals

The [public path guide](protocol.md#server-path-resolution) distinguishes opaque
strings from `AbsolutePathBuf`. An absolute value produced by the server does
not narrow a plain-string schema. Likewise, a server constructor that resolves
`PathBuf` does not establish `AbsolutePathBuf` JSON deserialization behavior.

For path changes, inventory production `normalize*AbsolutePath*` callers and
response readers, then check the schema and pinned server behavior at every
admission stage. Request selectors and returned grouping values belong to the
same analysis. Existing negative fixtures alone do not establish supported
inputs. Preserve approval metadata as opaque path evidence for the application;
do not resolve or normalize it locally before the approval handler sees it.

## Diagnostics and JSON selection

The [threat
model](threat-model.md#json-rpc-framing-dispatch-and-resource-exhaustion)
describes implemented diagnostic bounds and their limits. Native error ownership
comes from the destination and caller, not an error type or message prefix. The
shared [decoder](../internal/jsondecode/unmarshal.go) distinguishes standard
JSON, text and streaming application codecs; the shared
[encoder](../internal/jsonencode/marshal.go) bounds native Number failures
before SDK wrappers retain them. Application error identity and chains remain
owned by the application.

For SDK-owned rejected peer-value diagnostics, bound previews before quoting or
wrapping. Verify escaping growth and enclosing error chains, native versus
application codec ownership, and the original display spelling of an identifier
as well as its canonical matching key. Use the shared encoder at every SDK-owned protocol
marshaling boundary before enclosing codecs wrap errors. Establish ownership
from the destination and caller; an error type, message prefix or private
interface marker does not prove native ownership or seal nested extensions. A
frame limit does not bound diagnostic allocation or retained errors by itself.

Borrowed-field selection in [jsonobject](../internal/jsonobject/selection.go)
avoids decoding ignored metadata. Preserve duplicate selection, aliases, nulls
and malformed-prefix behavior, not only valid final values. Measure temporary
allocations while varying ignored metadata, duplicate selected fields, identity
length and depth. Follow selected identities into typed callbacks and fallback
attribution. Distinguish ignored-property work from necessary semantic decoding
of selected payloads. Verify successful typed/cache paths, malformed semantic
fields before and after dense metadata, and nested recovery. Measure isolated
selection costs and complete transport operations. The [transport
benchmarks](transport-benchmarks.md) provide complete-operation workloads.

## Test fixtures

Protocol tests use `MockTransport` for immediate responses, recorded requests
and incoming server messages, and `SlowMockTransport` for delayed responses.
For schema changes, run
`go test ./appserver/protocol -run TestSpecCoverage`. This checks that schema
names have corresponding Go types, not field preservation, enum admission or
round-trip correctness. Those fixtures and type coverage complement public-call,
serialization and receiver checks; they do not replace them. [Spec-sync
maintenance](spec-sync.md) records schema snapshot evidence and the
synchronization workflow.
