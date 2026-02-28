package toml

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoolRepositoryRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "pools.toml")
	cfg := viper.New()
	cfg.Set("pools.path", path)

	repo, err := NewPoolRepository(cfg)
	require.NoError(t, err)

	pool := domain.Pool{
		ID:              "default-openai",
		Name:            "default",
		Provider:        domain.ProviderOpenAI,
		Strategy:        domain.PoolStrategyLeastWeeklyUsed,
		Active:          true,
		AutoSyncMembers: true,
		Members:         []domain.AccountID{"1", "2"},
		UpdatedAt:       time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
	}

	require.NoError(t, repo.Save(context.Background(), pool))

	got, err := repo.GetByID(context.Background(), pool.ID)
	require.NoError(t, err)
	assert.Equal(t, pool, got)

	all, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []domain.Pool{pool}, all)
}

func TestPoolRuntimeRepositoryRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "pool_runtime.toml")
	cfg := viper.New()
	cfg.Set("pool.runtime.path", path)

	repo, err := NewPoolRuntimeRepository(cfg)
	require.NoError(t, err)

	runtime := domain.PoolRuntime{
		PoolID:          "default-openai",
		ActiveAccountID: "2",
		LastSyncedAt:    time.Date(2026, 2, 28, 10, 30, 0, 0, time.UTC),
		Sessions: map[string]domain.SessionLedger{
			"workspace-a": {
				LogicalSessionID: "workspace-a",
				AccountSessions: map[domain.AccountID]string{
					"1": "sess-1",
					"2": "sess-2",
				},
				Memory: domain.MemoryPacket{
					Summary:      "foo",
					Decisions:    []string{},
					PendingTasks: []string{},
					LastCodeRefs: []string{},
					UpdatedAt:    time.Date(2026, 2, 28, 10, 35, 0, 0, time.UTC),
				},
			},
		},
	}

	require.NoError(t, repo.Save(context.Background(), runtime))

	got, err := repo.GetByPoolID(context.Background(), runtime.PoolID)
	require.NoError(t, err)
	assert.Equal(t, runtime, got)
}
