package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// ImageAction represents an image action button
type ImageAction struct {
	Name string
	Key  string
}

// ImageList represents a list of Docker images with inline expansion
type ImageList struct {
	images         []service.Image
	selectedIndex  int
	expandedIndex  int // -1 means no image is expanded
	focused        bool
	width          int
	height         int
	actionIndex    int
	actionsFocused bool
}

// NewImageList creates a new image list with the given images
func NewImageList(images []service.Image) *ImageList {
	return &ImageList{
		images:         images,
		selectedIndex:  0,
		expandedIndex:  -1,
		focused:        false,
		width:          80,
		height:         20,
		actionIndex:    0,
		actionsFocused: false,
	}
}

// SetImages updates the list of images
func (i *ImageList) SetImages(images []service.Image) {
	i.images = images
	// Reset selection if out of bounds
	if i.selectedIndex >= len(images) {
		i.selectedIndex = 0
	}
	// Reset expanded state if out of bounds
	if i.expandedIndex >= len(images) {
		i.expandedIndex = -1
	}
}

// SetSize sets the width and height of the image list
func (i *ImageList) SetSize(width, height int) {
	i.width = width
	i.height = height
}

// SetFocused sets the focused state of the image list
func (i *ImageList) SetFocused(focused bool) {
	i.focused = focused
}

// IsFocused returns whether the image list is focused
func (i *ImageList) IsFocused() bool {
	return i.focused
}

// SelectedIndex returns the currently selected image index
func (i *ImageList) SelectedIndex() int {
	return i.selectedIndex
}

// SelectedImage returns the currently selected image, or nil if none
func (i *ImageList) SelectedImage() *service.Image {
	if len(i.images) == 0 || i.selectedIndex < 0 || i.selectedIndex >= len(i.images) {
		return nil
	}
	return &i.images[i.selectedIndex]
}

// IsExpanded returns whether the image at the given index is expanded
func (i *ImageList) IsExpanded(index int) bool {
	return i.expandedIndex == index
}

// MoveUp moves the selection up, wrapping to the bottom if at the top
func (i *ImageList) MoveUp() {
	if len(i.images) == 0 {
		return
	}
	i.selectedIndex--
	if i.selectedIndex < 0 {
		i.selectedIndex = len(i.images) - 1
	}
	i.actionIndex = 0
	i.actionsFocused = false
}

// MoveDown moves the selection down, wrapping to the top if at the bottom
func (i *ImageList) MoveDown() {
	if len(i.images) == 0 {
		return
	}
	i.selectedIndex++
	if i.selectedIndex >= len(i.images) {
		i.selectedIndex = 0
	}
	i.actionIndex = 0
	i.actionsFocused = false
}

// ToggleExpand toggles the expansion of the currently selected image
func (i *ImageList) ToggleExpand() {
	if len(i.images) == 0 {
		return
	}
	if i.expandedIndex == i.selectedIndex {
		// Collapse if already expanded
		i.expandedIndex = -1
		i.actionsFocused = false
		i.actionIndex = 0
	} else {
		// Expand the selected image
		i.expandedIndex = i.selectedIndex
		i.actionIndex = 0
	}
}

// SetActionsFocused sets whether the action buttons are focused
func (i *ImageList) SetActionsFocused(focused bool) {
	i.actionsFocused = focused
}

// ActionsFocused returns whether the action buttons are focused
func (i *ImageList) ActionsFocused() bool {
	return i.actionsFocused
}

// SelectedAction returns the currently selected action index
func (i *ImageList) SelectedAction() int {
	return i.actionIndex
}

// MoveActionLeft moves the action selection left
func (i *ImageList) MoveActionLeft() {
	if i.actionIndex > 0 {
		i.actionIndex--
	}
}

// MoveActionRight moves the action selection right
func (i *ImageList) MoveActionRight() {
	actions := i.getActions()
	if i.actionIndex < len(actions)-1 {
		i.actionIndex++
	}
}

// getActions returns the available actions for images
func (i *ImageList) getActions() []ImageAction {
	return []ImageAction{
		{Name: "Inspect", Key: "i"},
		{Name: "Remove", Key: "d"},
	}
}

// formatSize formats bytes into human-readable size (e.g., 1.2 GB, 500 MB)
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// View renders the image list as a string
func (i *ImageList) View() string {
	if len(i.images) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(theme.TextMuted).
			Italic(true)
		return emptyStyle.Render("No images found")
	}

	var b strings.Builder

	for idx, image := range i.images {
		isSelected := idx == i.selectedIndex
		isExpanded := idx == i.expandedIndex

		// Render the image row
		b.WriteString(i.renderImageRow(image, isSelected, isExpanded))
		b.WriteString("\n")

		// Render expanded details if this image is expanded
		if isExpanded {
			b.WriteString(i.renderExpandedDetails(image))
		}
	}

	return b.String()
}

// renderImageRow renders a single image row
func (i *ImageList) renderImageRow(image service.Image, isSelected, isExpanded bool) string {
	// Determine the expansion icon
	expandIcon := theme.IconCollapsed
	if isExpanded {
		expandIcon = theme.IconExpanded
	}

	// Image icon
	imageIcon := theme.IconImage

	// Format dangling indicator
	danglingStr := ""
	if image.Dangling {
		danglingStyle := lipgloss.NewStyle().Foreground(theme.StatusPaused)
		danglingStr = " " + danglingStyle.Render("[dangling]")
	}

	// Build the row content
	// Row: icon, repo, tag, size (formatted), dangling indicator
	rowContent := fmt.Sprintf("%s %s %s:%s  %s%s",
		expandIcon,
		imageIcon,
		image.Repo,
		image.Tag,
		formatSize(image.Size),
		danglingStr,
	)

	// Apply styling based on selection
	var style lipgloss.Style
	if isSelected {
		style = theme.ListItemSelectedStyle
	} else {
		style = theme.ListItemStyle
	}

	return style.Width(i.width - 4).Render(rowContent)
}

// renderExpandedDetails renders the expanded view of an image
func (i *ImageList) renderExpandedDetails(image service.Image) string {
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
	b.WriteString(valueStyle.Render(image.ID))
	b.WriteString("\n")

	// Created
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("Created:"))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(image.Created.Format("2006-01-02 15:04:05")))
	b.WriteString("\n")

	// Size
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("Size:"))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(formatSize(image.Size)))
	b.WriteString("\n")

	// Used by count
	b.WriteString(indent)
	b.WriteString(labelStyle.Render("Used by:"))
	b.WriteString(" ")
	usedByCount := len(image.UsedBy)
	usedByStr := fmt.Sprintf("%d container(s)", usedByCount)
	b.WriteString(valueStyle.Render(usedByStr))
	b.WriteString("\n")

	// Action buttons
	b.WriteString(indent)
	b.WriteString(i.renderActionButtons())
	b.WriteString("\n")

	return b.String()
}

// renderActionButtons renders the action buttons for the expanded image
func (i *ImageList) renderActionButtons() string {
	actions := i.getActions()
	if len(actions) == 0 {
		return ""
	}

	var parts []string
	for idx, action := range actions {
		var style lipgloss.Style
		if i.actionsFocused && idx == i.actionIndex {
			style = theme.ActionButtonActiveStyle
		} else {
			style = theme.ActionButtonStyle
		}
		parts = append(parts, style.Render(action.Name))
	}

	return strings.Join(parts, " ")
}
