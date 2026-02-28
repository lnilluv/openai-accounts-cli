package application

import (
	"context"
	"testing"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionContinuityReuseMappedSession(t *testing.T) {
	t.Parallel()

	repo := &inMemoryPoolRuntimeRepo{runtimes: map[domain.PoolID]domain.PoolRuntime{
		"default-openai": {
			PoolID: "default-openai",
			Sessions: map[string]domain.SessionLedger{
				"proj-a": {
					LogicalSessionID: "proj-a",
					AccountSessions: map[domain.AccountID]string{
						"2": "session-2",
					},
				},
			},
		},
	}}

	svc := NewSessionContinuityService(repo, fixedClock{now: time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)})

	sessionID, bootstrapped, err := svc.GetOrAttachAccountSession(context.Background(), "default-openai", "proj-a", "2")
	require.NoError(t, err)
	assert.Equal(t, "session-2", sessionID)
	assert.False(t, bootstrapped)
}

func TestSessionContinuityBootstrapsMissingSessionAndSavesMemory(t *testing.T) {
	t.Parallel()

	repo := &inMemoryPoolRuntimeRepo{runtimes: map[domain.PoolID]domain.PoolRuntime{}}
	svc := NewSessionContinuityService(repo, fixedClock{now: time.Date(2026, 2, 28, 12, 30, 0, 0, time.UTC)})

	sessionID, bootstrapped, err := svc.GetOrAttachAccountSession(context.Background(), "default-openai", "proj-a", "3")
	require.NoError(t, err)
	assert.Equal(t, "proj-a:3", sessionID)
	assert.True(t, bootstrapped)

	err = svc.UpdateMemoryPacket(context.Background(), "default-openai", "proj-a", domain.MemoryPacket{
		Summary:      "working on api",
		Decisions:    []string{"use wrapper"},
		PendingTasks: []string{"write tests"},
		LastCodeRefs: []string{"cmd/run.go"},
	})
	require.NoError(t, err)

	runtime, err := repo.GetByPoolID(context.Background(), "default-openai")
	require.NoError(t, err)
	ledger := runtime.Sessions["proj-a"]
	assert.Equal(t, "proj-a:3", ledger.AccountSessions["3"])
	assert.Equal(t, "working on api", ledger.Memory.Summary)
	require.False(t, ledger.Memory.UpdatedAt.IsZero())
}

func TestSessionContinuityResolveLogicalSessionPerWindow(t *testing.T) {
	t.Parallel()

	svc := NewSessionContinuityService(&inMemoryPoolRuntimeRepo{runtimes: map[domain.PoolID]domain.PoolRuntime{}}, fixedClock{now: time.Now()})

	one := svc.ResolveLogicalSessionID("/repo/a", "window-1")
	two := svc.ResolveLogicalSessionID("/repo/a", "window-2")
	three := svc.ResolveLogicalSessionID("/repo/b", "window-1")

	assert.NotEqual(t, one, two)
	assert.NotEqual(t, one, three)
	assert.NotEmpty(t, one)
}

type inMemoryPoolRuntimeRepo struct {
	runtimes map[domain.PoolID]domain.PoolRuntime
}

func (r *inMemoryPoolRuntimeRepo) GetByPoolID(_ context.Context, poolID domain.PoolID) (domain.PoolRuntime, error) {
	runtime, ok := r.runtimes[poolID]
	if !ok {
		return domain.PoolRuntime{}, domain.ErrPoolNotFound
	}
	return runtime, nil
}

func (r *inMemoryPoolRuntimeRepo) Save(_ context.Context, runtime domain.PoolRuntime) error {
	if r.runtimes == nil {
		r.runtimes = map[domain.PoolID]domain.PoolRuntime{}
	}
	r.runtimes[runtime.PoolID] = runtime
	return nil
}
