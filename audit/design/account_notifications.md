# Design

> Findings that describe behavior which is correct by design.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### LoginId field uses spec casing instead of Go acronym convention

**Location:** `account_notifications.go:25` — AccountLoginCompletedNotification.LoginId
**Date:** 2026-02-27

**Reason:** The spec schema (`AccountLoginCompletedNotification.json`) defines the wire field
as `"loginId"`. The Go field name `LoginId` mirrors the spec. Renaming to `LoginID` would be
more idiomatic Go, but the project's spec compliance rules prohibit renaming public fields
that map to spec schemas. The JSON struct tag preserves wire compatibility regardless.
