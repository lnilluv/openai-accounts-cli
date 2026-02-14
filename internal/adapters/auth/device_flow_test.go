package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestDeviceCodeParsesSuccessResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "client-123", r.Form.Get("client_id"))
		assert.Equal(t, "openid profile", r.Form.Get("scope"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"device_code":"device-auth-id","user_code":"A1B2-C3D4","verification_uri":"https://example.com/activate","interval":5}`))
	}))
	t.Cleanup(server.Close)

	adapter := DeviceFlowAdapter{
		API: API{
			BaseURL:        server.URL,
			DeviceCodePath: "/oauth/device/code",
			TokenPath:      "/oauth/token",
		},
		HTTPClient: server.Client(),
	}

	result, err := adapter.RequestDeviceCode(context.Background(), "client-123", []string{"openid", "profile"})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/activate", result.VerificationURL)
	assert.Equal(t, "A1B2-C3D4", result.UserCode)
	assert.Equal(t, 5*time.Second, result.PollInterval)
	assert.Equal(t, "device-auth-id", result.DeviceAuthID)
}

func TestRequestDeviceCodeTimesOutWithoutCallerDeadline(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"device_code":"device-auth-id","user_code":"A1B2-C3D4","verification_uri":"https://example.com/activate","interval":5}`))
	}))
	t.Cleanup(server.Close)

	adapter := DeviceFlowAdapter{
		API: API{
			BaseURL:        server.URL,
			DeviceCodePath: "/oauth/device/code",
			TokenPath:      "/oauth/token",
		},
		HTTPClient:     server.Client(),
		RequestTimeout: 20 * time.Millisecond,
	}

	_, err := adapter.RequestDeviceCode(context.Background(), "client-123", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request device code")
}

func TestPollTokenReturnsSuccessAfterPending(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		count := attempts.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token-abc","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(server.Close)

	adapter := DeviceFlowAdapter{
		API: API{
			BaseURL:        server.URL,
			DeviceCodePath: "/oauth/device/code",
			TokenPath:      "/oauth/token",
		},
		HTTPClient: server.Client(),
	}

	token, err := adapter.PollToken(context.Background(), DevicePollRequest{
		ClientID:     "client-123",
		DeviceAuthID: "device-auth-id",
		PollInterval: 5 * time.Millisecond,
		Timeout:      500 * time.Millisecond,
	})
	require.NoError(t, err)
	assert.Equal(t, "token-abc", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, int64(3600), token.ExpiresIn)
	assert.Equal(t, int32(2), attempts.Load())
}

func TestPollTokenTimesOutWhenStillPending(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
	}))
	t.Cleanup(server.Close)

	adapter := DeviceFlowAdapter{
		API: API{
			BaseURL:        server.URL,
			DeviceCodePath: "/oauth/device/code",
			TokenPath:      "/oauth/token",
		},
		HTTPClient: server.Client(),
	}

	_, err := adapter.PollToken(context.Background(), DevicePollRequest{
		ClientID:     "client-123",
		DeviceAuthID: "device-auth-id",
		PollInterval: 5 * time.Millisecond,
		Timeout:      25 * time.Millisecond,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDeviceFlowTimeout))
}

func TestPollTokenHandlesSlowDownAndEventuallySucceeds(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"slow_down","interval":0}`))
			return
		}

		_, _ = w.Write([]byte(`{"access_token":"token-slow","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(server.Close)

	adapter := DeviceFlowAdapter{
		API: API{
			BaseURL:        server.URL,
			DeviceCodePath: "/oauth/device/code",
			TokenPath:      "/oauth/token",
		},
		HTTPClient: server.Client(),
	}

	token, err := adapter.PollToken(context.Background(), DevicePollRequest{
		ClientID:     "client-123",
		DeviceAuthID: "device-auth-id",
		PollInterval: 5 * time.Millisecond,
		Timeout:      6 * time.Second,
	})
	require.NoError(t, err)
	assert.Equal(t, "token-slow", token.AccessToken)
	assert.Equal(t, int32(2), attempts.Load())
}
