package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// Action represents a container action button
type Action struct {
	Name string
	Key  string
}

// ContainerList represents a list of Docker containers with inline expansion
type ContainerList struct {
	containers     []service.Container
	selectedIndex  int
	expandedIndex  int // -1 means no container is expanded
	focused        bool
	width          int
	height         int
	actionIndex    int
	actionsFocused bool
}

// NewContainerList creates a new container list with the given containers
func NewContainerList(containers []service.Container) *ContainerList {
	return &ContainerList{
		containers:     containers,
		selectedIndex:  0,
		expandedIndex:  -1,
		focused:        false,
		width:          80,
		height:         20,
		actionIndex:    0,
		actionsFocused: false,
	}
}

// SetContainers updates the list of containers
func (c *ContainerList) SetContainers(containers []service.Container) {
	c.containers = containers
	// Reset selection if out of bounds
	if c.selectedIndex >= len(containers) {
		c.selectedIndex = 0
	}
	// Reset expanded state if out of bounds
	if c.expandedIndex >= len(containers) {
		c.expandedIndex = -1
	}
}

// SetSize sets the width and height of the container list
func (c *ContainerList) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// SetFocused sets the focused state of the container list
func (c *ContainerList) SetFocused(focused bool) {
	c.focused = focused
}

// IsFocused returns whether the container list is focused
func (c *ContainerList) IsFocused() bool {
	return c.focused
}

// SelectedIndex returns the currently selected container index
func (c *ContainerList) SelectedIndex() int {
	return c.selectedIndex
}

// SelectedContainer returns the currently selected container, or nil if none
func (c *ContainerList) SelectedContainer() *service.Container {
	if len(c.containers) == 0 || c.selectedIndex < 0 || c.selectedIndex >= len(c.containers) {
		return nil
	}
	return &c.containers[c.selectedIndex]
}

// IsExpanded returns whether the container at the given index is expanded
func (c *ContainerList) IsExpanded(index int) bool {
	return c.expandedIndex == index
}

// MoveUp moves the selection up, wrapping to the bottom if at the top
func (c *ContainerList) MoveUp() {
	if len(c.containers) == 0 {
		return
	}
	c.selectedIndex--
	if c.selectedIndex < 0 {
		c.selectedIndex = len(c.containers) - 1
	}
	c.actionIndex = 0
	c.actionsFocused = false
}

// MoveDown moves the selection down, wrapping to the top if at the bottom
func (c *ContainerList) MoveDown() {
	if len(c.containers) == 0 {
		return
	}
	c.selectedIndex++
	if c.selectedIndex >= len(c.containers) {
		c.selectedIndex = 0
	}
	c.actionIndex = 0
	c.actionsFocused = false
}

// ToggleExpand toggles the expansion of the currently selected container
func (c *ContainerList) ToggleExpand() {
	if len(c.containers) == 0 {
		return
	}
	if c.expandedIndex == c.selectedIndex {
		// Collapse if already expanded
		c.expandedIndex = -1
		c.actionsFocused = false
		c.actionIndex = 0
	} else {
		// Expand the selected container
		c.expandedIndex = c.selectedIndex
		c.actionIndex = 0
	}
}

// SetActionsFocused sets whether the action buttons are focused
func (c *ContainerList) SetActionsFocused(focused bool) {
	c.actionsFocused = focused
}

// ActionsFocused returns whether the action buttons are focused
func (c *ContainerList) ActionsFocused() bool {
	return c.actionsFocused
}

// SelectedAction returns the currently selected action index
func (c *ContainerList) SelectedAction() int {
	return c.actionIndex
}

// MoveActionLeft moves the action selection left
func (c *ContainerList) MoveActionLeft() {
	if c.actionIndex > 0 {
		c.actionIndex--
	}
}

// MoveActionRight moves the action selection right
func (c *ContainerList) MoveActionRight() {
	actions := c.getActions()
	if c.actionIndex < len(actions)-1 {
		c.actionIndex++
	}
}

// getActions returns the appropriate actions based on the selected container's state
func (c *ContainerList) getActions() []Action {
	container := c.SelectedContainer()
	if container == nil {
		return []Action{}
	}

	switch container.State {
	case service.StateRunning:
		return []Action{
			{Name: "Logs", Key: "l"},
			{Name: "Shell", Key: "s"},
			{Name: "Stop", Key: "S"},
			{Name: "Restart", Key: "r"},
			{Name: "Remove", Key: "d"},
		}
	case service.StatePaused:
		return []Action{
			{Name: "Logs", Key: "l"},
			{Name: "Unpause", Key: "u"},
			{Name: "Stop", Key: "S"},
			{Name: "Remove", Key: "d"},
		}
	default: // Stopped, Exited, etc.
		return []Action{
			{Name: "Start", Key: "s"},
			{Name: "Remove", Key: "d"},
		}
	}
}

// RunningCount returns the number of running containers
func (c *ContainerList) RunningCount() int {
	count := 0
	for _, container := range c.containers {
		if container.State == service.StateRunning {
			count++
		}
	}
	return count
}

// StoppedCount returns the number of stopped containers
func (c *ContainerList) StoppedCount() int {
	count := 0
	for _, container := range c.containers {
		if container.State == service.StateStopped {
			count++
		}
	}
	return count
}

// formatPorts formats port mappings for display
func formatPorts(ports []service.PortMapping) string {
	if len(ports) == 0 {
		return "-"
	}
	var parts []string
	for _, p := range ports {
		if p.HostPort > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.HostPort, p.ContainerPort, p.Protocol))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
		}
	}
	return strings.Join(parts, ", ")
}

// formatMounts formats mount points for display
func formatMounts(mounts []service.Mount) string {
	if len(mounts) == 0 {
		return "-"
	}
	var parts []string
	for _, m := range mounts {
		parts = append(parts, fmt.Sprintf("%s -> %s", m.Source, m.Destination))
	}
	return strings.Join(parts, ", ")
}

// View renders the container list as a string
func (c *ContainerList) View() string {
	if len(c.containers) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(theme.TextMuted).
			Italic(true)
		return emptyStyle.Render("No containers found")
	}

	var b strings.Builder

	for i, container := range c.containers {
		isSelected := i == c.selectedIndex
		isExpanded := i == c.expandedIndex

		// Render the container row
		b.WriteString(c.renderContainerRow(container, isSelected, isExpanded))
		b.WriteString("\n")

		// Render expanded details if this container is expanded
		if isExpanded {
			b.WriteString(c.renderExpandedDetails(container))
		}
	}

	return b.String()
}

// renderContainerRow renders a single container row
func (c *ContainerList) renderContainerRow(container service.Container, isSelected, isExpanded bool) string {
	// Determine the expansion icon
	expandIcon := theme.IconCollapsed
	if isExpanded {
		expandIcon = theme.IconExpanded
	}

	// Get state icon and style
	stateIcon := theme.StatusIcon(string(container.State))
	stateStyle := theme.StatusStyle(string(container.State))

	// Format ports for the row
	portsStr := ""
	if len(container.Ports) > 0 {
		portsStr = fmt.Sprintf(" [%s]", formatPorts(container.Ports))
	}

	// Build the row content
	rowContent := fmt.Sprintf("%s %s %s  %s%s",
		expandIcon,
		stateStyle.Render(stateIcon),
		container.Name,
		container.Status,
		portsStr,
	)

	// Apply styling based on selection
	var style lipgloss.Style
	if isSelected {
		style = theme.ListItemSelectedStyle
	} else {
		style = theme.ListItemStyle
	}

	return style.Width(c.width - 4).Render(rowContent)
}

// renderExpandedDetails renders the expanded view of a container
func (c *ContainerList) renderExpandedDetails(container service.Container) string {
	var b strings.Builder

	// Indent for expanded content
	indent := "    "

	// Detail row style
	labelStyle := theme.DetailLabelStyle
	valueStyle := theme.DetailValueStyle

	// ID
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("ID:"))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(container.ID))
	b.WriteString("\n")

	// Image
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("Image:"))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(container.Image))
	b.WriteString("\n")

	// Status
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("Status:"))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(container.Status))
	b.WriteString("\n")

	// Ports
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("Ports:"))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(formatPorts(container.Ports)))
	b.WriteString("\n")

	// Mounts
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("Mounts:"))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(formatMounts(container.Mounts)))
	b.WriteString("\n")

	// Action buttons
	b.WriteString(indent)
	b.WriteString(c.renderActionButtons())
	b.WriteString("\n")

	return b.String()
}

// renderActionButtons renders the action buttons for the expanded container
func (c *ContainerList) renderActionButtons() string {
	actions := c.getActions()
	if len(actions) == 0 {
		return ""
	}

	var parts []string
	for i, action := range actions {
		var style lipgloss.Style
		if c.actionsFocused && i == c.actionIndex {
			style = theme.ActionButtonActiveStyle
		} else {
			style = theme.ActionButtonStyle
		}
		parts = append(parts, style.Render(action.Name))
	}

	return strings.Join(parts, " ")
}
