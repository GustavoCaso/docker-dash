package panel

import tea "github.com/charmbracelet/bubbletea"

type Panel interface {
	Init(containerID string) tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	View() string
	Close()
	SetSize(width, height int)
}
