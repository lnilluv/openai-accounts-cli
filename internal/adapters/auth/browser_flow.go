package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const maxTokenResponseBytes = 1 << 20

var (
	ErrStateMismatch   = errors.New("oauth callback state mismatch")
	ErrCallbackTimeout = errors.New("timed out waiting for oauth callback")
	ErrMissingState    = errors.New("expected state is required")
)

type AuthorizationRequest struct {
	AuthURL       string
	ClientID      string
	RedirectURI   string
	Scopes        []string
	State         string
	CodeChallenge string
	Originator    string
}

type TokenExchangeRequest struct {
	Issuer       string
	ClientID     string
	RedirectURI  string
	Code         string
	CodeVerifier string
}

type ExchangedTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func NewState() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func BuildAuthorizationURL(req AuthorizationRequest) (string, error) {
	if req.AuthURL == "" {
		return "", errors.New("auth url is required")
	}
	if req.ClientID == "" {
		return "", errors.New("client id is required")
	}
	if req.RedirectURI == "" {
		return "", errors.New("redirect uri is required")
	}
	if req.State == "" {
		return "", errors.New("state is required")
	}
	if req.CodeChallenge == "" {
		return "", errors.New("code challenge is required")
	}

	parsed, err := url.Parse(req.AuthURL)
	if err != nil {
		return "", fmt.Errorf("parse auth url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("auth url must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("auth url host is required")
	}

	q := parsed.Query()
	q.Set("response_type", "code")
	q.Set("client_id", req.ClientID)
	q.Set("redirect_uri", req.RedirectURI)
	if len(req.Scopes) > 0 {
		q.Set("scope", strings.Join(req.Scopes, " "))
	}
	q.Set("state", req.State)
	q.Set("code_challenge", req.CodeChallenge)
	q.Set("code_challenge_method", PKCEChallengeMethodS256)
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	originator := req.Originator
	if originator == "" {
		originator = "oa"
	}
	q.Set("originator", originator)
	parsed.RawQuery = q.Encode()

	return parsed.String(), nil
}

type CallbackServer struct {
	expectedState string
	listener      net.Listener
	server        *http.Server
	resultCh      chan callbackResult
	resultOnce    sync.Once
	closeOnce     sync.Once
}

type callbackResult struct {
	code string
	err  error
}

func StartCallbackServer(listenAddr string, expectedState string) (*CallbackServer, error) {
	if expectedState == "" {
		return nil, ErrMissingState
	}
	if listenAddr == "" {
		listenAddr = "127.0.0.1:0"
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen callback server: %w", err)
	}

	cb := &CallbackServer{
		expectedState: expectedState,
		listener:      listener,
		resultCh:      make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", cb.handleCallback)

	cb.server = &http.Server{Handler: mux}

	go func() {
		if serveErr := cb.server.Serve(cb.listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			cb.trySendResult(callbackResult{err: serveErr})
		}
	}()

	return cb, nil
}

func (c *CallbackServer) RedirectURI() string {
	if tcpAddr, ok := c.listener.Addr().(*net.TCPAddr); ok {
		return fmt.Sprintf("http://localhost:%d/auth/callback", tcpAddr.Port)
	}
	return fmt.Sprintf("http://localhost/auth/callback")
}

func (c *CallbackServer) WaitForCode(timeout time.Duration) (string, error) {
	defer c.Close()

	select {
	case result := <-c.resultCh:
		return result.code, result.err
	case <-time.After(timeout):
		return "", ErrCallbackTimeout
	}
}

func (c *CallbackServer) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		closeErr = c.server.Close()
	})
	return closeErr
}

func (c *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if c.expectedState != "" && state != c.expectedState {
		c.trySendResult(callbackResult{err: ErrStateMismatch})
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	if oauthError := r.URL.Query().Get("error"); oauthError != "" {
		description := r.URL.Query().Get("error_description")
		if description != "" {
			oauthError = oauthError + ": " + description
		}
		c.trySendResult(callbackResult{err: errors.New(oauthError)})
		http.Error(w, "oauth error", http.StatusBadRequest)
		return
	}
	if code == "" {
		c.trySendResult(callbackResult{err: errors.New("missing authorization code")})
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	c.trySendResult(callbackResult{code: code})
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Authentication complete. You can close this window."))
}

func (c *CallbackServer) trySendResult(result callbackResult) {
	c.resultOnce.Do(func() {
		c.resultCh <- result
	})
}

func ExchangeCodeForTokens(client *http.Client, req TokenExchangeRequest) (ExchangedTokens, error) {
	if req.Issuer == "" {
		return ExchangedTokens{}, errors.New("issuer is required")
	}
	if req.ClientID == "" {
		return ExchangedTokens{}, errors.New("client id is required")
	}
	if req.RedirectURI == "" {
		return ExchangedTokens{}, errors.New("redirect uri is required")
	}
	if req.Code == "" {
		return ExchangedTokens{}, errors.New("authorization code is required")
	}
	if req.CodeVerifier == "" {
		return ExchangedTokens{}, errors.New("code verifier is required")
	}

	if client == nil {
		client = http.DefaultClient
	}

	issuer := strings.TrimRight(req.Issuer, "/")
	endpoint := issuer + "/oauth/token"

	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", req.Code)
	values.Set("redirect_uri", req.RedirectURI)
	values.Set("client_id", req.ClientID)
	values.Set("code_verifier", req.CodeVerifier)

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return ExchangedTokens{}, fmt.Errorf("create token exchange request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(httpReq)
	if err != nil {
		return ExchangedTokens{}, fmt.Errorf("exchange code for tokens: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ExchangedTokens{}, fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}

	var tokens ExchangedTokens
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxTokenResponseBytes)).Decode(&tokens); err != nil {
		return ExchangedTokens{}, fmt.Errorf("decode token response: %w", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.IDToken == "" {
		return ExchangedTokens{}, errors.New("token response missing required fields")
	}

	return tokens, nil
}
