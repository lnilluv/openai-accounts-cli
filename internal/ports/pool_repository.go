package ports

import (
	"context"

	"github.com/bnema/openai-accounts-cli/internal/domain"
)

type PoolRepository interface {
	GetByID(ctx context.Context, id domain.PoolID) (domain.Pool, error)
	List(ctx context.Context) ([]domain.Pool, error)
	Save(ctx context.Context, pool domain.Pool) error
}

type PoolRuntimeRepository interface {
	GetByPoolID(ctx context.Context, poolID domain.PoolID) (domain.PoolRuntime, error)
	Save(ctx context.Context, runtime domain.PoolRuntime) error
}
