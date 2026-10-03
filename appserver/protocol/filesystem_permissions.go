package protocol

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
)

// FileSystemAccessMode controls access to a filesystem entry.
type FileSystemAccessMode string

const (
	FileSystemAccessModeRead  FileSystemAccessMode = "read"
	FileSystemAccessModeWrite FileSystemAccessMode = "write"
	FileSystemAccessModeDeny  FileSystemAccessMode = "deny"
)

var validFileSystemAccessModes = map[FileSystemAccessMode]struct{}{
	FileSystemAccessModeRead: {}, FileSystemAccessModeWrite: {}, FileSystemAccessModeDeny: {},
}

func (m *FileSystemAccessMode) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "filesystem access", validFileSystemAccessModes, m)
}
func (m FileSystemAccessMode) MarshalJSON() ([]byte, error) {
	return marshalEnumString("filesystem access", m, validFileSystemAccessModes)
}

// FileSystemSandboxEntry applies an access mode to a path or special location.
type FileSystemSandboxEntry struct {
	Access FileSystemAccessMode  `json:"access"`
	Path   FileSystemPathWrapper `json:"path"`
}

func (e *FileSystemSandboxEntry) UnmarshalJSON(data []byte) error {
	type wire FileSystemSandboxEntry
	var decoded wire
	required := []string{"access", "path"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*e = FileSystemSandboxEntry(decoded)
	return nil
}

// FileSystemPath is a path, glob pattern, or special filesystem location.
type FileSystemPath interface{ isFileSystemPath() }

// PathFileSystemPath retains an opaque legacy path without normalization.
type PathFileSystemPath struct {
	Path string `json:"path"`
}

func (PathFileSystemPath) isFileSystemPath() {}

// GlobPatternFileSystemPath retains a filesystem pattern without expanding it.
type GlobPatternFileSystemPath struct {
	Pattern string `json:"pattern"`
}

func (GlobPatternFileSystemPath) isFileSystemPath() {}

// SpecialFileSystemPath identifies a special runtime filesystem location.
type SpecialFileSystemPath struct {
	Value FileSystemSpecialPathWrapper `json:"value"`
}

func (SpecialFileSystemPath) isFileSystemPath() {}

// FileSystemPathWrapper serializes a filesystem path with its discriminator.
type FileSystemPathWrapper struct{ Value FileSystemPath }

type filesystemPathType string

func (v *filesystemPathType) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "filesystem path type", map[filesystemPathType]struct{}{
		"path": {}, "glob_pattern": {}, "special": {},
	}, v)
}

func (w *FileSystemPathWrapper) UnmarshalJSON(data []byte) error {
	var tag struct {
		Type filesystemPathType `json:"type"`
	}
	if err := unmarshalInboundObject(data, &tag, []string{"type"}, []string{"type"}); err != nil {
		return err
	}
	var value FileSystemPath
	var dest any
	var required []string
	switch tag.Type {
	case "path":
		v := new(PathFileSystemPath)
		value, dest, required = v, v, []string{"path"}
	case "glob_pattern":
		v := new(GlobPatternFileSystemPath)
		value, dest, required = v, v, []string{"pattern"}
	case "special":
		v := new(SpecialFileSystemPath)
		value, dest, required = v, v, []string{"value"}
	default:
		return fmt.Errorf("invalid filesystem path type %s", quotedValueDiagnostic(string(tag.Type)))
	}
	if err := unmarshalInboundObject(data, dest, required, required); err != nil {
		return err
	}
	w.Value = value
	return nil
}

func (w FileSystemPathWrapper) MarshalJSON() ([]byte, error) {
	switch v := permissionUnionValue(w.Value).(type) {
	case PathFileSystemPath:
		return json.Marshal(struct {
			Type string `json:"type"`
			PathFileSystemPath
		}{"path", v})
	case GlobPatternFileSystemPath:
		return json.Marshal(struct {
			Type string `json:"type"`
			GlobPatternFileSystemPath
		}{"glob_pattern", v})
	case SpecialFileSystemPath:
		return json.Marshal(struct {
			Type string `json:"type"`
			SpecialFileSystemPath
		}{"special", v})
	default:
		return nil, fmt.Errorf("invalid filesystem path value %T", w.Value)
	}
}

// FileSystemSpecialPath identifies a special location interpreted by the runtime.
type FileSystemSpecialPath interface{ isFileSystemSpecialPath() }

// RootFileSystemSpecialPath identifies the filesystem root.
type RootFileSystemSpecialPath struct{}

// MinimalFileSystemSpecialPath identifies the runtime's minimal filesystem scope.
type MinimalFileSystemSpecialPath struct{}

// ProjectRootsFileSystemSpecialPath identifies project roots and an optional subpath.
type ProjectRootsFileSystemSpecialPath struct {
	Subpath *string `json:"subpath,omitempty"`
}

// TmpdirFileSystemSpecialPath identifies the runtime temporary directory.
type TmpdirFileSystemSpecialPath struct{}

// SlashTmpFileSystemSpecialPath identifies the /tmp special location.
type SlashTmpFileSystemSpecialPath struct{}

// UnknownFileSystemSpecialPath retains an explicitly described special location.
type UnknownFileSystemSpecialPath struct {
	Path    string  `json:"path"`
	Subpath *string `json:"subpath,omitempty"`
}

func (RootFileSystemSpecialPath) isFileSystemSpecialPath()         {}
func (MinimalFileSystemSpecialPath) isFileSystemSpecialPath()      {}
func (ProjectRootsFileSystemSpecialPath) isFileSystemSpecialPath() {}
func (TmpdirFileSystemSpecialPath) isFileSystemSpecialPath()       {}
func (SlashTmpFileSystemSpecialPath) isFileSystemSpecialPath()     {}
func (UnknownFileSystemSpecialPath) isFileSystemSpecialPath()      {}

// FileSystemSpecialPathWrapper serializes a special location with its kind.
type FileSystemSpecialPathWrapper struct{ Value FileSystemSpecialPath }

type filesystemSpecialKind string

func (v *filesystemSpecialKind) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "filesystem special path kind", map[filesystemSpecialKind]struct{}{
		"root": {}, "minimal": {}, "project_roots": {}, "tmpdir": {}, "slash_tmp": {}, "unknown": {},
	}, v)
}

func (w *FileSystemSpecialPathWrapper) UnmarshalJSON(data []byte) error {
	var tag struct {
		Kind filesystemSpecialKind `json:"kind"`
	}
	if err := unmarshalInboundObject(data, &tag, []string{"kind"}, []string{"kind"}); err != nil {
		return err
	}
	var value FileSystemSpecialPath
	var dest any
	var required []string
	switch tag.Kind {
	case "root":
		value = RootFileSystemSpecialPath{}
	case "minimal":
		value = MinimalFileSystemSpecialPath{}
	case "project_roots":
		v := new(ProjectRootsFileSystemSpecialPath)
		value, dest = v, v
	case "tmpdir":
		value = TmpdirFileSystemSpecialPath{}
	case "slash_tmp":
		value = SlashTmpFileSystemSpecialPath{}
	case "unknown":
		v := new(UnknownFileSystemSpecialPath)
		value, dest, required = v, v, []string{"path"}
	default:
		return fmt.Errorf("invalid filesystem special path kind %s", quotedValueDiagnostic(string(tag.Kind)))
	}
	if dest != nil {
		if err := unmarshalInboundObject(data, dest, required, required); err != nil {
			return err
		}
	}
	w.Value = value
	return nil
}

func (w FileSystemSpecialPathWrapper) MarshalJSON() ([]byte, error) {
	switch v := permissionUnionValue(w.Value).(type) {
	case RootFileSystemSpecialPath:
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{"root"})
	case MinimalFileSystemSpecialPath:
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{"minimal"})
	case ProjectRootsFileSystemSpecialPath:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			ProjectRootsFileSystemSpecialPath
		}{"project_roots", v})
	case TmpdirFileSystemSpecialPath:
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{"tmpdir"})
	case SlashTmpFileSystemSpecialPath:
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{"slash_tmp"})
	case UnknownFileSystemSpecialPath:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			UnknownFileSystemSpecialPath
		}{"unknown", v})
	default:
		return nil, fmt.Errorf("invalid filesystem special path value %T", w.Value)
	}
}

func permissionUnionValue(value any) any {
	if isNilInterfaceValue(value) {
		return nil
	}
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Pointer {
		return reflected.Elem().Interface()
	}
	return value
}

// Unlike []string decoding, this rejects null array elements instead of turning
// them into an empty path. Null arrays remain valid optional permission lists.
type permissionPathList []string

func (p *permissionPathList) UnmarshalJSON(data []byte) error {
	var paths []*string
	if err := jsondecode.Unmarshal(data, &paths); err != nil {
		return err
	}
	var values permissionPathList
	if paths != nil {
		values = make(permissionPathList, len(paths))
	}
	for i, path := range paths {
		if path == nil {
			return fmt.Errorf("filesystem path at index %d must not be null", i)
		}
		values[i] = *path
	}
	*p = values
	return nil
}

type permissionScanDepth uint64

func (d *permissionScanDepth) UnmarshalJSON(data []byte) error {
	var value uint64
	if err := jsondecode.Unmarshal(data, &value); err != nil {
		return err
	}
	if value == 0 {
		return fmt.Errorf("globScanMaxDepth must be at least 1")
	}
	*d = permissionScanDepth(value)
	return nil
}

func (p *AdditionalFileSystemPermissions) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Entries          *[]FileSystemSandboxEntry `json:"entries"`
		GlobScanMaxDepth *permissionScanDepth      `json:"globScanMaxDepth"`
		Read             permissionPathList        `json:"read"`
		Write            permissionPathList        `json:"write"`
	}
	if err := unmarshalInboundObject(data, &decoded, nil, nil); err != nil {
		return err
	}
	value := AdditionalFileSystemPermissions{Entries: decoded.Entries, Read: decoded.Read, Write: decoded.Write}
	if decoded.GlobScanMaxDepth != nil {
		depth := uint64(*decoded.GlobScanMaxDepth)
		value.GlobScanMaxDepth = &depth
	}
	if err := value.validate(); err != nil {
		return err
	}
	*p = value
	return nil
}
func (p AdditionalFileSystemPermissions) validate() error {
	if p.GlobScanMaxDepth != nil && *p.GlobScanMaxDepth == 0 {
		return fmt.Errorf("globScanMaxDepth must be at least 1")
	}
	return nil
}
func (p AdditionalFileSystemPermissions) MarshalJSON() ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	type wire AdditionalFileSystemPermissions
	return json.Marshal(wire(p))
}
