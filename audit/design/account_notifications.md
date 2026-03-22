### LoginId field uses spec casing instead of Go acronym convention

**Location:** `25`

**Reason:** The spec schema (`AccountLoginCompletedNotification.json`) defines the wire field
as `"loginId"`. The Go field name `LoginId` mirrors the spec. Renaming to `LoginID` would be
more idiomatic Go, but the project's spec compliance rules prohibit renaming public fields
that map to spec schemas. The JSON struct tag preserves wire compatibility regardless.
