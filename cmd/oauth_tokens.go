package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type oauthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
}

func decodeOAuthTokens(secretValue string) (oauthTokens, error) {
	var tokens oauthTokens
	if err := json.Unmarshal([]byte(secretValue), &tokens); err != nil {
		return oauthTokens{}, fmt.Errorf("decode oauth tokens: %w", err)
	}
	if strings.TrimSpace(tokens.AccessToken) == "" {
		return oauthTokens{}, fmt.Errorf("oauth tokens missing access_token")
	}
	return tokens, nil
}

func encodeOAuthTokens(tokens oauthTokens) (string, error) {
	payload, err := json.Marshal(tokens)
	if err != nil {
		return "", fmt.Errorf("encode oauth tokens: %w", err)
	}
	return string(payload), nil
}

func withCalculatedExpiry(tokens oauthTokens, now time.Time) oauthTokens {
	if tokens.ExpiresIn > 0 {
		tokens.ExpiresAt = now.Add(time.Duration(tokens.ExpiresIn) * time.Second).Unix()
	}
	return tokens
}

func tokenExpiringSoon(tokens oauthTokens, now time.Time, skew time.Duration) bool {
	if tokens.ExpiresAt <= 0 {
		return false
	}
	expiresAt := time.Unix(tokens.ExpiresAt, 0)
	return !expiresAt.After(now.Add(skew))
}
