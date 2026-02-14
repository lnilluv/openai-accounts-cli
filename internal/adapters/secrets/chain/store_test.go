package chain

import (
	"context"
	"errors"
	"testing"

	portmocks "github.com/bnema/openai-accounts-cli/internal/ports/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStoreGetUsesPrimaryWhenItSucceeds(t *testing.T) {
	t.Parallel()

	primary := portmocks.NewMockSecretStore(t)
	fallback := portmocks.NewMockSecretStore(t)
	store := NewStore(primary, fallback)

	primary.EXPECT().Get(mock.Anything, "codex/oa/accounts/acc-1/api_key").Return("from-pass", nil).Once()

	value, err := store.Get(context.Background(), "codex/oa/accounts/acc-1/api_key")
	require.NoError(t, err)
	assert.Equal(t, "from-pass", value)
}

func TestStoreGetFallsBackWhenPrimaryFails(t *testing.T) {
	t.Parallel()

	primary := portmocks.NewMockSecretStore(t)
	fallback := portmocks.NewMockSecretStore(t)
	store := NewStore(primary, fallback)

	primary.EXPECT().Get(mock.Anything, "codex/oa/accounts/acc-1/api_key").Return("", errors.New("pass unavailable")).Once()
	fallback.EXPECT().Get(mock.Anything, "codex/oa/accounts/acc-1/api_key").Return("from-file", nil).Once()

	value, err := store.Get(context.Background(), "codex/oa/accounts/acc-1/api_key")
	require.NoError(t, err)
	assert.Equal(t, "from-file", value)
}

func TestStoreGetReturnsCombinedErrorWhenBothBackendsFail(t *testing.T) {
	t.Parallel()

	primary := portmocks.NewMockSecretStore(t)
	fallback := portmocks.NewMockSecretStore(t)
	store := NewStore(primary, fallback)

	primary.EXPECT().Get(mock.Anything, "codex/oa/accounts/acc-1/api_key").Return("", errors.New("pass failed")).Once()
	fallback.EXPECT().Get(mock.Anything, "codex/oa/accounts/acc-1/api_key").Return("", errors.New("file failed")).Once()

	_, err := store.Get(context.Background(), "codex/oa/accounts/acc-1/api_key")
	require.Error(t, err)
	assert.ErrorContains(t, err, "primary backend")
	assert.ErrorContains(t, err, "fallback backend")
	assert.ErrorContains(t, err, "pass failed")
	assert.ErrorContains(t, err, "file failed")
}

func TestStorePutFallsBackWhenPrimaryFails(t *testing.T) {
	t.Parallel()

	primary := portmocks.NewMockSecretStore(t)
	fallback := portmocks.NewMockSecretStore(t)
	store := NewStore(primary, fallback)

	primary.EXPECT().Put(mock.Anything, "codex/oa/accounts/acc-1/api_key", "secret").Return(errors.New("pass failed")).Once()
	fallback.EXPECT().Put(mock.Anything, "codex/oa/accounts/acc-1/api_key", "secret").Return(nil).Once()

	err := store.Put(context.Background(), "codex/oa/accounts/acc-1/api_key", "secret")
	require.NoError(t, err)
}

func TestStorePutDoesNotCallFallbackWhenPrimarySucceeds(t *testing.T) {
	t.Parallel()

	primary := portmocks.NewMockSecretStore(t)
	fallback := portmocks.NewMockSecretStore(t)
	store := NewStore(primary, fallback)

	primary.EXPECT().Put(mock.Anything, "codex/oa/accounts/acc-1/api_key", "secret").Return(nil).Once()

	err := store.Put(context.Background(), "codex/oa/accounts/acc-1/api_key", "secret")
	require.NoError(t, err)
}

func TestStoreDeleteFallsBackWhenPrimaryFails(t *testing.T) {
	t.Parallel()

	primary := portmocks.NewMockSecretStore(t)
	fallback := portmocks.NewMockSecretStore(t)
	store := NewStore(primary, fallback)

	primary.EXPECT().Delete(mock.Anything, "codex/oa/accounts/acc-1/api_key").Return(errors.New("pass failed")).Once()
	fallback.EXPECT().Delete(mock.Anything, "codex/oa/accounts/acc-1/api_key").Return(nil).Once()

	err := store.Delete(context.Background(), "codex/oa/accounts/acc-1/api_key")
	require.NoError(t, err)
}

func TestStoreDeleteDoesNotCallFallbackWhenPrimarySucceeds(t *testing.T) {
	t.Parallel()

	primary := portmocks.NewMockSecretStore(t)
	fallback := portmocks.NewMockSecretStore(t)
	store := NewStore(primary, fallback)

	primary.EXPECT().Delete(mock.Anything, "codex/oa/accounts/acc-1/api_key").Return(nil).Once()

	err := store.Delete(context.Background(), "codex/oa/accounts/acc-1/api_key")
	require.NoError(t, err)
}

func TestStoreGetDoesNotFallbackOnCanceledContextError(t *testing.T) {
	t.Parallel()

	primary := portmocks.NewMockSecretStore(t)
	fallback := portmocks.NewMockSecretStore(t)
	store := NewStore(primary, fallback)

	primary.EXPECT().Get(mock.Anything, "codex/oa/accounts/acc-1/api_key").Return("", context.Canceled).Once()

	_, err := store.Get(context.Background(), "codex/oa/accounts/acc-1/api_key")
	require.ErrorIs(t, err, context.Canceled)
}
