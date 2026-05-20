package login

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dominicnunez/codex-sdk-go/login/auth"
)

const maxTokenResponseBytes int64 = 1 << 20
const maxErrorBodyBytes int64 = 4096

const (
	grantTypeAuthorizationCode = "authorization_code"
	grantTypeRefreshToken      = "refresh_token"
)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type refreshRequest struct {
	ClientID     string `json:"client_id"`
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

func ExchangeCode(ctx context.Context, cfg Config, code string, verifier string) (auth.Credentials, error) {
	cfg = cfg.withDefaults()
	form := url.Values{}
	form.Set("grant_type", grantTypeAuthorizationCode)
	form.Set("client_id", cfg.ClientID)
	form.Set("code", strings.TrimSpace(code))
	form.Set("code_verifier", strings.TrimSpace(verifier))
	form.Set("redirect_uri", cfg.RedirectURI)
	return postTokenForm(ctx, cfg, form, "exchange")
}

func Refresh(ctx context.Context, cfg Config, refreshToken string) (auth.Credentials, error) {
	cfg = cfg.withDefaults()
	payload, err := json.Marshal(refreshRequest{
		ClientID:     cfg.ClientID,
		GrantType:    grantTypeRefreshToken,
		RefreshToken: strings.TrimSpace(refreshToken),
	})
	if err != nil {
		return auth.Credentials{}, fmt.Errorf("encode OpenAI Codex token refresh request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenEndpoint, bytes.NewReader(payload))
	if err != nil {
		return auth.Credentials{}, fmt.Errorf("create OpenAI Codex token refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return doTokenRequest(cfg, req, "refresh")
}

func postTokenForm(ctx context.Context, cfg Config, form url.Values, operation string) (auth.Credentials, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return auth.Credentials{}, fmt.Errorf("create OpenAI Codex token %s request: %w", operation, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	return doTokenRequest(cfg, req, operation)
}

func doTokenRequest(cfg Config, req *http.Request, operation string) (auth.Credentials, error) {
	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return auth.Credentials{}, fmt.Errorf("OpenAI Codex token %s error: %w", operation, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return auth.Credentials{}, fmt.Errorf("OpenAI Codex token %s failed (%d): %s", operation, resp.StatusCode, message)
	}

	var decoded tokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxTokenResponseBytes)).Decode(&decoded); err != nil {
		return auth.Credentials{}, fmt.Errorf("decode OpenAI Codex token %s response: %w", operation, err)
	}
	if strings.TrimSpace(decoded.AccessToken) == "" || strings.TrimSpace(decoded.RefreshToken) == "" || decoded.ExpiresIn <= 0 {
		return auth.Credentials{}, fmt.Errorf("%w: %+v", auth.ErrMissingTokenFields, decoded)
	}

	claims, err := auth.ExtractTokenClaims(decoded.AccessToken)
	if err != nil {
		return auth.Credentials{}, err
	}
	return auth.Credentials{
		AccessToken:  decoded.AccessToken,
		RefreshToken: decoded.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(decoded.ExpiresIn) * time.Second),
		AccountID:    claims.AccountID,
		PlanType:     claims.PlanType,
	}, nil
}
