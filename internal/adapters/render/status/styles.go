package status

import "github.com/charmbracelet/lipgloss"

type styles struct {
	title        lipgloss.Style
	header       lipgloss.Style
	account      lipgloss.Style
	detail       lipgloss.Style
	warning      lipgloss.Style
	section      lipgloss.Style
	empty        lipgloss.Style
	limitKey     lipgloss.Style
	limitMeta    lipgloss.Style
	barBracket   lipgloss.Style
	barFill      lipgloss.Style
	barEmpty     lipgloss.Style
	barText      lipgloss.Style
	barTextFaint lipgloss.Style
}

func newStyles() styles {
	return styles{
		title:        lipgloss.NewStyle().Bold(true),
		header:       lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		account:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		detail:       lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		warning:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203")),
		section:      lipgloss.NewStyle().MarginTop(1),
		empty:        lipgloss.NewStyle().Faint(true),
		limitKey:     lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		limitMeta:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		barBracket:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		barFill:      lipgloss.NewStyle().Foreground(lipgloss.Color("159")),
		barEmpty:     lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		barText:      lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		barTextFaint: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}
