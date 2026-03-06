package confirmation

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

const (
	modalWidth   = 50
	modalPadding = 2
)

var (
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, modalPadding).
			Width(modalWidth)
	titleStyle = lipgloss.NewStyle().Bold(true)
	hintStyle  = lipgloss.NewStyle().Faint(true)
)

// Model holds the state for the confirmation modal.
type Model struct {
	title string
	body  string
}

// New returns a zero-value Model ready for use.
func New() Model {
	return Model{}
}

// Init sets the title and body shown in the modal.
func (m *Model) Init(title, body string) {
	m.title = title
	m.body = body
}

// View renders the modal box.
func (m Model) View() string {
	content := fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		titleStyle.Render(m.title),
		m.body,
		hintStyle.Render("[y] confirm    [n/esc] cancel"),
	)
	return modalStyle.Render(content)
}
