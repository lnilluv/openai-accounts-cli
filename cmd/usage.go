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
	"sync"
	"time"

	authadapter "github.com/bnema/openai-accounts-cli/internal/adapters/auth"
	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

var errUsageSessionExpired = errors.New("usage session expired")
var refreshLocks sync.Map

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

type subscriptionPayload struct {
	PlanType        string `json:"plan_type"`
	ActiveStart     string `json:"active_start"`
	ActiveUntil     string `json:"active_until"`
	WillRenew       bool   `json:"will_renew"`
	BillingPeriod   string `json:"billing_period"`
	BillingCurrency string `json:"billing_currency"`
	IsDelinquent    bool   `json:"is_delinquent"`
}

type fetchResult struct {
	accountID domain.AccountID
	err       error
}

func runUsageFetch(cmd *cobra.Command, app *app, accountID string, asJSON bool) error {
	statuses, err := loadStatuses(cmd, app.service, accountID)
	if err != nil {
		return err
	}

	chatgptAccounts := filterChatGPTAccounts(statuses)

	fetchCmd := func(ctx context.Context) error {
		if len(chatgptAccounts) == 0 {
			return nil
		}
		return fetchAccountsConcurrently(ctx, app, chatgptAccounts, cmd.ErrOrStderr())
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

func filterChatGPTAccounts(statuses []application.Status) []domain.Account {
	accounts := make([]domain.Account, 0, len(statuses))
	for _, status := range statuses {
		if status.Account.Auth.Method == domain.AuthMethodChatGPT {
			accounts = append(accounts, status.Account)
		}
	}
	return accounts
}

func fetchAccountsConcurrently(ctx context.Context, app *app, accounts []domain.Account, errWriter io.Writer) error {
	const maxConcurrent = 5
	results := make(chan fetchResult, len(accounts))
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, account := range accounts {
		wg.Add(1)
		go func(acc domain.Account) {
			defer wg.Done()

			// Try to acquire semaphore or exit early on context cancellation
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results <- fetchResult{accountID: acc.ID, err: ctx.Err()}
				return
			}

			err := fetchAndPersistLimits(ctx, app, acc)
			results <- fetchResult{accountID: acc.ID, err: err}
		}(account)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var successes []domain.AccountID
	var failures []fetchResult

	for result := range results {
		if result.err == nil {
			successes = append(successes, result.accountID)
		} else {
			failures = append(failures, result)
		}
	}

	if len(failures) > 0 {
		fmt.Fprintln(errWriter, "\nFailed to fetch:")
		for _, failure := range failures {
			fmt.Fprintf(errWriter, "  - %v\n", failure.err)
		}
	}

	if len(failures) == len(accounts) {
		if len(accounts) == 1 {
			return failures[0].err
		}
		return fmt.Errorf("all accounts failed to fetch")
	}

	if len(successes) > 0 && len(failures) > 0 {
		fmt.Fprintf(errWriter, "\n%d/%d accounts updated successfully\n", len(successes), len(accounts))
	}

	return nil
}

func fetchAndPersistLimits(ctx context.Context, app *app, account domain.Account) error {
	// Check if we have fresh data (within 5 minutes)
	// Reload account from repository to get the latest persisted state
	const cacheDuration = 5 * time.Minute
	currentTime := app.now()

	status, err := app.service.GetStatus(ctx, account.ID)
	if err != nil {
		// If we can't load status, proceed with fetch
		return fetchAndPersistLimitsUncached(ctx, app, account)
	}

	// Check the most recent capture time across all limits
	var mostRecent time.Time
	if status.DailyLimit != nil && !status.DailyLimit.CapturedAt.IsZero() {
		mostRecent = status.DailyLimit.CapturedAt
	}
	if status.WeeklyLimit != nil && !status.WeeklyLimit.CapturedAt.IsZero() {
		if mostRecent.IsZero() || status.WeeklyLimit.CapturedAt.After(mostRecent) {
			mostRecent = status.WeeklyLimit.CapturedAt
		}
	}

	// Skip fetch if we have recent data
	if !mostRecent.IsZero() && currentTime.Sub(mostRecent) < cacheDuration {
		return nil // Skip fetch, data is fresh
	}

	return fetchAndPersistLimitsUncached(ctx, app, account)
}

func fetchAndPersistLimitsUncached(ctx context.Context, app *app, account domain.Account) error {

	secretRef := strings.TrimSpace(account.Auth.SecretRef)
	if secretRef == "" {
		return fmt.Errorf("account %s: auth secret reference is empty", account.ID)
	}

	secretValue, err := app.secretStore.Get(ctx, secretRef)
	if err != nil {
		return fmt.Errorf("account %s: load auth secret: %w", account.ID, err)
	}

	tokens, err := decodeOAuthTokens(secretValue)
	if err != nil {
		return fmt.Errorf("account %s: %w", account.ID, err)
	}

	tokens, err = ensureFreshTokens(ctx, app, account, tokens, false)
	if err != nil {
		if errors.Is(err, authadapter.ErrRefreshTokenInvalid) {
			return fmt.Errorf("%s: session expired, please re-login with `oa auth login browser --account %s`", usageAccountLabel(account, tokens), account.ID)
		}
		return fmt.Errorf("account %s: refresh oauth tokens: %w", account.ID, err)
	}

	claims := parseTokenClaims(tokens.IDToken)

	payload, err := fetchUsagePayload(ctx, app.httpClient, app.usageBaseURL, tokens)
	if err != nil {
		if errors.Is(err, errUsageSessionExpired) {
			staleToken := tokens.AccessToken
			tokens, err = ensureFreshTokens(ctx, app, account, tokens, true)
			if err != nil {
				if errors.Is(err, authadapter.ErrRefreshTokenInvalid) {
					return fmt.Errorf("%s: session expired, please re-login with `oa auth login browser --account %s`", usageAccountLabel(account, tokens), account.ID)
				}
				return fmt.Errorf("account %s: refresh oauth tokens after unauthorized usage response: %w", account.ID, err)
			}
			if strings.TrimSpace(tokens.AccessToken) == strings.TrimSpace(staleToken) {
				return fmt.Errorf("%s: session expired, please re-login with `oa auth login browser --account %s`", usageAccountLabel(account, tokens), account.ID)
			}
			payload, err = fetchUsagePayload(ctx, app.httpClient, app.usageBaseURL, tokens)
			if err != nil {
				if errors.Is(err, errUsageSessionExpired) {
					return fmt.Errorf("%s: session expired, please re-login with `oa auth login browser --account %s`", usageAccountLabel(account, tokens), account.ID)
				}
				return fmt.Errorf("account %s: fetch usage after refresh: %w", account.ID, err)
			}
		} else {
			return fmt.Errorf("account %s: fetch usage: %w", account.ID, err)
		}
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

	subPayload, subErr := fetchSubscriptionPayload(ctx, app.httpClient, app.usageBaseURL, tokens)
	if errors.Is(subErr, errUsageSessionExpired) {
		staleToken := tokens.AccessToken
		tokens, err = ensureFreshTokens(ctx, app, account, tokens, true)
		if err == nil && strings.TrimSpace(tokens.AccessToken) != strings.TrimSpace(staleToken) {
			subPayload, subErr = fetchSubscriptionPayload(ctx, app.httpClient, app.usageBaseURL, tokens)
		}
	}
	if subErr == nil {
		activeStart, _ := time.Parse(time.RFC3339, subPayload.ActiveStart)
		activeUntil, _ := time.Parse(time.RFC3339, subPayload.ActiveUntil)
		sub := domain.Subscription{
			ActiveStart:     activeStart,
			ActiveUntil:     activeUntil,
			WillRenew:       subPayload.WillRenew,
			BillingPeriod:   subPayload.BillingPeriod,
			BillingCurrency: subPayload.BillingCurrency,
			IsDelinquent:    subPayload.IsDelinquent,
		}
		if err := app.service.SetSubscription(ctx, account.ID, sub); err != nil {
			return fmt.Errorf("account %s: save subscription: %w", account.ID, err)
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

func fetchSubscriptionPayload(ctx context.Context, client *http.Client, baseURL string, tokens oauthTokens) (subscriptionPayload, error) {
	accountID := accountIDFromToken(tokens.IDToken)

	endpoint := strings.TrimRight(baseURL, "/") + "/subscriptions"
	if accountID != "" {
		endpoint += "?account_id=" + accountID
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return subscriptionPayload{}, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	request.Header.Set("User-Agent", "oa/usage")

	response, err := client.Do(request)
	if err != nil {
		return subscriptionPayload{}, fmt.Errorf("perform request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return subscriptionPayload{}, fmt.Errorf("read response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			return subscriptionPayload{}, fmt.Errorf("%w: status %d: %s", errUsageSessionExpired, response.StatusCode, strings.TrimSpace(string(body)))
		}
		return subscriptionPayload{}, fmt.Errorf("status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload subscriptionPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return subscriptionPayload{}, fmt.Errorf("decode payload: %w", err)
	}

	return payload, nil
}

func ensureFreshTokens(ctx context.Context, app *app, account domain.Account, existing oauthTokens, force bool) (oauthTokens, error) {
	const proactiveRefreshSkew = 2 * time.Minute
	secretRef := strings.TrimSpace(account.Auth.SecretRef)
	if secretRef == "" {
		return existing, fmt.Errorf("account %s: auth secret reference is empty", account.ID)
	}

	staleAccessToken := strings.TrimSpace(existing.AccessToken)

	lock := lockForSecretRef(secretRef)
	lock.Lock()
	defer lock.Unlock()

	storedValue, err := app.secretStore.Get(ctx, secretRef)
	if err != nil {
		return existing, fmt.Errorf("account %s: load auth secret for refresh: %w", account.ID, err)
	}
	storedTokens, err := decodeOAuthTokens(storedValue)
	if err != nil {
		return existing, fmt.Errorf("account %s: %w", account.ID, err)
	}

	if force {
		if staleAccessToken != "" && strings.TrimSpace(storedTokens.AccessToken) != "" && strings.TrimSpace(storedTokens.AccessToken) != staleAccessToken {
			return storedTokens, nil
		}
	} else if !tokenExpiringSoon(storedTokens, app.now(), proactiveRefreshSkew) {
		return storedTokens, nil
	}

	if strings.TrimSpace(storedTokens.RefreshToken) == "" {
		return storedTokens, fmt.Errorf("%w: refresh_token missing", authadapter.ErrRefreshTokenInvalid)
	}

	refreshed, err := authadapter.RefreshTokens(app.httpClient, authadapter.RefreshTokenRequest{
		Issuer:       app.browserLogin.Issuer,
		ClientID:     app.browserLogin.ClientID,
		RefreshToken: storedTokens.RefreshToken,
	})
	if err != nil {
		return storedTokens, err
	}

	updated := oauthTokens{
		AccessToken:  refreshed.AccessToken,
		RefreshToken: refreshed.RefreshToken,
		IDToken:      refreshed.IDToken,
		TokenType:    refreshed.TokenType,
		ExpiresIn:    refreshed.ExpiresIn,
	}
	if strings.TrimSpace(updated.RefreshToken) == "" {
		updated.RefreshToken = storedTokens.RefreshToken
	}
	if strings.TrimSpace(updated.IDToken) == "" {
		updated.IDToken = storedTokens.IDToken
	}
	updated = withCalculatedExpiry(updated, app.now())

	encoded, err := encodeOAuthTokens(updated)
	if err != nil {
		return storedTokens, err
	}
	if err := app.secretStore.Put(ctx, secretRef, encoded); err != nil {
		return storedTokens, fmt.Errorf("account %s: persist refreshed oauth tokens: %w", account.ID, err)
	}

	return updated, nil
}

func lockForSecretRef(secretRef string) *sync.Mutex {
	lock, _ := refreshLocks.LoadOrStore(secretRef, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func usageAccountLabel(account domain.Account, tokens oauthTokens) string {
	email := strings.TrimSpace(parseTokenClaims(tokens.IDToken).Email)
	if email == "" {
		email = strings.TrimSpace(account.Name)
	}
	classification := domain.AccountClassification(account.Metadata.PlanType)
	id := strings.TrimSpace(string(account.ID))
	if email == "" {
		return fmt.Sprintf("account %s", id)
	}
	return fmt.Sprintf("account %s (%s, %s)", id, email, classification)
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
