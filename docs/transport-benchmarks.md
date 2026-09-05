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

Pull requests also measure the base commit's real pipeline on the same runner.
The base checkout SHA and results are saved beside the candidate results. This
controls hardware differences, although load and measurement order can still
affect timing. Compare medians and sample ranges, not a single iteration.

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

## Initial baseline (2026-09-05)

[Run 33980160127](https://github.com/dominicnunez/codex-sdk-go/actions/runs/33980160127)
established the baseline for PR #56 at head `98de4b8e9f`.
Five samples, 500 ms minimum per sample, Go 1.25.11:

| 9 MiB case | Linux median | Windows median | Allocated per operation |
| --- | ---: | ---: | ---: |
| Plain response: route + payload | 121.64 ms | 79.03 ms | 18.01 MiB |
| Plain response: concrete reference | 46.31 ms | 35.19 ms | 9.00 MiB |
| Escaped diff: pipe to typed handler | 258.44 ms | 171.13 ms | about 114.2 MiB |

Linux used an AMD EPYC 7763 runner and Windows an AMD EPYC 9V74 runner; these
columns are separate baselines, not an OS comparison. Profiles identify JSON
scanning/validation as the dominant CPU work. Cumulative `encoding/json.Unmarshal`
time includes nested functions and must not be added to their percentages.
Allocation profiles also identify line-buffer growth and decoder refill buffers.
Profiles include setup work outside benchmark timing, so use B/op for per-message
allocation comparisons.

A local experiment borrowing envelope payload bytes saved about 9 MiB per
9 MiB message but showed no latency improvement. It was not retained. Direct
decoding of built-in string fields in schema validation instead avoids one raw
copy and validation scan; named/custom types keep their original path. Compare
that change using the real pipeline baseline, since the simple parse benchmarks
do not exercise schema validation.
