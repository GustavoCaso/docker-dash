// Package components provides UI components for the docker-dash TUI application.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// View represents the different views available in the sidebar
type View int

const (
	ViewContainers View = iota
	ViewImages
	ViewVolumes
)

// String returns the string representation of the View
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

// sidebarItem represents a single item in the sidebar navigation
type sidebarItem struct {
	icon  string
	label string
	view  View
}

// Sidebar represents the sidebar navigation component
type Sidebar struct {
	items       []sidebarItem
	activeIndex int
	focused     bool
	height      int
}

// NewSidebar creates a new sidebar with default navigation items
func NewSidebar() *Sidebar {
	return &Sidebar{
		items: []sidebarItem{
			{icon: theme.IconContainer, label: "Containers", view: ViewContainers},
			{icon: theme.IconImage, label: "Images", view: ViewImages},
			{icon: theme.IconVolume, label: "Volumes", view: ViewVolumes},
		},
		activeIndex: 0,
		focused:     false,
		height:      0,
	}
}

// SetHeight sets the height of the sidebar
func (s *Sidebar) SetHeight(height int) {
	s.height = height
}

// SetFocused sets the focused state of the sidebar
func (s *Sidebar) SetFocused(focused bool) {
	s.focused = focused
}

// IsFocused returns whether the sidebar is focused
func (s *Sidebar) IsFocused() bool {
	return s.focused
}

// ActiveIndex returns the currently selected item index
func (s *Sidebar) ActiveIndex() int {
	return s.activeIndex
}

// ActiveView returns the View type of the currently selected item
func (s *Sidebar) ActiveView() View {
	if s.activeIndex >= 0 && s.activeIndex < len(s.items) {
		return s.items[s.activeIndex].view
	}
	return ViewContainers
}

// MoveUp moves the selection up, wrapping to the bottom if at the top
func (s *Sidebar) MoveUp() {
	s.activeIndex--
	if s.activeIndex < 0 {
		s.activeIndex = len(s.items) - 1
	}
}

// MoveDown moves the selection down, wrapping to the top if at the bottom
func (s *Sidebar) MoveDown() {
	s.activeIndex++
	if s.activeIndex >= len(s.items) {
		s.activeIndex = 0
	}
}

// View renders the sidebar as a string
func (s *Sidebar) View() string {
	var b strings.Builder

	// Docker logo header
	logoStyle := lipgloss.NewStyle().
		Foreground(theme.DockerBlue).
		Bold(true).
		MarginBottom(2)

	b.WriteString(logoStyle.Render(theme.IconDocker + " Docker"))
	b.WriteString("\n\n")

	// Render navigation items
	for i, item := range s.items {
		var style lipgloss.Style
		if i == s.activeIndex {
			style = theme.SidebarActiveStyle
		} else {
			style = theme.SidebarItemStyle
		}

		// Add focus indicator if sidebar is focused and this is the active item
		itemText := item.icon + " " + item.label
		b.WriteString(style.Render(itemText))
		b.WriteString("\n")
	}

	// Calculate remaining space and add padding if needed
	contentHeight := 3 + len(s.items) // logo + spacing + items
	if s.height > contentHeight {
		padding := s.height - contentHeight
		for i := 0; i < padding; i++ {
			b.WriteString("\n")
		}
	}

	// Apply sidebar container style
	return theme.SidebarStyle.Height(s.height).Render(b.String())
}
