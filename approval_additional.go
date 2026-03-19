package codex

import "fmt"

// AdditionalFileSystemPermissions requests or grants extra filesystem access.
type AdditionalFileSystemPermissions struct {
	Read  []string `json:"read,omitempty"`
	Write []string `json:"write,omitempty"`
}

// AdditionalNetworkPermissions requests or grants extra network access.
type AdditionalNetworkPermissions struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// RequestPermissionProfile is the requested permission profile for an approval prompt.
type RequestPermissionProfile struct {
	FileSystem *AdditionalFileSystemPermissions `json:"fileSystem,omitempty"`
	Network    *AdditionalNetworkPermissions    `json:"network,omitempty"`
}

// PermissionsRequestApprovalParams represents an item/permissions/requestApproval request.
type PermissionsRequestApprovalParams struct {
	ItemID      string                   `json:"itemId"`
	Permissions RequestPermissionProfile `json:"permissions"`
	Reason      *string                  `json:"reason,omitempty"`
	ThreadID    string                   `json:"threadId"`
	TurnID      string                   `json:"turnId"`
}

func (p *PermissionsRequestApprovalParams) UnmarshalJSON(data []byte) error {
	type wire PermissionsRequestApprovalParams
	var decoded wire
	required := []string{"itemId", "permissions", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*p = PermissionsRequestApprovalParams(decoded)
	return nil
}

// GrantedPermissionProfile is the granted permission profile returned to the server.
type GrantedPermissionProfile struct {
	FileSystem *AdditionalFileSystemPermissions `json:"fileSystem,omitempty"`
	Network    *AdditionalNetworkPermissions    `json:"network,omitempty"`
}

// PermissionGrantScope controls how long granted permissions last.
type PermissionGrantScope string

const (
	PermissionGrantScopeTurn    PermissionGrantScope = "turn"
	PermissionGrantScopeSession PermissionGrantScope = "session"
)

// PermissionsRequestApprovalResponse represents the approval response for additional permissions.
type PermissionsRequestApprovalResponse struct {
	Permissions GrantedPermissionProfile `json:"permissions"`
	Scope       *PermissionGrantScope    `json:"scope,omitempty"`
}

func (r PermissionsRequestApprovalResponse) validate() error {
	if r.Scope == nil {
		return nil
	}
	switch *r.Scope {
	case PermissionGrantScopeTurn, PermissionGrantScopeSession:
		return nil
	default:
		return fmt.Errorf("invalid scope %q", *r.Scope)
	}
}

// McpServerElicitationMode indicates how an MCP server wants user input collected.
type McpServerElicitationMode string

const (
	McpServerElicitationModeForm McpServerElicitationMode = "form"
	McpServerElicitationModeURL  McpServerElicitationMode = "url"
)

// McpElicitationSchema is the typed form schema for an MCP elicitation request.
type McpElicitationSchema struct {
	Schema     *string                `json:"$schema,omitempty"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
	Type       string                 `json:"type"`
}

func (s *McpElicitationSchema) UnmarshalJSON(data []byte) error {
	type wire McpElicitationSchema
	var decoded wire
	required := []string{"properties", "type"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*s = McpElicitationSchema(decoded)
	return nil
}

// McpServerElicitationRequestParams represents a server request for MCP elicitation.
type McpServerElicitationRequestParams struct {
	ServerName      string                   `json:"serverName"`
	ThreadID        string                   `json:"threadId"`
	TurnID          *string                  `json:"turnId,omitempty"`
	Meta            interface{}              `json:"_meta,omitempty"`
	Message         string                   `json:"message"`
	Mode            McpServerElicitationMode `json:"mode"`
	RequestedSchema *McpElicitationSchema    `json:"requestedSchema,omitempty"`
	ElicitationID   *string                  `json:"elicitationId,omitempty"`
	URL             *string                  `json:"url,omitempty"`
}

func (p *McpServerElicitationRequestParams) UnmarshalJSON(data []byte) error {
	type wire McpServerElicitationRequestParams
	var decoded wire
	required := []string{"serverName", "threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	if err := validateMcpServerElicitationVariant(data, decoded.Mode); err != nil {
		return err
	}
	*p = McpServerElicitationRequestParams(decoded)
	return nil
}

func validateMcpServerElicitationVariant(data []byte, mode McpServerElicitationMode) error {
	switch mode {
	case McpServerElicitationModeForm:
		required := []string{"message", "mode", "requestedSchema"}
		return validateInboundObjectFields(data, required, required)
	case McpServerElicitationModeURL:
		required := []string{"elicitationId", "message", "mode", "url"}
		return validateInboundObjectFields(data, required, required)
	default:
		return fmt.Errorf("unsupported elicitation mode %q", mode)
	}
}

// McpServerElicitationAction is the client response action for an elicitation request.
type McpServerElicitationAction string

const (
	McpServerElicitationActionAccept  McpServerElicitationAction = "accept"
	McpServerElicitationActionDecline McpServerElicitationAction = "decline"
	McpServerElicitationActionCancel  McpServerElicitationAction = "cancel"
)

// McpServerElicitationRequestResponse represents the response to an MCP elicitation request.
type McpServerElicitationRequestResponse struct {
	Meta    interface{}                `json:"_meta,omitempty"`
	Action  McpServerElicitationAction `json:"action"`
	Content interface{}                `json:"content,omitempty"`
}

func (r McpServerElicitationRequestResponse) validate() error {
	switch r.Action {
	case McpServerElicitationActionAccept, McpServerElicitationActionDecline, McpServerElicitationActionCancel:
		return nil
	default:
		return fmt.Errorf("invalid action %q", r.Action)
	}
}
