package toml

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryRoundTrip(t *testing.T) {
	t.Parallel()

	accountsPath := filepath.Join(t.TempDir(), "accounts.toml")
	config := viper.New()
	config.Set("accounts.path", accountsPath)

	repo, err := NewRepository(config)
	require.NoError(t, err)

	first := domain.Account{
		ID:   "acc-1",
		Name: "Primary",
		Metadata: domain.AccountMetadata{
			Provider:  "openai",
			Model:     "gpt-5",
			SecretRef: "openai://acc-1",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1"},
	}
	second := domain.Account{
		ID:   "acc-2",
		Name: "Backup",
		Metadata: domain.AccountMetadata{
			Provider:  "openai",
			Model:     "gpt-4o-mini",
			SecretRef: "openai://acc-2",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-2"},
	}

	require.NoError(t, repo.Save(context.Background(), first))
	require.NoError(t, repo.Save(context.Background(), second))

	got, err := repo.GetByID(context.Background(), first.ID)
	require.NoError(t, err)
	assert.Equal(t, first, got)

	accounts, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.ElementsMatch(t, []domain.Account{first, second}, accounts)
}

func TestRepositoryRoundTripPersistsUsageAndLimitSnapshots(t *testing.T) {
	t.Parallel()

	accountsPath := filepath.Join(t.TempDir(), "accounts.toml")
	config := viper.New()
	config.Set("accounts.path", accountsPath)

	repo, err := NewRepository(config)
	require.NoError(t, err)

	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)
	account := domain.Account{
		ID:    "acc-1",
		Name:  "Primary",
		Usage: domain.Usage{InputTokens: 100, OutputTokens: 50, CachedInputTokens: 25},
		Limits: domain.AccountLimitSnapshots{
			Daily: &domain.AccountLimitSnapshot{
				Percent:    80,
				ResetsAt:   now.Add(12 * time.Hour),
				CapturedAt: now,
			},
			Weekly: &domain.AccountLimitSnapshot{
				Percent:    40,
				ResetsAt:   now.Add(6 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}

	require.NoError(t, repo.Save(context.Background(), account))

	got, err := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	assert.Equal(t, account.Usage, got.Usage)
	require.NotNil(t, got.Limits.Daily)
	assert.Equal(t, account.Limits.Daily.Percent, got.Limits.Daily.Percent)
	require.NotNil(t, got.Limits.Weekly)
	assert.Equal(t, account.Limits.Weekly.Percent, got.Limits.Weekly.Percent)
}

func TestRepositoryBackwardCompatibleWhenUsageAndLimitsMissing(t *testing.T) {
	t.Parallel()

	accountsPath := filepath.Join(t.TempDir(), "accounts.toml")
	require.NoError(t, os.WriteFile(accountsPath, []byte(strings.Join([]string{
		"version = 1",
		"",
		"[[accounts]]",
		"id = \"acc-1\"",
		"name = \"Primary\"",
		"",
		"[accounts.metadata]",
		"provider = \"openai\"",
		"model = \"gpt-5\"",
		"",
		"[accounts.auth]",
		"method = \"\"",
		"secret_ref = \"\"",
		"",
	}, "\n")), 0o600))

	config := viper.New()
	config.Set("accounts.path", accountsPath)

	repo, err := NewRepository(config)
	require.NoError(t, err)

	account, err := repo.GetByID(context.Background(), "acc-1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), account.Usage.BlendedTotal())
	assert.Nil(t, account.Limits.Daily)
	assert.Nil(t, account.Limits.Weekly)
}

func TestRepositorySaveCreatesDefaultPathAndEnforcesPermissions(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repo, err := NewRepository(viper.New())
	require.NoError(t, err)

	err = repo.Save(context.Background(), domain.Account{
		ID:   "acc-1",
		Name: "Primary",
		Metadata: domain.AccountMetadata{
			Provider: "openai",
			Model:    "gpt-5",
		},
	})
	require.NoError(t, err)

	accountsPath := filepath.Join(homeDir, ".codex", "accounts.toml")
	info, err := os.Stat(accountsPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestRepositoryMissingFileBehaviors(t *testing.T) {
	t.Parallel()

	accountsPath := filepath.Join(t.TempDir(), "missing", "accounts.toml")
	config := viper.New()
	config.Set("accounts.path", accountsPath)

	repo, err := NewRepository(config)
	require.NoError(t, err)

	accounts, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, accounts)

	_, err = repo.GetByID(context.Background(), "acc-1")
	require.ErrorIs(t, err, domain.ErrAccountNotFound)
}

func TestRepositoryListMalformedTOMLReturnsError(t *testing.T) {
	t.Parallel()

	accountsPath := filepath.Join(t.TempDir(), "accounts.toml")
	require.NoError(t, os.WriteFile(accountsPath, []byte("accounts = ["), 0o600))

	config := viper.New()
	config.Set("accounts.path", accountsPath)

	repo, err := NewRepository(config)
	require.NoError(t, err)

	_, err = repo.List(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "decode accounts file")
}

func TestRepositorySaveCanceledContextReturnsContextError(t *testing.T) {
	t.Parallel()

	accountsPath := filepath.Join(t.TempDir(), "accounts.toml")
	config := viper.New()
	config.Set("accounts.path", accountsPath)

	repo, err := NewRepository(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = repo.Save(ctx, domain.Account{ID: "acc-1", Name: "Primary"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestRepositoryConcurrentSavesAcrossInstancesPreserveBothAccounts(t *testing.T) {
	t.Parallel()

	accountsPath := filepath.Join(t.TempDir(), "accounts.toml")

	newRepo := func() *Repository {
		config := viper.New()
		config.Set("accounts.path", accountsPath)
		repo, err := NewRepository(config)
		require.NoError(t, err)
		return repo
	}

	repoA := newRepo()
	repoB := newRepo()

	const perRepoWrites = 100
	start := make(chan struct{})
	errCh := make(chan error, perRepoWrites*2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < perRepoWrites; i++ {
			errCh <- repoA.Save(context.Background(), domain.Account{ID: domain.AccountID("acc-a-" + strconv.Itoa(i)), Name: "A"})
		}
	}()

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < perRepoWrites; i++ {
			errCh <- repoB.Save(context.Background(), domain.Account{ID: domain.AccountID("acc-b-" + strconv.Itoa(i)), Name: "B"})
		}
	}()

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	accounts, err := repoA.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, accounts, perRepoWrites*2)
}

func TestRepositorySaveSerializedTOMLIncludesVersion(t *testing.T) {
	t.Parallel()

	accountsPath := filepath.Join(t.TempDir(), "accounts.toml")
	config := viper.New()
	config.Set("accounts.path", accountsPath)

	repo, err := NewRepository(config)
	require.NoError(t, err)

	require.NoError(t, repo.Save(context.Background(), domain.Account{ID: "acc-1", Name: "Primary"}))

	data, err := os.ReadFile(accountsPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "version = 1")
}

func TestRepositoryFutureSchemaVersionReturnsError(t *testing.T) {
	t.Parallel()

	accountsPath := filepath.Join(t.TempDir(), "accounts.toml")
	require.NoError(t, os.WriteFile(accountsPath, []byte(strings.Join([]string{
		"version = 999",
		"",
		"accounts = []",
		"",
	}, "\n")), 0o600))

	config := viper.New()
	config.Set("accounts.path", accountsPath)
	repo, err := NewRepository(config)
	require.NoError(t, err)

	_, err = repo.List(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported accounts schema version")
}
