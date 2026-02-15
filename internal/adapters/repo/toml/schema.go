package toml

import "fmt"

const currentSchemaVersion = 1

type fileSchema struct {
	Version  int             `toml:"version"`
	Accounts []accountSchema `toml:"accounts"`
}

func (s *fileSchema) applyDefaults() {
	if s.Version == 0 {
		s.Version = currentSchemaVersion
	}
}

func (s fileSchema) validateVersion() error {
	if s.Version > currentSchemaVersion {
		return fmt.Errorf("unsupported accounts schema version %d (current %d)", s.Version, currentSchemaVersion)
	}

	return nil
}

type accountSchema struct {
	ID           string              `toml:"id"`
	Name         string              `toml:"name"`
	Metadata     metadataSchema      `toml:"metadata"`
	Auth         authSchema          `toml:"auth"`
	Usage        usageSchema         `toml:"usage,omitempty"`
	Limits       limitsSchema        `toml:"limits,omitempty"`
	Subscription *subscriptionSchema `toml:"subscription,omitempty"`
}

type metadataSchema struct {
	Provider  string `toml:"provider"`
	Model     string `toml:"model"`
	SecretRef string `toml:"secret_ref"`
	PlanType  string `toml:"plan_type,omitempty"`
}

type authSchema struct {
	Method    string `toml:"method"`
	SecretRef string `toml:"secret_ref"`
}

type usageSchema struct {
	InputTokens       int64 `toml:"input_tokens"`
	OutputTokens      int64 `toml:"output_tokens"`
	CachedInputTokens int64 `toml:"cached_input_tokens"`
}

type limitsSchema struct {
	Daily  *limitSnapshotSchema `toml:"daily,omitempty"`
	Weekly *limitSnapshotSchema `toml:"weekly,omitempty"`
}

type limitSnapshotSchema struct {
	Percent    float64 `toml:"percent"`
	ResetsAt   string  `toml:"resets_at"`
	CapturedAt string  `toml:"captured_at"`
}

type subscriptionSchema struct {
	ActiveStart     string `toml:"active_start"`
	ActiveUntil     string `toml:"active_until"`
	WillRenew       bool   `toml:"will_renew"`
	BillingPeriod   string `toml:"billing_period"`
	BillingCurrency string `toml:"billing_currency"`
	IsDelinquent    bool   `toml:"is_delinquent"`
	CapturedAt      string `toml:"captured_at"`
}
