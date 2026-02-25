package cmd

import (
	"fmt"
	"net/http"

	authadapter "github.com/bnema/openai-accounts-cli/internal/adapters/auth"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

func newLoginCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Start account login flows",
	}

	cmd.AddCommand(newLoginBrowserCmd(app), newLoginDeviceCmd(app))

	return cmd
}

func newLoginBrowserCmd(app *app) *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "browser",
		Short: "Start browser login flow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolvedAccountID, err := resolveAccountID(cmd.Context(), app, accountID)
			if err != nil {
				return err
			}
			return runBrowserLogin(cmd, app, resolvedAccountID)
		},
	}

	cmd.Flags().StringVar(&accountID, "account", "0", "Account ID (0 or empty auto-assigns next: 1,2,...)")

	return cmd
}

func newLoginDeviceCmd(app *app) *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "device",
		Short: "Start device login flow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolvedAccountID, err := resolveAccountID(cmd.Context(), app, accountID)
			if err != nil {
				return err
			}
			return fmt.Errorf("%s for account %s: %w", cmd.CommandPath(), resolvedAccountID, errNotImplementedYet)
		},
	}

	cmd.Flags().StringVar(&accountID, "account", "0", "Account ID (0 or empty auto-assigns next: 1,2,...)")

	return cmd
}

func runBrowserLogin(cmd *cobra.Command, app *app, accountID domain.AccountID) error {
	pkce, err := authadapter.NewPKCEPair()
	if err != nil {
		return fmt.Errorf("generate pkce: %w", err)
	}
	state, err := authadapter.NewState()
	if err != nil {
		return fmt.Errorf("generate oauth state: %w", err)
	}

	server, err := authadapter.StartCallbackServer(app.browserLogin.ListenAddr, state)
	if err != nil {
		return fmt.Errorf("start callback server: %w", err)
	}

	authURL, err := authadapter.BuildAuthorizationURL(authadapter.AuthorizationRequest{
		AuthURL:       app.browserLogin.Issuer + "/oauth/authorize",
		ClientID:      app.browserLogin.ClientID,
		RedirectURI:   server.RedirectURI(),
		Scopes:        []string{"openid", "profile", "email", "offline_access"},
		State:         state,
		CodeChallenge: pkce.Challenge,
	})
	if err != nil {
		_ = server.Close()
		return fmt.Errorf("build authorization url: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Open this URL to authenticate account %s:\n%s\n", accountID, authURL)

	code, err := server.WaitForCode(app.browserLogin.Timeout)
	if err != nil {
		return fmt.Errorf("wait for oauth callback: %w", err)
	}

	tokens, err := authadapter.ExchangeCodeForTokens(http.DefaultClient, authadapter.TokenExchangeRequest{
		Issuer:       app.browserLogin.Issuer,
		ClientID:     app.browserLogin.ClientID,
		RedirectURI:  server.RedirectURI(),
		Code:         code,
		CodeVerifier: pkce.Verifier,
	})
	if err != nil {
		return fmt.Errorf("exchange code for tokens: %w", err)
	}

	secretValue, err := encodeOAuthTokens(withCalculatedExpiry(oauthTokens{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		IDToken:      tokens.IDToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
	}, app.now()))
	if err != nil {
		return err
	}

	secretKey := fmt.Sprintf("openai://%s/oauth_tokens", accountID)
	if err := app.service.SetAuth(cmd.Context(), accountID, domain.AuthMethodChatGPT, secretKey, secretValue); err != nil {
		return fmt.Errorf("save account oauth auth: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Authenticated account %s\n", accountID)
	return nil
}
