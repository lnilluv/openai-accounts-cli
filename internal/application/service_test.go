package application

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	tomlrepo "github.com/bnema/openai-accounts-cli/internal/adapters/repo/toml"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/bnema/openai-accounts-cli/internal/ports/mocks"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServiceSetAuthSuccess(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	account := domain.Account{ID: "acc-1", Name: "openai"}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	store.EXPECT().Put(mockAnyContext(), "openai://acc-1/api_key", "secret-value").Return(nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/api_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/api_key"},
	}).Return(nil)

	err := service.SetAuth(context.Background(), "acc-1", domain.AuthMethodAPIKey, "openai://acc-1/api_key", "secret-value")
	require.NoError(t, err)
}

func TestServiceSetAuthRotationDeletesPreviousSecretRef(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	account := domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/old_api_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/old_api_key"},
	}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	store.EXPECT().Put(mockAnyContext(), "openai://acc-1/new_api_key", "secret-value").Return(nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/new_api_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/new_api_key"},
	}).Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/old_api_key").Return(nil)

	err := service.SetAuth(context.Background(), "acc-1", domain.AuthMethodAPIKey, "openai://acc-1/new_api_key", "secret-value")
	require.NoError(t, err)
}

func TestServiceSetAuthRotationReturnsErrorWhenPreviousSecretDeleteFails(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	deleteErr := errors.New("delete old secret failed")
	account := domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/old_api_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/old_api_key"},
	}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	store.EXPECT().Put(mockAnyContext(), "openai://acc-1/new_api_key", "secret-value").Return(nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/new_api_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/new_api_key"},
	}).Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/old_api_key").Return(deleteErr)
	repo.EXPECT().Save(mockAnyContext(), account).Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/new_api_key").Return(nil)

	err := service.SetAuth(context.Background(), "acc-1", domain.AuthMethodAPIKey, "openai://acc-1/new_api_key", "secret-value")
	require.ErrorIs(t, err, deleteErr)
}

func TestServiceSetAuthRotationRestoresOnlyRemainingOldRefOnPartialDeleteFailure(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	deleteErr := errors.New("delete old auth ref failed")
	account := domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/old_metadata_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/old_auth_key"},
	}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	store.EXPECT().Put(mockAnyContext(), "openai://acc-1/new_api_key", "secret-value").Return(nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/new_api_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/new_api_key"},
	}).Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/old_metadata_key").Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/old_auth_key").Return(deleteErr)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/old_auth_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/old_auth_key"},
	}).Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/new_api_key").Return(nil)

	err := service.SetAuth(context.Background(), "acc-1", domain.AuthMethodAPIKey, "openai://acc-1/new_api_key", "secret-value")
	require.ErrorIs(t, err, deleteErr)
}

func TestServiceSetAuthFailsWhenSecretStorePutFails(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	putErr := errors.New("put failed")
	account := domain.Account{ID: "acc-1", Name: "openai"}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	store.EXPECT().Put(mockAnyContext(), "openai://acc-1/api_key", "secret-value").Return(putErr)

	err := service.SetAuth(context.Background(), "acc-1", domain.AuthMethodAPIKey, "openai://acc-1/api_key", "secret-value")
	require.ErrorIs(t, err, putErr)
}

func TestServiceSetAuthFailsWhenSaveFailsAndCompensatesSecretWrite(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	saveErr := errors.New("save failed")
	account := domain.Account{ID: "acc-1", Name: "openai"}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	store.EXPECT().Put(mockAnyContext(), "openai://acc-1/api_key", "secret-value").Return(nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/api_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/api_key"},
	}).Return(saveErr)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/api_key").Return(nil)

	err := service.SetAuth(context.Background(), "acc-1", domain.AuthMethodAPIKey, "openai://acc-1/api_key", "secret-value")
	require.ErrorIs(t, err, saveErr)
}

func TestServiceSetAuthFailsWhenRollbackDeleteFails(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	saveErr := errors.New("save failed")
	rollbackErr := errors.New("rollback failed")
	account := domain.Account{ID: "acc-1", Name: "openai"}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	store.EXPECT().Put(mockAnyContext(), "openai://acc-1/api_key", "secret-value").Return(nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/api_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/api_key"},
	}).Return(saveErr)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/api_key").Return(rollbackErr)

	err := service.SetAuth(context.Background(), "acc-1", domain.AuthMethodAPIKey, "openai://acc-1/api_key", "secret-value")
	require.ErrorIs(t, err, saveErr)
	require.ErrorIs(t, err, rollbackErr)
}

func TestServiceRemoveAuthSuccess(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	account := domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/api_key"},
	}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:       "acc-1",
		Name:     "openai",
		Metadata: domain.AccountMetadata{},
		Auth:     domain.Auth{},
	}).Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/api_key").Return(nil)

	err := service.RemoveAuth(context.Background(), "acc-1")
	require.NoError(t, err)
}

func TestServiceRemoveAuthReturnsErrorWhenDeleteFails(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	deleteErr := errors.New("delete failed")
	account := domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/api_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/api_key"},
	}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:       "acc-1",
		Name:     "openai",
		Metadata: domain.AccountMetadata{},
		Auth:     domain.Auth{},
	}).Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/api_key").Return(deleteErr)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/api_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/api_key"},
	}).Return(nil)

	err := service.RemoveAuth(context.Background(), "acc-1")
	require.ErrorIs(t, err, deleteErr)
}

func TestServiceRemoveAuthDeletesBothUniqueSecretRefsWhenTheyDiffer(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	account := domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/metadata_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/auth_key"},
	}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:       "acc-1",
		Name:     "openai",
		Metadata: domain.AccountMetadata{},
		Auth:     domain.Auth{},
	}).Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/metadata_key").Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/auth_key").Return(nil)

	err := service.RemoveAuth(context.Background(), "acc-1")
	require.NoError(t, err)
}

func TestServiceRemoveAuthRestoresOnlyRemainingRefAfterPartialDeleteFailure(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	deleteErr := errors.New("delete auth ref failed")
	account := domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/metadata_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/auth_key"},
	}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:       "acc-1",
		Name:     "openai",
		Metadata: domain.AccountMetadata{},
		Auth:     domain.Auth{},
	}).Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/metadata_key").Return(nil)
	store.EXPECT().Delete(mockAnyContext(), "openai://acc-1/auth_key").Return(deleteErr)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Metadata: domain.AccountMetadata{
			SecretRef: "openai://acc-1/auth_key",
		},
		Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-1/auth_key"},
	}).Return(nil)

	err := service.RemoveAuth(context.Background(), "acc-1")
	require.ErrorIs(t, err, deleteErr)
}

func TestServiceSetUsageFailsForMissingAccount(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(domain.Account{}, domain.ErrAccountNotFound)

	err := service.SetUsage(context.Background(), "acc-1", domain.Usage{InputTokens: 1})
	require.ErrorIs(t, err, domain.ErrAccountNotFound)
}

func TestServiceSetUsageAndGetStatus(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	account := domain.Account{ID: "acc-1", Name: "openai"}
	updated := domain.Account{
		ID:    "acc-1",
		Name:  "openai",
		Usage: domain.Usage{InputTokens: 1, OutputTokens: 2, CachedInputTokens: 3},
	}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil).Once()
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(updated, nil).Once()
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:    "acc-1",
		Name:  "openai",
		Usage: domain.Usage{InputTokens: 1, OutputTokens: 2, CachedInputTokens: 3},
	}).Return(nil)

	usage := domain.Usage{InputTokens: 1, OutputTokens: 2, CachedInputTokens: 3}
	err := service.SetUsage(context.Background(), "acc-1", usage)
	require.NoError(t, err)

	status, err := service.GetStatus(context.Background(), "acc-1")
	require.NoError(t, err)
	assert.Equal(t, usage, status.Usage)
}

func TestServiceSetAccountName(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(domain.Account{ID: "acc-1", Name: "Primary"}, nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{ID: "acc-1", Name: "email@adress.com"}).Return(nil)

	err := service.SetAccountName(context.Background(), "acc-1", "email@adress.com")
	require.NoError(t, err)
}

func TestServiceSetAccountPlanType(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(domain.Account{ID: "acc-1", Name: "Primary"}, nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:   "acc-1",
		Name: "Primary",
		Metadata: domain.AccountMetadata{
			PlanType: "team",
		},
	}).Return(nil)

	err := service.SetAccountPlanType(context.Background(), "acc-1", "team")
	require.NoError(t, err)
}

func TestServiceSetLimitUsesClockWhenCapturedAtZero(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	now := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)
	resetsAt := now.Add(24 * time.Hour)
	account := domain.Account{ID: "acc-1", Name: "openai"}
	updated := domain.Account{
		ID:   "acc-1",
		Name: "openai",
		Limits: domain.AccountLimitSnapshots{
			Daily: &domain.AccountLimitSnapshot{
				Percent:    73.2,
				ResetsAt:   resetsAt,
				CapturedAt: now,
			},
		},
	}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(account, nil).Once()
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(updated, nil).Once()
	clock.EXPECT().Now().Return(now).Once()
	repo.EXPECT().Save(mockAnyContext(), mock.MatchedBy(func(saved domain.Account) bool {
		return saved.ID == "acc-1" && saved.Name == "openai" && saved.Limits.Daily != nil && saved.Limits.Daily.Percent == 73.2 && saved.Limits.Weekly == nil
	})).Return(nil)

	err := service.SetLimit(context.Background(), "acc-1", LimitWindowDaily, 73.2, resetsAt, time.Time{})
	require.NoError(t, err)

	status, err := service.GetStatus(context.Background(), "acc-1")
	require.NoError(t, err)
	require.NotNil(t, status.DailyLimit)
	assert.Equal(t, LimitWindowDaily, status.DailyLimit.Window)
	assert.Equal(t, 73.2, status.DailyLimit.Percent)
	assert.True(t, status.DailyLimit.ResetsAt.Equal(resetsAt))
	assert.True(t, status.DailyLimit.CapturedAt.Equal(now))
}

func TestServiceUsageAndLimitsPersistAcrossServiceInstances(t *testing.T) {
	t.Parallel()

	accountsPath := filepath.Join(t.TempDir(), "accounts.toml")
	cfg := viper.New()
	cfg.Set("accounts.path", accountsPath)

	repo, err := tomlrepo.NewRepository(cfg)
	require.NoError(t, err)

	require.NoError(t, repo.Save(context.Background(), domain.Account{ID: "acc-1", Name: "Primary"}))

	clock := mocks.NewMockClock(t)
	now := time.Date(2026, time.February, 14, 11, 0, 0, 0, time.UTC)
	clock.EXPECT().Now().Return(now).Once()

	serviceA := NewService(repo, nil, clock)
	require.NoError(t, serviceA.SetUsage(context.Background(), "acc-1", domain.Usage{
		InputTokens:       100,
		OutputTokens:      50,
		CachedInputTokens: 25,
	}))
	require.NoError(t, serviceA.SetLimit(context.Background(), "acc-1", LimitWindowDaily, 80, now.Add(12*time.Hour), time.Time{}))
	require.NoError(t, serviceA.SetLimit(context.Background(), "acc-1", LimitWindowWeekly, 40, now.Add(6*24*time.Hour), now))

	serviceB := NewService(repo, nil, mocks.NewMockClock(t))
	status, err := serviceB.GetStatus(context.Background(), "acc-1")
	require.NoError(t, err)

	assert.Equal(t, int64(100), status.Usage.InputTokens)
	assert.Equal(t, int64(50), status.Usage.OutputTokens)
	assert.Equal(t, int64(25), status.Usage.CachedInputTokens)
	require.NotNil(t, status.DailyLimit)
	assert.Equal(t, 80.0, status.DailyLimit.Percent)
	require.NotNil(t, status.WeeklyLimit)
	assert.Equal(t, 40.0, status.WeeklyLimit.Percent)
}

func TestServiceGetStatusAll(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	accounts := []domain.Account{{ID: "acc-1", Name: "one", Usage: domain.Usage{InputTokens: 100}}, {ID: "acc-2", Name: "two"}}
	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(accounts[0], nil)
	repo.EXPECT().Save(mockAnyContext(), domain.Account{
		ID:    "acc-1",
		Name:  "one",
		Usage: domain.Usage{InputTokens: 100},
	}).Return(nil)
	repo.EXPECT().List(mockAnyContext()).Return(accounts, nil)

	err := service.SetUsage(context.Background(), "acc-1", domain.Usage{InputTokens: 100})
	require.NoError(t, err)

	all, err := service.GetStatusAll(context.Background())
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, domain.AccountID("acc-1"), all[0].Account.ID)
	assert.Equal(t, int64(100), all[0].Usage.InputTokens)
}

func TestServiceGetStatusReturnsRepositoryError(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	repo.EXPECT().GetByID(mockAnyContext(), domain.AccountID("acc-1")).Return(domain.Account{}, domain.ErrAccountNotFound)

	_, err := service.GetStatus(context.Background(), "acc-1")
	require.ErrorIs(t, err, domain.ErrAccountNotFound)
}

func TestServiceGetStatusAllReturnsListError(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	listErr := errors.New("list failed")
	repo.EXPECT().List(mockAnyContext()).Return(nil, listErr)

	_, err := service.GetStatusAll(context.Background())
	require.ErrorIs(t, err, listErr)
}

func TestServiceSetLimitRejectsUnsupportedWindow(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	err := service.SetLimit(context.Background(), "acc-1", LimitWindowKind("monthly"), 20, time.Now(), time.Now())
	require.Error(t, err)
}

func mockAnyContext() interface{} {
	return mock.Anything
}
