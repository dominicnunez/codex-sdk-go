### External-agent migration item types are already validated during decode

**Location:** `external_agent.go:25` — `ExternalAgentConfigMigrationItemType.UnmarshalJSON`

**Reason:** The current implementation already validates `itemType` against the closed enum set in
`validExternalAgentConfigMigrationItemTypes`. Because `ExternalAgentConfigMigrationItem.ItemType`
uses that type directly, unknown values are rejected during `json.Unmarshal`. The behavior is
verified by `external_agent_test.go:204-220`.
