package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

const openAIAuthClaim = "https://api.openai.com/auth"

type TokenClaims struct {
	AccountID string
	PlanType  *string
}

func ExtractTokenClaims(accessToken string) (TokenClaims, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) < 2 {
		return TokenClaims{}, fmt.Errorf("decode access token claims: JWT payload missing")
	}

	payload, err := decodeJWTPayload(parts[1])
	if err != nil {
		return TokenClaims{}, fmt.Errorf("decode access token claims: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return TokenClaims{}, fmt.Errorf("parse access token claims: %w", err)
	}

	authClaims, _ := claims[openAIAuthClaim].(map[string]any)
	accountID, _ := authClaims["chatgpt_account_id"].(string)
	if strings.TrimSpace(accountID) == "" {
		return TokenClaims{}, ErrMissingAccountID
	}

	planType := firstClaimString(authClaims, "chatgpt_plan_type", "plan_type")
	if planType == nil {
		planType = firstClaimString(claims, "chatgpt_plan_type", "plan_type")
	}
	return TokenClaims{AccountID: strings.TrimSpace(accountID), PlanType: planType}, nil
}

func decodeJWTPayload(part string) ([]byte, error) {
	if decoded, err := base64.RawURLEncoding.DecodeString(part); err == nil {
		return decoded, nil
	}
	decoded, err := base64.URLEncoding.DecodeString(part)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func firstClaimString(claims map[string]any, keys ...string) *string {
	for _, key := range keys {
		value, _ := claims[key].(string)
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return &trimmed
		}
	}
	return nil
}
