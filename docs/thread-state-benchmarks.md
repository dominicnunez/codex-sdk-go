# Thread-state cloning benchmark

Measured on 2026-09-05 with Go 1.25.11, Windows amd64, AMD Ryzen 9 5900X.
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
