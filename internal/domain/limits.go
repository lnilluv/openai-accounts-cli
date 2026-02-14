package domain

import "time"

type Window string

const (
	WindowHour  Window = "hour"
	WindowDay   Window = "day"
	WindowWeek  Window = "week"
	WindowMonth Window = "month"
)

func (w Window) Label() string {
	switch w {
	case WindowHour:
		return "1h"
	case WindowDay:
		return "24h"
	case WindowWeek:
		return "7d"
	case WindowMonth:
		return "30d"
	default:
		return string(w)
	}
}

type LimitSnapshot struct {
	AsOf time.Time
}

type AccountLimitSnapshots struct {
	Daily  *AccountLimitSnapshot
	Weekly *AccountLimitSnapshot
}

type AccountLimitSnapshot struct {
	Percent    float64
	ResetsAt   time.Time
	CapturedAt time.Time
}

func (s LimitSnapshot) IsStale(now time.Time, maxAge time.Duration) bool {
	if s.AsOf.IsZero() {
		return true
	}

	if maxAge <= 0 {
		return false
	}

	return now.Sub(s.AsOf) > maxAge
}
