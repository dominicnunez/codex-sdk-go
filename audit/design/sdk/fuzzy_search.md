### Fuzzy search results accept unvalidated filesystem paths

**Line:** `56`

**Reason:** The behavior is real, but it matches the protocol contract. The local schemas for `FuzzyFileSearchResult` define both `path` and `root` as plain strings, not `AbsolutePathBuf`, and Codex's underlying file-search result treats `path` as relative to the search root.

Rejecting relative or non-absolute `path` values here would reject valid protocol results. Callers that need a full filesystem path should join `root` and `path` and validate that derived path for their own operation.
