# JSON backend migration

The SDK uses Go 1.27.3's default implementation behind `encoding/json`.
Build and test normally:

```sh
go test ./...
```

Remove `GOEXPERIMENT=nojsonv2` from application build environments. The legacy
backend is no longer supported. This migration keeps the existing
`encoding/json` API; importing `encoding/json/v2` throughout applications is not
required.

## Compatibility changes

Native errors follow the current standard library. `json.UnmarshalTypeError`
paths can include array indices and the actual spelling of aliased JSON keys;
the root struct name may differ from older releases. Error messages and native
causes may also differ. Prefer `errors.As`, `errors.Is` and structured fields
over matching complete error strings. The Config decoder restores its public
root name after decoding its private wire representation.

The default decoder retains the first failure and may continue through later
fields after custom decoder errors. A direct decode or an application envelope
can therefore contain partial updates after failure. Discard the result when
decoding returns an error. SDK service methods continue to reject failed
admission and return zero responses; invalid metadata is not a successful
response. Schema validation and receiver guarantees enforced by individual SDK
owners remain in place.

The default backend checks nesting depth during encoding too. Deep extension
payloads can fail before snapshot decoding. Failed initialization still sends
no request, records no successful initialization, and permits a later valid
attempt.

Native diagnostics for oversized numeric/string conversion values, dynamic
field paths and malformed `json.Number` serialization remain bounded at audited
SDK boundaries. Application JSON, text and streaming codec errors retain their
original identity and chains. These controls do not impose a total input or
heap limit; see the [threat model](threat-model.md).

The [Go 1.27 release notes](https://go.dev/doc/go1.27) describe the default
implementation change.
