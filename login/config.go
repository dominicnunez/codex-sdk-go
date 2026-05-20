package login

import (
	"net/http"
	"os"
	"strings"
)

const (
	DefaultAuthorizeEndpoint = "https://auth.openai.com/oauth/authorize"
	DefaultTokenEndpoint     = "https://auth.openai.com/oauth/token"
	DefaultClientID          = "app_EMoamEEZ73f0CkXaXp7hrann"
	DefaultRedirectURI       = "http://localhost:1455/auth/callback"
	DefaultScope             = "openid profile email offline_access api.connectors.read api.connectors.invoke"
	DefaultOriginator        = "codex-sdk-go"

	DefaultCallbackHost = "127.0.0.1"
	DefaultCallbackPort = 1455
	CallbackHostEnv     = "PI_OAUTH_CALLBACK_HOST"
)

const callbackPath = "/auth/callback"

type Config struct {
	AuthorizeEndpoint string
	TokenEndpoint     string
	ClientID          string
	RedirectURI       string
	Scope             string
	Originator        string
	CallbackHost      string
	CallbackPort      int
	HTTPClient        *http.Client
}

func (c Config) withDefaults() Config {
	c.AuthorizeEndpoint = firstNonEmpty(c.AuthorizeEndpoint, DefaultAuthorizeEndpoint)
	c.TokenEndpoint = firstNonEmpty(c.TokenEndpoint, DefaultTokenEndpoint)
	c.ClientID = firstNonEmpty(c.ClientID, DefaultClientID)
	c.RedirectURI = firstNonEmpty(c.RedirectURI, DefaultRedirectURI)
	c.Scope = firstNonEmpty(c.Scope, DefaultScope)
	c.Originator = firstNonEmpty(c.Originator, DefaultOriginator)
	c.CallbackHost = firstNonEmpty(c.CallbackHost, callbackHostFromEnv(), DefaultCallbackHost)
	if c.CallbackPort == 0 {
		c.CallbackPort = DefaultCallbackPort
	}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}
	return c
}

func callbackHostFromEnv() string {
	return strings.TrimSpace(os.Getenv(CallbackHostEnv))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
