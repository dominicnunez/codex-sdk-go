package codex

import (
	"context"
	"encoding/json"
)

// MarketplaceAddParams adds a marketplace source.
type MarketplaceAddParams struct {
	RefName     *string  `json:"refName,omitempty"`
	Source      string   `json:"source"`
	SparsePaths []string `json:"sparsePaths,omitempty"`
}

// MarketplaceAddResponse describes the installed marketplace.
type MarketplaceAddResponse struct {
	AlreadyAdded    bool   `json:"alreadyAdded"`
	InstalledRoot   string `json:"installedRoot"`
	MarketplaceName string `json:"marketplaceName"`
}

func (r *MarketplaceAddResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "alreadyAdded", "installedRoot", "marketplaceName"); err != nil {
		return err
	}
	type wire MarketplaceAddResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedInstalledRoot, err := validateInboundAbsolutePathField("marketplace.add.installedRoot", decoded.InstalledRoot)
	if err != nil {
		return err
	}
	decoded.InstalledRoot = validatedInstalledRoot
	*r = MarketplaceAddResponse(decoded)
	return nil
}

// MarketplaceRemoveParams removes an installed marketplace by name.
type MarketplaceRemoveParams struct {
	MarketplaceName string `json:"marketplaceName"`
}

// MarketplaceRemoveResponse describes the removed marketplace.
type MarketplaceRemoveResponse struct {
	InstalledRoot   *string `json:"installedRoot,omitempty"`
	MarketplaceName string  `json:"marketplaceName"`
}

func (r *MarketplaceRemoveResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "marketplaceName"); err != nil {
		return err
	}
	type wire MarketplaceRemoveResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedInstalledRoot, err := validateInboundAbsolutePathPointerField("marketplace.remove.installedRoot", decoded.InstalledRoot)
	if err != nil {
		return err
	}
	decoded.InstalledRoot = validatedInstalledRoot
	*r = MarketplaceRemoveResponse(decoded)
	return nil
}

// MarketplaceUpgradeParams upgrades marketplaces, optionally scoped to one name.
type MarketplaceUpgradeParams struct {
	MarketplaceName *string `json:"marketplaceName,omitempty"`
}

// MarketplaceUpgradeErrorInfo describes a marketplace upgrade failure.
type MarketplaceUpgradeErrorInfo struct {
	MarketplaceName string `json:"marketplaceName"`
	Message         string `json:"message"`
}

// MarketplaceUpgradeResponse describes upgraded marketplaces and failures.
type MarketplaceUpgradeResponse struct {
	Errors               []MarketplaceUpgradeErrorInfo `json:"errors"`
	SelectedMarketplaces []string                      `json:"selectedMarketplaces"`
	UpgradedRoots        []string                      `json:"upgradedRoots"`
}

func (r *MarketplaceUpgradeResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "errors", "selectedMarketplaces", "upgradedRoots"); err != nil {
		return err
	}
	type wire MarketplaceUpgradeResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedUpgradedRoots, err := validateInboundAbsolutePathSliceField("marketplace.upgrade.upgradedRoots", decoded.UpgradedRoots)
	if err != nil {
		return err
	}
	decoded.UpgradedRoots = validatedUpgradedRoots
	*r = MarketplaceUpgradeResponse(decoded)
	return nil
}

// MarketplaceService provides marketplace operations.
type MarketplaceService struct {
	client *Client
}

func newMarketplaceService(client *Client) *MarketplaceService {
	return &MarketplaceService{client: client}
}

// Add adds a marketplace source.
func (s *MarketplaceService) Add(ctx context.Context, params MarketplaceAddParams) (MarketplaceAddResponse, error) {
	var resp MarketplaceAddResponse
	if err := s.client.sendRequest(ctx, methodMarketplaceAdd, params, &resp); err != nil {
		return MarketplaceAddResponse{}, err
	}
	return resp, nil
}

// Remove removes an installed marketplace.
func (s *MarketplaceService) Remove(ctx context.Context, params MarketplaceRemoveParams) (MarketplaceRemoveResponse, error) {
	var resp MarketplaceRemoveResponse
	if err := s.client.sendRequest(ctx, methodMarketplaceRemove, params, &resp); err != nil {
		return MarketplaceRemoveResponse{}, err
	}
	return resp, nil
}

// Upgrade upgrades marketplaces.
func (s *MarketplaceService) Upgrade(ctx context.Context, params MarketplaceUpgradeParams) (MarketplaceUpgradeResponse, error) {
	var resp MarketplaceUpgradeResponse
	if err := s.client.sendRequest(ctx, methodMarketplaceUpgrade, params, &resp); err != nil {
		return MarketplaceUpgradeResponse{}, err
	}
	return resp, nil
}
