# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Collector tests never exercise RunStreamedWithCollector overflow handling

**Location:** `stream_collector_test.go:358` — `TestRunStreamedWithCollectorReportsOverflowInSummary`

**Reason:** The collector suite already starts `RunStreamedWithCollector`, withholds event
consumption, overflows the bounded queue, and asserts that both `Events()` and
`collector.Summary()` report the overflow. This finding describes a missing regression test that is
already present.
