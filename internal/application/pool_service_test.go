package application

import (
	"context"
	"testing"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoolServiceActivateDefaultOpenAIPoolCreatesPool(t *testing.T) {
	t.Parallel()

	repo := &inMemoryAccountRepo{accounts: []domain.Account{
		{ID: "1", Metadata: domain.AccountMetadata{Provider: "openai"}},
		{ID: "2", Metadata: domain.AccountMetadata{Provider: "openai"}},
		{ID: "x", Metadata: domain.AccountMetadata{Provider: "anthropic"}},
	}}
	pools := &inMemoryPoolRepo{}
	svc := NewPoolService(repo, pools, nil)

	pool, err := svc.ActivateDefaultOpenAIPool(context.Background())
	require.NoError(t, err)
	assert.Equal(t, domain.PoolID("default-openai"), pool.ID)
	assert.Equal(t, []domain.AccountID{"1", "2"}, pool.Members)
	assert.True(t, pool.Active)
	assert.True(t, pool.AutoSyncMembers)
}

func TestPoolServiceActivateDefaultPoolSyncsMembers(t *testing.T) {
	t.Parallel()

	repo := &inMemoryAccountRepo{accounts: []domain.Account{
		{ID: "1", Metadata: domain.AccountMetadata{Provider: "openai"}},
		{ID: "2", Metadata: domain.AccountMetadata{Provider: "openai"}},
	}}
	pools := &inMemoryPoolRepo{pools: map[domain.PoolID]domain.Pool{
		"default-openai": {
			ID:              "default-openai",
			Name:            "default",
			Provider:        domain.ProviderOpenAI,
			Strategy:        domain.PoolStrategyLeastWeeklyUsed,
			Active:          false,
			AutoSyncMembers: true,
			Members:         []domain.AccountID{"1"},
		},
	}}

	svc := NewPoolService(repo, pools, fixedClock{now: time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)})

	pool, err := svc.ActivateDefaultOpenAIPool(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []domain.AccountID{"1", "2"}, pool.Members)
	assert.True(t, pool.Active)
}

func TestPoolServicePickAccountSkipsExhausted(t *testing.T) {
	t.Parallel()

	repo := &inMemoryAccountRepo{accounts: []domain.Account{
		{ID: "1", Metadata: domain.AccountMetadata{Provider: "openai"}, Limits: domain.AccountLimitSnapshots{Weekly: &domain.AccountLimitSnapshot{Percent: 100}}},
		{ID: "2", Metadata: domain.AccountMetadata{Provider: "openai"}, Limits: domain.AccountLimitSnapshots{Weekly: &domain.AccountLimitSnapshot{Percent: 30}}},
		{ID: "3", Metadata: domain.AccountMetadata{Provider: "openai"}, Limits: domain.AccountLimitSnapshots{Weekly: &domain.AccountLimitSnapshot{Percent: 10}}},
	}}
	pools := &inMemoryPoolRepo{pools: map[domain.PoolID]domain.Pool{
		"default-openai": {
			ID:       "default-openai",
			Provider: domain.ProviderOpenAI,
			Active:   true,
			Members:  []domain.AccountID{"1", "2", "3"},
		},
	}}
	svc := NewPoolService(repo, pools, nil)

	picked, failover, err := svc.PickAccount(context.Background(), "default-openai")
	require.NoError(t, err)
	assert.Equal(t, domain.AccountID("3"), picked)
	assert.Equal(t, []domain.AccountID{"2"}, failover)
}

func TestPoolServicePickAccountFailsWhenPoolIsInactive(t *testing.T) {
	t.Parallel()

	repo := &inMemoryAccountRepo{accounts: []domain.Account{
		{ID: "1", Metadata: domain.AccountMetadata{Provider: "openai"}},
	}}
	pools := &inMemoryPoolRepo{pools: map[domain.PoolID]domain.Pool{
		"default-openai": {
			ID:       "default-openai",
			Provider: domain.ProviderOpenAI,
			Active:   false,
			Members:  []domain.AccountID{"1"},
		},
	}}
	svc := NewPoolService(repo, pools, nil)

	_, _, err := svc.PickAccount(context.Background(), "default-openai")
	require.ErrorIs(t, err, domain.ErrPoolInactive)
}

type inMemoryPoolRepo struct {
	pools map[domain.PoolID]domain.Pool
}

func (r *inMemoryPoolRepo) GetByID(_ context.Context, id domain.PoolID) (domain.Pool, error) {
	if r.pools == nil {
		r.pools = map[domain.PoolID]domain.Pool{}
	}
	pool, ok := r.pools[id]
	if !ok {
		return domain.Pool{}, domain.ErrPoolNotFound
	}
	return pool, nil
}

func (r *inMemoryPoolRepo) List(_ context.Context) ([]domain.Pool, error) {
	result := make([]domain.Pool, 0, len(r.pools))
	for _, pool := range r.pools {
		result = append(result, pool)
	}
	return result, nil
}

func (r *inMemoryPoolRepo) Save(_ context.Context, pool domain.Pool) error {
	if r.pools == nil {
		r.pools = map[domain.PoolID]domain.Pool{}
	}
	r.pools[pool.ID] = pool
	return nil
}

type inMemoryAccountRepo struct {
	accounts []domain.Account
}

func (r *inMemoryAccountRepo) GetByID(_ context.Context, id domain.AccountID) (domain.Account, error) {
	for _, account := range r.accounts {
		if account.ID == id {
			return account, nil
		}
	}
	return domain.Account{}, domain.ErrAccountNotFound
}

func (r *inMemoryAccountRepo) List(_ context.Context) ([]domain.Account, error) {
	return r.accounts, nil
}

func (r *inMemoryAccountRepo) Save(_ context.Context, account domain.Account) error {
	for i := range r.accounts {
		if r.accounts[i].ID == account.ID {
			r.accounts[i] = account
			return nil
		}
	}
	r.accounts = append(r.accounts, account)
	return nil
}

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time {
	return f.now
}
