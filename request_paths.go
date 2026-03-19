package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	pathpkg "path"
	"reflect"
	"strings"
)

type requestPreparer interface {
	prepareRequest() (interface{}, error)
}

func prepareRequestParams(params interface{}) (interface{}, error) {
	preparer, ok := params.(requestPreparer)
	if !ok {
		return params, nil
	}
	return preparer.prepareRequest()
}

func isNilInterfaceValue(value interface{}) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func validateRequiredJSONObjectField(field string, value interface{}) error {
	if isNilInterfaceValue(value) {
		return invalidParamsError("%s must not be null", field)
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	encoded = bytes.TrimSpace(encoded)
	if bytes.Equal(encoded, []byte("null")) {
		return invalidParamsError("%s must not be null", field)
	}
	if len(encoded) == 0 || encoded[0] != '{' {
		return invalidParamsError("%s must be a JSON object", field)
	}
	return nil
}

func normalizeAbsolutePathField(field, value string) (string, error) {
	normalized, err := normalizeAbsolutePath(value)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", errInvalidParams, field, err)
	}
	return normalized, nil
}

func normalizeAbsolutePathSliceField(field string, values []string) ([]string, error) {
	normalized := make([]string, len(values))
	for i, value := range values {
		path, err := normalizeAbsolutePathField(fmt.Sprintf("%s[%d]", field, i), value)
		if err != nil {
			return nil, err
		}
		normalized[i] = path
	}
	return normalized, nil
}

func validateInboundAbsolutePathField(field, value string) (string, error) {
	normalized, err := normalizeAbsolutePath(value)
	if err != nil {
		return "", fmt.Errorf("%s: %w", field, err)
	}
	if normalized != value {
		return "", fmt.Errorf("%s: must be normalized, got %q", field, value)
	}
	return normalized, nil
}

func validateInboundAbsolutePathPointerField(field string, value *string) (*string, error) {
	if value == nil {
		return value, nil
	}
	validated, err := validateInboundAbsolutePathField(field, *value)
	if err != nil {
		return nil, err
	}
	return &validated, nil
}

func validateInboundAbsolutePathSliceField(field string, values []string) ([]string, error) {
	validated := make([]string, len(values))
	for i, value := range values {
		path, err := validateInboundAbsolutePathField(fmt.Sprintf("%s[%d]", field, i), value)
		if err != nil {
			return nil, err
		}
		validated[i] = path
	}
	return validated, nil
}

func normalizeAdditionalFileSystemPermissionsField(
	field string,
	value *AdditionalFileSystemPermissions,
) (*AdditionalFileSystemPermissions, error) {
	if value == nil {
		return value, nil
	}

	normalized := *value
	var err error
	normalized.Read, err = normalizeAbsolutePathSliceField(field+".read", value.Read)
	if err != nil {
		return nil, err
	}
	normalized.Write, err = normalizeAbsolutePathSliceField(field+".write", value.Write)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func normalizeRequestPermissionProfileField(
	field string,
	value RequestPermissionProfile,
) (RequestPermissionProfile, error) {
	var err error
	value.FileSystem, err = normalizeAdditionalFileSystemPermissionsField(field+".fileSystem", value.FileSystem)
	if err != nil {
		return RequestPermissionProfile{}, err
	}
	return value, nil
}

func normalizeGrantedPermissionProfileField(
	field string,
	value GrantedPermissionProfile,
) (GrantedPermissionProfile, error) {
	var err error
	value.FileSystem, err = normalizeAdditionalFileSystemPermissionsField(field+".fileSystem", value.FileSystem)
	if err != nil {
		return GrantedPermissionProfile{}, err
	}
	return value, nil
}

func normalizeAbsolutePath(value string) (string, error) {
	switch {
	case value == "":
		return "", fmt.Errorf("must be an absolute path, got empty string")
	case isWindowsExtendedUNCPath(value):
		return normalizeWindowsExtendedUNCPath(value)
	case isWindowsExtendedAbsolutePath(value):
		return normalizeWindowsExtendedAbsolutePath(value)
	case isWindowsDriveAbsolutePath(value):
		return normalizeWindowsDriveAbsolutePath(value)
	case isWindowsUNCPath(value):
		return normalizeWindowsUNCPath(value)
	case strings.HasPrefix(value, "/"):
		return pathpkg.Clean(value), nil
	default:
		return "", fmt.Errorf("must be an absolute path: %q", value)
	}
}

func isWindowsDriveAbsolutePath(value string) bool {
	return len(value) >= 3 &&
		isASCIILetter(value[0]) &&
		value[1] == ':' &&
		isWindowsPathSeparator(value[2])
}

func isWindowsUNCPath(value string) bool {
	if !strings.HasPrefix(value, `\\`) {
		return false
	}
	prefix, _, ok := splitWindowsUNCPath(value)
	return ok && prefix != ""
}

func isWindowsExtendedAbsolutePath(value string) bool {
	if strings.HasPrefix(value, `\\?\`) {
		_, _, ok := splitWindowsExtendedAbsolutePath(value[4:])
		return ok
	}
	if strings.HasPrefix(value, `//?/`) {
		_, _, ok := splitWindowsExtendedAbsolutePath(value[4:])
		return ok
	}
	return false
}

func isWindowsExtendedUNCPath(value string) bool {
	if strings.HasPrefix(value, `\\?\UNC\`) {
		_, _, ok := splitWindowsUNCPath(`\\` + value[8:])
		return ok
	}
	if strings.HasPrefix(value, `//?/UNC/`) {
		_, _, ok := splitWindowsUNCPath(`//` + value[8:])
		return ok
	}
	return false
}

func normalizeWindowsDriveAbsolutePath(value string) (string, error) {
	return normalizeWindowsPath(value[:2], value[2:], true), nil
}

func normalizeWindowsUNCPath(value string) (string, error) {
	prefix, rest, ok := splitWindowsUNCPath(value)
	if !ok {
		return "", fmt.Errorf("must be an absolute path: %q", value)
	}
	return normalizeWindowsPath(prefix, rest, false), nil
}

func normalizeWindowsExtendedAbsolutePath(value string) (string, error) {
	prefix, rest, ok := splitWindowsExtendedAbsolutePath(value[4:])
	if !ok {
		return "", fmt.Errorf("must be an absolute path: %q", value)
	}
	return normalizeWindowsPath(`\\?\`+prefix, rest, true), nil
}

func normalizeWindowsExtendedUNCPath(value string) (string, error) {
	prefix, rest, ok := splitWindowsUNCPath(`\\` + value[8:])
	if !ok {
		return "", fmt.Errorf("must be an absolute path: %q", value)
	}
	return normalizeWindowsPath(`\\?\UNC`+prefix[1:], rest, false), nil
}

func splitWindowsUNCPath(value string) (string, string, bool) {
	if len(value) < 5 || !isWindowsPathSeparator(value[0]) || !isWindowsPathSeparator(value[1]) {
		return "", "", false
	}
	normalized := strings.ReplaceAll(value[2:], "/", `\`)
	parts := strings.SplitN(normalized, `\`, 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	prefix := `\\` + parts[0] + `\` + parts[1]
	if len(parts) == 2 {
		return prefix, "", true
	}
	return prefix, `\` + parts[2], true
}

func splitWindowsExtendedAbsolutePath(value string) (string, string, bool) {
	normalized := strings.ReplaceAll(value, "/", `\`)
	parts := strings.SplitN(normalized, `\`, 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], `\` + parts[1], true
}

func normalizeWindowsPath(prefix, rest string, rootNeedsSeparator bool) string {
	cleaned := pathpkg.Clean(strings.ReplaceAll(rest, `\`, "/"))
	if cleaned == "/" {
		if rootNeedsSeparator {
			return prefix + `\`
		}
		return prefix
	}
	return prefix + strings.ReplaceAll(cleaned, "/", `\`)
}

func isASCIILetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isWindowsPathSeparator(value byte) bool {
	return value == '\\' || value == '/'
}

func normalizeReadOnlyAccessWrapperField(field string, value *ReadOnlyAccessWrapper) (*ReadOnlyAccessWrapper, error) {
	if value == nil {
		return value, nil
	}
	normalized, err := normalizeReadOnlyAccessField(field, value.Value)
	if err != nil {
		return nil, err
	}
	return &ReadOnlyAccessWrapper{Value: normalized}, nil
}

func normalizeReadOnlyAccessField(field string, value ReadOnlyAccess) (ReadOnlyAccess, error) {
	switch v := normalizeReadOnlyAccess(value).(type) {
	case nil:
		return value, nil
	case ReadOnlyAccessRestricted:
		var err error
		v.ReadableRoots, err = normalizeAbsolutePathSliceField(field+".readableRoots", v.ReadableRoots)
		if err != nil {
			return nil, err
		}
		return v, nil
	default:
		return v, nil
	}
}

func validateInboundReadOnlyAccessWrapperField(field string, value *ReadOnlyAccessWrapper) (*ReadOnlyAccessWrapper, error) {
	if value == nil {
		return value, nil
	}
	validated, err := validateInboundReadOnlyAccessField(field, value.Value)
	if err != nil {
		return nil, err
	}
	return &ReadOnlyAccessWrapper{Value: validated}, nil
}

func validateInboundReadOnlyAccessField(field string, value ReadOnlyAccess) (ReadOnlyAccess, error) {
	switch v := normalizeReadOnlyAccess(value).(type) {
	case nil:
		return value, nil
	case ReadOnlyAccessRestricted:
		var err error
		v.ReadableRoots, err = validateInboundAbsolutePathSliceField(field+".readableRoots", v.ReadableRoots)
		if err != nil {
			return nil, err
		}
		return v, nil
	default:
		return v, nil
	}
}

func normalizeSandboxPolicyWrapperField(field string, value *SandboxPolicyWrapper) (*SandboxPolicyWrapper, error) {
	if value == nil {
		return value, nil
	}
	normalized, err := normalizeSandboxPolicyField(field, value.Value)
	if err != nil {
		return nil, err
	}
	return &SandboxPolicyWrapper{Value: normalized}, nil
}

func normalizeSandboxPolicyPointerField(field string, value *SandboxPolicy) (*SandboxPolicy, error) {
	if value == nil {
		return value, nil
	}
	normalized, err := normalizeSandboxPolicyField(field, *value)
	if err != nil {
		return nil, err
	}
	policy := normalized
	return &policy, nil
}

func normalizeSandboxPolicyField(field string, value SandboxPolicy) (SandboxPolicy, error) {
	switch v := normalizeSandboxPolicy(value).(type) {
	case nil:
		return value, nil
	case SandboxPolicyReadOnly:
		var err error
		v.Access, err = normalizeReadOnlyAccessWrapperField(field+".access", v.Access)
		if err != nil {
			return nil, err
		}
		return v, nil
	case SandboxPolicyWorkspaceWrite:
		var err error
		v.ReadOnlyAccess, err = normalizeReadOnlyAccessWrapperField(field+".readOnlyAccess", v.ReadOnlyAccess)
		if err != nil {
			return nil, err
		}
		v.WritableRoots, err = normalizeAbsolutePathSliceField(field+".writableRoots", v.WritableRoots)
		if err != nil {
			return nil, err
		}
		return v, nil
	default:
		return v, nil
	}
}

func validateInboundSandboxPolicyField(field string, value SandboxPolicy) (SandboxPolicy, error) {
	switch v := normalizeSandboxPolicy(value).(type) {
	case nil:
		return value, nil
	case SandboxPolicyReadOnly:
		var err error
		v.Access, err = validateInboundReadOnlyAccessWrapperField(field+".access", v.Access)
		if err != nil {
			return nil, err
		}
		return v, nil
	case SandboxPolicyWorkspaceWrite:
		var err error
		v.ReadOnlyAccess, err = validateInboundReadOnlyAccessWrapperField(field+".readOnlyAccess", v.ReadOnlyAccess)
		if err != nil {
			return nil, err
		}
		v.WritableRoots, err = validateInboundAbsolutePathSliceField(field+".writableRoots", v.WritableRoots)
		if err != nil {
			return nil, err
		}
		return v, nil
	default:
		return v, nil
	}
}

func (p FsReadFileParams) prepareRequest() (interface{}, error) {
	var err error
	p.Path, err = normalizeAbsolutePathField("path", p.Path)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p FsWriteFileParams) prepareRequest() (interface{}, error) {
	var err error
	p.Path, err = normalizeAbsolutePathField("path", p.Path)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p FsCreateDirectoryParams) prepareRequest() (interface{}, error) {
	var err error
	p.Path, err = normalizeAbsolutePathField("path", p.Path)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p FsGetMetadataParams) prepareRequest() (interface{}, error) {
	var err error
	p.Path, err = normalizeAbsolutePathField("path", p.Path)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p FsReadDirectoryParams) prepareRequest() (interface{}, error) {
	var err error
	p.Path, err = normalizeAbsolutePathField("path", p.Path)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p FsRemoveParams) prepareRequest() (interface{}, error) {
	var err error
	p.Path, err = normalizeAbsolutePathField("path", p.Path)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p FsCopyParams) prepareRequest() (interface{}, error) {
	var err error
	p.SourcePath, err = normalizeAbsolutePathField("sourcePath", p.SourcePath)
	if err != nil {
		return nil, err
	}
	p.DestinationPath, err = normalizeAbsolutePathField("destinationPath", p.DestinationPath)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p PluginListParams) prepareRequest() (interface{}, error) {
	var err error
	p.Cwds, err = normalizeAbsolutePathSliceField("cwds", p.Cwds)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p PluginReadParams) prepareRequest() (interface{}, error) {
	var err error
	p.MarketplacePath, err = normalizeAbsolutePathField("marketplacePath", p.MarketplacePath)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p PluginInstallParams) prepareRequest() (interface{}, error) {
	var err error
	p.MarketplacePath, err = normalizeAbsolutePathField("marketplacePath", p.MarketplacePath)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p CommandExecParams) prepareRequest() (interface{}, error) {
	if err := validateCommandExecParams(p); err != nil {
		return nil, err
	}

	var err error
	p.SandboxPolicy, err = normalizeSandboxPolicyWrapperField("sandboxPolicy", p.SandboxPolicy)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p CommandExecWriteParams) prepareRequest() (interface{}, error) {
	if err := validateRequiredNonEmptyStringField("processId", p.ProcessID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p CommandExecTerminateParams) prepareRequest() (interface{}, error) {
	if err := validateRequiredNonEmptyStringField("processId", p.ProcessID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p CommandExecResizeParams) prepareRequest() (interface{}, error) {
	if err := validateRequiredNonEmptyStringField("processId", p.ProcessID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p TurnStartParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	if p.Input == nil {
		return nil, invalidParamsError("input must not be null")
	}
	for i, input := range p.Input {
		if isNilInterfaceValue(input) {
			return nil, invalidParamsError("input[%d] must not be null", i)
		}
	}

	var err error
	p.SandboxPolicy, err = normalizeSandboxPolicyPointerField("sandboxPolicy", p.SandboxPolicy)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p TurnInterruptParams) prepareRequest() (interface{}, error) {
	if err := validateTurnScopedRequest(p.ThreadID, p.TurnID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p TurnSteerParams) prepareRequest() (interface{}, error) {
	if err := validateExpectedTurnScopedRequest(p.ThreadID, p.ExpectedTurnID); err != nil {
		return nil, err
	}
	if p.Input == nil {
		return nil, invalidParamsError("input must not be null")
	}
	for i, input := range p.Input {
		if isNilInterfaceValue(input) {
			return nil, invalidParamsError("input[%d] must not be null", i)
		}
	}
	return p, nil
}

func (p ThreadReadParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p ThreadResumeParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p ThreadForkParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p ThreadRollbackParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p ThreadSetNameParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p ThreadMetadataUpdateParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p ThreadArchiveParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p ThreadUnarchiveParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p ThreadUnsubscribeParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p ThreadCompactStartParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

func (p WindowsSandboxSetupStartParams) prepareRequest() (interface{}, error) {
	if p.Cwd != nil {
		normalized, err := normalizeAbsolutePathField("cwd", *p.Cwd)
		if err != nil {
			return nil, err
		}
		p.Cwd = &normalized
	}
	return p, nil
}

func validateCommandExecParams(params CommandExecParams) error {
	if len(params.Command) == 0 {
		return invalidParamsError("command array must not be empty")
	}
	if err := validateOptionalNonEmptyStringField("processId", params.ProcessID); err != nil {
		return err
	}
	if boolPointerValue(params.DisableOutputCap) && params.OutputBytesCap != nil {
		return invalidParamsError("disableOutputCap cannot be combined with outputBytesCap")
	}
	if boolPointerValue(params.DisableTimeout) && params.TimeoutMs != nil {
		return invalidParamsError("disableTimeout cannot be combined with timeoutMs")
	}

	ttyEnabled := boolPointerValue(params.TTY)
	if params.Size != nil && !ttyEnabled {
		return invalidParamsError("size requires tty")
	}
	if ttyEnabled || boolPointerValue(params.StreamStdin) || boolPointerValue(params.StreamStdoutStderr) {
		if params.ProcessID == nil {
			return invalidParamsError("processId is required when tty, streamStdin, or streamStdoutStderr is enabled")
		}
	}

	return nil
}

func validateOptionalNonEmptyStringField(field string, value *string) error {
	if value == nil {
		return nil
	}
	return validateRequiredNonEmptyStringField(field, *value)
}

func validateThreadScopedRequest(threadID string) error {
	return validateRequiredNonEmptyStringField("threadId", threadID)
}

func validateTurnScopedRequest(threadID, turnID string) error {
	if err := validateThreadScopedRequest(threadID); err != nil {
		return err
	}
	return validateRequiredNonEmptyStringField("turnId", turnID)
}

func validateExpectedTurnScopedRequest(threadID, expectedTurnID string) error {
	if err := validateThreadScopedRequest(threadID); err != nil {
		return err
	}
	return validateRequiredNonEmptyStringField("expectedTurnId", expectedTurnID)
}

func validateRequiredNonEmptyStringField(field, value string) error {
	if value == "" {
		return invalidParamsError("%s must not be empty", field)
	}
	return nil
}

func boolPointerValue(value *bool) bool {
	return value != nil && *value
}

func invalidParamsError(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", errInvalidParams, fmt.Sprintf(format, args...))
}
