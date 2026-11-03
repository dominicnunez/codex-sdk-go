package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// Keep public records method-free. Decode the existing response first so its
// native and typed-field errors retain precedence over raw metadata admission.
type requirementsReadResult struct {
	value ConfigRequirementsReadResponse
	raw   []byte
}

func (r *requirementsReadResult) UnmarshalJSON(data []byte) error {
	if err := jsondecode.Unmarshal(data, &r.value); err != nil {
		return err
	}
	r.raw = data
	return nil
}

func (r *requirementsReadResult) validate() error {
	defer func() { r.raw = nil }()
	return validateOptionalFields(r.raw, func(_ string, raw []byte) error {
		if isNullJSONValue(raw) {
			return nil
		}
		return validateOptionalFields(raw, validateRequirement,
			"allowedPermissionProfiles", "autoReview", "browserUse", "feedback", "models", "modelProviders")
	}, "requirements")
}

func validateRequirement(name string, raw []byte) error {
	if isNullJSONValue(raw) {
		return nil
	}
	switch name {
	case "allowedPermissionProfiles":
		return requirementMap(raw, func(value []byte) error { return requirementBool(value, false) })
	case "autoReview":
		return requirementObject(raw, func(data []byte) error {
			return validateFields(data, func(_ string, value []byte) error {
				return requirementArray(value, true, func(element []byte) error { return requirementString(element, false, false) })
			}, false, "ignoreRules", "requiredOnModels")
		})
	case "browserUse":
		return requirementObject(raw, validateBrowserRequirements)
	case "feedback":
		return requirementObject(raw, func(data []byte) error {
			return validateFields(data, func(_ string, value []byte) error { return requirementBool(value, true) }, false, "enabled")
		})
	case "models":
		return requirementObject(raw, func(data []byte) error {
			return validateFields(data, func(_ string, value []byte) error {
				if isNullJSONValue(value) {
					return nil
				}
				return requirementObject(value, validateNewThreadDefaults)
			}, false, "newThread")
		})
	default: // modelProviders values are deliberately unconstrained.
		return requirementObject(raw, nil)
	}
}

func validateBrowserRequirements(data []byte) error {
	return validateFields(data, func(name string, raw []byte) error {
		if isNullJSONValue(raw) {
			return nil
		}
		switch name {
		case "defaultOriginPolicy":
			return requirementObject(raw, validateOriginPolicy)
		case "origins":
			return requirementMap(raw, func(value []byte) error { return requirementObject(value, validateOriginPolicy) })
		case "extension":
			return requirementObject(raw, func(data []byte) error {
				return validateFields(data, func(_ string, value []byte) error {
					return requirementArray(value, true, validateRequirementHeader)
				}, false, "requestHeaders")
			})
		default:
			return requirementBool(raw, false)
		}
	}, false, "allowGlobalPersistentApproval", "allowHistoryAccess", "allowWebmcp", "disableAutoReview", "defaultOriginPolicy", "origins", "extension")
}

func validateOriginPolicy(data []byte) error {
	return validateFields(data, func(name string, raw []byte) error {
		if isNullJSONValue(raw) {
			return nil
		}
		if name == "persistentApproval" {
			return requirementBool(raw, false)
		}
		allowed := []string{"allow", "deny"}
		if name == "accessApprovalLifetime" {
			allowed = []string{"turn", "thread"}
		}
		if err := requirementString(raw, false, false); err != nil {
			return err
		}
		var value string
		if err := jsondecode.Unmarshal(raw, &value); err != nil {
			return err
		}
		for _, candidate := range allowed {
			if value == candidate {
				return nil
			}
		}
		return fmt.Errorf("invalid origin policy enum")
	}, false, "access", "accessApprovalLifetime", "autoReview", "downloads", "fullCdpAccess", "persistentApproval", "uploads")
}

func validateNewThreadDefaults(data []byte) error {
	return validateFields(data, func(name string, raw []byte) error {
		return requirementString(raw, true, name == "modelReasoningEffort")
	}, false, "model", "modelReasoningEffort", "serviceTier")
}

func validateRequirementHeader(raw []byte) error {
	return requirementObject(raw, func(data []byte) error {
		if err := validateRequiredObjectFields(data, "name", "value"); err != nil {
			return err
		}
		return validateFields(data, func(_ string, value []byte) error { return requirementString(value, false, false) }, false, "name", "value")
	})
}

// Inputs are borrowed spans of a response that has already passed JSON syntax
// validation. These checks inspect every occurrence without re-encoding values,
// interpreting origin/provider keys, or constraining open extension properties.
func requirementObject(raw []byte, validate func([]byte) error) error {
	if isNullJSONValue(raw) {
		return ErrNullResultField
	}
	if len(raw) == 0 || raw[0] != '{' {
		return fmt.Errorf("must be an object")
	}
	if validate != nil {
		return validate(raw)
	}
	return nil
}

func requirementMap(raw []byte, validate func([]byte) error) error {
	return requirementObject(raw, func(data []byte) error {
		var err error
		jsonobject.WalkFields(data, true, func(_ []byte, value []byte) {
			if err == nil {
				err = validate(value)
			}
		})
		return err
	})
}

func requirementArray(raw []byte, nullable bool, validate func([]byte) error) error {
	if isNullJSONValue(raw) {
		if nullable {
			return nil
		}
		return ErrNullResultField
	}
	if len(raw) == 0 || raw[0] != '[' {
		return fmt.Errorf("must be an array")
	}
	return validateArrayElements(raw, func(index int, element []byte) error {
		if err := validate(element); err != nil {
			return fmt.Errorf("element %d: %w", index, err)
		}
		return nil
	})
}

func requirementBool(raw []byte, nullable bool) error {
	if isNullJSONValue(raw) {
		if nullable {
			return nil
		}
		return ErrNullResultField
	}
	if string(raw) != "true" && string(raw) != "false" {
		return fmt.Errorf("must be a boolean")
	}
	return nil
}

func requirementString(raw []byte, nullable, nonempty bool) error {
	if isNullJSONValue(raw) {
		if nullable {
			return nil
		}
		return ErrNullResultField
	}
	if len(raw) == 0 || raw[0] != '"' {
		return fmt.Errorf("must be a string")
	}
	if nonempty && len(raw) == 2 {
		return fmt.Errorf("must be a non-empty string")
	}
	return nil
}
