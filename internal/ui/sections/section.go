package sections

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type SectionName string

var ImagesSection SectionName = "images"
var ContainersSection SectionName = "containers"
var VolumesSection SectionName = "volumes"
var NetworksSection SectionName = "networks"
var ComposeSection SectionName = "compose"

type ListItem interface {
	ID() string
	InnerItem() any
	Title() string
	Description() string
	FilterValue() string
}

type Panel interface {
	Name() string
	Init(item ListItem) tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	View() string
	Close() tea.Cmd
	SetSize(width, height int)
}

type Section interface {
	// Initialize Section
	Init() tea.Cmd
	// SetSize sets dimensions.
	SetSize(width, height int)
	// Update handles messages.
	Update(msg tea.Msg) tea.Cmd
	// View renders the list.
	View() string
	// IsFilter returns if the filter is active or nor
	IsFilter() bool
	// ActivePanel returns the active panel
	ActivePanel() Panel
	// ActivePanelName returns the active panel name or an empty string when the
	// section has no panels.
	ActivePanelName() string
	// IsPanelFocused returns true when a section with panels has the focus on
	// the panel side, false otherwise.
	IsPanelFocused() bool
	// UpdateItems update the list items
	// If there are no items to set it reset the section otherewise if update the active panel
	UpdateItems(items []list.Item) []tea.Cmd
	// RemoveItem removes the item at idx from the list and clamps the selection.
	// Use this for sections without a detail panel; for panel sections use
	// RemoveItemAndUpdatePanel instead.
	RemoveItem(idx int)
	// RemoveItemAndUpdatePanel removes the item at idx from the list, clamps the
	// selection, and re-initialises the active panel for the new selection.
	// When the list becomes empty it closes the active panel instead.
	RemoveItemAndUpdatePanel(idx int) tea.Cmd
	// Reset reset internal state to when a component is first initialized.
	Reset() tea.Cmd
}
