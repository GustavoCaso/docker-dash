package components

import (
	"fmt"
	"strings"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
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
	listStyle          = lipgloss.NewStyle()
	listFocusedStyle   = listStyle.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("205"))
	listUnfocusedStyle = listStyle.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
)

// ImageItem implements list.Item interface
type ImageItem struct {
	image service.Image
}

func (i ImageItem) Title() string { return fmt.Sprintf("%s:%s", i.image.Repo, i.image.Tag) }
func (i ImageItem) Description() string {
	return formatSize(i.image.Size) + formatContainerUse(i.image)
}
func (i ImageItem) FilterValue() string { return i.image.Repo + ":" + i.image.Tag }

// ImageList wraps bubbles/list
type ImageList struct {
	list          list.Model
	viewport      viewport.Model
	width, height int
	lastSelected  int
	focused       focusedPane
	showLayers    bool
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

	// Account for padding and borders
	listX, listY := listFocusedStyle.GetFrameSize()

	if i.showLayers {
		// Split view: 40% list, 60% details
		listWidth := int(float64(width) * 0.4)
		detailWidth := width - listWidth

		i.list.SetSize(listWidth-listX, height-listY)
		i.viewport.Width = detailWidth - listX
		i.viewport.Height = height - listY
	} else {
		// Full width list when viewport is hidden
		i.list.SetSize(width-listX, height-listY)
	}
}

// Update handles messages
func (i *ImageList) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Handle focus switching and actions
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
		case "l":
			if i.focused == focusList {
				i.showLayers = !i.showLayers
				i.SetSize(i.width, i.height) // Recalculate layout
				i.updateDetails()
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
	// Choose styles based on focus
	var currentListStyle, currentDetailStyle lipgloss.Style
	if i.focused == focusList {
		currentListStyle = listFocusedStyle
		currentDetailStyle = listUnfocusedStyle
	} else {
		currentListStyle = listUnfocusedStyle
		currentDetailStyle = listFocusedStyle
	}

	// Build help bar
	helpBar := theme.HelpStyle.Render("l: layers")

	// Combine list and help bar vertically
	listWithHelp := lipgloss.JoinVertical(lipgloss.Left,
		i.list.View(),
		helpBar,
	)

	listView := currentListStyle.
		Width(i.list.Width()).
		Render(listWithHelp)

	// Only show viewport when layers are toggled on
	if !i.showLayers {
		return listView
	}

	detailView := currentDetailStyle.
		Width(i.viewport.Width).
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
	fmt.Fprintf(&content, "Layers for %s:%s\n", img.Repo, img.Tag)
	content.WriteString("═══════════════════════\n\n")

	if len(img.Layers) == 0 {
		content.WriteString("No layer information available\n")
	} else {
		for idx, layer := range img.Layers {
			cmd := truncateCommand(layer.Command, 50)
			fmt.Fprintf(&content, "%2d. %s\n", idx+1, cmd)
			fmt.Fprintf(&content, "    Size: %-10s  ID: %s\n\n",
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

func formatContainerUse(img service.Image) string {
	if img.Containers > 0 {
		return fmt.Sprintf(" used by %d containers", img.Containers)
	}

	return " unused"
}
