package toml

import "fmt"

const currentPoolsSchemaVersion = 1
const currentPoolRuntimeSchemaVersion = 1

type poolsFileSchema struct {
	Version int          `toml:"version"`
	Pools   []poolSchema `toml:"pools"`
}

func (s *poolsFileSchema) applyDefaults() {
	if s.Version == 0 {
		s.Version = currentPoolsSchemaVersion
	}
}

func (s poolsFileSchema) validateVersion() error {
	if s.Version > currentPoolsSchemaVersion {
		return fmt.Errorf("unsupported pools schema version %d (current %d)", s.Version, currentPoolsSchemaVersion)
	}

	return nil
}

type poolSchema struct {
	ID              string   `toml:"id"`
	Name            string   `toml:"name"`
	Provider        string   `toml:"provider"`
	Strategy        string   `toml:"strategy"`
	Active          bool     `toml:"active"`
	AutoSyncMembers bool     `toml:"auto_sync_members"`
	Members         []string `toml:"members"`
	UpdatedAt       string   `toml:"updated_at"`
}

type poolRuntimeFileSchema struct {
	Version  int                 `toml:"version"`
	Runtimes []poolRuntimeSchema `toml:"runtimes"`
}

func (s *poolRuntimeFileSchema) applyDefaults() {
	if s.Version == 0 {
		s.Version = currentPoolRuntimeSchemaVersion
	}
}

func (s poolRuntimeFileSchema) validateVersion() error {
	if s.Version > currentPoolRuntimeSchemaVersion {
		return fmt.Errorf("unsupported pool runtime schema version %d (current %d)", s.Version, currentPoolRuntimeSchemaVersion)
	}

	return nil
}

type poolRuntimeSchema struct {
	PoolID          string                `toml:"pool_id"`
	ActiveAccountID string                `toml:"active_account_id"`
	LastSyncedAt    string                `toml:"last_synced_at"`
	Sessions        []sessionLedgerSchema `toml:"sessions"`
}

type sessionLedgerSchema struct {
	LogicalSessionID string               `toml:"logical_session_id"`
	AccountSessions  []accountSessionPair `toml:"account_sessions"`
	Memory           memoryPacketSchema   `toml:"memory"`
}

type accountSessionPair struct {
	AccountID string `toml:"account_id"`
	SessionID string `toml:"session_id"`
}

type memoryPacketSchema struct {
	Summary      string   `toml:"summary"`
	Decisions    []string `toml:"decisions"`
	PendingTasks []string `toml:"pending_tasks"`
	LastCodeRefs []string `toml:"last_code_refs"`
	UpdatedAt    string   `toml:"updated_at"`
}
