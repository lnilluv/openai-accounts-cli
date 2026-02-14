package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type usageFetchDoneMsg struct {
	err error
}

type usageFetchSpinnerModel struct {
	spinner spinner.Model
	label   string
	fetch   tea.Cmd
	err     error
	done    bool
}

func newUsageFetchSpinnerModel(label string, fetch tea.Cmd) usageFetchSpinnerModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("69"))),
	)

	return usageFetchSpinnerModel{
		spinner: s,
		label:   label,
		fetch:   fetch,
	}
}

func (m usageFetchSpinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetch)
}

func (m usageFetchSpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case usageFetchDoneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m usageFetchSpinnerModel) View() string {
	if m.done {
		return ""
	}

	return fmt.Sprintf("%s %s", m.spinner.View(), m.label)
}

func runUsageFetchSpinner(ctx context.Context, output io.Writer, fetch func(context.Context) error) error {
	fetchCmd := func() tea.Msg {
		return usageFetchDoneMsg{err: fetch(ctx)}
	}

	p := tea.NewProgram(
		newUsageFetchSpinnerModel("Fetching usage limits...", fetchCmd),
		tea.WithInput(nil),
		tea.WithOutput(output),
		tea.WithContext(ctx),
	)

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	result, ok := finalModel.(usageFetchSpinnerModel)
	if !ok {
		return fmt.Errorf("unexpected final spinner model type %T", finalModel)
	}

	return result.err
}
