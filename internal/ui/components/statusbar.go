package components

import (
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

// StatusBar renders contextual help at the bottom of the screen
type StatusBar struct {
	help     help.Model
	bindings []key.Binding
	width    int
	height   int
}

// NewStatusBar creates a new status bar
func NewStatusBar() *StatusBar {
	h := help.New()
	return &StatusBar{help: h}
}

// SetWidth sets the status bar width
func (s *StatusBar) SetSize(width, height int) {
	s.width = width
	s.help.Width = width
	s.height = height
}

// SetBindings sets the key bindings to display
func (s *StatusBar) SetBindings(bindings []key.Binding) {
	s.bindings = bindings
}

// View renders the status bar
func (s *StatusBar) View() string {
	helpContent := s.help.ShortHelpView(s.bindings)

	return theme.HelpStyle.Render(helpContent)
}
