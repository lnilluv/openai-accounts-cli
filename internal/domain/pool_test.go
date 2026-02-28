package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPoolValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pool    Pool
		wantErr string
	}{
		{
			name: "valid",
			pool: Pool{ID: "default-openai", Name: "default", Provider: ProviderOpenAI, Strategy: PoolStrategyLeastWeeklyUsed},
		},
		{
			name:    "missing id",
			pool:    Pool{Name: "default", Provider: ProviderOpenAI, Strategy: PoolStrategyLeastWeeklyUsed},
			wantErr: "id is required",
		},
		{
			name:    "missing provider",
			pool:    Pool{ID: "default-openai", Name: "default", Strategy: PoolStrategyLeastWeeklyUsed},
			wantErr: "provider is required",
		},
		{
			name:    "unsupported provider",
			pool:    Pool{ID: "default-openai", Name: "default", Provider: "foo", Strategy: PoolStrategyLeastWeeklyUsed},
			wantErr: "unsupported provider",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.pool.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestPoolNormalizeMembersDeduplicatesAndDropsEmpty(t *testing.T) {
	t.Parallel()

	pool := Pool{Members: []AccountID{"1", "", "2", "1", "2", "3"}}
	pool.NormalizeMembers()

	assert.Equal(t, []AccountID{"1", "2", "3"}, pool.Members)
}
