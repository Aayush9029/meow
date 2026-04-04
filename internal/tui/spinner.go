package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ActionDoneMsg signals a successful non-file operation.
type ActionDoneMsg struct {
	Result string
}

// ActionErrMsg signals a failed operation.
type ActionErrMsg struct {
	Err error
}

var (
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// SpinnerModel shows a spinner during operations without progress (URL upload, delete, album).
type SpinnerModel struct {
	spinner spinner.Model
	status  string
	done    bool
	Result  string
	Err     error
	action  func() tea.Msg
}

func NewSpinnerModel(status string, action func() tea.Msg) *SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle
	return &SpinnerModel{
		spinner: s,
		status:  status,
		action:  action,
	}
}

func (m *SpinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg { return m.action() })
}

func (m *SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case ActionDoneMsg:
		m.done = true
		m.Result = msg.Result
		return m, tea.Quit
	case ActionErrMsg:
		m.done = true
		m.Err = msg.Err
		return m, tea.Quit
	case spinner.TickMsg:
		if !m.done {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *SpinnerModel) View() string {
	if m.Err != nil {
		return errStyle.Render("✗ " + m.Err.Error())
	}
	if m.done {
		return "" // main.go handles final output
	}
	return fmt.Sprintf("  %s %s", m.spinner.View(), m.status)
}
