package application

import (
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
)

type LimitWindowKind string

const (
	LimitWindowDaily  LimitWindowKind = "daily"
	LimitWindowWeekly LimitWindowKind = "weekly"
)

func (k LimitWindowKind) Valid() bool {
	switch k {
	case LimitWindowDaily, LimitWindowWeekly:
		return true
	default:
		return false
	}
}

type SetAuthCommand struct {
	ID          domain.AccountID
	Method      domain.AuthMethod
	SecretKey   string
	SecretValue string
}

type RemoveAuthCommand struct {
	ID domain.AccountID
}

type SetUsageCommand struct {
	ID    domain.AccountID
	Usage domain.Usage
}

type SetLimitCommand struct {
	ID         domain.AccountID
	Window     LimitWindowKind
	Percent    float64
	ResetsAt   time.Time
	CapturedAt time.Time
}
