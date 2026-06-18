package form

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// Model wraps a huh.Form.
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
	cmd := m.form.Init()
	if cmd == nil {
		return nil
	}
	return cmd
}

// Update advances the form state.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	var cmds []tea.Cmd
	if cmd != nil {
		cmds = append(cmds, cmd)
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
