package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreRejectsInvalidKeys(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	testCases := []struct {
		name    string
		key     string
		wantErr string
	}{
		{name: "empty", key: "", wantErr: "secret key is empty"},
		{name: "whitespace", key: "   ", wantErr: "secret key is empty"},
		{name: "absolute", key: "/absolute/path", wantErr: "invalid secret key"},
		{name: "traversal", key: "../escape", wantErr: "invalid secret key"},
		{name: "deep traversal", key: "../../secret", wantErr: "invalid secret key"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := store.Put(context.Background(), tc.key, "value")
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestStorePutGetRoundTripAndPermissions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	key := "codex/oa/accounts/acc-1/api_key"
	want := "top-secret"

	err := store.Put(context.Background(), key, want)
	require.NoError(t, err)

	got, err := store.Get(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	secretPath := filepath.Join(root, key)
	info, err := os.Stat(secretPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(secretFileMod), info.Mode().Perm())
}

func TestStoreDeleteIsIdempotentWhenSecretMissing(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	key := "codex/oa/accounts/acc-1/api_key"

	err := store.Delete(context.Background(), key)
	require.NoError(t, err)

	err = store.Delete(context.Background(), key)
	require.NoError(t, err)
}
