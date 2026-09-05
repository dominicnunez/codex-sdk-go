# Inbound transport baseline

Tracks [issue #4](https://github.com/dominicnunez/codex-sdk-go/issues/4).
Runtime behavior is unchanged by the benchmark harness.

Run on Go 1.25.11:

```sh
go test ./appserver/transport -run '^$' -bench 'BenchmarkInbound(Large|Pipeline)' -benchmem -benchtime=500ms -count=5
```

The Transport benchmarks workflow collects this baseline on Linux and Windows,
records the commit and environment, and uploads raw results, CPU/allocation
profiles, and profile summaries. It runs on relevant pull requests and can be
dispatched manually. Results are observations, not timing gates: hosted runners
vary, so compare repeated runs on equivalent environments rather than comparing
absolute Linux and Windows timings as an OS performance ranking.

## Parse microbenchmarks

`BenchmarkInboundLarge` covers notifications, requests, and responses with plain
base64-like strings and escaped diff-like strings. Sizes are approximately 64 KiB,
1 MiB, 5 MiB, and 9 MiB of encoded content; the entire frame is checked against
the 10 MiB limit. Fixture creation is outside timed loops.

- `RouteOnly`: the production envelope decoder, including RawMessage copies.
- `PayloadOnly`: decoding the payload into a simple concrete struct.
- `RouteAndUnmarshal`: both stages, including their allocations.
- `SingleConcreteParse`: a reference that knows the payload type in advance.
  It preserves envelope field decoder types but does not perform dispatch. It
  is not a compatible replacement for arbitrary-method JSON-RPC routing.

Throughput uses frame bytes except in PayloadOnly, which uses payload bytes.
These simple payloads do not model the extra validation in all protocol types.

## Real notification path

`BenchmarkInboundPipeline` writes newline-delimited `turn/diff/updated` frames
through an in-memory pipe and waits for the protocol client's typed callback.
It includes bounded line reading, envelope parsing, queue classification,
dispatch, and typed schema validation. One message is in flight at a time.
The callback checks decoded length; a timeout bounds failures. Transport creation,
fixture construction, and cleanup are outside timing. Pipe scheduling, completion
signaling and watchdog allocation are included. This is not a server/model or
whole-application latency benchmark, and it does not measure burst throughput.

## Interpretation

First compare time and allocated bytes across the parse modes, then inspect the
real pipeline CPU and allocation profiles. Significant microbenchmark overhead
justifies an experiment, not automatically a new parser. A proposed change must
preserve ID precision, arbitrary key ordering, malformed-message handling, frame
limits, and ownership of payload bytes retained by asynchronous handlers. Run
existing transport tests, race tests, and differential/fuzz tests where parsing
changes. Measure the same baseline before and after the candidate change.

Do not close the issue using the suggested 1% application-latency threshold until
an application workload supplies the denominator. This harness alone cannot
establish that ratio.
