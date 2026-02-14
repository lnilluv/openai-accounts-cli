package status

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

type RenderOptions struct {
	Now        time.Time
	StaleAfter time.Duration
}

func renderView(statuses []application.Status, opts RenderOptions, s styles) string {
	lines := []string{
		s.title.Render("OpenAI Account Usage"),
		s.header.Render(fmt.Sprintf("accounts: %d", len(statuses))),
	}

	if len(statuses) == 0 {
		lines = append(lines, s.empty.Render("No account statuses available."))
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	for _, status := range statuses {
		lines = append(lines, s.section.Render(renderAccount(status, opts, s)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func renderAccount(status application.Status, opts RenderOptions, s styles) string {
	parts := []string{
		s.account.Render(accountTitle(status.Account.Name, status.Account.ID, status.Account.Metadata.PlanType)),
	}

	for _, line := range limitLines(status, opts, s) {
		parts = append(parts, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func authLabel(method domain.AuthMethod) string {
	if method == "" {
		return "none"
	}

	return string(method)
}

func limitLines(status application.Status, opts RenderOptions, s styles) []string {
	lines := make([]string, 0, 2)
	for _, limit := range []*application.StatusLimit{status.DailyLimit, status.WeeklyLimit} {
		if limit == nil {
			continue
		}

		lines = append(lines, limitLine(limit, opts, s))
	}

	if len(lines) == 0 {
		return []string{s.detail.Render("limit: n/a")}
	}

	return lines
}

func limitLine(limit *application.StatusLimit, opts RenderOptions, s styles) string {
	leftPercent := clampPercent(100 - limit.Percent)
	bar := renderProgressBar(limit.Percent, 24, s)
	label := s.limitKey.Render(fmt.Sprintf("%s limit:", windowLabel(limit.Window)))
	percentColor := interpolateColor(leftPercent, 0, 100)
	percentStyle := lipgloss.NewStyle().Foreground(percentColor)
	meta := percentStyle.Render(fmt.Sprintf("%2.0f%% left", leftPercent))

	resetColor := resetTimeColor(limit.ResetsAt, opts.Now, limit.Window)
	resetStyle := lipgloss.NewStyle().Foreground(resetColor)
	reset := resetStyle.Render(fmt.Sprintf("(%s)", formatResetRelative(limit.ResetsAt, opts.Now)))

	line := lipgloss.JoinHorizontal(
		lipgloss.Top,
		label,
		" ",
		bar,
		" ",
		meta,
		" ",
		reset,
	)

	now := opts.Now
	if now.IsZero() {
		return line
	}

	if (domain.LimitSnapshot{AsOf: limit.CapturedAt}).IsStale(now, opts.StaleAfter) {
		line += " " + s.warning.Render("[stale]")
	}

	return line
}

func usageLine(status application.Status) string {
	if status.Account.Auth.Method == domain.AuthMethodChatGPT && status.Usage.BlendedTotal() == 0 {
		return "usage: n/a (live token totals unavailable)"
	}

	return fmt.Sprintf("usage: %d tokens", status.Usage.BlendedTotal())
}

func renderProgressBar(usedPercent float64, width int, s styles) string {
	if width <= 0 {
		return ""
	}

	used := clampPercent(usedPercent)
	leftFraction := (100.0 - used) / 100.0
	filled := int(math.Round(float64(width) * leftFraction))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	empty := width - filled
	fillSegment := s.barFill.Render(strings.Repeat("=", filled))
	emptySegment := s.barEmpty.Render(strings.Repeat("-", empty))

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		s.barBracket.Render("["),
		fillSegment,
		emptySegment,
		s.barBracket.Render("]"),
	)
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func formatResetAt(resetsAt, now time.Time) string {
	if resetsAt.IsZero() {
		return "unknown"
	}
	if now.IsZero() {
		return resetsAt.Format(time.RFC3339)
	}

	yearA, monthA, dayA := now.Date()
	yearB, monthB, dayB := resetsAt.Date()
	if yearA == yearB && monthA == monthB && dayA == dayB {
		return resetsAt.Format("15:04")
	}

	return resetsAt.Format("15:04 on 02 Jan")
}

func formatResetRelative(resetsAt, now time.Time) string {
	if now.IsZero() {
		return "resets " + formatResetAt(resetsAt, now)
	}

	if resetsAt.Before(now) {
		return "reset now"
	}

	remaining := resetsAt.Sub(now)
	if remaining < 24*time.Hour {
		hours := int(math.Ceil(remaining.Hours()))
		if hours < 1 {
			hours = 1
		}
		suffix := "hours"
		if hours == 1 {
			suffix = "hour"
		}
		return fmt.Sprintf("resets in %d %s (%s)", hours, suffix, resetsAt.Format("15:04"))
	}

	days := int(math.Ceil(remaining.Hours() / 24))
	if days < 1 {
		days = 1
	}
	suffix := "days"
	if days == 1 {
		suffix = "day"
	}

	return fmt.Sprintf("resets in %d %s (%s)", days, suffix, resetsAt.Format("15:04 on 02 Jan"))
}

func accountTitle(name string, id domain.AccountID, planType string) string {
	trimmed := strings.TrimSpace(name)
	if strings.Contains(trimmed, "@") {
		classification := accountClassification(planType)
		return fmt.Sprintf("Account: %s (%s)", trimmed, classification)
	}
	return fmt.Sprintf("%s (%s)", trimmed, id)
}

func accountClassification(planType string) string {
	switch strings.ToLower(strings.TrimSpace(planType)) {
	case "team":
		return "Team"
	case "business", "enterprise", "education", "edu", "k12", "quorum", "free_workspace":
		return "Business"
	default:
		return "Personal"
	}
}

func windowLabel(window application.LimitWindowKind) string {
	switch window {
	case application.LimitWindowDaily:
		return "5hours"
	case application.LimitWindowWeekly:
		return "weekly"
	default:
		return "unknown"
	}
}

func interpolateColor(value, min, max float64) lipgloss.Color {
	// Guard against division by zero
	if max == min {
		return lipgloss.Color("255")
	}

	// Normalize value between 0 and 1
	normalized := (value - min) / (max - min)
	if normalized < 0 {
		normalized = 0
	}
	if normalized > 1 {
		normalized = 1
	}

	// Color 240 (gray/faded) at min, Color 255 (bright white) at max
	// ANSI 256 greyscale ramp: 232 (darkest) to 255 (brightest)
	baseColor := 240.0
	targetColor := 255.0

	// Linear interpolation
	interpolated := baseColor + (targetColor-baseColor)*normalized
	colorCode := int(interpolated)

	return lipgloss.Color(fmt.Sprintf("%d", colorCode))
}

func resetTimeColor(resetsAt, now time.Time, window application.LimitWindowKind) lipgloss.Color {
	if now.IsZero() || resetsAt.Before(now) {
		return lipgloss.Color("255") // Bright white when no time context
	}

	remaining := resetsAt.Sub(now)

	// For daily limits: fade from 5 hours to 0
	// For weekly limits: fade from 7 days to 0
	var maxDuration time.Duration
	if window == application.LimitWindowDaily {
		maxDuration = 5 * time.Hour
	} else {
		maxDuration = 7 * 24 * time.Hour
	}

	// Closer to 0 = whiter (255), at max or beyond = more faded (240)
	// Invert the remaining time so that 0 remaining maps to max (white)
	inverted := maxDuration.Seconds() - remaining.Seconds()
	return interpolateColor(inverted, 0, maxDuration.Seconds())
}
