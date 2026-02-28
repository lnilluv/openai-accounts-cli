package application

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/bnema/openai-accounts-cli/internal/ports"
)

type SessionContinuityService struct {
	runtime ports.PoolRuntimeRepository
	clock   ports.Clock
}

func NewSessionContinuityService(runtime ports.PoolRuntimeRepository, clock ports.Clock) *SessionContinuityService {
	if clock == nil {
		clock = ports.SystemClock{}
	}

	return &SessionContinuityService{runtime: runtime, clock: clock}
}

func (s *SessionContinuityService) ResolveLogicalSessionID(workspaceRoot, windowFingerprint string) string {
	raw := strings.TrimSpace(workspaceRoot) + "|" + strings.TrimSpace(windowFingerprint)
	hash := sha1.Sum([]byte(raw))
	return hex.EncodeToString(hash[:])
}

func (s *SessionContinuityService) GetOrAttachAccountSession(ctx context.Context, poolID domain.PoolID, logicalSessionID string, accountID domain.AccountID) (string, bool, error) {
	runtime, err := s.loadRuntime(ctx, poolID)
	if err != nil {
		return "", false, err
	}

	ledger := runtime.Sessions[logicalSessionID]
	if ledger.LogicalSessionID == "" {
		ledger.LogicalSessionID = logicalSessionID
	}
	if ledger.AccountSessions == nil {
		ledger.AccountSessions = map[domain.AccountID]string{}
	}

	if sessionID, ok := ledger.AccountSessions[accountID]; ok && strings.TrimSpace(sessionID) != "" {
		return sessionID, false, nil
	}

	sessionID := fmt.Sprintf("%s:%s", logicalSessionID, accountID)
	ledger.AccountSessions[accountID] = sessionID
	runtime.Sessions[logicalSessionID] = ledger
	runtime.ActiveAccountID = accountID
	runtime.LastSyncedAt = s.clock.Now()

	if err := s.runtime.Save(ctx, runtime); err != nil {
		return "", false, fmt.Errorf("save pool runtime: %w", err)
	}

	return sessionID, true, nil
}

func (s *SessionContinuityService) UpdateMemoryPacket(ctx context.Context, poolID domain.PoolID, logicalSessionID string, memory domain.MemoryPacket) error {
	runtime, err := s.loadRuntime(ctx, poolID)
	if err != nil {
		return err
	}

	ledger := runtime.Sessions[logicalSessionID]
	if ledger.LogicalSessionID == "" {
		ledger.LogicalSessionID = logicalSessionID
	}
	memory.UpdatedAt = s.clock.Now()
	ledger.Memory = memory
	runtime.Sessions[logicalSessionID] = ledger
	runtime.LastSyncedAt = s.clock.Now()

	if err := s.runtime.Save(ctx, runtime); err != nil {
		return fmt.Errorf("save pool runtime: %w", err)
	}

	return nil
}

func (s *SessionContinuityService) loadRuntime(ctx context.Context, poolID domain.PoolID) (domain.PoolRuntime, error) {
	runtime, err := s.runtime.GetByPoolID(ctx, poolID)
	if err != nil {
		if err != domain.ErrPoolNotFound {
			return domain.PoolRuntime{}, err
		}
		runtime = domain.PoolRuntime{PoolID: poolID, Sessions: map[string]domain.SessionLedger{}}
	}
	if runtime.Sessions == nil {
		runtime.Sessions = map[string]domain.SessionLedger{}
	}

	return runtime, nil
}
