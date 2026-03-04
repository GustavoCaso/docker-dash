package sections

import tea "github.com/charmbracelet/bubbletea"

type Section interface {
	// SetSize sets dimensions.
	SetSize(width, height int)
	// Update handles messages.
	Update(msg tea.Msg) tea.Cmd
	// View renders the list.
	View() string
	// Reset reset internal state to when a component isfirst initialized.
	Reset() tea.Cmd
}
