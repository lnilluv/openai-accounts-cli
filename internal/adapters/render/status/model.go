package status

import (
	"errors"
	"io"

	"github.com/bnema/openai-accounts-cli/internal/application"
	tea "github.com/charmbracelet/bubbletea"
)

var ErrUnexpectedRenderModel = errors.New("unexpected final bubbletea model type")

type renderReadyMsg struct{}

type model struct {
	statuses []application.Status
	opts     RenderOptions
	styles   styles
	output   string
}

func newModel(statuses []application.Status, opts RenderOptions) model {
	return model{
		statuses: statuses,
		opts:     opts,
		styles:   newStyles(),
	}
}

func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		return renderReadyMsg{}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case renderReadyMsg:
		m.output = renderView(m.statuses, m.opts, m.styles)
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m model) View() string {
	return m.output
}

func Render(statuses []application.Status, opts RenderOptions) (string, error) {
	p := tea.NewProgram(
		newModel(statuses, opts),
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
	)

	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	rendered, ok := finalModel.(model)
	if !ok {
		return "", ErrUnexpectedRenderModel
	}

	return rendered.View(), nil
}
