package statusbar

import (
	"github.com/charmbracelet/bubbles/help"

	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// StatusBar renders contextual help at the bottom of the screen.
type StatusBar struct {
	help   help.Model
	keyMap help.KeyMap
	width  int
	height int
}

// New creates a new status bar.
func New() *StatusBar {
	h := help.New()
	return &StatusBar{help: h}
}

// SetSize sets the status bar size.
func (s *StatusBar) SetSize(width, height int) {
	s.width = width
	s.help.Width = width
	s.height = height
}

// SetKeyMap sets the keyMap to display.
func (s *StatusBar) SetKeyMap(keyMap help.KeyMap) {
	s.keyMap = keyMap
}

func (s *StatusBar) ToggleFullView() {
	s.help.ShowAll = !s.help.ShowAll
}

func (s *StatusBar) IsFullView() bool {
	return s.help.ShowAll
}

// View renders the status bar.
func (s *StatusBar) View() string {
	helpContent := s.help.View(s.keyMap)

	return theme.HelpStyle.Render(helpContent)
}
