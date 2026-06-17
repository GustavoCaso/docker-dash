package form

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/huh"
)

// Model wraps a huh.Form with a v2 callback that fires when the form completes.
// Note: huh internally uses the old charmbracelet/bubbletea, so cross-module
// message passing is handled via the v2 message loop in app.go.
type Model struct {
	title         string
	form          *huh.Form
	callback      func(*huh.Form) tea.Cmd
	callbackFired bool
}

func New(title string, form *huh.Form, callback func(*huh.Form) tea.Cmd) *Model {
	return &Model{
		title:    title,
		form:     form,
		callback: callback,
	}
}

// Init initialises the form.
func (m *Model) Init() tea.Cmd {
	oldCmd := m.form.Init()
	if oldCmd == nil {
		return nil
	}
	return func() tea.Msg { return oldCmd() }
}

// Update advances the form state. It bridges old-bubbletea messages from huh
// into the v2 message loop.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	form, oldCmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	var cmds []tea.Cmd
	if oldCmd != nil {
		cmds = append(cmds, func() tea.Msg { return oldCmd() })
	}

	if m.form.State == huh.StateCompleted && !m.callbackFired {
		m.callbackFired = true
		cmds = append(cmds, m.callback(m.form))
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) State() huh.FormState {
	return m.form.State
}

func (m *Model) WithWidth(width int) {
	m.form.WithWidth(width)
}

func (m *Model) WithHeight(height int) {
	m.form.WithHeight(height)
}

func (m *Model) View() string {
	if m.form.State == huh.StateCompleted {
		return ""
	}
	var s strings.Builder
	s.WriteString(m.title)
	s.WriteString("\n")
	s.WriteString(m.form.View())
	return s.String()
}
