package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bnema/openai-accounts-cli/internal/ports"
)

const (
	storeDirMode  = 0o700
	secretFileMod = 0o600
)

type Store struct {
	root string
	mu   sync.RWMutex
}

var _ ports.SecretStore = (*Store)(nil)

func NewStore(root string) *Store {
	return &Store{root: filepath.Clean(root)}
}

func (s *Store) Put(ctx context.Context, key string, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.pathForKey(key)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), storeDirMode); err != nil {
		return fmt.Errorf("create file secret directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(value), secretFileMod); err != nil {
		return fmt.Errorf("write file secret %q: %w", key, err)
	}

	return nil
}

func (s *Store) Get(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	path, err := s.pathForKey(key)
	if err != nil {
		return "", err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("file secret %q not found: %w", key, err)
		}
		return "", fmt.Errorf("read file secret %q: %w", key, err)
	}

	return string(data), nil
}

func (s *Store) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.pathForKey(key)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err = os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete file secret %q: %w", key, err)
	}

	return nil
}

func (s *Store) pathForKey(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", errors.New("secret key is empty")
	}

	cleaned := filepath.Clean(trimmed)
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") || cleaned == "." {
		return "", fmt.Errorf("invalid secret key %q", key)
	}

	return filepath.Join(s.root, cleaned), nil
}
