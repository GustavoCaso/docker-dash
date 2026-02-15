package components

import (
	"context"
	"fmt"
	"strings"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type focusedPane int

const (
	focusList focusedPane = iota
	focusViewport
)

// imagesLoadedMsg is sent when images have been loaded asynchronously
type imagesLoadedMsg struct {
	items []list.Item
}

// containerRunMsg is sent when a container is created
type containerRunMsg struct {
	containerID string
	error       error
}

// ImageRemovedMsg is sent when an image deletion completes
type ImageRemovedMsg struct {
	ID    string
	Idx   int
	Error error
}

var (
	listStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
)

// Key bindings for image list actions
var layersKey = key.NewBinding(
	key.WithKeys("l"),
	key.WithHelp("l", "layers"),
)

var refreshKey = key.NewBinding(
	key.WithKeys("r"),
	key.WithHelp("r", "refresh"),
)

var mainNavKey = key.NewBinding(
	key.WithKeys("up", "down"),
	key.WithHelp("up/down", "navigate main panel"),
)

var secondaryNavKey = key.NewBinding(
	key.WithKeys("k", "k"),
	key.WithHelp("j/k", "navigate right panel"),
)

var deleteKey = key.NewBinding(
	key.WithKeys("d"),
	key.WithHelp("d", "delete"),
)

var runKey = key.NewBinding(
	key.WithKeys("R"),
	key.WithHelp("R", "run container"),
)

// KeyBindings returns the key bindings for the current state
func (i *ImageList) KeyBindings() []key.Binding {
	return []key.Binding{mainNavKey, secondaryNavKey, layersKey, refreshKey, deleteKey, runKey}
}

// ImageItem implements list.Item interface
type ImageItem struct {
	image service.Image
}

func (i ImageItem) ID() string    { return i.image.ID }
func (i ImageItem) Title() string { return fmt.Sprintf("%s:%s", i.image.Repo, i.image.Tag) }
func (i ImageItem) Description() string {
	return formatSize(i.image.Size) + formatContainerUse(i.image)
}
func (i ImageItem) FilterValue() string { return i.image.Repo + ":" + i.image.Tag }

// ImageList wraps bubbles/list
type ImageList struct {
	list             list.Model
	viewport         viewport.Model
	imageService     service.ImageService
	containerService service.ContainerService
	width, height    int
	lastSelected     int
	showLayers       bool
	loading          bool
	spinner          spinner.Model
}

// NewImageList creates a new image list
func NewImageList(images []service.Image, client service.DockerClient) *ImageList {
	items := make([]list.Item, len(images))
	for i, img := range images {
		items[i] = ImageItem{image: img}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)

	vp := viewport.New(0, 0)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	il := &ImageList{
		list:             l,
		viewport:         vp,
		lastSelected:     -1,
		imageService:     client.Images(),
		containerService: client.Containers(),
		spinner:          sp,
	}
	il.updateDetails()

	return il
}

// SetSize sets dimensions
func (i *ImageList) SetSize(width, height int) {
	i.width = width
	i.height = height

	// Account for padding and borders
	listX, listY := listStyle.GetFrameSize()

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

	// Handle spinner ticks while loading
	if i.loading {
		var spinnerCmd tea.Cmd
		i.spinner, spinnerCmd = i.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	// Handle images loaded message
	if loadedMsg, ok := msg.(imagesLoadedMsg); ok {
		i.loading = false
		cmd := i.list.SetItems(loadedMsg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	}

	if removeMsg, ok := msg.(ImageRemovedMsg); ok {
		if removeMsg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error deleting image: %s", removeMsg.Error.Error()),
					IsError: true,
				}
			}
		}
		i.list.RemoveItem(removeMsg.Idx)
		return func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Image %s deleted", shortID(removeMsg.ID)),
				IsError: false,
			}
		}
	}

	if containerRunMsg, ok := msg.(containerRunMsg); ok {
		i.loading = false
		if containerRunMsg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: containerRunMsg.error.Error(),
					IsError: true,
				}
			}
		}

		return func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Container %s created", shortID(containerRunMsg.containerID)),
				IsError: false,
			}
		}
	}

	// Handle focus switching and actions
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "l":
			i.showLayers = !i.showLayers
			i.SetSize(i.width, i.height) // Recalculate layout
			return nil
		case "r":
			i.loading = true
			return tea.Batch(i.spinner.Tick, i.updateImagesCmd())
		case "d":
			return i.deleteImageCmd()
		case "R":
			return i.createContainerCmdAndRun()
		case "up", "down":
			var listCmd tea.Cmd
			i.list, listCmd = i.list.Update(msg)
			if i.list.Index() != i.lastSelected {
				i.lastSelected = i.list.Index()
			}
			return listCmd
		case "j", "k":
			var vpCmd tea.Cmd
			i.viewport, vpCmd = i.viewport.Update(msg)
			return vpCmd
		}
	}

	// Send the remaining of msg to both panels
	var listCmd tea.Cmd
	i.list, listCmd = i.list.Update(msg)
	cmds = append(cmds, listCmd)

	var vpCmd tea.Cmd
	i.viewport, vpCmd = i.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return tea.Batch(cmds...)
}

// View renders the list
func (i *ImageList) View() string {
	listContent := i.list.View()

	// Overlay spinner in bottom right corner when loading
	if i.loading {
		spinnerText := i.spinner.View() + " Refreshing..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, i.list.Width())
	}

	listView := listStyle.
		Width(i.list.Width()).
		Render(listContent)

	// Only show list when layers is not enabled
	if !i.showLayers {
		return listView
	}

	i.updateDetails()

	detailView := listStyle.
		Width(i.viewport.Width).
		Render(i.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

func (i *ImageList) deleteImageCmd() tea.Cmd {
	svc := i.imageService
	items := i.list.Items()
	idx := i.lastSelected
	item := items[idx]
	if item == nil {
		return nil
	}

	dockerImage, ok := item.(ImageItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		err := svc.Remove(ctx, dockerImage.ID(), true)

		return ImageRemovedMsg{ID: dockerImage.ID(), Idx: idx, Error: err}
	}
}

func (i *ImageList) updateImagesCmd() tea.Cmd {
	svc := i.imageService
	return func() tea.Msg {
		ctx := context.Background()
		images, _ := svc.List(ctx)
		items := make([]list.Item, len(images))
		for idx, img := range images {
			items[idx] = ImageItem{image: img}
		}
		return imagesLoadedMsg{items: items}
	}
}

func (i *ImageList) createContainerCmdAndRun() tea.Cmd {
	svc := i.containerService
	items := i.list.Items()
	idx := i.lastSelected
	item := items[idx]
	if item == nil {
		return nil
	}

	dockerImage, ok := item.(ImageItem)
	if !ok {
		return nil
	}
	i.loading = true
	return func() tea.Msg {
		ctx := context.Background()
		containerID, err := svc.Run(ctx, dockerImage.image)

		return containerRunMsg{containerID: containerID, error: err}
	}
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
