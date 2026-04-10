// Package base provides a reusable Section struct that consolidates the
// bubbles/list setup, spinner management, and filter-mode handling that is
// otherwise duplicated across every section package.
package base

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// Section holds the state that is common to every section: the bubbles list,
// a loading spinner, filter-mode tracking, and the current terminal size.
// Embed this struct in each concrete section type and delegate the shared
// behaviour to its methods.
type Section struct {
	List     list.Model
	Spinner  spinner.Model
	Loading  bool
	IsFilter bool
	Width    int
	Height   int
}

// NewList returns a bubbles list pre-configured with the standard section
// settings (no title, no help bar, status bar visible).
func NewList(items []list.Item) list.Model {
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	return l
}

// NewSpinner returns a spinner using the standard section style.
func NewSpinner() spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return sp
}

// UpdateSpinner advances the spinner animation when the section is loading.
// It should be called at the top of each section's Update method.
func (b *Section) UpdateSpinner(msg tea.Msg) tea.Cmd {
	if !b.Loading {
		return nil
	}
	var cmd tea.Cmd
	b.Spinner, cmd = b.Spinner.Update(msg)
	return cmd
}

// HandleFilterKey processes keyboard events while filter mode is active.
// It forwards every key to the list and, when Esc is pressed, deactivates
// filter mode and clears the contextual key bindings.
//
// Returns (true, cmds) when filter mode was active and the event was consumed,
// or (false, nil) when filter mode is not active.
func (b *Section) HandleFilterKey(msg tea.KeyMsg) (bool, []tea.Cmd) {
	if !b.IsFilter {
		return false, nil
	}
	var cmds []tea.Cmd
	var listCmd tea.Cmd
	b.List, listCmd = b.List.Update(msg)
	cmds = append(cmds, listCmd)
	if key.Matches(msg, keys.Keys.Esc) {
		b.IsFilter = false
		cmds = append(cmds, func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} })
	}
	return true, cmds
}

// ToggleFilter enables filter mode, forwards the triggering key to the list,
// and appends the contextual Esc key binding.  Call this when handling the
// Filter key binding in a section's Update method.
func (b *Section) ToggleFilter(msg tea.KeyMsg) []tea.Cmd {
	b.IsFilter = !b.IsFilter
	var listCmd tea.Cmd
	b.List, listCmd = b.List.Update(msg)
	cmds := []tea.Cmd{listCmd}
	if b.IsFilter {
		cmds = append(cmds, b.ExtendFilterHelpCommand())
	}
	return cmds
}

// ExtendFilterHelpCommand returns a tea.Cmd that adds the Esc key binding to
// the contextual help bar while filter mode is active.
func (b *Section) ExtendFilterHelpCommand() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "exit"),
			),
		}}
	}
}

// SetListSize stores the terminal dimensions and resizes the list, accounting
// for the list style's frame (padding + borders).
func (b *Section) SetListSize(width, height int) {
	b.Width = width
	b.Height = height
	listX, listY := theme.ListStyle.GetFrameSize()
	b.List.SetSize(width-listX, height-listY)
}

// RenderList renders the list content and, when loading, overlays the spinner
// in the bottom-right corner.  loadingText is the label shown next to the
// spinner (e.g. "Loading..." or "Refreshing...").
func (b *Section) RenderList(loadingText string) string {
	content := b.List.View()
	if b.Loading {
		spinnerText := b.Spinner.View() + " " + loadingText
		content = helper.OverlayBottomRight(1, content, spinnerText, b.List.Width())
	}
	return theme.ListStyle.
		Width(b.List.Width()).
		Render(content)
}
