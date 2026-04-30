package codex

import (
	"encoding/json"
	"errors"
	"fmt"
)

// SessionSource represents the source of a thread session
type SessionSource interface {
	isSessionSource()
}

// Simple session source literals
type sessionSourceLiteral string

func (sessionSourceLiteral) isSessionSource() {}

const (
	SessionSourceCLI       sessionSourceLiteral = "cli"
	SessionSourceVSCode    sessionSourceLiteral = "vscode"
	SessionSourceExec      sessionSourceLiteral = "exec"
	SessionSourceAppServer sessionSourceLiteral = "appServer"
	SessionSourceUnknown   sessionSourceLiteral = "unknown"
)

// SessionSourceSubAgent represents a sub-agent session source
type SessionSourceSubAgent struct {
	SubAgent SubAgentSource `json:"subAgent"`
}

func (SessionSourceSubAgent) isSessionSource() {}

// SubAgentSource represents the type of sub-agent
type SubAgentSource interface {
	isSubAgentSource()
}

// Simple sub-agent source literals
type subAgentSourceLiteral string

func (subAgentSourceLiteral) isSubAgentSource() {}

const (
	SubAgentSourceReview              subAgentSourceLiteral = "review"
	SubAgentSourceCompact             subAgentSourceLiteral = "compact"
	SubAgentSourceMemoryConsolidation subAgentSourceLiteral = "memory_consolidation"
)

// SubAgentSourceThreadSpawn represents a thread spawn sub-agent
type SubAgentSourceThreadSpawn struct {
	ThreadSpawn struct {
		AgentNickname  string `json:"agent_nickname"`
		AgentRole      string `json:"agent_role"`
		Depth          uint32 `json:"depth"`
		ParentThreadID string `json:"parent_thread_id"`
	} `json:"thread_spawn"`
}

func (SubAgentSourceThreadSpawn) isSubAgentSource() {}

func (s *SubAgentSourceThreadSpawn) UnmarshalJSON(data []byte) error {
	var raw struct {
		ThreadSpawn json.RawMessage `json:"thread_spawn"`
	}
	if err := unmarshalInboundObject(data, &raw, []string{"thread_spawn"}, []string{"thread_spawn"}); err != nil {
		return err
	}
	if err := validateRequiredObjectFields(raw.ThreadSpawn, "depth", "parent_thread_id"); err != nil {
		return err
	}
	type wire SubAgentSourceThreadSpawn
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = SubAgentSourceThreadSpawn(decoded)
	return nil
}

// SubAgentSourceOther represents an unknown sub-agent type
type SubAgentSourceOther struct {
	Other string `json:"other"`
}

func (SubAgentSourceOther) isSubAgentSource() {}

func (s *SubAgentSourceOther) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "other"); err != nil {
		return err
	}

	type wire SubAgentSourceOther
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*s = SubAgentSourceOther(decoded)
	return nil
}

// UnknownSubAgentSource represents an unrecognized sub-agent source object from a newer protocol version.
type UnknownSubAgentSource struct {
	Raw json.RawMessage `json:"-"`
}

func (UnknownSubAgentSource) isSubAgentSource() {}

func (u UnknownSubAgentSource) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// UnknownSessionSource represents an unrecognized session source from a newer protocol version.
type UnknownSessionSource struct {
	Raw json.RawMessage `json:"-"`
}

func (UnknownSessionSource) isSessionSource() {}

func (u UnknownSessionSource) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// SessionSourceWrapper wraps SessionSource for JSON marshaling
type SessionSourceWrapper struct {
	Value SessionSource
}

// UnmarshalJSON for SessionSourceWrapper handles the union type
func (s *SessionSourceWrapper) UnmarshalJSON(data []byte) error {
	// Try string literal first
	var literal string
	if err := json.Unmarshal(data, &literal); err == nil {
		s.Value = sessionSourceLiteral(literal)
		return nil
	}

	// Try object
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		if subAgentRaw, hasKey := raw["subAgent"]; hasKey {
			subAgent, err := unmarshalSubAgentSource(subAgentRaw)
			if err != nil {
				return fmt.Errorf("unmarshal session source subAgent: %w", err)
			}
			s.Value = SessionSourceSubAgent{SubAgent: subAgent}
			return nil
		}
		// Unknown object variant — preserve for forward compatibility
		s.Value = UnknownSessionSource{Raw: append(json.RawMessage(nil), data...)}
		return nil
	}

	return fmt.Errorf("unable to unmarshal SessionSource from: %.200s", data)
}

// unmarshalSubAgentSource dispatches the SubAgentSource discriminated union.
func unmarshalSubAgentSource(data json.RawMessage) (SubAgentSource, error) {
	// Try string literal first
	var literal string
	if err := json.Unmarshal(data, &literal); err == nil {
		return subAgentSourceLiteral(literal), nil
	}

	// Try object variants
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("unable to unmarshal SubAgentSource: %w", err)
	}

	if _, ok := keys["thread_spawn"]; ok {
		var ts SubAgentSourceThreadSpawn
		if err := json.Unmarshal(data, &ts); err != nil {
			return nil, fmt.Errorf("unmarshal thread_spawn: %w", err)
		}
		return ts, nil
	}

	if _, ok := keys["other"]; ok {
		var other SubAgentSourceOther
		if err := json.Unmarshal(data, &other); err != nil {
			return nil, fmt.Errorf("unmarshal other: %w", err)
		}
		return other, nil
	}

	return nil, errors.New("sub-agent source: missing discriminator")
}

// MarshalJSON for SessionSourceWrapper
func (s SessionSourceWrapper) MarshalJSON() ([]byte, error) {
	if s.Value == nil {
		return []byte("null"), nil
	}
	switch v := s.Value.(type) {
	case sessionSourceLiteral:
		return json.Marshal(string(v))
	case SessionSourceSubAgent:
		return json.Marshal(v)
	case UnknownSessionSource:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown SessionSource type: %T", v)
	}
}
