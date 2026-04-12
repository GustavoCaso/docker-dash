package sections

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
)

type SectionName string

var ImagesSection SectionName = "images"
var ContainersSection SectionName = "containers"
var VolumesSection SectionName = "volumes"
var NetworksSection SectionName = "networks"
var ComposeSection SectionName = "compose"

type Section interface {
	// Initialize Section
	Init() tea.Cmd
	// SetSize sets dimensions.
	SetSize(width, height int)
	// Update handles messages.
	Update(msg tea.Msg) tea.Cmd
	// View renders the list.
	View() string
	// ActivePanel returns the active panel
	ActivePanel() panel.Panel
	// ActivePanelName returns the active panel name or an empty string when the
	// section has no panels.
	ActivePanelName() string
	// RemoveItem removes the item at idx from the list and clamps the selection.
	// Use this for sections without a detail panel; for panel sections use
	// RemoveItemAndUpdatePanel instead.
	RemoveItem(idx int)
	// RemoveItemAndUpdatePanel removes the item at idx from the list, clamps the
	// selection, and re-initialises the active panel for the new selection.
	// When the list becomes empty it closes the active panel instead.
	RemoveItemAndUpdatePanel(idx int) tea.Cmd
	// Reset reset internal state to when a component isfirst initialized.
	Reset() tea.Cmd
}
