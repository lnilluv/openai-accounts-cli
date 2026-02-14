package pass

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorePutUsesPassInsert(t *testing.T) {
	t.Parallel()

	called := false
	store := &Store{
		run: func(ctx context.Context, input string, args ...string) (string, string, error) {
			called = true
			assert.Equal(t, context.Background(), ctx)
			assert.Equal(t, []string{"insert", "-m", "-f", "codex/oa/accounts/acc-1/api_key"}, args)
			assert.Equal(t, "top-secret\n", input)
			return "", "", nil
		},
	}

	err := store.Put(context.Background(), "codex/oa/accounts/acc-1/api_key", "top-secret")
	require.NoError(t, err)
	assert.True(t, called)
}

func TestStoreGetUsesPassShowAndTrimsTrailingNewline(t *testing.T) {
	t.Parallel()

	store := &Store{
		run: func(ctx context.Context, input string, args ...string) (string, string, error) {
			assert.Equal(t, []string{"show", "codex/oa/accounts/acc-1/api_key"}, args)
			assert.Empty(t, input)
			return "top-secret\n", "", nil
		},
	}

	value, err := store.Get(context.Background(), "codex/oa/accounts/acc-1/api_key")
	require.NoError(t, err)
	assert.Equal(t, "top-secret", value)
}

func TestStoreDeleteUsesPassRemove(t *testing.T) {
	t.Parallel()

	store := &Store{
		run: func(ctx context.Context, input string, args ...string) (string, string, error) {
			assert.Equal(t, []string{"rm", "-f", "codex/oa/accounts/acc-1/api_key"}, args)
			assert.Empty(t, input)
			return "", "", nil
		},
	}

	err := store.Delete(context.Background(), "codex/oa/accounts/acc-1/api_key")
	require.NoError(t, err)
}

func TestStoreGetReturnsClearError(t *testing.T) {
	t.Parallel()

	store := &Store{
		run: func(ctx context.Context, input string, args ...string) (string, string, error) {
			return "", "entry not found", errors.New("exit status 1")
		},
	}

	_, err := store.Get(context.Background(), "codex/oa/accounts/acc-1/api_key")
	require.Error(t, err)
	assert.ErrorContains(t, err, "pass get")
	assert.ErrorContains(t, err, "codex/oa/accounts/acc-1/api_key")
	assert.ErrorContains(t, err, "entry not found")
}
