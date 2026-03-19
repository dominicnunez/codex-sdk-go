### Collector tests never exercise RunStreamedWithCollector overflow handling

**Location:** `stream_collector_test.go:358` — `TestRunStreamedWithCollectorReportsOverflowInSummary`

**Reason:** The collector suite already starts `RunStreamedWithCollector`, withholds event
consumption, overflows the bounded queue, and asserts that both `Events()` and
`collector.Summary()` report the overflow. This finding describes a missing regression test that is
already present.
