package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
)

type opencodeOAuthAuth struct {
	Type      string `json:"type"`
	Refresh   string `json:"refresh"`
	Access    string `json:"access"`
	ExpiresMS int64  `json:"expires,omitempty"`
	AccountID string `json:"accountId,omitempty"`
}

func syncOpencodeAuthForAccount(ctx context.Context, app *app, accountID domain.AccountID) error {
	status, err := app.service.GetStatus(ctx, accountID)
	if err != nil {
		return fmt.Errorf("load account for opencode auth sync: %w", err)
	}

	if status.Account.Auth.Method != domain.AuthMethodChatGPT {
		return nil
	}

	secretRef := strings.TrimSpace(status.Account.Auth.SecretRef)
	if secretRef == "" {
		return nil
	}

	secretValue, err := app.secretStore.Get(ctx, secretRef)
	if err != nil {
		return fmt.Errorf("load oauth secret for opencode auth sync: %w", err)
	}

	tokens, err := decodeOAuthTokens(secretValue)
	if err != nil {
		return fmt.Errorf("decode oauth secret for opencode auth sync: %w", err)
	}

	entry := opencodeOAuthAuth{
		Type:      "oauth",
		Refresh:   tokens.RefreshToken,
		Access:    tokens.AccessToken,
		ExpiresMS: tokenExpiryMillis(tokens, app.now),
		AccountID: accountIDFromToken(tokens.IDToken),
	}

	path, err := opencodeAuthPath()
	if err != nil {
		return err
	}

	content, err := readOpencodeAuthMap(path)
	if err != nil {
		return err
	}
	content["openai"] = entry

	return writeOpencodeAuthMap(path, content)
}

func shouldSyncOpencodeAuth(command string) bool {
	return filepath.Base(strings.TrimSpace(command)) == "opencode"
}

func opencodeAuthPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for opencode auth sync: %w", err)
	}

	return filepath.Join(homeDir, ".local", "share", "opencode", "auth.json"), nil
}

func readOpencodeAuthMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read opencode auth file: %w", err)
	}

	var content map[string]any
	if err := json.Unmarshal(data, &content); err != nil {
		return nil, fmt.Errorf("decode opencode auth file: %w", err)
	}

	if content == nil {
		content = map[string]any{}
	}

	return content, nil
}

func writeOpencodeAuthMap(path string, content map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create opencode auth directory: %w", err)
	}

	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return fmt.Errorf("encode opencode auth file: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "auth-*.json")
	if err != nil {
		return fmt.Errorf("create temp opencode auth file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp opencode auth file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp opencode auth file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp opencode auth file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace opencode auth file: %w", err)
	}
	cleanup = false

	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("chmod opencode auth file: %w", err)
	}

	return nil
}

func tokenExpiryMillis(tokens oauthTokens, now func() time.Time) int64 {
	if tokens.ExpiresAt > 0 {
		return tokens.ExpiresAt * 1000
	}
	if tokens.ExpiresIn > 0 {
		return now().Add(time.Duration(tokens.ExpiresIn) * time.Second).UnixMilli()
	}
	return 0
}
