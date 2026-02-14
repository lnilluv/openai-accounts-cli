package chain

import (
	"context"
	"errors"
	"fmt"

	filestore "github.com/bnema/openai-accounts-cli/internal/adapters/secrets/file"
	passstore "github.com/bnema/openai-accounts-cli/internal/adapters/secrets/pass"
	"github.com/bnema/openai-accounts-cli/internal/ports"
)

type Store struct {
	primary  ports.SecretStore
	fallback ports.SecretStore
}

var _ ports.SecretStore = (*Store)(nil)

var (
	errNilPrimaryStore  = errors.New("primary secret store is nil")
	errNilFallbackStore = errors.New("fallback secret store is nil")
)

func NewStore(primary ports.SecretStore, fallback ports.SecretStore) *Store {
	store, err := NewStoreChecked(primary, fallback)
	if err != nil {
		panic(err)
	}

	return store
}

func NewStoreChecked(primary ports.SecretStore, fallback ports.SecretStore) (*Store, error) {
	if primary == nil {
		return nil, errNilPrimaryStore
	}
	if fallback == nil {
		return nil, errNilFallbackStore
	}

	return &Store{primary: primary, fallback: fallback}, nil
}

func NewPassFirstWithFileFallback(fileRoot string) (*Store, error) {
	return NewStoreChecked(passstore.NewStore(), filestore.NewStore(fileRoot))
}

func (s *Store) Put(ctx context.Context, key string, value string) error {
	err := s.primary.Put(ctx, key, value)
	if err == nil {
		return nil
	}
	if shouldSkipFallback(err) {
		return err
	}

	fallbackErr := s.fallback.Put(ctx, key, value)
	if fallbackErr == nil {
		return nil
	}

	return fmt.Errorf("primary backend put failed: %w; fallback backend put failed: %w", err, fallbackErr)
}

func (s *Store) Get(ctx context.Context, key string) (string, error) {
	value, err := s.primary.Get(ctx, key)
	if err == nil {
		return value, nil
	}
	if shouldSkipFallback(err) {
		return "", err
	}

	fallbackValue, fallbackErr := s.fallback.Get(ctx, key)
	if fallbackErr == nil {
		return fallbackValue, nil
	}

	return "", fmt.Errorf("primary backend get failed: %w; fallback backend get failed: %w", err, fallbackErr)
}

func (s *Store) Delete(ctx context.Context, key string) error {
	err := s.primary.Delete(ctx, key)
	if err == nil {
		return nil
	}
	if shouldSkipFallback(err) {
		return err
	}

	fallbackErr := s.fallback.Delete(ctx, key)
	if fallbackErr == nil {
		return nil
	}

	return fmt.Errorf("primary backend delete failed: %w; fallback backend delete failed: %w", err, fallbackErr)
}

func shouldSkipFallback(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
