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
		switch {
		case r.URL.Path == "/wham/usage":
			assert.Equal(t, "Bearer access-token-123", r.Header.Get("Authorization"))
			_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
		case r.URL.Path == "/subscriptions":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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
		switch {
		case r.URL.Path == "/wham/usage":
			assert.Equal(t, "Bearer access-token-123", r.Header.Get("Authorization"))
			_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
		case r.URL.Path == "/subscriptions":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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

func TestUsageCommandRefreshesExpiredAccessTokenAndRetries(t *testing.T) {
	var oldTokenCalls int
	var newTokenCalls int
	var refreshCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/token":
			refreshCalls++
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
			assert.Equal(t, "test-client-id", r.Form.Get("client_id"))
			assert.Equal(t, "refresh-token-123", r.Form.Get("refresh_token"))
			_, _ = fmt.Fprint(w, `{"access_token":"new-token","refresh_token":"refresh-token-456","id_token":"","token_type":"Bearer","expires_in":3600}`)
		case r.URL.Path == "/wham/usage":
			authz := r.Header.Get("Authorization")
			if authz == "Bearer old-token" {
				oldTokenCalls++
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprint(w, `{"error":"invalid_token"}`)
				return
			}
			assert.Equal(t, "Bearer new-token", authz)
			newTokenCalls++
			_, _ = fmt.Fprint(w, `{"plan_type":"pro","rate_limit":{"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_at":1893888000}}}`)
		case r.URL.Path == "/subscriptions":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("OA_USAGE_BASE_URL", server.URL)
	t.Setenv("OA_AUTH_ISSUER", server.URL)
	t.Setenv("OA_AUTH_CLIENT_ID", "test-client-id")

	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "chatgpt",
		"--secret-key", "openai://acc-1/oauth_tokens",
		"--secret-value", `{"access_token":"old-token","refresh_token":"refresh-token-123","id_token":"","expires_at":1}`,
	)
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "usage", "--account", "acc-1")
	require.NoError(t, err)
	assert.LessOrEqual(t, oldTokenCalls, 1)
	assert.GreaterOrEqual(t, refreshCalls, 1)
	assert.GreaterOrEqual(t, newTokenCalls, 1)
	assert.Contains(t, stdout, "5hours limit:")
}

func TestUsageCommandExpiredErrorIncludesEmailAndType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"invalid_token"}`)
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
		"--secret-value", fmt.Sprintf(`{"access_token":"bad-token","id_token":"%s"}`, idToken),
	)
	require.NoError(t, err)

	_, stderr, err := executeCLI(t, home, "usage", "--account", "acc-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account acc-1 (email@adress.com, Unknown): session expired")
	assert.Contains(t, stderr, "account acc-1 (email@adress.com, Unknown): session expired")
}

func TestUsageCommandFetchesSubscriptionAndRendersRenewal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/wham/usage":
			_, _ = fmt.Fprint(w, `{"plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":21,"limit_window_seconds":18000,"reset_after_seconds":120,"reset_at":1893456000},"secondary_window":{"used_percent":47,"limit_window_seconds":604800,"reset_after_seconds":3600,"reset_at":1893888000}}}`)
		case r.URL.Path == "/subscriptions":
			_, _ = fmt.Fprint(w, `{"plan_type":"plus","active_start":"2026-02-14T07:41:19Z","active_until":"2026-03-14T07:41:19Z","will_renew":true,"billing_period":"monthly","billing_currency":"EUR","is_delinquent":false}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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
	assert.Contains(t, stdout, "renewal:")
	assert.Contains(t, stdout, "renews in")
	assert.Contains(t, stdout, "14 Mar")
}

func TestPoolActivateCreatesDefaultPool(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	stdout, _, err := executeCLI(t, home, "pool", "activate")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Activated pool default-openai")
	assert.Contains(t, stdout, "members: 1")
}

func TestPoolStatusReportsPoolState(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixtureWithTwoNamedAccounts(home))

	_, _, err := executeCLI(t, home, "pool", "activate")
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "pool", "status")
	require.NoError(t, err)
	assert.Contains(t, stdout, "pool: default-openai")
	assert.Contains(t, stdout, "active: true")
	assert.Contains(t, stdout, "members: chatgpt@nilluv.com")
	assert.Contains(t, stdout, "members: chatgpt@nilluv.com, chatgpt+alt@nilluv.com")
}

func TestPoolDeactivateDisablesDefaultPool(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "pool", "activate")
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "pool", "deactivate")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Deactivated pool default-openai")

	statusOut, _, err := executeCLI(t, home, "pool", "status")
	require.NoError(t, err)
	assert.Contains(t, statusOut, "active: false")
}

func TestRunUsesSelectedPoolAccountInChildEnv(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "pool", "activate")
	require.NoError(t, err)

	stdout, _, err := executeCLI(t, home, "run", "--pool", "default-openai", "--", "sh", "-c", "printf '%s:%s' \"$OA_POOL_ID\" \"$OA_ACTIVE_ACCOUNT\"")
	require.NoError(t, err)
	assert.Contains(t, stdout, "default-openai:acc-1")
}

func TestRunRequiresCommandAfterDoubleDash(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "run", "--pool", "default-openai")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a command after '--'")
}

func TestRunKeepsLogicalSessionStableForSameWorkspaceAndWindow(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, writeAccountsFixture(home))

	_, _, err := executeCLI(t, home, "pool", "activate")
	require.NoError(t, err)

	t.Setenv("OA_WINDOW_FINGERPRINT", "window-a")
	one, _, err := executeCLI(t, home, "run", "--pool", "default-openai", "--", "sh", "-c", "printf '%s|%s' \"$OA_LOGICAL_SESSION_ID\" \"$OA_PROVIDER_SESSION_ID\"")
	require.NoError(t, err)
	two, _, err := executeCLI(t, home, "run", "--pool", "default-openai", "--", "sh", "-c", "printf '%s|%s' \"$OA_LOGICAL_SESSION_ID\" \"$OA_PROVIDER_SESSION_ID\"")
	require.NoError(t, err)

	assert.Equal(t, one, two)
	assert.Contains(t, one, "|")
	assert.NotEqual(t, "|", one)
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

func writeAccountsFixtureWithTwoNamedAccounts(home string) error {
	configDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	accounts := `version = 1

[[accounts]]
id = "1"
name = "chatgpt@nilluv.com"

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = ""
secret_ref = ""

[[accounts]]
id = "2"
name = "chatgpt+alt@nilluv.com"

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = ""
secret_ref = ""
`

	return os.WriteFile(filepath.Join(configDir, "accounts.toml"), []byte(accounts), 0o644)
}

func fakeJWT(payload string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return header + "." + body + ".sig"
}
