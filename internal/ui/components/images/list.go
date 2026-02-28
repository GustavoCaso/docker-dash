package images

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

const listSplitRatio = 0.4

// imagesLoadedMsg is sent when images have been loaded asynchronously.
type imagesLoadedMsg struct {
	error error
	items []list.Item
}

// containerRunMsg is sent when a container is created.
type containerRunMsg struct {
	containerID string
	error       error
}

// imageRemovedMsg is sent when an image deletion completes.
type imageRemovedMsg struct {
	ID    string
	Idx   int
	Error error
}

// imageItem implements list.Item interface.
type imageItem struct {
	image client.Image
}

func (i imageItem) ID() string    { return i.image.ID }
func (i imageItem) Title() string { return fmt.Sprintf("%s:%s", i.image.Repo, i.image.Tag) }
func (i imageItem) Description() string {
	stateIcon := theme.GetImageStatusIcon(i.image.Containers)
	stateStyle := theme.GetImageStatusStyle(i.image.Containers)
	state := stateStyle.Render(stateIcon)
	return state + " " + formatSize(i.image.Size)
}
func (i imageItem) FilterValue() string { return i.image.Repo + ":" + i.image.Tag }

// List wraps bubbles/list.
type List struct {
	list             list.Model
	viewport         viewport.Model
	imageService     client.ImageService
	containerService client.ContainerService
	width, height    int
	showLayers       bool
	loading          bool
	isFilter         bool
	spinner          spinner.Model
}

// New creates a new image list.
func New(images []client.Image, client client.Client) *List {
	items := make([]list.Item, len(images))
	for i, img := range images {
		items[i] = imageItem{image: img}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)

	vp := viewport.New(0, 0)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	il := &List{
		list:             l,
		viewport:         vp,
		imageService:     client.Images(),
		containerService: client.Containers(),
		spinner:          sp,
	}
	il.updateDetails()

	return il
}

// SetSize sets dimensions.
func (i *List) SetSize(width, height int) {
	i.width = width
	i.height = height

	// Account for padding and borders
	listX, listY := theme.ListStyle.GetFrameSize()

	if i.showLayers {
		// Split view: 40% list, 60% details
		listWidth := int(float64(width) * listSplitRatio)
		detailWidth := width - listWidth

		i.list.SetSize(listWidth-listX, height-listY)
		i.viewport.Width = detailWidth - listX
		i.viewport.Height = height - listY
	} else {
		// Full width list when viewport is hidden
		i.list.SetSize(width-listX, height-listY)
	}
}

// Update handles messages.
func (i *List) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Handle spinner ticks while loading
	if i.loading {
		var spinnerCmd tea.Cmd
		i.spinner, spinnerCmd = i.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case imagesLoadedMsg:
		i.loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error loading images: %s", msg.error.Error()),
					IsError: true,
				}
			}
		}
		cmd := i.list.SetItems(msg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	case imageRemovedMsg:
		if msg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error deleting image: %s", msg.Error.Error()),
					IsError: true,
				}
			}
		}
		i.list.RemoveItem(msg.Idx)
		return func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Image %s deleted", shortID(msg.ID)),
				IsError: false,
			}
		}
	case containerRunMsg:
		i.loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: msg.error.Error(),
					IsError: true,
				}
			}
		}

		banner := func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Container %s created", shortID(msg.containerID)),
				IsError: false,
			}
		}

		refreshComponents := func() tea.Msg {
			return message.BubbleUpMsg{
				KeyMsg: tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune{'r'},
				},
				OnlyActive: false,
			}
		}

		return tea.Batch(banner, refreshComponents)

	case tea.KeyMsg:
		if i.isFilter {
			var filterCmds []tea.Cmd
			var listCmd tea.Cmd
			i.list, listCmd = i.list.Update(msg)
			filterCmds = append(filterCmds, listCmd)

			if key.Matches(msg, keys.Keys.Esc) {
				i.isFilter = !i.isFilter
				filterCmds = append(filterCmds, func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} })
			}
			return tea.Batch(filterCmds...)
		}

		switch {
		case key.Matches(msg, keys.Keys.ImageLayers):
			i.showLayers = !i.showLayers
			i.SetSize(i.width, i.height) // Recalculate layout
			return nil
		case key.Matches(msg, keys.Keys.Refresh):
			i.loading = true
			return tea.Batch(i.spinner.Tick, i.updateImagesCmd())
		case key.Matches(msg, keys.Keys.Delete):
			return i.deleteImageCmd()
		case key.Matches(msg, keys.Keys.CreateAndRunContainer):
			return i.createContainerCmdAndRun()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			var listCmd tea.Cmd
			i.list, listCmd = i.list.Update(msg)
			return listCmd
		case key.Matches(msg, keys.Keys.ScrollUp, keys.Keys.ScrollDown):
			var vpCmd tea.Cmd
			i.viewport, vpCmd = i.viewport.Update(msg)
			return vpCmd
		case key.Matches(msg, keys.Keys.Filter):
			i.isFilter = !i.isFilter
			var listCmd tea.Cmd
			i.list, listCmd = i.list.Update(msg)
			return tea.Batch(listCmd, i.extendFilterHelpCommand())
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

// View renders the list.
func (i *List) View() string {
	listContent := i.list.View()

	// Overlay spinner in bottom right corner when loading
	if i.loading {
		spinnerText := i.spinner.View() + " Refreshing..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, i.list.Width())
	}

	listView := theme.ListStyle.
		Width(i.list.Width()).
		Render(listContent)

	// Only show list when layers is not enabled
	if !i.showLayers {
		return listView
	}

	i.updateDetails()

	detailView := theme.ListStyle.
		Width(i.viewport.Width).
		Render(i.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

func (i *List) extendFilterHelpCommand() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "exit"),
			),
		}}
	}
}

func (i *List) deleteImageCmd() tea.Cmd {
	svc := i.imageService
	items := i.list.Items()
	idx := i.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	dockerImage, ok := items[idx].(imageItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		err := svc.Remove(ctx, dockerImage.ID(), true)

		return imageRemovedMsg{ID: dockerImage.ID(), Idx: idx, Error: err}
	}
}

func (i *List) updateImagesCmd() tea.Cmd {
	svc := i.imageService
	return func() tea.Msg {
		ctx := context.Background()
		images, err := svc.List(ctx)
		if err != nil {
			return imagesLoadedMsg{error: err}
		}
		items := make([]list.Item, len(images))
		for idx, img := range images {
			items[idx] = imageItem{image: img}
		}
		return imagesLoadedMsg{items: items}
	}
}

func (i *List) createContainerCmdAndRun() tea.Cmd {
	svc := i.containerService
	items := i.list.Items()
	idx := i.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	dockerImage, ok := items[idx].(imageItem)
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

// updateDetails updates the viewport content based on selected image.
func (i *List) updateDetails() {
	selected := i.list.SelectedItem()
	if selected == nil {
		i.viewport.SetContent("No image selected")
		return
	}

	item, ok := selected.(imageItem)
	if !ok {
		return
	}
	img := item.image

	var content strings.Builder

	// Header
	fmt.Fprintf(&content, "Layers for %s:%s\n", img.Repo, img.Tag)
	content.WriteString("═══════════════════════\n\n")

	const maxLayerCmdLen = 50 // max chars to show for a layer command
	if len(img.Layers) == 0 {
		content.WriteString("No layer information available\n")
	} else {
		for idx, layer := range img.Layers {
			cmd := truncateCommand(layer.Command, maxLayerCmdLen)
			fmt.Fprintf(&content, "%2d. %s\n", idx+1, cmd)
			fmt.Fprintf(&content, "    Size: %-10s  ID: %s\n\n",
				formatSize(layer.Size),
				shortID(layer.ID))
		}
	}

	i.viewport.SetContent(content.String())
}

const shortIDLength = 12 // length of shortened container/image IDs

// shortID returns first 12 characters of an ID.
func shortID(id string) string {
	// Remove sha256: prefix if present
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > shortIDLength {
		return id[:shortIDLength]
	}
	return id
}

// truncateCommand shortens a command string.
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
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
