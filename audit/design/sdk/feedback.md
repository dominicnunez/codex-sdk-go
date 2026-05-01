### Feedback uploads accept unvalidated extra log file paths

**Line:** `9`

**Reason:** The behavior is real, but the protocol source of truth does not define `extraLogFiles` as absolute paths. The local schema for `FeedbackUploadParams` represents `extraLogFiles` as string entries, and upstream Codex defines the field as `Option<Vec<PathBuf>>`, not `AbsolutePathBuf`.

The server uses these values as attachment paths, but the upstream protocol/source does not require absolute or normalized entries. Adding `normalizeAbsolutePathSliceField` here would narrow accepted request values beyond the protocol contract, so this is a correct-by-design exception rather than a repo bug.
