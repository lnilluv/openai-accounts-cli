package toml

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/bnema/openai-accounts-cli/internal/ports"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
)

const (
	configName         = "config"
	configType         = "toml"
	accountsPathKey    = "accounts.path"
	accountsFileMode   = 0o600
	accountsDirMode    = 0o700
	accountsConfigDir  = ".codex"
	accountsConfigFile = "accounts.toml"
	tempFilePattern    = ".accounts-*.toml.tmp"
)

type Repository struct {
	accountsPath string
	mu           *sync.RWMutex
}

var (
	lockRegistryMu sync.Mutex
	pathLockMap    = map[string]*sync.RWMutex{}
)

var _ ports.AccountRepository = (*Repository)(nil)

func NewRepository(cfg *viper.Viper) (*Repository, error) {
	if cfg == nil {
		cfg = viper.New()
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	defaultPath := filepath.Join(homeDir, accountsConfigDir, accountsConfigFile)

	cfg.SetConfigName(configName)
	cfg.SetConfigType(configType)
	cfg.AddConfigPath(filepath.Join(homeDir, accountsConfigDir))
	cfg.SetDefault(accountsPathKey, defaultPath)

	err = cfg.ReadInConfig()
	if err != nil {
		var configNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &configNotFound) {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	accountsPath := cfg.GetString(accountsPathKey)
	if accountsPath == "" {
		return nil, errors.New("accounts path is empty")
	}
	accountsPath, err = normalizeAccountsPath(accountsPath)
	if err != nil {
		return nil, err
	}

	return &Repository{accountsPath: accountsPath, mu: lockForPath(accountsPath)}, nil
}

func (r *Repository) Save(ctx context.Context, account domain.Account) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	file, err := r.readSchema()
	if err != nil {
		return err
	}
	file.applyDefaults()

	encoded := toSchema(account)
	updated := false
	for i := range file.Accounts {
		if file.Accounts[i].ID == encoded.ID {
			file.Accounts[i] = encoded
			updated = true
			break
		}
	}

	if !updated {
		file.Accounts = append(file.Accounts, encoded)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := r.writeSchema(file); err != nil {
		return err
	}

	return nil
}

func (r *Repository) GetByID(ctx context.Context, id domain.AccountID) (domain.Account, error) {
	if err := ctx.Err(); err != nil {
		return domain.Account{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	file, err := r.readSchema()
	if err != nil {
		return domain.Account{}, err
	}
	file.applyDefaults()

	for _, entry := range file.Accounts {
		if entry.ID == string(id) {
			return fromSchema(entry), nil
		}
	}

	return domain.Account{}, domain.ErrAccountNotFound
}

func (r *Repository) List(ctx context.Context) ([]domain.Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	file, err := r.readSchema()
	if err != nil {
		return nil, err
	}
	file.applyDefaults()

	accounts := make([]domain.Account, 0, len(file.Accounts))
	for _, entry := range file.Accounts {
		accounts = append(accounts, fromSchema(entry))
	}

	return accounts, nil
}

func (r *Repository) readSchema() (fileSchema, error) {
	data, err := os.ReadFile(r.accountsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileSchema{}, nil
		}
		return fileSchema{}, fmt.Errorf("read accounts file: %w", err)
	}

	var file fileSchema
	if err := toml.Unmarshal(data, &file); err != nil {
		return fileSchema{}, fmt.Errorf("decode accounts file: %w", err)
	}
	if err := file.validateVersion(); err != nil {
		return fileSchema{}, err
	}
	file.applyDefaults()

	return file, nil
}

func normalizeAccountsPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve accounts path: %w", err)
	}

	return filepath.Clean(absPath), nil
}

func lockForPath(path string) *sync.RWMutex {
	lockRegistryMu.Lock()
	defer lockRegistryMu.Unlock()

	if mu, ok := pathLockMap[path]; ok {
		return mu
	}

	mu := &sync.RWMutex{}
	pathLockMap[path] = mu
	return mu
}

func (r *Repository) writeSchema(file fileSchema) error {
	file.applyDefaults()

	if err := os.MkdirAll(filepath.Dir(r.accountsPath), accountsDirMode); err != nil {
		return fmt.Errorf("create accounts directory: %w", err)
	}

	data, err := toml.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode accounts file: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(r.accountsPath), tempFilePattern)
	if err != nil {
		return fmt.Errorf("create temp accounts file: %w", err)
	}

	tempName := tempFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp accounts file: %w", err)
	}

	if err := tempFile.Chmod(accountsFileMode); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("chmod temp accounts file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp accounts file: %w", err)
	}

	if err := os.Rename(tempName, r.accountsPath); err != nil {
		return fmt.Errorf("replace accounts file: %w", err)
	}

	cleanup = false

	if err := os.Chmod(r.accountsPath, accountsFileMode); err != nil {
		return fmt.Errorf("chmod accounts file: %w", err)
	}

	return nil
}

func toSchema(account domain.Account) accountSchema {
	limits := limitsSchema{}
	if account.Limits.Daily != nil {
		limits.Daily = toLimitSnapshotSchema(account.Limits.Daily)
	}
	if account.Limits.Weekly != nil {
		limits.Weekly = toLimitSnapshotSchema(account.Limits.Weekly)
	}

	return accountSchema{
		ID:   string(account.ID),
		Name: account.Name,
		Metadata: metadataSchema{
			Provider:  account.Metadata.Provider,
			Model:     account.Metadata.Model,
			SecretRef: account.Metadata.SecretRef,
			PlanType:  account.Metadata.PlanType,
		},
		Auth: authSchema{
			Method:    string(account.Auth.Method),
			SecretRef: account.Auth.SecretRef,
		},
		Usage: usageSchema{
			InputTokens:       account.Usage.InputTokens,
			OutputTokens:      account.Usage.OutputTokens,
			CachedInputTokens: account.Usage.CachedInputTokens,
		},
		Limits: limits,
	}
}

func fromSchema(account accountSchema) domain.Account {
	metadataSecretRef := account.Metadata.SecretRef
	if metadataSecretRef == "" {
		metadataSecretRef = account.Auth.SecretRef
	}

	authSecretRef := account.Auth.SecretRef
	if authSecretRef == "" {
		authSecretRef = account.Metadata.SecretRef
	}

	return domain.Account{
		ID:   domain.AccountID(account.ID),
		Name: account.Name,
		Metadata: domain.AccountMetadata{
			Provider:  account.Metadata.Provider,
			Model:     account.Metadata.Model,
			SecretRef: metadataSecretRef,
			PlanType:  account.Metadata.PlanType,
		},
		Auth: domain.Auth{
			Method:    domain.AuthMethod(account.Auth.Method),
			SecretRef: authSecretRef,
		},
		Usage: domain.Usage{
			InputTokens:       account.Usage.InputTokens,
			OutputTokens:      account.Usage.OutputTokens,
			CachedInputTokens: account.Usage.CachedInputTokens,
		},
		Limits: domain.AccountLimitSnapshots{
			Daily:  fromLimitSnapshotSchema(account.Limits.Daily),
			Weekly: fromLimitSnapshotSchema(account.Limits.Weekly),
		},
	}
}

func toLimitSnapshotSchema(snapshot *domain.AccountLimitSnapshot) *limitSnapshotSchema {
	if snapshot == nil {
		return nil
	}

	return &limitSnapshotSchema{
		Percent:    snapshot.Percent,
		ResetsAt:   formatTime(snapshot.ResetsAt),
		CapturedAt: formatTime(snapshot.CapturedAt),
	}
}

func fromLimitSnapshotSchema(snapshot *limitSnapshotSchema) *domain.AccountLimitSnapshot {
	if snapshot == nil {
		return nil
	}

	return &domain.AccountLimitSnapshot{
		Percent:    snapshot.Percent,
		ResetsAt:   parseTime(snapshot.ResetsAt),
		CapturedAt: parseTime(snapshot.CapturedAt),
	}
}

func parseTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}

	return parsed
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.Format(time.RFC3339)
}
