package toml

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/bnema/openai-accounts-cli/internal/ports"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
)

const (
	poolsPathKey    = "pools.path"
	poolConfigFile  = "pools.toml"
	poolRuntimePath = "pool.runtime.path"
	runtimeFileName = "pool_runtime.toml"
)

type PoolRepository struct {
	path string
	mu   *sync.RWMutex
}

var _ ports.PoolRepository = (*PoolRepository)(nil)

func NewPoolRepository(cfg *viper.Viper) (*PoolRepository, error) {
	if cfg == nil {
		cfg = viper.New()
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	path := cfg.GetString(poolsPathKey)
	if path == "" {
		path = filepath.Join(homeDir, accountsConfigDir, poolConfigFile)
	}

	path, err = normalizeAccountsPath(path)
	if err != nil {
		return nil, err
	}

	return &PoolRepository{path: path, mu: lockForPath(path)}, nil
}

func (r *PoolRepository) Save(ctx context.Context, pool domain.Pool) error {
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

	encoded := toPoolSchema(pool)
	updated := false
	for i := range file.Pools {
		if file.Pools[i].ID == encoded.ID {
			file.Pools[i] = encoded
			updated = true
			break
		}
	}
	if !updated {
		file.Pools = append(file.Pools, encoded)
	}

	return writeTOMLFile(r.path, file)
}

func (r *PoolRepository) GetByID(ctx context.Context, id domain.PoolID) (domain.Pool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Pool{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	file, err := r.readSchema()
	if err != nil {
		return domain.Pool{}, err
	}

	for _, entry := range file.Pools {
		if entry.ID == string(id) {
			return fromPoolSchema(entry), nil
		}
	}

	return domain.Pool{}, domain.ErrPoolNotFound
}

func (r *PoolRepository) List(ctx context.Context) ([]domain.Pool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	file, err := r.readSchema()
	if err != nil {
		return nil, err
	}

	pools := make([]domain.Pool, 0, len(file.Pools))
	for _, entry := range file.Pools {
		pools = append(pools, fromPoolSchema(entry))
	}

	return pools, nil
}

func (r *PoolRepository) readSchema() (poolsFileSchema, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return poolsFileSchema{}, nil
		}
		return poolsFileSchema{}, fmt.Errorf("read pools file: %w", err)
	}

	var file poolsFileSchema
	if err := toml.Unmarshal(data, &file); err != nil {
		return poolsFileSchema{}, fmt.Errorf("decode pools file: %w", err)
	}
	if err := file.validateVersion(); err != nil {
		return poolsFileSchema{}, err
	}
	file.applyDefaults()

	return file, nil
}

type PoolRuntimeRepository struct {
	path string
	mu   *sync.RWMutex
}

var _ ports.PoolRuntimeRepository = (*PoolRuntimeRepository)(nil)

func NewPoolRuntimeRepository(cfg *viper.Viper) (*PoolRuntimeRepository, error) {
	if cfg == nil {
		cfg = viper.New()
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	path := cfg.GetString(poolRuntimePath)
	if path == "" {
		path = filepath.Join(homeDir, accountsConfigDir, runtimeFileName)
	}

	path, err = normalizeAccountsPath(path)
	if err != nil {
		return nil, err
	}

	return &PoolRuntimeRepository{path: path, mu: lockForPath(path)}, nil
}

func (r *PoolRuntimeRepository) GetByPoolID(ctx context.Context, poolID domain.PoolID) (domain.PoolRuntime, error) {
	if err := ctx.Err(); err != nil {
		return domain.PoolRuntime{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	file, err := r.readSchema()
	if err != nil {
		return domain.PoolRuntime{}, err
	}

	for _, runtime := range file.Runtimes {
		if runtime.PoolID == string(poolID) {
			return fromPoolRuntimeSchema(runtime), nil
		}
	}

	return domain.PoolRuntime{}, domain.ErrPoolNotFound
}

func (r *PoolRuntimeRepository) Save(ctx context.Context, runtime domain.PoolRuntime) error {
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

	encoded := toPoolRuntimeSchema(runtime)
	updated := false
	for i := range file.Runtimes {
		if file.Runtimes[i].PoolID == encoded.PoolID {
			file.Runtimes[i] = encoded
			updated = true
			break
		}
	}
	if !updated {
		file.Runtimes = append(file.Runtimes, encoded)
	}

	return writeTOMLFile(r.path, file)
}

func (r *PoolRuntimeRepository) readSchema() (poolRuntimeFileSchema, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return poolRuntimeFileSchema{}, nil
		}
		return poolRuntimeFileSchema{}, fmt.Errorf("read pool runtime file: %w", err)
	}

	var file poolRuntimeFileSchema
	if err := toml.Unmarshal(data, &file); err != nil {
		return poolRuntimeFileSchema{}, fmt.Errorf("decode pool runtime file: %w", err)
	}
	if err := file.validateVersion(); err != nil {
		return poolRuntimeFileSchema{}, err
	}
	file.applyDefaults()

	return file, nil
}

func writeTOMLFile(path string, file any) error {
	if err := os.MkdirAll(filepath.Dir(path), accountsDirMode); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	data, err := toml.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode file: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), tempFilePattern)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
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
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tempFile.Chmod(accountsFileMode); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}

	cleanup = false

	if err := os.Chmod(path, accountsFileMode); err != nil {
		return fmt.Errorf("chmod file: %w", err)
	}

	return nil
}

func toPoolSchema(pool domain.Pool) poolSchema {
	members := make([]string, 0, len(pool.Members))
	for _, member := range pool.Members {
		members = append(members, string(member))
	}

	return poolSchema{
		ID:              string(pool.ID),
		Name:            pool.Name,
		Provider:        string(pool.Provider),
		Strategy:        string(pool.Strategy),
		Active:          pool.Active,
		AutoSyncMembers: pool.AutoSyncMembers,
		Members:         members,
		UpdatedAt:       formatTime(pool.UpdatedAt),
	}
}

func fromPoolSchema(schema poolSchema) domain.Pool {
	members := make([]domain.AccountID, 0, len(schema.Members))
	for _, member := range schema.Members {
		members = append(members, domain.AccountID(member))
	}

	return domain.Pool{
		ID:              domain.PoolID(schema.ID),
		Name:            schema.Name,
		Provider:        domain.Provider(schema.Provider),
		Strategy:        domain.PoolStrategy(schema.Strategy),
		Active:          schema.Active,
		AutoSyncMembers: schema.AutoSyncMembers,
		Members:         members,
		UpdatedAt:       parseTime(schema.UpdatedAt),
	}
}

func toPoolRuntimeSchema(runtime domain.PoolRuntime) poolRuntimeSchema {
	sessions := make([]sessionLedgerSchema, 0, len(runtime.Sessions))
	for _, session := range runtime.Sessions {
		pairs := make([]accountSessionPair, 0, len(session.AccountSessions))
		for accountID, sessionID := range session.AccountSessions {
			pairs = append(pairs, accountSessionPair{AccountID: string(accountID), SessionID: sessionID})
		}

		sessions = append(sessions, sessionLedgerSchema{
			LogicalSessionID: session.LogicalSessionID,
			AccountSessions:  pairs,
			Memory: memoryPacketSchema{
				Summary:      session.Memory.Summary,
				Decisions:    session.Memory.Decisions,
				PendingTasks: session.Memory.PendingTasks,
				LastCodeRefs: session.Memory.LastCodeRefs,
				UpdatedAt:    formatTime(session.Memory.UpdatedAt),
			},
		})
	}

	return poolRuntimeSchema{
		PoolID:          string(runtime.PoolID),
		ActiveAccountID: string(runtime.ActiveAccountID),
		LastSyncedAt:    formatTime(runtime.LastSyncedAt),
		Sessions:        sessions,
	}
}

func fromPoolRuntimeSchema(schema poolRuntimeSchema) domain.PoolRuntime {
	sessions := make(map[string]domain.SessionLedger, len(schema.Sessions))
	for _, session := range schema.Sessions {
		accountSessions := make(map[domain.AccountID]string, len(session.AccountSessions))
		for _, pair := range session.AccountSessions {
			accountSessions[domain.AccountID(pair.AccountID)] = pair.SessionID
		}

		sessions[session.LogicalSessionID] = domain.SessionLedger{
			LogicalSessionID: session.LogicalSessionID,
			AccountSessions:  accountSessions,
			Memory: domain.MemoryPacket{
				Summary:      session.Memory.Summary,
				Decisions:    session.Memory.Decisions,
				PendingTasks: session.Memory.PendingTasks,
				LastCodeRefs: session.Memory.LastCodeRefs,
				UpdatedAt:    parseTime(session.Memory.UpdatedAt),
			},
		}
	}

	return domain.PoolRuntime{
		PoolID:          domain.PoolID(schema.PoolID),
		ActiveAccountID: domain.AccountID(schema.ActiveAccountID),
		LastSyncedAt:    parseTime(schema.LastSyncedAt),
		Sessions:        sessions,
	}
}
