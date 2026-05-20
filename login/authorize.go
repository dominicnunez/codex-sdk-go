package login

import (
	"fmt"
	"net/url"
	"strings"
)

func BuildAuthorizeURL(cfg Config, pkce PKCE, state string) (string, error) {
	cfg = cfg.withDefaults()
	if strings.TrimSpace(pkce.Challenge) == "" {
		return "", fmt.Errorf("PKCE challenge is required")
	}
	if strings.TrimSpace(state) == "" {
		return "", fmt.Errorf("state is required")
	}

	u, err := url.Parse(cfg.AuthorizeEndpoint)
	if err != nil {
		return "", fmt.Errorf("parse authorize endpoint: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("scope", cfg.Scope)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	q.Set("originator", cfg.Originator)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
