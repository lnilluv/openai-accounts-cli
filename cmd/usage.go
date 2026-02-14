package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

var errUsageSessionExpired = errors.New("usage session expired")

func newUsageCmd(app *app) *cobra.Command {
	var accountID string
	var asJSON bool

	cmd := &cobra.Command{
		Use:     "usage",
		Aliases: []string{"status"},
		Short:   "Fetch and display account usage limits",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUsageFetch(cmd, app, accountID, asJSON)
		},
	}

	cmd.Flags().StringVar(&accountID, "account", "", "Account ID (default: all accounts)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Render JSON output")

	return cmd
}

type oauthTokens struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

type tokenClaims struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	Email            string `json:"email"`
	APIAuth          struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	} `json:"https://api.openai.com/auth"`
}

type usageWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int     `json:"limit_window_seconds"`
	ResetAt            int64   `json:"reset_at"`
}

type usageRateLimit struct {
	PrimaryWindow   *usageWindow `json:"primary_window"`
	SecondaryWindow *usageWindow `json:"secondary_window"`
}

type usageAdditionalRateLimit struct {
	RateLimit *usageRateLimit `json:"rate_limit"`
}

type usagePayload struct {
	PlanType             string                     `json:"plan_type"`
	RateLimit            *usageRateLimit            `json:"rate_limit"`
	AdditionalRateLimits []usageAdditionalRateLimit `json:"additional_rate_limits"`
}

func runUsageFetch(cmd *cobra.Command, app *app, accountID string, asJSON bool) error {
	statuses, err := loadStatuses(cmd, app.service, accountID)
	if err != nil {
		return err
	}

	fetchCmd := func(ctx context.Context) error {
		for _, status := range statuses {
			if status.Account.Auth.Method != domain.AuthMethodChatGPT {
				continue
			}

			if err := fetchAndPersistLimits(ctx, app, status.Account); err != nil {
				return err
			}
		}

		return nil
	}

	if asJSON {
		if err := fetchCmd(cmd.Context()); err != nil {
			return err
		}
	} else {
		if err := runUsageFetchSpinner(cmd.Context(), cmd.ErrOrStderr(), fetchCmd); err != nil {
			return err
		}
	}

	updated, err := loadStatuses(cmd, app.service, accountID)
	if err != nil {
		return err
	}

	return writeStatusesOutput(cmd, app, updated, 6*time.Hour, asJSON)
}

func fetchAndPersistLimits(ctx context.Context, app *app, account domain.Account) error {
	secretRef := strings.TrimSpace(account.Auth.SecretRef)
	if secretRef == "" {
		return fmt.Errorf("account %s: auth secret reference is empty", account.ID)
	}

	secretValue, err := app.secretStore.Get(ctx, secretRef)
	if err != nil {
		return fmt.Errorf("account %s: load auth secret: %w", account.ID, err)
	}

	var tokens oauthTokens
	if err := json.Unmarshal([]byte(secretValue), &tokens); err != nil {
		return fmt.Errorf("account %s: decode oauth tokens: %w", account.ID, err)
	}
	if strings.TrimSpace(tokens.AccessToken) == "" {
		return fmt.Errorf("account %s: oauth tokens missing access_token", account.ID)
	}

	claims := parseTokenClaims(tokens.IDToken)

	payload, err := fetchUsagePayload(ctx, app.httpClient, app.usageBaseURL, tokens)
	if err != nil {
		if errors.Is(err, errUsageSessionExpired) {
			return fmt.Errorf("account %s: session expired, please re-login with `oa login browser --account %s`", account.ID, account.ID)
		}
		return fmt.Errorf("account %s: fetch usage: %w", account.ID, err)
	}

	daily, weekly := pickDailyWeeklyWindows(payload)
	if daily == nil && weekly == nil {
		return fmt.Errorf("account %s: missing limit snapshots in usage payload", account.ID)
	}

	now := app.now()
	if daily != nil {
		if err := app.service.SetLimit(ctx, account.ID, "daily", daily.UsedPercent, time.Unix(daily.ResetAt, 0).UTC(), now); err != nil {
			return fmt.Errorf("account %s: save daily limit snapshot: %w", account.ID, err)
		}
	}
	if weekly != nil {
		if err := app.service.SetLimit(ctx, account.ID, "weekly", weekly.UsedPercent, time.Unix(weekly.ResetAt, 0).UTC(), now); err != nil {
			return fmt.Errorf("account %s: save weekly limit snapshot: %w", account.ID, err)
		}
	}

	if email := strings.TrimSpace(claims.Email); email != "" && account.Name != email {
		if err := app.service.SetAccountName(ctx, account.ID, email); err != nil {
			return fmt.Errorf("account %s: save account name from token email: %w", account.ID, err)
		}
	}

	if planType := strings.TrimSpace(payload.PlanType); planType != "" && account.Metadata.PlanType != planType {
		if err := app.service.SetAccountPlanType(ctx, account.ID, planType); err != nil {
			return fmt.Errorf("account %s: save account plan type: %w", account.ID, err)
		}
	}

	return nil
}

func fetchUsagePayload(ctx context.Context, client *http.Client, baseURL string, tokens oauthTokens) (usagePayload, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/wham/usage"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return usagePayload{}, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	request.Header.Set("User-Agent", "oa/usage")
	if accountID := accountIDFromToken(tokens.IDToken); accountID != "" {
		request.Header.Set("ChatGPT-Account-Id", accountID)
	}

	response, err := client.Do(request)
	if err != nil {
		return usagePayload{}, fmt.Errorf("perform request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return usagePayload{}, fmt.Errorf("read response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			return usagePayload{}, fmt.Errorf("%w: status %d: %s", errUsageSessionExpired, response.StatusCode, strings.TrimSpace(string(body)))
		}
		return usagePayload{}, fmt.Errorf("status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload usagePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return usagePayload{}, fmt.Errorf("decode payload: %w", err)
	}

	return payload, nil
}

func accountIDFromToken(token string) string {
	claims := parseTokenClaims(token)

	if claims.ChatGPTAccountID != "" {
		return claims.ChatGPTAccountID
	}

	return claims.APIAuth.ChatGPTAccountID
}

func parseTokenClaims(token string) tokenClaims {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return tokenClaims{}
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenClaims{}
	}

	var claims tokenClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return tokenClaims{}
	}

	return claims
}

func pickDailyWeeklyWindows(payload usagePayload) (*usageWindow, *usageWindow) {
	windows := collectWindows(payload)
	var daily *usageWindow
	var weekly *usageWindow

	for i := range windows {
		window := windows[i]
		if window == nil || window.ResetAt <= 0 {
			continue
		}

		if isWeeklyWindow(window.LimitWindowSeconds) {
			if weekly == nil || window.LimitWindowSeconds > weekly.LimitWindowSeconds {
				weekly = window
			}
			continue
		}

		if daily == nil || window.LimitWindowSeconds < daily.LimitWindowSeconds {
			daily = window
		}
	}

	return daily, weekly
}

func collectWindows(payload usagePayload) []*usageWindow {
	windows := make([]*usageWindow, 0, 8)
	appendRateLimitWindows := func(limit *usageRateLimit) {
		if limit == nil {
			return
		}
		windows = append(windows, limit.PrimaryWindow, limit.SecondaryWindow)
	}

	appendRateLimitWindows(payload.RateLimit)
	for _, additional := range payload.AdditionalRateLimits {
		appendRateLimitWindows(additional.RateLimit)
	}

	return windows
}

func isWeeklyWindow(seconds int) bool {
	return seconds >= 6*24*60*60
}
