### AppsListParams.ForceRefetch described as missing omitempty but it has omitempty

**Location:** `apps.go:11` — ForceRefetch field tag
**Date:** 2026-02-27

**Reason:** The audit claims `ForceRefetch bool` has "no `omitempty` tag" and that "the zero value
`false` is always serialized." This is factually wrong. The actual field declaration is
`ForceRefetch bool \`json:"forceRefetch,omitempty"\`` — it already has omitempty. With `bool` +
`omitempty`, the `false` value is *omitted* (not sent), which is the opposite of what the audit
describes. The stated problem ("missing omitempty sends false as default") does not occur.
