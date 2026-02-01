package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// VolumeAction represents a volume action button
type VolumeAction struct {
	Name string
	Key  string
}

// VolumeList represents a list of Docker volumes with inline expansion
type VolumeList struct {
	volumes        []service.Volume
	selectedIndex  int
	expandedIndex  int // -1 means no volume is expanded
	focused        bool
	width          int
	height         int
	actionIndex    int
	actionsFocused bool
}

// NewVolumeList creates a new volume list with the given volumes
func NewVolumeList(volumes []service.Volume) *VolumeList {
	return &VolumeList{
		volumes:        volumes,
		selectedIndex:  0,
		expandedIndex:  -1,
		focused:        false,
		width:          80,
		height:         20,
		actionIndex:    0,
		actionsFocused: false,
	}
}

// SetVolumes updates the list of volumes
func (v *VolumeList) SetVolumes(volumes []service.Volume) {
	v.volumes = volumes
	// Reset selection if out of bounds
	if v.selectedIndex >= len(volumes) {
		v.selectedIndex = 0
	}
	// Reset expanded state if out of bounds
	if v.expandedIndex >= len(volumes) {
		v.expandedIndex = -1
	}
}

// SetSize sets the width and height of the volume list
func (v *VolumeList) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetFocused sets the focused state of the volume list
func (v *VolumeList) SetFocused(focused bool) {
	v.focused = focused
}

// IsFocused returns whether the volume list is focused
func (v *VolumeList) IsFocused() bool {
	return v.focused
}

// SelectedIndex returns the currently selected volume index
func (v *VolumeList) SelectedIndex() int {
	return v.selectedIndex
}

// SelectedVolume returns the currently selected volume, or nil if none
func (v *VolumeList) SelectedVolume() *service.Volume {
	if len(v.volumes) == 0 || v.selectedIndex < 0 || v.selectedIndex >= len(v.volumes) {
		return nil
	}
	return &v.volumes[v.selectedIndex]
}

// IsExpanded returns whether the volume at the given index is expanded
func (v *VolumeList) IsExpanded(index int) bool {
	return v.expandedIndex == index
}

// MoveUp moves the selection up, wrapping to the bottom if at the top
func (v *VolumeList) MoveUp() {
	if len(v.volumes) == 0 {
		return
	}
	v.selectedIndex--
	if v.selectedIndex < 0 {
		v.selectedIndex = len(v.volumes) - 1
	}
	v.actionIndex = 0
	v.actionsFocused = false
}

// MoveDown moves the selection down, wrapping to the top if at the bottom
func (v *VolumeList) MoveDown() {
	if len(v.volumes) == 0 {
		return
	}
	v.selectedIndex++
	if v.selectedIndex >= len(v.volumes) {
		v.selectedIndex = 0
	}
	v.actionIndex = 0
	v.actionsFocused = false
}

// ToggleExpand toggles the expansion of the currently selected volume
func (v *VolumeList) ToggleExpand() {
	if len(v.volumes) == 0 {
		return
	}
	if v.expandedIndex == v.selectedIndex {
		// Collapse if already expanded
		v.expandedIndex = -1
		v.actionsFocused = false
		v.actionIndex = 0
	} else {
		// Expand the selected volume
		v.expandedIndex = v.selectedIndex
		v.actionIndex = 0
	}
}

// SetActionsFocused sets whether the action buttons are focused
func (v *VolumeList) SetActionsFocused(focused bool) {
	v.actionsFocused = focused
}

// ActionsFocused returns whether the action buttons are focused
func (v *VolumeList) ActionsFocused() bool {
	return v.actionsFocused
}

// SelectedAction returns the currently selected action index
func (v *VolumeList) SelectedAction() int {
	return v.actionIndex
}

// MoveActionLeft moves the action selection left
func (v *VolumeList) MoveActionLeft() {
	if v.actionIndex > 0 {
		v.actionIndex--
	}
}

// MoveActionRight moves the action selection right
func (v *VolumeList) MoveActionRight() {
	actions := v.getActions()
	if v.actionIndex < len(actions)-1 {
		v.actionIndex++
	}
}

// getActions returns the available actions for volumes
func (v *VolumeList) getActions() []VolumeAction {
	return []VolumeAction{
		{Name: "Browse", Key: "b"},
		{Name: "Inspect", Key: "i"},
		{Name: "Remove", Key: "d"},
	}
}

// formatVolumeSize formats a volume size in bytes to a human-readable string
func formatVolumeSize(size int64) string {
	if size < 0 {
		return "-"
	}
	if size == 0 {
		return "0 B"
	}

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case size >= TB:
		return fmt.Sprintf("%.1f TB", float64(size)/float64(TB))
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// View renders the volume list as a string
func (v *VolumeList) View() string {
	if len(v.volumes) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(theme.TextMuted).
			Italic(true)
		return emptyStyle.Render("No volumes found")
	}

	var b strings.Builder

	for i, volume := range v.volumes {
		isSelected := i == v.selectedIndex
		isExpanded := i == v.expandedIndex

		// Render the volume row
		b.WriteString(v.renderVolumeRow(volume, isSelected, isExpanded))
		b.WriteString("\n")

		// Render expanded details if this volume is expanded
		if isExpanded {
			b.WriteString(v.renderExpandedDetails(volume))
		}
	}

	return b.String()
}

// renderVolumeRow renders a single volume row
func (v *VolumeList) renderVolumeRow(volume service.Volume, isSelected, isExpanded bool) string {
	// Determine the expansion icon
	expandIcon := theme.IconCollapsed
	if isExpanded {
		expandIcon = theme.IconExpanded
	}

	// Volume icon
	volumeIcon := theme.IconVolume

	// Format size for the row
	sizeStr := formatVolumeSize(volume.Size)

	// Build the row content
	rowContent := fmt.Sprintf("%s %s %s  %s  %s",
		expandIcon,
		volumeIcon,
		volume.Name,
		volume.Driver,
		sizeStr,
	)

	// Apply styling based on selection
	var style lipgloss.Style
	if isSelected {
		style = theme.ListItemSelectedStyle
	} else {
		style = theme.ListItemStyle
	}

	return style.Width(v.width - 4).Render(rowContent)
}

// renderExpandedDetails renders the expanded view of a volume
func (v *VolumeList) renderExpandedDetails(volume service.Volume) string {
	var b strings.Builder

	// Indent for expanded content
	indent := "    "

	// Detail row style
	labelStyle := theme.DetailLabelStyle
	valueStyle := theme.DetailValueStyle

	// Mount path
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("Mount path:"))
	b.WriteString(" ")
	mountPath := volume.MountPath
	if mountPath == "" {
		mountPath = "-"
	}
	b.WriteString(valueStyle.Render(mountPath))
	b.WriteString("\n")

	// Created
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("Created:"))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(volume.Created.Format("2006-01-02 15:04:05")))
	b.WriteString("\n")

	// Used by count
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("Used by:"))
	b.WriteString(" ")
	usedByCount := len(volume.UsedBy)
	usedByStr := fmt.Sprintf("%d container(s)", usedByCount)
	b.WriteString(valueStyle.Render(usedByStr))
	b.WriteString("\n")

	// Action buttons
	b.WriteString(indent)
	b.WriteString(v.renderActionButtons())
	b.WriteString("\n")

	return b.String()
}

// renderActionButtons renders the action buttons for the expanded volume
func (v *VolumeList) renderActionButtons() string {
	actions := v.getActions()
	if len(actions) == 0 {
		return ""
	}

	var parts []string
	for i, action := range actions {
		var style lipgloss.Style
		if v.actionsFocused && i == v.actionIndex {
			style = theme.ActionButtonActiveStyle
		} else {
			style = theme.ActionButtonStyle
		}
		parts = append(parts, style.Render(action.Name))
	}

	return strings.Join(parts, " ")
}
