package protocol

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

func (p *RequestPermissionProfile) UnmarshalJSON(data []byte) error {
	type wire RequestPermissionProfile
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, nil, nil); err != nil {
		return err
	}
	unknown := false
	jsonobject.WalkFields(data, true, func(key, _ []byte) {
		if !jsonobject.FieldMatches(key, "fileSystem") && !jsonobject.FieldMatches(key, "network") {
			unknown = true
		}
	})
	if unknown {
		return fmt.Errorf("unknown request permission profile field")
	}
	*p = RequestPermissionProfile(decoded)
	return nil
}

// Guardian actions remain generic maps for API compatibility. Validate only
// their known requestPermissions profile and retain exact scope integers there.
func decodeGuardianPermissionAction(data json.RawMessage, action any) error {
	rawType, present, rawObject := jsonobject.SelectField(data, "type")
	if !rawObject || !present {
		return nil
	}
	var actionType string
	if jsondecode.Unmarshal(rawType, &actionType) != nil || actionType != "requestPermissions" {
		return nil //nolint:nilerr // Other generic action shapes retain their existing decoding contract.
	}
	var decoded struct {
		Type        string                   `json:"type"`
		Permissions RequestPermissionProfile `json:"permissions"`
	}
	required := []string{"type", "permissions"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	filesystem := decoded.Permissions.FileSystem
	if filesystem == nil || filesystem.GlobScanMaxDepth == nil {
		return nil
	}
	// Existing generic actions use float64. This new uint field needs its exact
	// spelling, including values beyond 2^53, to preserve permission scope.
	object, ok := action.(map[string]interface{})
	if !ok {
		return fmt.Errorf("guardian permission action must be an object")
	}
	profile, ok := object["permissions"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("guardian permissions must be an object")
	}
	fs, ok := profile["fileSystem"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("guardian filesystem permissions must be an object")
	}
	fs["globScanMaxDepth"] = json.Number(strconv.FormatUint(*filesystem.GlobScanMaxDepth, 10))
	return nil
}

type guardianPermissionAction struct{ Value any }

func (a *guardianPermissionAction) UnmarshalJSON(data []byte) error {
	var value any
	if err := jsondecode.Unmarshal(data, &value); err != nil {
		return err
	}
	if err := decodeGuardianPermissionAction(data, value); err != nil {
		return err
	}
	a.Value = value
	return nil
}
