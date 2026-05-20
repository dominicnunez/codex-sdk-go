package login

import (
	"context"
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/login/auth"
)

const manualCodePrompt = "Paste the authorization code (or full redirect URL):"

type LoginOptions struct {
	Config     Config
	OnAuthURL  func(context.Context, string) error
	ManualCode func(context.Context, AuthPrompt) (string, error)
}

type AuthPrompt struct {
	Message string
	AuthURL string
}

func Login(ctx context.Context, opts LoginOptions) (auth.Credentials, error) {
	pkce, err := GeneratePKCE()
	if err != nil {
		return auth.Credentials{}, fmt.Errorf("generate PKCE: %w", err)
	}
	state, err := GenerateState()
	if err != nil {
		return auth.Credentials{}, fmt.Errorf("generate OAuth state: %w", err)
	}
	authURL, err := BuildAuthorizeURL(opts.Config, pkce, state)
	if err != nil {
		return auth.Credentials{}, err
	}

	callbackServer, callbackErr := StartCallbackServer(ctx, opts.Config, state)
	if callbackErr == nil {
		defer callbackServer.Close()
	}

	if opts.OnAuthURL != nil {
		if err := opts.OnAuthURL(ctx, authURL); err != nil {
			return auth.Credentials{}, fmt.Errorf("handle OAuth authorization URL: %w", err)
		}
	}

	code, err := waitForAuthorizationCode(ctx, callbackServer, callbackErr, opts.ManualCode, authURL, state)
	if err != nil {
		return auth.Credentials{}, err
	}
	return ExchangeCode(ctx, opts.Config, code.Code, pkce.Verifier)
}

func waitForAuthorizationCode(
	ctx context.Context,
	callbackServer *CallbackServer,
	callbackErr error,
	manualCode func(context.Context, AuthPrompt) (string, error),
	authURL string,
	state string,
) (AuthorizationCode, error) {
	results := make(chan callbackResult, 2)
	if callbackServer != nil {
		go func() {
			code, err := callbackServer.Wait(ctx)
			results <- callbackResult{code: code, err: err}
		}()
	}
	if manualCode != nil {
		go func() {
			input, err := manualCode(ctx, AuthPrompt{Message: manualCodePrompt, AuthURL: authURL})
			if err != nil {
				results <- callbackResult{err: err}
				return
			}
			code, err := ParseAuthorizationInput(input, state)
			results <- callbackResult{code: code, err: err}
		}()
	}

	if callbackServer == nil && manualCode == nil {
		if callbackErr != nil {
			return AuthorizationCode{}, callbackErr
		}
		return AuthorizationCode{}, ErrMissingAuthorizationCode
	}

	select {
	case result := <-results:
		return result.code, result.err
	case <-ctx.Done():
		return AuthorizationCode{}, ctx.Err()
	}
}
