package protocol

import (
	"encoding/json"
	"fmt"
)

// MigrationDetails lists the external-agent resources selected for migration.
type MigrationDetails struct {
	Commands   []CommandMigration   `json:"commands,omitzero"`
	Hooks      []HookMigration      `json:"hooks,omitzero"`
	McpServers []McpServerMigration `json:"mcpServers,omitzero"`
	Memory     []string             `json:"memory,omitzero"`
	Plugins    []PluginsMigration   `json:"plugins,omitzero"`
	Sessions   []SessionMigration   `json:"sessions,omitzero"`
	Skills     []SkillMigration     `json:"skills,omitzero"`
	Subagents  []SubagentMigration  `json:"subagents,omitzero"`
}

func (d *MigrationDetails) UnmarshalJSON(data []byte) error {
	type wire MigrationDetails
	var decoded struct {
		wire
		Memory nonNullStringList `json:"memory,omitzero"`
	}
	nonNull := []string{"commands", "hooks", "mcpServers", "memory", "plugins", "sessions", "skills", "subagents"}
	if err := unmarshalInboundObject(data, &decoded, nil, nonNull); err != nil {
		return err
	}
	decoded.wire.Memory = []string(decoded.Memory)
	*d = MigrationDetails(decoded.wire)
	return nil
}

// CommandMigration identifies an external command selected for migration.
type CommandMigration struct {
	Name string `json:"name"`
}

func (m *CommandMigration) UnmarshalJSON(data []byte) error {
	type wire CommandMigration
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"name"}, []string{"name"}); err != nil {
		return err
	}
	*m = CommandMigration(decoded)
	return nil
}

// HookMigration identifies an external hook selected for migration.
type HookMigration struct {
	Name string `json:"name"`
}

func (m *HookMigration) UnmarshalJSON(data []byte) error {
	type wire HookMigration
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"name"}, []string{"name"}); err != nil {
		return err
	}
	*m = HookMigration(decoded)
	return nil
}

// McpServerMigration identifies an external MCP server selected for migration.
type McpServerMigration struct {
	Name string `json:"name"`
}

func (m *McpServerMigration) UnmarshalJSON(data []byte) error {
	type wire McpServerMigration
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"name"}, []string{"name"}); err != nil {
		return err
	}
	*m = McpServerMigration(decoded)
	return nil
}

// PluginsMigration identifies external plugins selected from a marketplace.
type PluginsMigration struct {
	MarketplaceName string   `json:"marketplaceName"`
	PluginNames     []string `json:"pluginNames"`
}

func (m *PluginsMigration) UnmarshalJSON(data []byte) error {
	type wire PluginsMigration
	var decoded struct {
		wire
		PluginNames nonNullStringList `json:"pluginNames"`
	}
	required := []string{"marketplaceName", "pluginNames"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	decoded.wire.PluginNames = []string(decoded.PluginNames)
	*m = PluginsMigration(decoded.wire)
	return nil
}

func (m PluginsMigration) MarshalJSON() ([]byte, error) {
	if m.PluginNames == nil {
		return nil, fmt.Errorf("migration.pluginNames must not be null")
	}
	type wire PluginsMigration
	return json.Marshal(wire(m))
}

// SessionMigration identifies an external session selected for migration.
// Cwd and Path are plain protocol strings and may be relative paths.
type SessionMigration struct {
	Cwd   string  `json:"cwd"`
	Path  string  `json:"path"`
	Title *string `json:"title,omitempty"`
}

func (m *SessionMigration) UnmarshalJSON(data []byte) error {
	type wire SessionMigration
	var decoded wire
	required := []string{"cwd", "path"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*m = SessionMigration(decoded)
	return nil
}

// SkillMigration identifies an external skill selected for migration.
type SkillMigration struct {
	Name string `json:"name"`
}

func (m *SkillMigration) UnmarshalJSON(data []byte) error {
	type wire SkillMigration
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"name"}, []string{"name"}); err != nil {
		return err
	}
	*m = SkillMigration(decoded)
	return nil
}

// SubagentMigration identifies an external subagent selected for migration.
type SubagentMigration struct {
	Name string `json:"name"`
}

func (m *SubagentMigration) UnmarshalJSON(data []byte) error {
	type wire SubagentMigration
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"name"}, []string{"name"}); err != nil {
		return err
	}
	*m = SubagentMigration(decoded)
	return nil
}

// nonNullStringList permits a nullable array while rejecting null string items.
// Containing decoders enforce array-level requiredness and nullability.
type nonNullStringList []string

func (s *nonNullStringList) UnmarshalJSON(data []byte) error {
	var decoded []*string
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded == nil {
		*s = nil
		return nil
	}
	values := make(nonNullStringList, len(decoded))
	for i, value := range decoded {
		if value == nil {
			return fmt.Errorf("string at index %d must not be null", i)
		}
		values[i] = *value
	}
	*s = values
	return nil
}
