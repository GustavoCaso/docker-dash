package components

import (
	"fmt"
	"strings"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type focusedPane int

const (
	focusList focusedPane = iota
	focusViewport
)

var (
	listStyle          = lipgloss.NewStyle().PaddingTop(1).PaddingRight(1)
	listFocusedStyle   = listStyle.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("205"))
	listUnfocusedStyle = listStyle.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))

	detailFocusedStyle   = lipgloss.NewStyle().PaddingTop(1).PaddingLeft(1).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("205"))
	detailUnfocusedStyle = lipgloss.NewStyle().PaddingTop(1).PaddingLeft(1).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
)

// ImageItem implements list.Item interface
type ImageItem struct {
	image service.Image
}

func (i ImageItem) Title() string       { return fmt.Sprintf("%s:%s", i.image.Repo, i.image.Tag) }
func (i ImageItem) Description() string { return formatSize(i.image.Size) }
func (i ImageItem) FilterValue() string { return i.image.Repo + ":" + i.image.Tag }

// ImageList wraps bubbles/list
type ImageList struct {
	list          list.Model
	viewport      viewport.Model
	width, height int
	lastSelected  int
	focused       focusedPane
}

// NewImageList creates a new image list
func NewImageList(images []service.Image) *ImageList {
	items := make([]list.Item, len(images))
	for i, img := range images {
		items[i] = ImageItem{image: img}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false) // We still support filtering with /
	l.SetShowStatusBar(false)

	vp := viewport.New(0, 0)

	il := &ImageList{list: l, viewport: vp, lastSelected: -1}
	il.updateDetails()

	return il
}

// SetSize sets dimensions
func (i *ImageList) SetSize(width, height int) {
	i.width = width
	i.height = height

	// Calculate widths (40% list, 60% details)
	listWidth := int(float64(width) * 0.4)
	detailWidth := width - listWidth

	// Account for padding and borders
	listX, listY := listFocusedStyle.GetFrameSize()
	detailX, detailY := detailFocusedStyle.GetFrameSize()

	i.list.SetSize(listWidth-listX, height-listY)
	i.viewport.Width = detailWidth - detailX
	i.viewport.Height = height - detailY
}

// Update handles messages
func (i *ImageList) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Handle focus switching
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			if i.focused == focusList {
				i.focused = focusViewport
				return nil
			}
		case "esc":
			if i.focused == focusViewport {
				i.focused = focusList
				return nil
			}
		}
	}

	// Only send key messages to the focused pane
	switch i.focused {
	case focusList:
		var listCmd tea.Cmd
		i.list, listCmd = i.list.Update(msg)
		cmds = append(cmds, listCmd)

		// Update details if selection changed
		if i.list.Index() != i.lastSelected {
			i.lastSelected = i.list.Index()
			i.updateDetails()
		}
	case focusViewport:
		var vpCmd tea.Cmd
		i.viewport, vpCmd = i.viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}

	return tea.Batch(cmds...)
}

// View renders the list
func (i *ImageList) View() string {
	// Calculate widths (40% list, 60% details)
	listWidth := int(float64(i.width) * 0.4)
	detailWidth := i.width - listWidth

	// Choose styles based on focus
	var currentListStyle, currentDetailStyle lipgloss.Style
	if i.focused == focusList {
		currentListStyle = listFocusedStyle
		currentDetailStyle = detailUnfocusedStyle
	} else {
		currentListStyle = listUnfocusedStyle
		currentDetailStyle = detailFocusedStyle
	}

	listView := currentListStyle.
		Width(listWidth - currentListStyle.GetHorizontalFrameSize()).
		Height(i.height - currentListStyle.GetVerticalFrameSize()).
		Render(i.list.View())

	detailView := currentDetailStyle.
		Width(detailWidth - currentDetailStyle.GetHorizontalFrameSize()).
		Height(i.height - currentDetailStyle.GetVerticalFrameSize()).
		Render(i.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

// updateDetails updates the viewport content based on selected image
func (i *ImageList) updateDetails() {
	selected := i.list.SelectedItem()
	if selected == nil {
		i.viewport.SetContent("No image selected")
		return
	}

	img := selected.(ImageItem).image

	var content strings.Builder

	// Header
	content.WriteString("Image Details\n")
	content.WriteString("═════════════\n\n")

	fmt.Fprintf(&content, "Name:    %s:%s\n", img.Repo, img.Tag)
	fmt.Fprintf(&content, "ID:      %s\n", shortID(img.ID))
	fmt.Fprintf(&content, "Size:    %s\n", formatSize(img.Size))
	fmt.Fprintf(&content, "Created: %s\n", img.Created.Format("2006-01-02 15:04:05"))

	if img.Dangling {
		content.WriteString("Status:  Dangling (untagged)\n")
	}

	// Layers section
	content.WriteString("\n")
	fmt.Fprintf(&content, "Layers (%d)\n", len(img.Layers))
	content.WriteString("──────────\n")

	if len(img.Layers) == 0 {
		content.WriteString("No layer information available\n")
	} else {
		for idx, layer := range img.Layers {
			cmd := truncateCommand(layer.Command, 50)
			fmt.Fprintf(&content, "\n%2d. %s\n", idx+1, cmd)
			fmt.Fprintf(&content, "    Size: %-10s  ID: %s\n",
				formatSize(layer.Size),
				shortID(layer.ID))
		}
	}

	i.viewport.SetContent(content.String())
	i.viewport.GotoTop()
}

// shortID returns first 12 characters of an ID
func shortID(id string) string {
	// Remove sha256: prefix if present
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// truncateCommand shortens a command string
func truncateCommand(cmd string, maxLen int) string {
	// Clean up common prefixes
	cmd = strings.TrimPrefix(cmd, "/bin/sh -c ")
	cmd = strings.TrimPrefix(cmd, "#(nop) ")
	cmd = strings.TrimSpace(cmd)

	if len(cmd) > maxLen {
		return cmd[:maxLen-3] + "..."
	}
	return cmd
}

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
