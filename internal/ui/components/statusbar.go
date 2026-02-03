package components

import (
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// StatusBar renders contextual help at the bottom of the screen
type StatusBar struct {
	help     help.Model
	bindings []key.Binding
	width    int
}

// NewStatusBar creates a new status bar
func NewStatusBar() *StatusBar {
	h := help.New()
	h.Styles.ShortKey = theme.HelpStyle
	h.Styles.ShortDesc = theme.HelpStyle
	h.Styles.ShortSeparator = theme.HelpStyle
	return &StatusBar{help: h}
}

// SetWidth sets the status bar width
func (s *StatusBar) SetWidth(width int) {
	s.width = width
	s.help.Width = width
}

// SetBindings sets the key bindings to display
func (s *StatusBar) SetBindings(bindings []key.Binding) {
	s.bindings = bindings
}

// View renders the status bar
func (s *StatusBar) View() string {
	helpContent := s.help.ShortHelpView(s.bindings)

	return lipgloss.PlaceHorizontal(s.width, lipgloss.Left, theme.StatusBarStyle.Render(helpContent))
}
