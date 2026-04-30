package codex

import (
	"encoding/json"
	"fmt"
)

// SandboxPolicy represents sandbox access policy
type SandboxPolicy interface {
	isSandboxPolicy()
}

// SandboxPolicyDangerFullAccess allows full system access
type SandboxPolicyDangerFullAccess struct{}

func (SandboxPolicyDangerFullAccess) isSandboxPolicy() {}

// SandboxPolicyReadOnly allows read-only access
type SandboxPolicyReadOnly struct {
	Access *ReadOnlyAccessWrapper `json:"access,omitempty"`
}

func (SandboxPolicyReadOnly) isSandboxPolicy() {}

// SandboxPolicyExternalSandbox uses external sandbox
type SandboxPolicyExternalSandbox struct {
	NetworkAccess *NetworkAccess `json:"networkAccess,omitempty"` // "restricted" or "enabled"
}

func (SandboxPolicyExternalSandbox) isSandboxPolicy() {}

// SandboxPolicyWorkspaceWrite allows workspace writes
type SandboxPolicyWorkspaceWrite struct {
	ExcludeSlashTmp     *bool                  `json:"excludeSlashTmp,omitempty"`
	ExcludeTmpdirEnvVar *bool                  `json:"excludeTmpdirEnvVar,omitempty"`
	NetworkAccess       *bool                  `json:"networkAccess,omitempty"`
	ReadOnlyAccess      *ReadOnlyAccessWrapper `json:"readOnlyAccess,omitempty"`
	WritableRoots       []string               `json:"writableRoots,omitempty"`
}

func (SandboxPolicyWorkspaceWrite) isSandboxPolicy() {}

// UnknownSandboxPolicy represents an unrecognized sandbox policy type from a newer protocol version.
type UnknownSandboxPolicy struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (UnknownSandboxPolicy) isSandboxPolicy() {}

func (u UnknownSandboxPolicy) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// ReadOnlyAccess is a discriminated union for read-only access configuration
type ReadOnlyAccess interface {
	isReadOnlyAccess()
}

// ReadOnlyAccessRestricted restricts read access to specific roots
type ReadOnlyAccessRestricted struct {
	IncludePlatformDefaults *bool    `json:"includePlatformDefaults,omitempty"`
	ReadableRoots           []string `json:"readableRoots,omitempty"`
}

func (ReadOnlyAccessRestricted) isReadOnlyAccess() {}

// ReadOnlyAccessFullAccess allows full read access
type ReadOnlyAccessFullAccess struct{}

func (ReadOnlyAccessFullAccess) isReadOnlyAccess() {}

// UnknownReadOnlyAccess represents an unrecognized read-only access type from a newer protocol version.
type UnknownReadOnlyAccess struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (UnknownReadOnlyAccess) isReadOnlyAccess() {}

func (u UnknownReadOnlyAccess) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// ReadOnlyAccessWrapper wraps the ReadOnlyAccess discriminated union for JSON
type ReadOnlyAccessWrapper struct {
	Value ReadOnlyAccess
}

// UnmarshalJSON for ReadOnlyAccessWrapper
func (w *ReadOnlyAccessWrapper) UnmarshalJSON(data []byte) error {
	typeField, err := decodeRequiredObjectTypeField(data, "read only access")
	if err != nil {
		return err
	}

	switch typeField {
	case "restricted":
		var v ReadOnlyAccessRestricted
		if err := validateRequiredTaggedObjectFields(data); err != nil {
			return err
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value, err = validateInboundReadOnlyAccessField("readOnlyAccess", v)
		if err != nil {
			return err
		}
	case "fullAccess":
		w.Value = ReadOnlyAccessFullAccess{}
	default:
		w.Value = UnknownReadOnlyAccess{Type: typeField, Raw: append(json.RawMessage(nil), data...)}
	}
	return nil
}

// MarshalJSON for ReadOnlyAccessWrapper injects the correct type discriminator
// so that client-constructed values marshal correctly without requiring callers
// to manually set the Type field.
func (w ReadOnlyAccessWrapper) MarshalJSON() ([]byte, error) {
	value := normalizeReadOnlyAccess(w.Value)
	if value == nil {
		return []byte("null"), nil
	}
	switch v := value.(type) {
	case ReadOnlyAccessRestricted:
		return json.Marshal(struct {
			Type string `json:"type"`
			ReadOnlyAccessRestricted
		}{Type: "restricted", ReadOnlyAccessRestricted: v})
	case ReadOnlyAccessFullAccess:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{Type: "fullAccess"})
	case UnknownReadOnlyAccess:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown ReadOnlyAccess type: %T", v)
	}
}

func normalizeReadOnlyAccess(value ReadOnlyAccess) ReadOnlyAccess {
	switch v := value.(type) {
	case nil:
		return nil
	case *ReadOnlyAccessRestricted:
		if v == nil {
			return nil
		}
		return *v
	case *ReadOnlyAccessFullAccess:
		if v == nil {
			return nil
		}
		return *v
	case *UnknownReadOnlyAccess:
		if v == nil {
			return nil
		}
		return *v
	default:
		return value
	}
}

// NetworkAccess represents network access control
type NetworkAccess string

const (
	NetworkAccessRestricted NetworkAccess = "restricted"
	NetworkAccessEnabled    NetworkAccess = "enabled"
)

// SandboxPolicyWrapper wraps SandboxPolicy for JSON marshaling
type SandboxPolicyWrapper struct {
	Value SandboxPolicy
}

// UnmarshalJSON for SandboxPolicyWrapper handles the discriminated union
func (s *SandboxPolicyWrapper) UnmarshalJSON(data []byte) error {
	typeField, err := decodeRequiredObjectTypeField(data, "sandbox policy")
	if err != nil {
		return err
	}

	switch typeField {
	case "dangerFullAccess":
		s.Value = SandboxPolicyDangerFullAccess{}
	case "readOnly":
		var policy SandboxPolicyReadOnly
		if err := validateRequiredTaggedObjectFields(data); err != nil {
			return err
		}
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}
		s.Value, err = validateInboundSandboxPolicyField("sandboxPolicy", policy)
		if err != nil {
			return err
		}
	case "externalSandbox":
		var policy SandboxPolicyExternalSandbox
		if err := validateRequiredTaggedObjectFields(data); err != nil {
			return err
		}
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}
		s.Value = policy
	case "workspaceWrite":
		var policy SandboxPolicyWorkspaceWrite
		if err := validateRequiredTaggedObjectFields(data); err != nil {
			return err
		}
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}
		s.Value, err = validateInboundSandboxPolicyField("sandboxPolicy", policy)
		if err != nil {
			return err
		}
	default:
		s.Value = UnknownSandboxPolicy{Type: typeField, Raw: append(json.RawMessage(nil), data...)}
	}

	return nil
}

// MarshalJSON for SandboxPolicyWrapper injects the correct type discriminator
// so that client-constructed values marshal correctly without requiring callers
// to manually set the Type field.
func (s SandboxPolicyWrapper) MarshalJSON() ([]byte, error) {
	value := normalizeSandboxPolicy(s.Value)
	if value == nil {
		return []byte("null"), nil
	}
	switch v := value.(type) {
	case SandboxPolicyDangerFullAccess:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{Type: "dangerFullAccess"})
	case SandboxPolicyReadOnly:
		return json.Marshal(struct {
			Type string `json:"type"`
			SandboxPolicyReadOnly
		}{Type: "readOnly", SandboxPolicyReadOnly: v})
	case SandboxPolicyExternalSandbox:
		return json.Marshal(struct {
			Type string `json:"type"`
			SandboxPolicyExternalSandbox
		}{Type: "externalSandbox", SandboxPolicyExternalSandbox: v})
	case SandboxPolicyWorkspaceWrite:
		return json.Marshal(struct {
			Type string `json:"type"`
			SandboxPolicyWorkspaceWrite
		}{Type: "workspaceWrite", SandboxPolicyWorkspaceWrite: v})
	case UnknownSandboxPolicy:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown SandboxPolicy type: %T", v)
	}
}

func normalizeSandboxPolicy(value SandboxPolicy) SandboxPolicy {
	switch v := value.(type) {
	case nil:
		return nil
	case *SandboxPolicyDangerFullAccess:
		if v == nil {
			return nil
		}
		return *v
	case *SandboxPolicyReadOnly:
		if v == nil {
			return nil
		}
		return *v
	case *SandboxPolicyExternalSandbox:
		if v == nil {
			return nil
		}
		return *v
	case *SandboxPolicyWorkspaceWrite:
		if v == nil {
			return nil
		}
		return *v
	case *UnknownSandboxPolicy:
		if v == nil {
			return nil
		}
		return *v
	default:
		return value
	}
}
