package codex

import (
	"context"
	"encoding/json"
	"errors"
)

// PluginHookSummary describes a hook bundled with a plugin.
type PluginHookSummary struct {
	EventName HookEventName `json:"eventName"`
	Key       string        `json:"key"`
}

// PluginShareDiscoverability controls who can discover a shared plugin.
type PluginShareDiscoverability string

const (
	PluginShareDiscoverabilityListed   PluginShareDiscoverability = "LISTED"
	PluginShareDiscoverabilityUnlisted PluginShareDiscoverability = "UNLISTED"
	PluginShareDiscoverabilityPrivate  PluginShareDiscoverability = "PRIVATE"
)

var validPluginShareDiscoverabilities = map[PluginShareDiscoverability]struct{}{
	PluginShareDiscoverabilityListed:   {},
	PluginShareDiscoverabilityUnlisted: {},
	PluginShareDiscoverabilityPrivate:  {},
}

func (d PluginShareDiscoverability) MarshalJSON() ([]byte, error) {
	return marshalEnumString("discoverability", d, validPluginShareDiscoverabilities)
}

func (d *PluginShareDiscoverability) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "discoverability", validPluginShareDiscoverabilities, d)
}

// PluginShareUpdateDiscoverability is the mutable subset of share visibility values.
type PluginShareUpdateDiscoverability string

const (
	PluginShareUpdateDiscoverabilityUnlisted PluginShareUpdateDiscoverability = "UNLISTED"
	PluginShareUpdateDiscoverabilityPrivate  PluginShareUpdateDiscoverability = "PRIVATE"
)

var validPluginShareUpdateDiscoverabilities = map[PluginShareUpdateDiscoverability]struct{}{
	PluginShareUpdateDiscoverabilityUnlisted: {},
	PluginShareUpdateDiscoverabilityPrivate:  {},
}

func (d PluginShareUpdateDiscoverability) MarshalJSON() ([]byte, error) {
	return marshalEnumString("discoverability", d, validPluginShareUpdateDiscoverabilities)
}

func (d *PluginShareUpdateDiscoverability) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "discoverability", validPluginShareUpdateDiscoverabilities, d)
}

// PluginSharePrincipalType identifies the type of share principal.
type PluginSharePrincipalType string

const (
	PluginSharePrincipalTypeUser      PluginSharePrincipalType = "user"
	PluginSharePrincipalTypeGroup     PluginSharePrincipalType = "group"
	PluginSharePrincipalTypeWorkspace PluginSharePrincipalType = "workspace"
)

var validPluginSharePrincipalTypes = map[PluginSharePrincipalType]struct{}{
	PluginSharePrincipalTypeUser:      {},
	PluginSharePrincipalTypeGroup:     {},
	PluginSharePrincipalTypeWorkspace: {},
}

func (t PluginSharePrincipalType) MarshalJSON() ([]byte, error) {
	return marshalEnumString("principalType", t, validPluginSharePrincipalTypes)
}

func (t *PluginSharePrincipalType) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "principalType", validPluginSharePrincipalTypes, t)
}

// PluginSharePrincipalRole identifies a principal's current share role.
type PluginSharePrincipalRole string

const (
	PluginSharePrincipalRoleReader PluginSharePrincipalRole = "reader"
	PluginSharePrincipalRoleEditor PluginSharePrincipalRole = "editor"
	PluginSharePrincipalRoleOwner  PluginSharePrincipalRole = "owner"
)

var validPluginSharePrincipalRoles = map[PluginSharePrincipalRole]struct{}{
	PluginSharePrincipalRoleReader: {},
	PluginSharePrincipalRoleEditor: {},
	PluginSharePrincipalRoleOwner:  {},
}

func (r *PluginSharePrincipalRole) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "role", validPluginSharePrincipalRoles, r)
}

// PluginShareTargetRole identifies a requested share role.
type PluginShareTargetRole string

const (
	PluginShareTargetRoleReader PluginShareTargetRole = "reader"
	PluginShareTargetRoleEditor PluginShareTargetRole = "editor"
)

var validPluginShareTargetRoles = map[PluginShareTargetRole]struct{}{
	PluginShareTargetRoleReader: {},
	PluginShareTargetRoleEditor: {},
}

func (r PluginShareTargetRole) MarshalJSON() ([]byte, error) {
	return marshalEnumString("role", r, validPluginShareTargetRoles)
}

func (r *PluginShareTargetRole) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "role", validPluginShareTargetRoles, r)
}

// PluginShareTarget is a requested share target.
type PluginShareTarget struct {
	PrincipalID   string                   `json:"principalId"`
	PrincipalType PluginSharePrincipalType `json:"principalType"`
	Role          PluginShareTargetRole    `json:"role"`
}

// PluginSharePrincipal is an existing shared principal.
type PluginSharePrincipal struct {
	Name          string                   `json:"name"`
	PrincipalID   string                   `json:"principalId"`
	PrincipalType PluginSharePrincipalType `json:"principalType"`
	Role          PluginSharePrincipalRole `json:"role"`
}

func (p *PluginSharePrincipal) UnmarshalJSON(data []byte) error {
	type pluginSharePrincipalWire struct {
		Name          *string                   `json:"name"`
		PrincipalID   *string                   `json:"principalId"`
		PrincipalType *PluginSharePrincipalType `json:"principalType"`
		Role          *PluginSharePrincipalRole `json:"role"`
	}

	var wire pluginSharePrincipalWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	switch {
	case wire.Name == nil:
		return errors.New("missing plugin.sharePrincipal.name")
	case wire.PrincipalID == nil:
		return errors.New("missing plugin.sharePrincipal.principalId")
	case wire.PrincipalType == nil:
		return errors.New("missing plugin.sharePrincipal.principalType")
	case wire.Role == nil:
		return errors.New("missing plugin.sharePrincipal.role")
	}

	p.Name = *wire.Name
	p.PrincipalID = *wire.PrincipalID
	p.PrincipalType = *wire.PrincipalType
	p.Role = *wire.Role
	return nil
}

// PluginShareContext contains remote sharing metadata for a plugin.
type PluginShareContext struct {
	CreatorAccountUserID *string                     `json:"creatorAccountUserId,omitempty"`
	CreatorName          *string                     `json:"creatorName,omitempty"`
	Discoverability      *PluginShareDiscoverability `json:"discoverability,omitempty"`
	RemotePluginID       string                      `json:"remotePluginId"`
	RemoteVersion        *string                     `json:"remoteVersion,omitempty"`
	SharePrincipals      []PluginSharePrincipal      `json:"sharePrincipals,omitempty"`
	ShareURL             *string                     `json:"shareUrl,omitempty"`
}

func (c *PluginShareContext) UnmarshalJSON(data []byte) error {
	type pluginShareContextWire struct {
		CreatorAccountUserID *string                     `json:"creatorAccountUserId"`
		CreatorName          *string                     `json:"creatorName"`
		Discoverability      *PluginShareDiscoverability `json:"discoverability"`
		RemotePluginID       *string                     `json:"remotePluginId"`
		RemoteVersion        *string                     `json:"remoteVersion"`
		SharePrincipals      []PluginSharePrincipal      `json:"sharePrincipals"`
		ShareURL             *string                     `json:"shareUrl"`
	}

	var wire pluginShareContextWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.RemotePluginID == nil {
		return errors.New("missing plugin.shareContext.remotePluginId")
	}

	c.CreatorAccountUserID = wire.CreatorAccountUserID
	c.CreatorName = wire.CreatorName
	c.Discoverability = wire.Discoverability
	c.RemotePluginID = *wire.RemotePluginID
	c.RemoteVersion = wire.RemoteVersion
	c.SharePrincipals = wire.SharePrincipals
	c.ShareURL = wire.ShareURL
	return nil
}

// PluginShareListParams lists remote shared plugins.
type PluginShareListParams struct{}

// PluginShareListItem describes a shared plugin list entry.
type PluginShareListItem struct {
	LocalPluginPath *string       `json:"localPluginPath,omitempty"`
	Plugin          PluginSummary `json:"plugin"`
}

func (i *PluginShareListItem) UnmarshalJSON(data []byte) error {
	type pluginShareListItemWire struct {
		LocalPluginPath *string        `json:"localPluginPath"`
		Plugin          *PluginSummary `json:"plugin"`
	}
	var wire pluginShareListItemWire
	if err := unmarshalResponseObject(data, &wire, []string{"plugin"}, []string{"plugin"}); err != nil {
		return err
	}
	validatedPath, err := validateInboundAbsolutePathPointerField("plugin.shareListItem.localPluginPath", wire.LocalPluginPath)
	if err != nil {
		return err
	}
	i.LocalPluginPath = validatedPath
	i.Plugin = *wire.Plugin
	return nil
}

// PluginShareListResponse contains shared plugin list entries.
type PluginShareListResponse struct {
	Data []PluginShareListItem `json:"data"`
}

func (r *PluginShareListResponse) UnmarshalJSON(data []byte) error {
	type wire PluginShareListResponse
	var decoded wire
	if err := unmarshalResponseObject(data, &decoded, []string{"data"}, []string{"data"}); err != nil {
		return err
	}
	*r = PluginShareListResponse(decoded)
	return nil
}

// PluginShareSaveParams creates or updates a remote shared plugin.
type PluginShareSaveParams struct {
	Discoverability *PluginShareDiscoverability `json:"discoverability,omitempty"`
	PluginPath      string                      `json:"pluginPath"`
	RemotePluginID  *string                     `json:"remotePluginId,omitempty"`
	ShareTargets    []PluginShareTarget         `json:"shareTargets,omitempty"`
}

// PluginShareSaveResponse contains the remote share identity.
type PluginShareSaveResponse struct {
	RemotePluginID string `json:"remotePluginId"`
	ShareURL       string `json:"shareUrl"`
}

func (r *PluginShareSaveResponse) UnmarshalJSON(data []byte) error {
	type wire PluginShareSaveResponse
	var decoded wire
	if err := unmarshalResponseObject(data, &decoded, []string{"remotePluginId", "shareUrl"}, []string{"remotePluginId", "shareUrl"}); err != nil {
		return err
	}
	*r = PluginShareSaveResponse(decoded)
	return nil
}

// PluginShareUpdateTargetsParams updates sharing targets for a remote plugin.
type PluginShareUpdateTargetsParams struct {
	Discoverability PluginShareUpdateDiscoverability `json:"discoverability"`
	RemotePluginID  string                           `json:"remotePluginId"`
	ShareTargets    []PluginShareTarget              `json:"shareTargets"`
}

// PluginShareUpdateTargetsResponse contains the updated sharing state.
type PluginShareUpdateTargetsResponse struct {
	Discoverability PluginShareDiscoverability `json:"discoverability"`
	Principals      []PluginSharePrincipal     `json:"principals"`
}

func (r *PluginShareUpdateTargetsResponse) UnmarshalJSON(data []byte) error {
	type wire PluginShareUpdateTargetsResponse
	var decoded wire
	if err := unmarshalResponseObject(data, &decoded, []string{"discoverability", "principals"}, []string{"discoverability", "principals"}); err != nil {
		return err
	}
	*r = PluginShareUpdateTargetsResponse(decoded)
	return nil
}

// PluginShareCheckoutParams checks out a remote shared plugin locally.
type PluginShareCheckoutParams struct {
	RemotePluginID string `json:"remotePluginId"`
}

// PluginShareCheckoutResponse contains the checked out plugin location.
type PluginShareCheckoutResponse struct {
	MarketplaceName string  `json:"marketplaceName"`
	MarketplacePath string  `json:"marketplacePath"`
	PluginID        string  `json:"pluginId"`
	PluginName      string  `json:"pluginName"`
	PluginPath      string  `json:"pluginPath"`
	RemotePluginID  string  `json:"remotePluginId"`
	RemoteVersion   *string `json:"remoteVersion,omitempty"`
}

func (r *PluginShareCheckoutResponse) UnmarshalJSON(data []byte) error {
	required := []string{
		"marketplaceName",
		"marketplacePath",
		"pluginId",
		"pluginName",
		"pluginPath",
		"remotePluginId",
	}
	type wire PluginShareCheckoutResponse
	var decoded wire
	if err := unmarshalResponseObject(data, &decoded, required, required); err != nil {
		return err
	}
	var err error
	decoded.MarketplacePath, err = validateInboundAbsolutePathField("plugin.shareCheckout.marketplacePath", decoded.MarketplacePath)
	if err != nil {
		return err
	}
	decoded.PluginPath, err = validateInboundAbsolutePathField("plugin.shareCheckout.pluginPath", decoded.PluginPath)
	if err != nil {
		return err
	}
	*r = PluginShareCheckoutResponse(decoded)
	return nil
}

// PluginShareDeleteParams deletes a remote shared plugin.
type PluginShareDeleteParams struct {
	RemotePluginID string `json:"remotePluginId"`
}

// PluginShareDeleteResponse is the empty response from plugin/share/delete.
type PluginShareDeleteResponse struct{}

// PluginSkillReadParams reads a remote plugin skill.
type PluginSkillReadParams struct {
	RemoteMarketplaceName string `json:"remoteMarketplaceName"`
	RemotePluginID        string `json:"remotePluginId"`
	SkillName             string `json:"skillName"`
}

// PluginSkillReadResponse contains optional skill contents.
type PluginSkillReadResponse struct {
	Contents *string `json:"contents,omitempty"`
}

// SkillRead reads a remote plugin skill.
func (s *PluginService) SkillRead(ctx context.Context, params PluginSkillReadParams) (PluginSkillReadResponse, error) {
	var resp PluginSkillReadResponse
	if err := s.client.sendRequest(ctx, methodPluginSkillRead, params, &resp); err != nil {
		return PluginSkillReadResponse{}, err
	}
	return resp, nil
}

// ShareSave creates or updates a remote shared plugin.
func (s *PluginService) ShareSave(ctx context.Context, params PluginShareSaveParams) (PluginShareSaveResponse, error) {
	var resp PluginShareSaveResponse
	if err := s.client.sendRequest(ctx, methodPluginShareSave, params, &resp); err != nil {
		return PluginShareSaveResponse{}, err
	}
	return resp, nil
}

// ShareUpdateTargets updates sharing targets for a remote plugin.
func (s *PluginService) ShareUpdateTargets(ctx context.Context, params PluginShareUpdateTargetsParams) (PluginShareUpdateTargetsResponse, error) {
	var resp PluginShareUpdateTargetsResponse
	if err := s.client.sendRequest(ctx, methodPluginShareUpdateTargets, params, &resp); err != nil {
		return PluginShareUpdateTargetsResponse{}, err
	}
	return resp, nil
}

// ShareList lists remote shared plugins.
func (s *PluginService) ShareList(ctx context.Context, params PluginShareListParams) (PluginShareListResponse, error) {
	var resp PluginShareListResponse
	if err := s.client.sendRequest(ctx, methodPluginShareList, params, &resp); err != nil {
		return PluginShareListResponse{}, err
	}
	return resp, nil
}

// ShareCheckout checks out a remote shared plugin locally.
func (s *PluginService) ShareCheckout(ctx context.Context, params PluginShareCheckoutParams) (PluginShareCheckoutResponse, error) {
	var resp PluginShareCheckoutResponse
	if err := s.client.sendRequest(ctx, methodPluginShareCheckout, params, &resp); err != nil {
		return PluginShareCheckoutResponse{}, err
	}
	return resp, nil
}

// ShareDelete deletes a remote shared plugin.
func (s *PluginService) ShareDelete(ctx context.Context, params PluginShareDeleteParams) (PluginShareDeleteResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodPluginShareDelete, params); err != nil {
		return PluginShareDeleteResponse{}, err
	}
	return PluginShareDeleteResponse{}, nil
}
