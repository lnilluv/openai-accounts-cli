package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthSetRequiresSecretValueFlag(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "api_key",
		"--secret-key", "openai://acc-1/api_key",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag(s) \"secret-value\" not set")
}

func TestStatusByAccountHappyPath(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdout, _, err := executeCLI(t, home, "status", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "accounts: 1")
	assert.Contains(t, stdout, "Primary (acc-1)")
}

func TestStatusByAccountJSONOutput(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdout, _, err := executeCLI(t, home, "status", "--account", "acc-1", "--json")
	require.NoError(t, err)
	assert.True(t, json.Valid([]byte(stdout)))
	assert.Contains(t, stdout, "\"Account\"")
	assert.Contains(t, stdout, "\"ID\": \"acc-1\"")
}

func TestAuthSetThenStatusShowsAuthMethod(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "api_key",
		"--secret-key", "openai://acc-1/api_key",
		"--secret-value", "test-secret-value",
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "status", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Primary (acc-1)")
}

func TestAuthSetAutoAssignsNextNumericAccountID(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--method", "api_key",
		"--secret-key", "openai://1/api_key",
		"--secret-value", "secret-1",
	)
	require.NoError(t, err)

	_, _, err = executeCLI(t, home,
		"auth", "set",
		"--method", "api_key",
		"--secret-key", "openai://2/api_key",
		"--secret-value", "secret-2",
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "status")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Account 1 (1)")
	assert.Contains(t, stdout, "Account 2 (2)")
}

func TestLoginDeviceReturnsNotImplemented(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "auth", "login", "device")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented yet")
}

func TestLimitCommandIsRemoved(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "limit")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command \"limit\"")
}

func TestAccountListShowsConfiguredAccounts(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdout, _, err := executeCLI(t, home, "account", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "acc-1")
	assert.Contains(t, stdout, "Primary")
}

func TestUsageSetSubcommandIsRemoved(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "usage", "set", "--account", "acc-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command \"set\"")
}

func TestUsageCommandFetchesLimitsAndRendersStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/wham/usage", r.URL.Path)
		assert.Equal(t, "Bearer access-token-123", r.Header.Get("Authorization"))
		_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-123","id_token":""}`,
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "usage", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "5hours limit:")
	assert.Contains(t, stdout, "weekly limit:")
	assert.Contains(t, stdout, "79% left")
	assert.Contains(t, stdout, "53% left")
}

func TestStatusAliasFetchesLimitsAndRendersStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/wham/usage", r.URL.Path)
		assert.Equal(t, "Bearer access-token-123", r.Header.Get("Authorization"))
		_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-123","id_token":""}`,
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "status", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "5hours limit:")
	assert.Contains(t, stdout, "weekly limit:")
	assert.Contains(t, stdout, "79% left")
	assert.Contains(t, stdout, "53% left")
}

func TestUsageCommandJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-123","id_token":""}`,
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "usage", "--account", "acc-1", "--json")
	require.NoError(t, err)
	assert.True(t, json.Valid([]byte(stdout)))
	assert.Contains(t, stdout, "\"DailyLimit\"")
	assert.Contains(t, stdout, "\"WeeklyLimit\"")
}

func TestUsageCommandShowsFetchingSpinnerMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"access-token-123","id_token":""}`,
	)
	require.NoError(t, err)

	_, stderr, err := executeCLI(t, home, "usage", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Fetching usage limits")
}

func TestUsageCommandReturnsFetchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"invalid_token"}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"bad-token","id_token":""}`,
	)
	require.NoError(t, err)

	_, _, err = executeCLI(t, home, "usage", "--account", "acc-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session expired")
	assert.Contains(t, err.Error(), "oa auth login browser --account acc-1")
}

func TestUsageCommandUpdatesAccountNameFromTokenEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"plan_type":"team","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":30,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":10,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	idToken := fakeJWT(`{"email":"email@adress.com"}`)
	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", fmt.Sprintf(`{"access_token":"ok-token","id_token":"%s"}`, idToken),
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "usage", "--account", "acc-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Account: email@adress.com (Team)")
}

func executeCLI(t *testing.T, home string, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("HOME", home)

	root := newRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func writeAccountsFixture(home string) error {
	configDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	accounts := `version = 1

[[accounts]]
id = "acc-1"
name = "Primary"

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = ""
secret_ref = ""
`

	return os.WriteFile(filepath.Join(configDir, "accounts.toml"), []byte(accounts), 0o644)
}

func writeAccountsFixtureWithChatGPTAuth(home string) error {
	configDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	accounts := `version = 1

[[accounts]]
id = "acc-1"
name = "Primary"

[accounts.metadata]
provider = "openai"
model = "gpt-5"
secret_ref = "openai://acc-1/oauth_tokens"

[accounts.auth]
method = "chatgpt"
secret_ref = "openai://acc-1/oauth_tokens"
`

	return os.WriteFile(filepath.Join(configDir, "accounts.toml"), []byte(accounts), 0o644)
}

func fakeJWT(payload string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return header + "." + body + ".sig"
}
