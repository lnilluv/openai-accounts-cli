package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/bnema/openai-accounts-cli/internal/ports"
)

const DefaultOpenAIPoolID domain.PoolID = "default-openai"

type PoolService struct {
	accounts ports.AccountRepository
	pools    ports.PoolRepository
	clock    ports.Clock
}

func NewPoolService(accounts ports.AccountRepository, pools ports.PoolRepository, clock ports.Clock) *PoolService {
	if clock == nil {
		clock = ports.SystemClock{}
	}

	return &PoolService{accounts: accounts, pools: pools, clock: clock}
}

func (s *PoolService) ActivateDefaultOpenAIPool(ctx context.Context) (domain.Pool, error) {
	accounts, err := s.accounts.List(ctx)
	if err != nil {
		return domain.Pool{}, fmt.Errorf("list accounts: %w", err)
	}

	members := openAIMembers(accounts)

	pool, err := s.pools.GetByID(ctx, DefaultOpenAIPoolID)
	if err != nil {
		if err != domain.ErrPoolNotFound {
			return domain.Pool{}, fmt.Errorf("load default pool: %w", err)
		}
		pool = domain.Pool{
			ID:              DefaultOpenAIPoolID,
			Name:            "default",
			Provider:        domain.ProviderOpenAI,
			Strategy:        domain.PoolStrategyLeastWeeklyUsed,
			AutoSyncMembers: true,
		}
	}

	if pool.AutoSyncMembers {
		pool.Members = members
	}
	pool.Active = true
	pool.UpdatedAt = s.clock.Now()
	pool.NormalizeMembers()

	if err := pool.Validate(); err != nil {
		return domain.Pool{}, err
	}

	if err := s.pools.Save(ctx, pool); err != nil {
		return domain.Pool{}, fmt.Errorf("save pool: %w", err)
	}

	return pool, nil
}

func (s *PoolService) DeactivatePool(ctx context.Context, poolID domain.PoolID) (domain.Pool, error) {
	pool, err := s.pools.GetByID(ctx, poolID)
	if err != nil {
		return domain.Pool{}, err
	}

	pool.Active = false
	pool.UpdatedAt = s.clock.Now()
	if err := s.pools.Save(ctx, pool); err != nil {
		return domain.Pool{}, fmt.Errorf("save pool: %w", err)
	}

	return pool, nil
}

func (s *PoolService) PickAccount(ctx context.Context, poolID domain.PoolID) (domain.AccountID, []domain.AccountID, error) {
	pool, err := s.pools.GetByID(ctx, poolID)
	if err != nil {
		return "", nil, err
	}

	accounts, err := s.accounts.List(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("list accounts: %w", err)
	}

	byID := make(map[domain.AccountID]domain.Account, len(accounts))
	for _, account := range accounts {
		byID[account.ID] = account
	}

	candidates := make([]domain.Account, 0, len(pool.Members))
	for _, member := range pool.Members {
		account, ok := byID[member]
		if !ok {
			continue
		}
		if strings.TrimSpace(account.Metadata.Provider) != string(pool.Provider) {
			continue
		}
		if account.Limits.Weekly != nil && account.Limits.Weekly.Percent >= 100 {
			continue
		}
		candidates = append(candidates, account)
	}

	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("no eligible accounts in pool %s", poolID)
	}

	sort.Slice(candidates, func(i, j int) bool {
		left := weeklyPercent(candidates[i])
		right := weeklyPercent(candidates[j])
		if left == right {
			return string(candidates[i].ID) < string(candidates[j].ID)
		}
		return left < right
	})

	picked := candidates[0].ID
	failover := make([]domain.AccountID, 0, len(candidates)-1)
	for _, candidate := range candidates[1:] {
		failover = append(failover, candidate.ID)
	}

	return picked, failover, nil
}

func (s *PoolService) GetPool(ctx context.Context, poolID domain.PoolID) (domain.Pool, error) {
	pool, err := s.pools.GetByID(ctx, poolID)
	if err != nil {
		return domain.Pool{}, err
	}

	if pool.AutoSyncMembers {
		accounts, err := s.accounts.List(ctx)
		if err != nil {
			return domain.Pool{}, fmt.Errorf("list accounts: %w", err)
		}
		pool.Members = openAIMembers(accounts)
		pool.NormalizeMembers()
	}

	return pool, nil
}

func openAIMembers(accounts []domain.Account) []domain.AccountID {
	members := make([]domain.AccountID, 0, len(accounts))
	for _, account := range accounts {
		provider := strings.TrimSpace(strings.ToLower(account.Metadata.Provider))
		if provider == string(domain.ProviderOpenAI) || account.Auth.Method == domain.AuthMethodChatGPT {
			members = append(members, account.ID)
		}
	}
	return members
}

func weeklyPercent(account domain.Account) float64 {
	if account.Limits.Weekly == nil {
		return 0
	}
	return account.Limits.Weekly.Percent
}
