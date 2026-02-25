package auth

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAuthorizationURLIncludesStateAndPKCEChallenge(t *testing.T) {
	t.Parallel()

	u, err := BuildAuthorizationURL(AuthorizationRequest{
		AuthURL:       "https://auth.example.com/oauth/authorize",
		ClientID:      "client-123",
		RedirectURI:   "http://localhost:3000/auth/callback",
		Scopes:        []string{"openid", "profile"},
		State:         "state-xyz",
		CodeChallenge: "challenge-abc",
		Originator:    "oa",
	})
	require.NoError(t, err)

	parsed, err := url.Parse(u)
	require.NoError(t, err)

	q := parsed.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "client-123", q.Get("client_id"))
	assert.Equal(t, "http://localhost:3000/auth/callback", q.Get("redirect_uri"))
	assert.Equal(t, "openid profile", q.Get("scope"))
	assert.Equal(t, "state-xyz", q.Get("state"))
	assert.Equal(t, "challenge-abc", q.Get("code_challenge"))
	assert.Equal(t, PKCEChallengeMethodS256, q.Get("code_challenge_method"))
	assert.Equal(t, "true", q.Get("id_token_add_organizations"))
	assert.Equal(t, "true", q.Get("codex_cli_simplified_flow"))
	assert.Equal(t, "oa", q.Get("originator"))
}

func TestBuildAuthorizationURLRejectsNonHTTPSScheme(t *testing.T) {
	t.Parallel()

	_, err := BuildAuthorizationURL(AuthorizationRequest{
		AuthURL:       "ftp://auth.example.com/oauth/authorize",
		ClientID:      "client-123",
		RedirectURI:   "http://localhost:3000/auth/callback",
		State:         "state-xyz",
		CodeChallenge: "challenge-abc",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http or https")
}

func TestCallbackServerReturnsCodeOnSuccess(t *testing.T) {
	t.Parallel()

	server, err := StartCallbackServer("127.0.0.1:0", "expected-state")
	require.NoError(t, err)
	defer func() { _ = server.Close() }()

	resp, err := http.Get(server.RedirectURI() + "?code=auth-code&state=expected-state")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "Authentication complete")

	code, err := server.WaitForCode(2 * time.Second)
	require.NoError(t, err)
	assert.Equal(t, "auth-code", code)
}

func TestCallbackServerReturnsErrorOnStateMismatch(t *testing.T) {
	t.Parallel()

	server, err := StartCallbackServer("127.0.0.1:0", "expected-state")
	require.NoError(t, err)
	defer func() { _ = server.Close() }()

	resp, err := http.Get(server.RedirectURI() + "?code=auth-code&state=wrong-state")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	_, err = server.WaitForCode(2 * time.Second)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrStateMismatch))
}

func TestCallbackServerTimesOutWaitingForCallback(t *testing.T) {
	t.Parallel()

	server, err := StartCallbackServer("127.0.0.1:0", "expected-state")
	require.NoError(t, err)
	defer func() { _ = server.Close() }()

	_, err = server.WaitForCode(50 * time.Millisecond)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCallbackTimeout))
}

func TestStartCallbackServerRequiresExpectedState(t *testing.T) {
	t.Parallel()

	_, err := StartCallbackServer("127.0.0.1:0", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMissingState))
}

func TestExchangeCodeForTokensSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "client-123", r.Form.Get("client_id"))
		assert.Equal(t, "http://localhost:1455/auth/callback", r.Form.Get("redirect_uri"))
		assert.Equal(t, "code-abc", r.Form.Get("code"))
		assert.Equal(t, "verifier-xyz", r.Form.Get("code_verifier"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt","id_token":"it","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	tokens, err := ExchangeCodeForTokens(http.DefaultClient, TokenExchangeRequest{
		Issuer:       server.URL,
		ClientID:     "client-123",
		RedirectURI:  "http://localhost:1455/auth/callback",
		Code:         "code-abc",
		CodeVerifier: "verifier-xyz",
	})
	require.NoError(t, err)
	assert.Equal(t, "at", tokens.AccessToken)
	assert.Equal(t, "rt", tokens.RefreshToken)
	assert.Equal(t, "it", tokens.IDToken)
}

func TestExchangeCodeForTokensReturnsErrorForFailureStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("denied"))
	}))
	defer server.Close()

	_, err := ExchangeCodeForTokens(http.DefaultClient, TokenExchangeRequest{
		Issuer:       server.URL,
		ClientID:     "client-123",
		RedirectURI:  "http://localhost:1455/auth/callback",
		Code:         "code-abc",
		CodeVerifier: "verifier-xyz",
	})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "token endpoint returned status"))
}

func TestRefreshTokensSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "client-123", r.Form.Get("client_id"))
		assert.Equal(t, "refresh-abc", r.Form.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at2","refresh_token":"rt2","id_token":"it2","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	tokens, err := RefreshTokens(http.DefaultClient, RefreshTokenRequest{
		Issuer:       server.URL,
		ClientID:     "client-123",
		RefreshToken: "refresh-abc",
	})
	require.NoError(t, err)
	assert.Equal(t, "at2", tokens.AccessToken)
	assert.Equal(t, "rt2", tokens.RefreshToken)
	assert.Equal(t, "it2", tokens.IDToken)
}

func TestRefreshTokensReturnsInvalidGrantSentinel(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"expired"}`))
	}))
	defer server.Close()

	_, err := RefreshTokens(http.DefaultClient, RefreshTokenRequest{
		Issuer:       server.URL,
		ClientID:     "client-123",
		RefreshToken: "refresh-abc",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRefreshTokenInvalid)
}
