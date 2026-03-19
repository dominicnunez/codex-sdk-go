# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Plugin read/install responses succeed with missing required fields

**Location:** `plugin.go:291` — plugin response decoding and validation

**Reason:** The report is stale against the current code. `PluginDetail`,
`PluginSummary`, `PluginSource`, `AppSummary`, and `SkillSummary` now reject missing
required fields during JSON unmarshaling, and `Plugin.Read` / `Plugin.Install` both
run explicit response validation before returning. The described zero-value success path for
missing `plugin`, `appsNeedingAuth`, or `authPolicy` fields no longer occurs.
