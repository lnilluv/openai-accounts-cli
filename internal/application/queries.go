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

type Status struct {
	Account     domain.Account
	Usage       domain.Usage
	DailyLimit  *StatusLimit
	WeeklyLimit *StatusLimit
}
