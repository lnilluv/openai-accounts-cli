package domain

import "time"

type MemoryPacket struct {
	Summary      string
	Decisions    []string
	PendingTasks []string
	LastCodeRefs []string
	UpdatedAt    time.Time
}

type SessionLedger struct {
	LogicalSessionID string
	AccountSessions  map[AccountID]string
	Memory           MemoryPacket
}

type PoolRuntime struct {
	PoolID          PoolID
	ActiveAccountID AccountID
	LastSyncedAt    time.Time
	Sessions        map[string]SessionLedger
}
