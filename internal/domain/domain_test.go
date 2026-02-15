package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageBlendedTotalAndCompact(t *testing.T) {
	u := Usage{
		InputTokens:       1_200,
		OutputTokens:      300,
		CachedInputTokens: 500,
	}

	require.Equal(t, int64(2_000), u.BlendedTotal())

	assert.Equal(t, "2.0k", u.BlendedTotalCompact())
}

func TestLimitSnapshotStaleDetection(t *testing.T) {
	asOf := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
	s := LimitSnapshot{AsOf: asOf}

	assert.False(t, s.IsStale(asOf.Add(5*time.Minute), 10*time.Minute))
	assert.True(t, s.IsStale(asOf.Add(11*time.Minute), 10*time.Minute))
}

func TestWindowLabelBasic(t *testing.T) {
	tests := []struct {
		name   string
		window Window
		want   string
	}{
		{name: "hour", window: WindowHour, want: "1h"},
		{name: "day", window: WindowDay, want: "24h"},
		{name: "week", window: WindowWeek, want: "7d"},
		{name: "month", window: WindowMonth, want: "30d"},
		{name: "unknown window returns raw value", window: Window("rolling_30m"), want: "rolling_30m"},
		{name: "zero-value window returns empty label", window: Window(""), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.window.Label())
		})
	}
}

func TestLimitSnapshotStaleDetectionNonPositiveMaxAge(t *testing.T) {
	asOf := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
	s := LimitSnapshot{AsOf: asOf}

	assert.False(t, s.IsStale(asOf.Add(24*time.Hour), 0))
	assert.False(t, s.IsStale(asOf.Add(24*time.Hour), -1*time.Minute))
}

func TestLimitSnapshotStaleDetectionZeroAsOf(t *testing.T) {
	now := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
	s := LimitSnapshot{}

	assert.True(t, s.IsStale(now, 10*time.Minute))
}

func TestSubscriptionRenewsIn(t *testing.T) {
	now := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)
	sub := Subscription{
		ActiveUntil: time.Date(2026, 3, 14, 7, 41, 19, 0, time.UTC),
		WillRenew:   true,
	}
	remaining := sub.ActiveUntil.Sub(now)
	assert.InDelta(t, 26.8, remaining.Hours()/24, 0.5)
}

func TestCompactNumberBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		value int64
		want  string
	}{
		{name: "below thousand", value: 999, want: "999"},
		{name: "thousand", value: 1_000, want: "1.0k"},
		{name: "below million", value: 999_999, want: "1000.0k"},
		{name: "million", value: 1_000_000, want: "1.0M"},
		{name: "negative small", value: -1, want: "-1"},
		{name: "negative large", value: -1_000, want: "-1000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, compactNumber(tt.value))
		})
	}
}
