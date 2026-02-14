package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"
const maxOAuthResponseBytes = 1 << 20

var ErrDeviceFlowTimeout = errors.New("timed out waiting for device authorization")

type API struct {
	BaseURL        string
	DeviceCodePath string
	TokenPath      string
}

type DeviceFlowAdapter struct {
	API            API
	HTTPClient     *http.Client
	RequestTimeout time.Duration
}

type DeviceCodeResult struct {
	VerificationURL string
	UserCode        string
	PollInterval    time.Duration
	DeviceAuthID    string
}

type DevicePollRequest struct {
	ClientID     string
	DeviceAuthID string
	PollInterval time.Duration
	Timeout      time.Duration
}

type TokenResult struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int64  `json:"interval"`
}

type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	Interval         int64  `json:"interval"`
}

func (a DeviceFlowAdapter) RequestDeviceCode(ctx context.Context, clientID string, scopes []string) (DeviceCodeResult, error) {
	if clientID == "" {
		return DeviceCodeResult{}, errors.New("client id is required")
	}

	endpoint, err := buildAPIURL(a.API.BaseURL, a.API.DeviceCodePath)
	if err != nil {
		return DeviceCodeResult{}, err
	}

	values := url.Values{}
	values.Set("client_id", clientID)
	if len(scopes) > 0 {
		values.Set("scope", strings.Join(scopes, " "))
	}

	requestCtx, cancel := a.requestContext(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return DeviceCodeResult{}, fmt.Errorf("create device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return DeviceCodeResult{}, fmt.Errorf("request device code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		oauthErr := decodeOAuthError(resp)
		return DeviceCodeResult{}, fmt.Errorf("request device code: %s", oauthErr)
	}

	var payload deviceCodeResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxOAuthResponseBytes)).Decode(&payload); err != nil {
		return DeviceCodeResult{}, fmt.Errorf("decode device code response: %w", err)
	}

	verificationURL := payload.VerificationURI
	if payload.VerificationURIComplete != "" {
		verificationURL = payload.VerificationURIComplete
	}
	if payload.DeviceCode == "" || payload.UserCode == "" || verificationURL == "" {
		return DeviceCodeResult{}, errors.New("device code response missing required fields")
	}

	interval := payload.Interval
	if interval <= 0 {
		interval = 5
	}

	return DeviceCodeResult{
		VerificationURL: verificationURL,
		UserCode:        payload.UserCode,
		PollInterval:    time.Duration(interval) * time.Second,
		DeviceAuthID:    payload.DeviceCode,
	}, nil
}

func (a DeviceFlowAdapter) PollToken(ctx context.Context, req DevicePollRequest) (TokenResult, error) {
	if req.ClientID == "" {
		return TokenResult{}, errors.New("client id is required")
	}
	if req.DeviceAuthID == "" {
		return TokenResult{}, errors.New("device auth id is required")
	}

	interval := req.PollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return TokenResult{}, ErrDeviceFlowTimeout
		}

		token, pollInterval, pending, err := a.pollTokenOnce(ctx, req.ClientID, req.DeviceAuthID, interval, deadline)
		if err != nil {
			return TokenResult{}, err
		}
		if !pending {
			return token, nil
		}
		if pollInterval > 0 {
			interval = pollInterval
		}

		waitUntil := time.Now().Add(interval)
		if waitUntil.After(deadline) {
			return TokenResult{}, ErrDeviceFlowTimeout
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return TokenResult{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func (a DeviceFlowAdapter) pollTokenOnce(ctx context.Context, clientID string, deviceAuthID string, interval time.Duration, deadline time.Time) (TokenResult, time.Duration, bool, error) {
	endpoint, err := buildAPIURL(a.API.BaseURL, a.API.TokenPath)
	if err != nil {
		return TokenResult{}, 0, false, err
	}

	values := url.Values{}
	values.Set("grant_type", deviceCodeGrantType)
	values.Set("client_id", clientID)
	values.Set("device_code", deviceAuthID)

	reqCtx := ctx
	if ctxDeadline, ok := ctx.Deadline(); !ok || deadline.Before(ctxDeadline) {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return TokenResult{}, 0, false, fmt.Errorf("create token request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient().Do(httpReq)
	if err != nil {
		return TokenResult{}, 0, false, fmt.Errorf("request token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		var token TokenResult
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxOAuthResponseBytes)).Decode(&token); err != nil {
			return TokenResult{}, 0, false, fmt.Errorf("decode token response: %w", err)
		}
		if token.AccessToken == "" {
			return TokenResult{}, 0, false, errors.New("token response missing access token")
		}
		return token, 0, false, nil
	}

	var oauthErr oauthErrorResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxOAuthResponseBytes)).Decode(&oauthErr); err != nil {
		return TokenResult{}, 0, false, fmt.Errorf("request token: status %d", resp.StatusCode)
	}

	nextInterval := interval
	if oauthErr.Interval > 0 {
		nextInterval = time.Duration(oauthErr.Interval) * time.Second
	}
	if oauthErr.Error == "slow_down" {
		nextInterval += 5 * time.Second
	}

	if oauthErr.Error == "authorization_pending" || oauthErr.Error == "slow_down" {
		return TokenResult{}, nextInterval, true, nil
	}

	return TokenResult{}, 0, false, fmt.Errorf("request token: %s", formatOAuthError(resp.StatusCode, oauthErr))
}

func (a DeviceFlowAdapter) httpClient() *http.Client {
	if a.HTTPClient != nil {
		return a.HTTPClient
	}
	return http.DefaultClient
}

func (a DeviceFlowAdapter) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}

	requestTimeout := a.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 30 * time.Second
	}

	return context.WithTimeout(ctx, requestTimeout)
}

func decodeOAuthError(resp *http.Response) string {
	var oauthErr oauthErrorResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxOAuthResponseBytes)).Decode(&oauthErr); err != nil {
		return fmt.Sprintf("status %d", resp.StatusCode)
	}
	return formatOAuthError(resp.StatusCode, oauthErr)
}

func formatOAuthError(statusCode int, oauthErr oauthErrorResponse) string {
	if oauthErr.Error == "" {
		return fmt.Sprintf("status %d", statusCode)
	}
	if oauthErr.ErrorDescription != "" {
		return oauthErr.Error + ": " + oauthErr.ErrorDescription
	}
	return oauthErr.Error
}

func buildAPIURL(baseURL string, path string) (string, error) {
	if baseURL == "" {
		return "", errors.New("api base url is required")
	}
	if path == "" {
		return "", errors.New("api path is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse api base url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("api base url must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("api base url host is required")
	}

	endpoint, err := parsed.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse api path: %w", err)
	}
	return endpoint.String(), nil
}
