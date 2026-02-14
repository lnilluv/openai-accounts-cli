package status

import (
	"testing"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSingleAccountStatus(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{
				ID:   "acc-1",
				Name: "Primary",
				Auth: domain.Auth{Method: domain.AuthMethodAPIKey},
			},
			Usage: domain.Usage{InputTokens: 1200, OutputTokens: 800, CachedInputTokens: 500},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    73.2,
				ResetsAt:   now.Add(13 * time.Hour),
				CapturedAt: now.Add(-15 * time.Minute),
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "accounts: 1")
	assert.Contains(t, output, "Primary")
	assert.Contains(t, output, "5hours limit:")
	assert.Contains(t, output, "27% left")
	assert.Contains(t, output, "resets in 13 hours (00:00)")
	assert.Contains(t, output, "[")
	assert.Contains(t, output, "]")
	assert.NotContains(t, output, "stale")
}

func TestRenderMultiAccountStatus(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{
				ID:   "acc-1",
				Name: "Primary",
				Auth: domain.Auth{Method: domain.AuthMethodAPIKey},
			},
			Usage: domain.Usage{InputTokens: 1000, OutputTokens: 500, CachedInputTokens: 100},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    52.5,
				ResetsAt:   now.Add(5 * time.Hour),
				CapturedAt: now,
			},
		},
		{
			Account: domain.Account{
				ID:   "acc-2",
				Name: "Backup",
				Auth: domain.Auth{Method: domain.AuthMethodAPIKey},
			},
			Usage: domain.Usage{InputTokens: 400, OutputTokens: 200, CachedInputTokens: 0},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    12.3,
				ResetsAt:   now.Add(4 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 24 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "accounts: 2")
	assert.Contains(t, output, "Primary")
	assert.Contains(t, output, "Backup")
	assert.Contains(t, output, "5hours limit:")
	assert.Contains(t, output, "weekly limit:")
	assert.Contains(t, output, "48% left")
	assert.Contains(t, output, "88% left")
	assert.Contains(t, output, "resets in 5 hours (16:00)")
	assert.Contains(t, output, "resets in 4 days (11:00 on 18 Feb)")
}

func TestRenderMarksStaleLimitSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{
				ID:   "acc-1",
				Name: "Primary",
				Auth: domain.Auth{Method: domain.AuthMethodAPIKey},
			},
			Usage: domain.Usage{InputTokens: 300, OutputTokens: 200, CachedInputTokens: 50},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    80,
				ResetsAt:   now.Add(8 * time.Hour),
				CapturedAt: now.Add(-48 * time.Hour),
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 12 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "5hours limit:")
	assert.Contains(t, output, "20% left")
	assert.Contains(t, output, "[stale]")
}

func TestRenderShowsDailyAndWeeklyLimitsWhenBothAvailable(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-1", Name: "Primary", Auth: domain.Auth{Method: domain.AuthMethodAPIKey}},
			Usage:   domain.Usage{InputTokens: 100, OutputTokens: 50, CachedInputTokens: 25},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    80,
				ResetsAt:   now.Add(12 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    45,
				ResetsAt:   now.Add(6 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "5hours limit:")
	assert.Contains(t, output, "weekly limit:")
	assert.Contains(t, output, "20% left")
	assert.Contains(t, output, "55% left")
}

func TestRenderDoesNotMarkStaleWhenNowNotProvided(t *testing.T) {
	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-1", Name: "Primary", Auth: domain.Auth{Method: domain.AuthMethodAPIKey}},
			Usage:   domain.Usage{InputTokens: 300, OutputTokens: 200, CachedInputTokens: 50},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    80,
				ResetsAt:   time.Date(2026, 2, 15, 11, 0, 0, 0, time.UTC),
				CapturedAt: time.Date(2026, 2, 10, 11, 0, 0, 0, time.UTC),
			},
		},
	}, RenderOptions{StaleAfter: 12 * time.Hour})

	require.NoError(t, err)
	assert.NotContains(t, output, "[stale]")
}

func TestRenderShowsUnavailableUsageHintForChatGPTWithoutTokenSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-1", Name: "Primary", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			Usage:   domain.Usage{},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    80,
				ResetsAt:   now.Add(5 * time.Hour),
				CapturedAt: now,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "Primary")
	assert.Contains(t, output, "5hours limit:")
}
