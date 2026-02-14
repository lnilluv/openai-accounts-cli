package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	statusadapter "github.com/bnema/openai-accounts-cli/internal/adapters/render/status"
	tomlrepo "github.com/bnema/openai-accounts-cli/internal/adapters/repo/toml"
	chainstore "github.com/bnema/openai-accounts-cli/internal/adapters/secrets/chain"
	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/ports"
	"github.com/spf13/viper"
)

var errNotImplementedYet = errors.New("not implemented yet")

type app struct {
	service        *application.Service
	secretStore    ports.SecretStore
	statusRenderer func([]application.Status, statusadapter.RenderOptions) (string, error)
	browserLogin   browserLoginConfig
	usageBaseURL   string
	httpClient     *http.Client
	now            func() time.Time
}

type browserLoginConfig struct {
	Issuer     string
	ClientID   string
	ListenAddr string
	Timeout    time.Duration
}

func wireApp() (*app, error) {
	repo, err := tomlrepo.NewRepository(viper.New())
	if err != nil {
		return nil, fmt.Errorf("wire account repository: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	secretStore, err := chainstore.NewPassFirstWithFileFallback(filepath.Join(homeDir, ".codex", "secrets"))
	if err != nil {
		return nil, fmt.Errorf("wire secret store chain: %w", err)
	}

	return &app{
		service:        application.NewService(repo, secretStore, ports.SystemClock{}),
		secretStore:    secretStore,
		statusRenderer: statusadapter.Render,
		browserLogin: browserLoginConfig{
			Issuer:     envOrDefault("OA_AUTH_ISSUER", "https://auth.openai.com"),
			ClientID:   envOrDefault("OA_AUTH_CLIENT_ID", "app_EMoamEEZ73f0CkXaXp7hrann"),
			ListenAddr: envOrDefault("OA_AUTH_LISTEN", "127.0.0.1:1455"),
			Timeout:    5 * time.Minute,
		},
		usageBaseURL: envOrDefault("OA_USAGE_BASE_URL", "https://chatgpt.com/backend-api"),
		httpClient:   http.DefaultClient,
		now:          time.Now,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
