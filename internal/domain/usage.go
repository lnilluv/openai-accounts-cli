package domain

import "fmt"

type Usage struct {
	InputTokens       int64
	OutputTokens      int64
	CachedInputTokens int64
}

// BlendedTotal returns InputTokens + CachedInputTokens + OutputTokens.
func (u Usage) BlendedTotal() int64 {
	return u.InputTokens + u.OutputTokens + u.CachedInputTokens
}

func (u Usage) BlendedTotalCompact() string {
	return compactNumber(u.BlendedTotal())
}

func compactNumber(v int64) string {
	if v < 1_000 {
		return fmt.Sprintf("%d", v)
	}

	if v < 1_000_000 {
		return fmt.Sprintf("%.1fk", float64(v)/1_000)
	}

	return fmt.Sprintf("%.1fM", float64(v)/1_000_000)
}
