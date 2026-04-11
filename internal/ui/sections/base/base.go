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
// (RemoveItemAndUpdatePanel)
//
// To eliminate per-section boilerplate, set the strategy callbacks
// (LoadingText, RefreshCmd, PruneCmd, HandleMsg, HandleKey).
// The shared Init, SetSize, View, Reset, and Update methods on Section will then
// handle the full lifecycle; each concrete section only needs handleMsg and
// handleKey for its domain-specific logic.
type Section struct {
	name string

	List     list.Model
	spinner  spinner.Model
	Loading  bool
	isFilter bool
	width    int
	height   int

	panels         []panel.Panel
	activePanelIdx int
	panelWidth     int
	panelHeight    int

	// ActivePanelInitFn extracts the string passed to Panel.Init from the
	// currently-selected list item.  Set this in New() for panel sections.
	ActivePanelInitFn func(list.Item) string

	LoadingText string
	RefreshCmd  func() tea.Cmd
	PruneCmd    func() tea.Cmd
	// HandleMsg handles section-specific messages inside Update.
	// Returns (cmd, true) when the message was consumed, (nil, false) otherwise.
	HandleMsg func(msg tea.Msg) (tea.Cmd, bool)
	// HandleKey handles section-specific key bindings inside Update.
	// It is called before the shared filter/refresh/prune/navigation keys, so it
	// can intercept any key (e.g. exec-panel routing in the containers section).
	// Returns (cmd, true) when the key was consumed, (nil, false) otherwise.
	HandleKey func(msg tea.KeyMsg) (tea.Cmd, bool)
}

func New(
	name string,
	items []list.Item,
	pannels []panel.Panel,
) *Section {
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &Section{
		name:           name,
		List:           l,
		spinner:        sp,
		panels:         pannels,
		activePanelIdx: 0,
	}
}

// Init implements the bubbletea Model Init method.  For panel sections it
// initialises the active panel for the currently-selected item; for sections
// without panels it is a no-op (updateActivePanel returns nil when
// ActivePanelInitFn is not set).
func (b *Section) Init() tea.Cmd {
	return b.updateActivePanel()
}

// SetSize sets dimensions, using a split-pane layout when panels are
// configured and a single-column layout otherwise.
func (b *Section) SetSize(width, height int) {
	if len(b.panels) > 0 {
		b.setSizeWithPanels(width, height)
	} else {
		b.setListSize(width, height)
	}
}

// View renders the section.
func (b *Section) View() string {
	if len(b.panels) > 0 {
		return b.renderWithPanels(b.LoadingText)
	}
	b.setListSize(b.width, b.height)
	return lipgloss.JoinHorizontal(lipgloss.Top, b.renderList(b.LoadingText))
}

// Reset resets internal state to the initial condition.
func (b *Section) Reset() tea.Cmd {
	b.isFilter = false
	if len(b.panels) > 0 {
		cmd := b.ActivePanel().Close()
		b.setSizeWithPanels(b.width, b.height)
		return cmd
	}
	b.setListSize(b.width, b.height)
	return nil
}

// Update handles messages.  It drives the shared scaffolding (spinner, panel
// navigation, filter mode, refresh, prune, list navigation) and delegates
// domain-specific work to the HandleMsg and HandleKey callbacks.
func (b *Section) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	if spinnerCmd := b.updateSpinner(msg); spinnerCmd != nil {
		cmds = append(cmds, spinnerCmd)
	}

	if b.HandleMsg != nil {
		if cmd, handled := b.HandleMsg(msg); handled {
			cmds = append(cmds, cmd)
			return tea.Batch(cmds...)
		}
	}

	//nolint:nestif // The complexity is acceptable because Update function
	// hanldes all the logic
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		log.Printf("[%s] KeyMsg: key=%q", b.name, keyMsg.String())

		if handled, filterCmds := b.handleFilterKey(keyMsg); handled {
			return tea.Batch(filterCmds...)
		}

		if len(b.panels) > 0 {
			if handled, cmd := b.handlePanelKeys(keyMsg); handled {
				return cmd
			}
		}

		if b.HandleKey != nil {
			if cmd, handled := b.HandleKey(keyMsg); handled {
				cmds = append(cmds, cmd)
				return tea.Batch(cmds...)
			}
		}

		switch {
		case key.Matches(keyMsg, keys.Keys.Refresh):
			if b.RefreshCmd != nil {
				b.Loading = true
				return tea.Batch(b.spinner.Tick, b.RefreshCmd())
			}
		case key.Matches(keyMsg, keys.Keys.Prune):
			if b.PruneCmd != nil {
				return b.PruneCmd()
			}
		case key.Matches(keyMsg, keys.Keys.Up, keys.Keys.Down):
			if len(b.panels) > 0 {
				currentPanel := b.ActivePanel()
				var listCmd tea.Cmd
				b.List, listCmd = b.List.Update(keyMsg)
				return tea.Batch(listCmd, currentPanel.Close(), b.updateActivePanel())
			}
			var listCmd tea.Cmd
			b.List, listCmd = b.List.Update(keyMsg)
			return listCmd
		case key.Matches(keyMsg, keys.Keys.Filter):
			return tea.Batch(b.toggleFilter(keyMsg)...)
		}
	}

	if len(b.panels) > 0 {
		cmds = append(cmds, b.ActivePanel().Update(msg))
	}

	return tea.Batch(cmds...)
}

// ActivePanel returns the currently active detail panel.
func (b *Section) ActivePanel() panel.Panel {
	return b.panels[b.activePanelIdx]
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
	return b.updateActivePanel()
}

// RemoveItem removes the item at idx from the list and clamps the selection.
// Use this for sections without a detail panel; for panel sections use
// RemoveItemAndUpdatePanel instead.
func (b *Section) RemoveItem(idx int) {
	b.List.RemoveItem(idx)
	if len(b.List.Items()) > 0 {
		b.List.Select(min(idx, len(b.List.Items())-1))
	}
}

// updateSpinner advances the spinner animation when the section is loading.
// It should be called at the top of each section's Update method.
func (b *Section) updateSpinner(msg tea.Msg) tea.Cmd {
	if !b.Loading {
		return nil
	}
	var cmd tea.Cmd
	b.spinner, cmd = b.spinner.Update(msg)
	return cmd
}

// handleFilterKey processes keyboard events while filter mode is active.
// It forwards every key to the list and, when Esc is pressed, deactivates
// filter mode and clears the contextual key bindings.
//
// Returns (true, cmds) when filter mode was active and the event was consumed,
// or (false, nil) when filter mode is not active.
func (b *Section) handleFilterKey(msg tea.KeyMsg) (bool, []tea.Cmd) {
	if !b.isFilter {
		return false, nil
	}
	var cmds []tea.Cmd
	var listCmd tea.Cmd
	b.List, listCmd = b.List.Update(msg)
	cmds = append(cmds, listCmd)
	if key.Matches(msg, keys.Keys.Esc) {
		b.isFilter = false
		cmds = append(cmds, func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} })
	}
	return true, cmds
}

// toggleFilter enables filter mode, forwards the triggering key to the list,
// and appends the contextual Esc key binding.  Call this when handling the
// Filter key binding in a section's Update method.
func (b *Section) toggleFilter(msg tea.KeyMsg) []tea.Cmd {
	b.isFilter = !b.isFilter
	var listCmd tea.Cmd
	b.List, listCmd = b.List.Update(msg)
	cmds := []tea.Cmd{listCmd}
	if b.isFilter {
		cmds = append(cmds, b.extendFilterHelpCommand())
	}
	return cmds
}

// extendFilterHelpCommand returns a tea.Cmd that adds the Esc key binding to
// the contextual help bar while filter mode is active.
func (b *Section) extendFilterHelpCommand() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			keys.Keys.Esc,
		}}
	}
}

// setListSize stores the terminal dimensions and resizes the list, accounting
// for the list style's frame (padding + borders).
func (b *Section) setListSize(width, height int) {
	b.width = width
	b.height = height
	listX, listY := theme.ListStyle.GetFrameSize()
	b.List.SetSize(width-listX, height-listY)
}

// renderList renders the list content and, when loading, overlays the spinner
// in the bottom-right corner.  LoadingText is the label shown next to the
// spinner (e.g. "loading..." or "Refreshing...").
func (b *Section) renderList(loadingText string) string {
	content := b.List.View()
	if b.Loading {
		spinnerText := b.spinner.View() + " " + loadingText
		content = helper.OverlayBottomRight(1, content, spinnerText, b.List.Width())
	}
	return theme.ListStyle.
		Width(b.List.Width()).
		Render(content)
}

// DetailsMenu renders the tab bar that appears above the active detail panel.
func (b *Section) detailsMenu() string {
	sectionsMenu := make([]string, 0, len(b.panels))
	for idx, p := range b.panels {
		if idx == b.activePanelIdx {
			sectionsMenu = append(sectionsMenu, theme.ActiveTab.Render(p.Name()))
		} else {
			sectionsMenu = append(sectionsMenu, theme.Tab.Render(p.Name()))
		}
	}

	detailsMenu := lipgloss.JoinHorizontal(lipgloss.Top, sectionsMenu...)
	gap := theme.TabGap.Render(strings.Repeat(" ", max(0, b.panelWidth-lipgloss.Width(detailsMenu))))

	return lipgloss.JoinHorizontal(lipgloss.Bottom, detailsMenu, gap)
}

// setSizeWithPanels sets dimensions for a section that shows a list alongside
// a detail panel.  It calculates the split widths, resizes both the list and
// the active panel, and stores panelWidth/panelHeight for use in View.
func (b *Section) setSizeWithPanels(width, height int) {
	b.width = width
	b.height = height

	// Account for details menu height
	menuHeight := lipgloss.Height(b.detailsMenu())
	menuX, menuY := theme.Tab.GetFrameSize()

	// Account for padding and borders
	listX, listY := theme.ListStyle.GetFrameSize()

	// Panel Style
	panelX, panelY := theme.NoBorders.GetFrameSize()

	listWidth := int(float64(width) * theme.SplitRatio)
	detailWidth := width - listWidth

	b.List.SetSize(listWidth-listX, height-listY)
	b.panelWidth = detailWidth - panelX - menuX
	// TODO: Figure out the + 1
	b.panelHeight = height - menuHeight - menuY - panelY + 1
	b.ActivePanel().SetSize(b.panelWidth, b.panelHeight)
}

// handlePanelKeys handles PanelNext and PanelPrev key bindings, cycling through
// b.panels.  b.name is used for log output.
// Returns (true, cmd) when a panel key was matched, (false, nil) otherwise.
func (b *Section) handlePanelKeys(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Keys.PanelNext):
		currentPanel := b.ActivePanel()
		b.activePanelIdx = (b.activePanelIdx + 1) % len(b.panels)
		log.Printf("[%s] switching panel to: %q", b.name, b.panels[b.activePanelIdx].Name())
		return true, tea.Batch(currentPanel.Close(), b.updateActivePanel())
	case key.Matches(msg, keys.Keys.PanelPrev):
		currentPanel := b.ActivePanel()
		b.activePanelIdx = (b.activePanelIdx - 1 + len(b.panels)) % len(b.panels)
		log.Printf("[%s] switching panel to: %q", b.name, b.panels[b.activePanelIdx].Name())
		return true, tea.Batch(currentPanel.Close(), b.updateActivePanel())
	}
	return false, nil
}

// updateActivePanel calls ActivePanelInitFn on the selected list item and
// passes the result to the active panel's Init method.
// Returns nil when no item is selected or ActivePanelInitFn is not set.
func (b *Section) updateActivePanel() tea.Cmd {
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

// renderWithPanels renders the full split-pane view: list on the left,
// tab menu + active panel on the right.  LoadingText is shown in the spinner
// overlay while loading.
func (b *Section) renderWithPanels(loadingText string) string {
	b.setSizeWithPanels(b.width, b.height)

	listView := b.renderList(loadingText)

	detailContent := b.ActivePanel().View()

	details := theme.NoBorders.
		Width(b.panelWidth).
		Height(b.panelHeight).
		Render(detailContent)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		listView,
		lipgloss.JoinVertical(lipgloss.Top, b.detailsMenu(), details),
	)
}
