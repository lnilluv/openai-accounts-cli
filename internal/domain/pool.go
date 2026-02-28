package domain

import (
	"fmt"
	"strings"
	"time"
)

type PoolID string
type Provider string
type PoolStrategy string

const (
	ProviderOpenAI Provider = "openai"

	PoolStrategyLeastWeeklyUsed PoolStrategy = "least_weekly_used"
)

type Pool struct {
	ID              PoolID
	Name            string
	Provider        Provider
	Strategy        PoolStrategy
	Active          bool
	AutoSyncMembers bool
	Members         []AccountID
	UpdatedAt       time.Time
}

func (p Pool) Validate() error {
	if strings.TrimSpace(string(p.ID)) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(string(p.Provider)) == "" {
		return fmt.Errorf("provider is required")
	}
	if p.Provider != ProviderOpenAI {
		return fmt.Errorf("unsupported provider %q", p.Provider)
	}
	if p.Strategy == "" {
		return fmt.Errorf("strategy is required")
	}

	return nil
}

func (p *Pool) NormalizeMembers() {
	if p == nil {
		return
	}

	members := make([]AccountID, 0, len(p.Members))
	seen := make(map[AccountID]struct{}, len(p.Members))
	for _, member := range p.Members {
		trimmed := AccountID(strings.TrimSpace(string(member)))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		members = append(members, trimmed)
	}

	p.Members = members
}
