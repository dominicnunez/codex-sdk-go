# Thread-state cloning benchmark

Measured on 2026-09-05 with Go 1.25.11, Windows amd64, AMD Ryzen 9 5900X.

For current runs, use Go 1.27.3 with the supported JSON backend:
Run from the repository root:

```sh
go test ./appserver/protocol -run '^$' -bench BenchmarkThreadState -benchmem -benchtime=100ms -count=3
```

The fixture holds 0, 100, or 1,000 turns with one agent-message item each.
It measures metadata mutation and full snapshot replacement with 0, 1, or 4
listeners. Callbacks retain their latest snapshot. Setup is excluded; cache
cloning and per-listener copies are included. This is a protocol microbenchmark,
not an end-to-end Conversation benchmark; it excludes Conversation's own copy,
transport, server latency, and concurrent contention.

Representative medians across three runs (metadata updates):

| Turns | Listeners | Time/op | Bytes/op | Allocations/op |
| ---: | ---: | ---: | ---: | ---: |
| 0 | 0 | 1.16 us | 1,368 | 9 |
| 100 | 0 | 85.3 us | 69,600 | 721 |
| 100 | 1 | 174 us | 138,864 | 1,440 |
| 100 | 4 | 450 us | 346,754 | 3,598 |
| 1,000 | 0 | 1.04 ms | 875,048 | 7,039 |
| 1,000 | 1 | 2.21 ms | 1,749,756 | 14,076 |
| 1,000 | 4 | 6.52 ms | 4,373,748 | 35,188 |

Full replacement follows similar scaling: 170 us for 100 turns/one listener,
2.27 ms for 1,000 turns/one listener, and 6.51 ms for 1,000 turns/four listeners.
Numbers are descriptive, not a regression threshold; the short runs and single
host do not support small percentage comparisons.

## Decision

Retain the current cloning implementation. The copies isolate the cache from
callers and each listener from other listeners. Removing them would change that
guarantee. Large histories have a measurable allocation cost, but this fixture
does not establish that metadata-update frequency makes it a production bottleneck.
No performance improvement is claimed by this change.

If workload profiling shows this path dominates allocation or latency, compare a
copy-on-write metadata implementation against this baseline. Such an experiment
must keep old snapshots immutable and test nested history mutations, concurrent
reads, and listener isolation. A field-by-field clone would also need schema-change
coverage to avoid silently sharing newly added reference fields.

## Pending thread-start observation

`BenchmarkThreadStartObservation` measures a complete public
`Thread.StartWithStateListener` call and unsubscribe, including response decoding,
notification admission, cache updates, snapshot ownership and listener delivery.
The fixture returns the matching started notification before 0, 32, 64 or 65
unrelated started notifications. Independently, 0, 8 or 64 other public starts
remain pending until cleanup cancels and joins them. There are no server or
transport delays in the measured operation. The cache and pending candidate maps
are warm after the first iteration; these results do not measure cold allocation
of every pending request's candidate map.

Run the workload with:

```sh
go test ./appserver/protocol -run '^$' -bench '^BenchmarkThreadStartObservation$' -benchmem -benchtime=100ms -count=2
```

On 2026-11-03 with Go 1.27.3 and its default JSON backend, the same workload before
and after adding pending-start observation produced these representative means
of two short runs:

| Other pending starts | Unrelated notifications | Before time/op | After time/op | Before bytes/op | After bytes/op |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 0 | 0 | 34.0 us | 35.7 us | 13,099 | 14,206 |
| 0 | 64 | 868 us | 890 us | 327,130 | 321,493 |
| 8 | 64 | 848 us | 902 us | 327,128 | 314,884 |
| 64 | 64 | 895 us | 1,149 us | 327,494 | 321,175 |
| 64 | 65 | 875 us | 1,156 us | 332,404 | 325,785 |

The added work scales with notification traffic and active pending starts:
admission visits each pending observation, retaining at most 64 identity records
per request. Shared cache pins avoid cloning a full Thread graph into each
observation. At capacity and overflow, cache eviction and pin release remain part
of the measured public call. This is a correctness tradeoff, not a performance
improvement; the application still controls pending-request concurrency and
complete Thread sizes. Short runs describe the cost without establishing small
percentage differences or a performance threshold.

A separate manual experiment staggered pending starts and admitted 64 fresh
identities between each new request. This creates disjoint retained candidate
sets rather than the benchmark's overlapping warm sets. At 8 pending starts,
512 cache entries remained pinned; at 64, 4,096 remained pinned. Across two runs,
the latter case took 218–284 ms to admit all 4,096 notifications, 390–416 us for
one further complete public start/unsubscribe, and 227–274 ms to cancel and join
all pending starts. The same workload before observation retained only 64 cache
entries, took 53–54 ms for admission and 44–55 us for the further public start.
These short samples are descriptive. Both revisions returned to at most 64 cache
entries after cancellation. The current cache's linear scans and repeated
eviction can make large disjoint retention and mass release substantially more
expensive than overlapping steady traffic; a per-request candidate bound is not
a constant-time guarantee or a global heap bound.
