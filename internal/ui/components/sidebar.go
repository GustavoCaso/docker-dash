package components

import (
	"strings"

	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// View represents the different views available in the sidebar
type View int

const (
	ViewContainers View = iota
	ViewImages
)

func (v View) String() string {
	switch v {
	case ViewContainers:
		return "Containers"
	case ViewImages:
		return "Images"
	default:
		return "Unknown"
	}
}

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
	width       int
	height      int
}

// NewSidebar creates a new sidebar
func NewSidebar() *Sidebar {
	return &Sidebar{
		items: []sidebarItem{
			{icon: theme.IconImage, label: "Images", view: ViewImages},
			{icon: theme.IconContainer, label: "Containers", view: ViewContainers},
		},
		activeIndex: 0,
		focused:     false,
		width:       24,
		height:      0,
	}
}

func (s *Sidebar) SetSize(width, height int) {
	s.width = width - theme.SidebarStyle.GetHorizontalFrameSize()
	s.height = height - theme.SidebarStyle.GetVerticalFrameSize()
}

func (s *Sidebar) SetFocused(focused bool) {
	s.focused = focused
}

func (s *Sidebar) IsFocused() bool {
	return s.focused
}

func (s *Sidebar) ActiveView() View {
	if s.activeIndex >= 0 && s.activeIndex < len(s.items) {
		return s.items[s.activeIndex].view
	}
	return ViewImages
}

func (s *Sidebar) MoveUp() {
	s.activeIndex--
	if s.activeIndex < 0 {
		s.activeIndex = len(s.items) - 1
	}
}

func (s *Sidebar) MoveDown() {
	s.activeIndex++
	if s.activeIndex >= len(s.items) {
		s.activeIndex = 0
	}
}

func (s *Sidebar) View() string {
	var b strings.Builder

	b.WriteString(theme.LogoStyle.Render(theme.IconDocker + " Docker"))
	b.WriteString("\n\n")

	for i, item := range s.items {
		var style lipgloss.Style
		if i == s.activeIndex {
			style = theme.SidebarActiveStyle
		} else {
			style = theme.SidebarItemStyle
		}

		itemText := item.icon + " " + item.label
		b.WriteString(style.Render(itemText))
		b.WriteString("\n")
	}

	return theme.SidebarStyle.Width(s.width).Height(s.height).Render(b.String())
}
