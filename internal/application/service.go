package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/bnema/openai-accounts-cli/internal/ports"
)

var ErrUnsupportedWindowKind = errors.New("unsupported limit window kind")

type Service struct {
	repo  ports.AccountRepository
	store ports.SecretStore
	clock ports.Clock
}

func NewService(repo ports.AccountRepository, store ports.SecretStore, clock ports.Clock) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}

	return &Service{
		repo:  repo,
		store: store,
		clock: clock,
	}
}

func (s *Service) SetAuth(ctx context.Context, id domain.AccountID, method domain.AuthMethod, secretKey, secretValue string) error {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if !errors.Is(err, domain.ErrAccountNotFound) {
			return fmt.Errorf("get account by id: %w", err)
		}
		account = domain.Account{ID: id, Name: fmt.Sprintf("Account %s", id)}
	}
	originalAccount := account

	previousSecretRefs := uniqueSecretRefs(account.Metadata.SecretRef, account.Auth.SecretRef)

	if err := s.store.Put(ctx, secretKey, secretValue); err != nil {
		return fmt.Errorf("store auth secret: %w", err)
	}

	account.Auth = domain.Auth{
		Method:    method,
		SecretRef: secretKey,
	}
	account.Metadata.SecretRef = secretKey

	if err := s.repo.Save(ctx, account); err != nil {
		if rollbackErr := s.store.Delete(ctx, secretKey); rollbackErr != nil {
			return fmt.Errorf("save account auth and rollback stored secret: %w", errors.Join(err, rollbackErr))
		}

		return fmt.Errorf("save account auth: %w", err)
	}

	for _, previousSecretRef := range previousSecretRefs {
		if previousSecretRef == secretKey {
			continue
		}
		if err := s.store.Delete(ctx, previousSecretRef); err != nil {
			remaining := remainingSecretRefs(previousSecretRefs, previousSecretRef)
			restoreAccount := originalAccount
			applySecretRefs(&restoreAccount, remaining)
			if len(remaining) == 0 {
				restoreAccount.Auth.Method = ""
			} else {
				restoreAccount.Auth.Method = originalAccount.Auth.Method
			}

			var rollbackErr error
			if restoreErr := s.repo.Save(ctx, restoreAccount); restoreErr != nil {
				rollbackErr = errors.Join(rollbackErr, restoreErr)
			}
			if newSecretDeleteErr := s.store.Delete(ctx, secretKey); newSecretDeleteErr != nil {
				rollbackErr = errors.Join(rollbackErr, newSecretDeleteErr)
			}
			if rollbackErr != nil {
				return fmt.Errorf("delete previous auth secret and rollback auth update: %w", errors.Join(err, rollbackErr))
			}
			return fmt.Errorf("delete previous auth secret: %w", err)
		}
	}

	return nil
}

func (s *Service) RemoveAuth(ctx context.Context, id domain.AccountID) error {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get account by id: %w", err)
	}
	originalAccount := account

	secretRefs := uniqueSecretRefs(account.Metadata.SecretRef, account.Auth.SecretRef)

	account.Auth = domain.Auth{}
	account.Metadata.SecretRef = ""

	if err := s.repo.Save(ctx, account); err != nil {
		return fmt.Errorf("save account auth: %w", err)
	}

	if len(secretRefs) == 0 {
		return nil
	}

	for _, secretRef := range secretRefs {
		if err := s.store.Delete(ctx, secretRef); err != nil {
			remaining := remainingSecretRefs(secretRefs, secretRef)
			restoreAccount := account
			applySecretRefs(&restoreAccount, remaining)
			if len(remaining) > 0 {
				restoreAccount.Auth.Method = originalAccount.Auth.Method
			}
			if restoreErr := s.repo.Save(ctx, restoreAccount); restoreErr != nil {
				return fmt.Errorf("delete auth secret and restore remaining refs: %w", errors.Join(err, restoreErr))
			}
			return fmt.Errorf("delete auth secret: %w", err)
		}
	}

	return nil
}

func uniqueSecretRefs(secretRefs ...string) []string {
	result := make([]string, 0, len(secretRefs))
	seen := make(map[string]struct{}, len(secretRefs))

	for _, secretRef := range secretRefs {
		if secretRef == "" {
			continue
		}
		if _, ok := seen[secretRef]; ok {
			continue
		}

		seen[secretRef] = struct{}{}
		result = append(result, secretRef)
	}

	return result
}

func remainingSecretRefs(secretRefs []string, failed string) []string {
	for i, secretRef := range secretRefs {
		if secretRef == failed {
			return secretRefs[i:]
		}
	}
	return nil
}

func applySecretRefs(account *domain.Account, secretRefs []string) {
	account.Metadata.SecretRef = ""
	account.Auth.SecretRef = ""

	if len(secretRefs) > 0 {
		account.Metadata.SecretRef = secretRefs[0]
		account.Auth.SecretRef = secretRefs[0]
	}
	if len(secretRefs) > 1 {
		account.Auth.SecretRef = secretRefs[1]
	}
}

func (s *Service) SetUsage(ctx context.Context, id domain.AccountID, usage domain.Usage) error {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get account by id: %w", err)
	}

	account.Usage = usage

	if err := s.repo.Save(ctx, account); err != nil {
		return fmt.Errorf("save account usage: %w", err)
	}

	return nil
}

func (s *Service) SetAccountName(ctx context.Context, id domain.AccountID, name string) error {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get account by id: %w", err)
	}

	account.Name = name

	if err := s.repo.Save(ctx, account); err != nil {
		return fmt.Errorf("save account name: %w", err)
	}

	return nil
}

func (s *Service) SetAccountPlanType(ctx context.Context, id domain.AccountID, planType string) error {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get account by id: %w", err)
	}

	account.Metadata.PlanType = planType

	if err := s.repo.Save(ctx, account); err != nil {
		return fmt.Errorf("save account plan type: %w", err)
	}

	return nil
}

func (s *Service) SetLimit(ctx context.Context, id domain.AccountID, kind LimitWindowKind, percent float64, resetsAt, capturedAt time.Time) error {
	if !kind.Valid() {
		return fmt.Errorf("%w: %q", ErrUnsupportedWindowKind, kind)
	}

	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get account by id: %w", err)
	}

	if capturedAt.IsZero() {
		capturedAt = s.clock.Now()
	}

	snapshot := &domain.AccountLimitSnapshot{
		Percent:    percent,
		ResetsAt:   resetsAt,
		CapturedAt: capturedAt,
	}
	switch kind {
	case LimitWindowDaily:
		account.Limits.Daily = snapshot
	case LimitWindowWeekly:
		account.Limits.Weekly = snapshot
	}

	if err := s.repo.Save(ctx, account); err != nil {
		return fmt.Errorf("save account limit: %w", err)
	}

	return nil
}

func (s *Service) GetStatus(ctx context.Context, id domain.AccountID) (Status, error) {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Status{}, fmt.Errorf("get account by id: %w", err)
	}

	return statusFromAccount(account), nil
}

func (s *Service) GetStatusAll(ctx context.Context) ([]Status, error) {
	accounts, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	statuses := make([]Status, 0, len(accounts))
	for _, account := range accounts {
		statuses = append(statuses, statusFromAccount(account))
	}

	return statuses, nil
}

func statusFromAccount(account domain.Account) Status {
	return Status{
		Account:     account,
		Usage:       account.Usage,
		DailyLimit:  toStatusLimit(LimitWindowDaily, account.Limits.Daily),
		WeeklyLimit: toStatusLimit(LimitWindowWeekly, account.Limits.Weekly),
	}
}

func toStatusLimit(kind LimitWindowKind, snapshot *domain.AccountLimitSnapshot) *StatusLimit {
	if snapshot == nil {
		return nil
	}

	return &StatusLimit{
		Window:     kind,
		Percent:    snapshot.Percent,
		ResetsAt:   snapshot.ResetsAt,
		CapturedAt: snapshot.CapturedAt,
	}
}
