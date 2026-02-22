package components

import (
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// View represents the different views available in the header
type View int

const (
	ViewContainers View = iota
	ViewImages
	ViewVolumes
)

func (v View) String() string {
	switch v {
	case ViewContainers:
		return "Containers"
	case ViewImages:
		return "Images"
	case ViewVolumes:
		return "Volumes"
	default:
		return "Unknown"
	}
}

type headerItem struct {
	icon  string
	label string
	view  View
}

// Header represents the top navigation tab bar component.
type Header struct {
	logo        string
	items       []headerItem
	activeIndex int
	width       int
}

// NewHeader creates a new header with all sections.
func NewHeader() *Header {
	return &Header{
		logo: theme.IconDocker + "  Docker Dash",
		items: []headerItem{
			{icon: theme.IconImage, label: "Images", view: ViewImages},
			{icon: theme.IconContainer, label: "Containers", view: ViewContainers},
			{icon: theme.IconVolume, label: "Volumes", view: ViewVolumes},
		},
		activeIndex: 0,
	}
}

// SetWidth sets the total available width for the header.
func (h *Header) SetWidth(width int) {
	h.width = width
}

// ActiveView returns the currently selected view.
func (h *Header) ActiveView() View {
	if h.activeIndex >= 0 && h.activeIndex < len(h.items) {
		return h.items[h.activeIndex].view
	}
	return ViewImages
}

// MoveLeft navigates to the previous section, wrapping around.
func (h *Header) MoveLeft() {
	h.activeIndex--
	if h.activeIndex < 0 {
		h.activeIndex = len(h.items) - 1
	}
}

// MoveRight navigates to the next section, wrapping around.
func (h *Header) MoveRight() {
	h.activeIndex++
	if h.activeIndex >= len(h.items) {
		h.activeIndex = 0
	}
}

// View renders the horizontal tab bar with Docker icon pinned to the right.
func (h *Header) View() string {
	var tabParts []string
	sep := theme.HeaderSeparatorStyle.Render("│")

	for i, item := range h.items {
		text := item.icon + " " + item.label
		var rendered string
		if i == h.activeIndex {
			rendered = theme.HeaderActiveItemStyle.Render(text)
		} else {
			rendered = theme.HeaderItemStyle.Render(text)
		}
		tabParts = append(tabParts, rendered)
		if i < len(h.items)-1 {
			tabParts = append(tabParts, sep)
		}
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Center, tabParts...)
	logo := theme.HeaderDockerStyle.Render(h.logo)

	tabBarWidth := lipgloss.Width(tabBar)
	iconWidth := lipgloss.Width(logo)
	spacerWidth := h.width - tabBarWidth - iconWidth
	if spacerWidth < 0 {
		spacerWidth = 0
	}
	spacer := lipgloss.NewStyle().Width(spacerWidth).Render("")

	row := lipgloss.JoinHorizontal(lipgloss.Center, tabBar, spacer, logo)
	return theme.HeaderBarStyle.Width(h.width).Render(row)
}
