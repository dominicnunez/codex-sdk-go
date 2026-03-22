### Plugin read/install responses succeed with missing required fields

**Location:** `291`

**Reason:** The report is stale against the current code. `PluginDetail`,
`PluginSummary`, `PluginSource`, `AppSummary`, and `SkillSummary` now reject missing
required fields during JSON unmarshaling, and `Plugin.Read` / `Plugin.Install` both
run explicit response validation before returning. The described zero-value success path for
missing `plugin`, `appsNeedingAuth`, or `authPolicy` fields no longer occurs.
