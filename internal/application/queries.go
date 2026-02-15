package application

import (
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
)

type StatusLimit struct {
	Window     LimitWindowKind
	Percent    float64
	ResetsAt   time.Time
	CapturedAt time.Time
}

type StatusSubscription struct {
	ActiveStart     time.Time
	ActiveUntil     time.Time
	WillRenew       bool
	BillingPeriod   string
	BillingCurrency string
	CapturedAt      time.Time
	IsDelinquent    bool
}

type Status struct {
	Account      domain.Account
	Usage        domain.Usage
	DailyLimit   *StatusLimit
	WeeklyLimit  *StatusLimit
	Subscription *StatusSubscription
}
