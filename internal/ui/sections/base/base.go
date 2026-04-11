// Package base provides a reusable Section struct that consolidates the
// bubbles/list setup, spinner management, and filter-mode handling that is
// otherwise duplicated across every section package.
package base

import (
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// Section holds the state that is common to every section: the bubbles list,
// a loading spinner, filter-mode tracking, and the current terminal size.
// Embed this struct in each concrete section type and delegate the shared
// behaviour to its methods.
//
// For sections that display a detail panel alongside the list, populate
// Panels and ActivePanelInitFn in New() and use the panel-aware helpers
// (SetSizeWithPanels, HandlePanelKeys, UpdateActivePanel, RemoveItemAndUpdatePanel,
// RenderWithPanels).
type Section struct {
	List     list.Model
	Spinner  spinner.Model
	Loading  bool
	IsFilter bool
	Width    int
	Height   int

	// Panel support — populated by sections that have a detail panel.
	Panels         []panel.Panel
	ActivePanelIdx int
	PanelWidth     int
	PanelHeight    int

	// ActivePanelInitFn extracts the string passed to Panel.Init from the
	// currently-selected list item.  Set this in New() for panel sections.
	ActivePanelInitFn func(list.Item) string
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

// ActivePanel returns the currently active detail panel.
func (b *Section) ActivePanel() panel.Panel {
	return b.Panels[b.ActivePanelIdx]
}

// DetailsMenu renders the tab bar that appears above the active detail panel.
func (b *Section) DetailsMenu() string {
	sectionsMenu := make([]string, 0, len(b.Panels))
	for idx, p := range b.Panels {
		if idx == b.ActivePanelIdx {
			sectionsMenu = append(sectionsMenu, theme.ActiveTab.Render(p.Name()))
		} else {
			sectionsMenu = append(sectionsMenu, theme.Tab.Render(p.Name()))
		}
	}

	detailsMenu := lipgloss.JoinHorizontal(lipgloss.Top, sectionsMenu...)
	gap := theme.TabGap.Render(strings.Repeat(" ", max(0, b.PanelWidth-lipgloss.Width(detailsMenu))))

	return lipgloss.JoinHorizontal(lipgloss.Bottom, detailsMenu, gap)
}

// SetSizeWithPanels sets dimensions for a section that shows a list alongside
// a detail panel.  It calculates the split widths, resizes both the list and
// the active panel, and stores PanelWidth/PanelHeight for use in View.
func (b *Section) SetSizeWithPanels(width, height int) {
	b.Width = width
	b.Height = height

	// Account for details menu height
	menuHeight := lipgloss.Height(b.DetailsMenu())
	menuX, menuY := theme.Tab.GetFrameSize()

	// Account for padding and borders
	listX, listY := theme.ListStyle.GetFrameSize()

	// Panel Style
	panelX, panelY := theme.NoBorders.GetFrameSize()

	listWidth := int(float64(width) * theme.SplitRatio)
	detailWidth := width - listWidth

	b.List.SetSize(listWidth-listX, height-listY)
	b.PanelWidth = detailWidth - panelX - menuX
	// TODO: Figure out the + 1
	b.PanelHeight = height - menuHeight - menuY - panelY + 1
	b.ActivePanel().SetSize(b.PanelWidth, b.PanelHeight)
}

// HandlePanelKeys handles PanelNext and PanelPrev key bindings, cycling through
// b.Panels.  sectionName is used only for log output.
// Returns (true, cmd) when a panel key was matched, (false, nil) otherwise.
func (b *Section) HandlePanelKeys(msg tea.KeyMsg, sectionName string) (bool, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Keys.PanelNext):
		currentPanel := b.ActivePanel()
		b.ActivePanelIdx = (b.ActivePanelIdx + 1) % len(b.Panels)
		log.Printf("[%s] switching panel to: %q", sectionName, b.Panels[b.ActivePanelIdx].Name())
		return true, tea.Batch(currentPanel.Close(), b.UpdateActivePanel())
	case key.Matches(msg, keys.Keys.PanelPrev):
		currentPanel := b.ActivePanel()
		b.ActivePanelIdx = (b.ActivePanelIdx - 1 + len(b.Panels)) % len(b.Panels)
		log.Printf("[%s] switching panel to: %q", sectionName, b.Panels[b.ActivePanelIdx].Name())
		return true, tea.Batch(currentPanel.Close(), b.UpdateActivePanel())
	}
	return false, nil
}

// UpdateActivePanel calls ActivePanelInitFn on the selected list item and
// passes the result to the active panel's Init method.
// Returns nil when no item is selected or ActivePanelInitFn is not set.
func (b *Section) UpdateActivePanel() tea.Cmd {
	if b.ActivePanelInitFn == nil {
		return nil
	}
	selected := b.List.SelectedItem()
	if selected == nil {
		return nil
	}
	id := b.ActivePanelInitFn(selected)
	if id == "" {
		return nil
	}
	return b.ActivePanel().Init(id)
}

// RemoveItemAndUpdatePanel removes the item at idx from the list, clamps the
// selection, and re-initialises the active panel for the new selection.
// When the list becomes empty it closes the active panel instead.
func (b *Section) RemoveItemAndUpdatePanel(idx int) tea.Cmd {
	b.List.RemoveItem(idx)
	if len(b.List.Items()) == 0 {
		return b.ActivePanel().Close()
	}
	b.List.Select(min(idx, len(b.List.Items())-1))
	return b.UpdateActivePanel()
}

// RenderWithPanels renders the full split-pane view: list on the left,
// tab menu + active panel on the right.  loadingText is shown in the spinner
// overlay while loading.
func (b *Section) RenderWithPanels(loadingText string) string {
	b.SetSizeWithPanels(b.Width, b.Height)

	listView := b.RenderList(loadingText)

	detailContent := b.ActivePanel().View()

	details := theme.NoBorders.
		Width(b.PanelWidth).
		Height(b.PanelHeight).
		Render(detailContent)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		listView,
		lipgloss.JoinVertical(lipgloss.Top, b.DetailsMenu(), details),
	)
}
