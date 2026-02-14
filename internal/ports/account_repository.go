package ports

import (
	"context"

	"github.com/bnema/openai-accounts-cli/internal/domain"
)

type AccountRepository interface {
	GetByID(ctx context.Context, id domain.AccountID) (domain.Account, error)
	List(ctx context.Context) ([]domain.Account, error)
	Save(ctx context.Context, account domain.Account) error
}
