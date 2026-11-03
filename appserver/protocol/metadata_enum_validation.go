package protocol

var validOnboardingEntrypoints = map[string]struct{}{"life_sciences": {}}
var validMcpStartupFailureReasons = map[string]struct{}{"reauthenticationRequired": {}}
var validCredentialStoreModes = map[string]struct{}{
	"file": {}, "keyring": {}, "auto": {}, "ephemeral": {},
}
var validWindowsSandboxImpls = map[string]struct{}{
	"elevated": {}, "unelevated": {}, "mxc": {},
}

func validateRequirementEnum(name string, raw []byte) error {
	if isNullJSONValue(raw) {
		return nil
	}
	switch name {
	case "cliAuthCredentialsStore":
		var value string
		return unmarshalEnumString(raw, name, validCredentialStoreModes, &value)
	case "allowedLoginMethods":
		return validateRequirementEnumArray(raw, name, validForcedLoginMethods)
	default: // allowedWindowsSandboxImplementations
		return validateRequirementEnumArray(raw, name, validWindowsSandboxImpls)
	}
}

func validateRequirementEnumArray[T ~string](raw []byte, field string, allowed map[T]struct{}) error {
	return requirementArray(raw, true, func(element []byte) error {
		if isNullJSONValue(element) {
			return ErrNullResultField
		}
		var value T
		return unmarshalEnumString(element, field, allowed, &value)
	})
}
