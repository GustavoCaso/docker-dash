package volumes

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
	"github.com/GustavoCaso/docker-dash/internal/ui/components/helpers"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

const listSplitRatio = 0.4

// volumesLoadedMsg is sent when volumes have been loaded asynchronously.
type volumesLoadedMsg struct {
	error error
	items []list.Item
}

// volumeTreeLoadedMsg is sent when the volume file tree is loaded.
type volumeTreeLoadedMsg struct {
	error    error
	fileTree client.VolumeFileTree
}

// volumeRemovedMsg is sent when a volume deletion completes.
type volumeRemovedMsg struct {
	Name  string
	Idx   int
	Error error
}

// volumeItem implements list.Item interface.
type volumeItem struct {
	volume client.Volume
}

func (v volumeItem) Title() string { return v.volume.Name }
func (v volumeItem) Description() string {
	var parts []string
	parts = append(parts, v.volume.Driver)
	parts = append(parts, helpers.FormatSize(v.volume.Size))
	if v.volume.UsedCount > 0 {
		inUse := theme.StatusRunningStyle.Render("● in use")
		parts = append(parts, inUse)
	} else {
		parts = append(parts, "unused")
	}
	return strings.Join(parts, " ")
}
func (v volumeItem) FilterValue() string { return v.volume.Name }

// List wraps bubbles/list for displaying volumes.
type List struct {
	list          list.Model
	isFilter      bool
	viewport      viewport.Model
	volumeService client.VolumeService
	width, height int
	showFileTree  bool
	loading       bool
	spinner       spinner.Model
}

// New creates a new volume list.
func New(volumes []client.Volume, svc client.VolumeService) *List {
	items := make([]list.Item, len(volumes))
	for i, v := range volumes {
		items[i] = volumeItem{volume: v}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)

	vp := viewport.New(0, 0)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &List{
		list:          l,
		viewport:      vp,
		volumeService: svc,
		spinner:       sp,
	}
}

// SetSize sets dimensions.
func (v *List) SetSize(width, height int) {
	v.width = width
	v.height = height

	listX, listY := theme.ListStyle.GetFrameSize()

	if v.showFileTree {
		listWidth := int(float64(width) * listSplitRatio)
		detailWidth := width - listWidth

		v.list.SetSize(listWidth-listX, height-listY)
		v.viewport.Width = detailWidth - listX
		v.viewport.Height = height - listY
	} else {
		v.list.SetSize(width-listX, height-listY)
	}
}

// Update handles messages.
func (v *List) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	if v.loading {
		var spinnerCmd tea.Cmd
		v.spinner, spinnerCmd = v.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case volumesLoadedMsg:
		v.loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error loading volumes: %s", msg.error.Error()),
					IsError: true,
				}
			}
		}
		cmd := v.list.SetItems(msg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	case volumeTreeLoadedMsg:
		v.loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: msg.error.Error(),
					IsError: true,
				}
			}
		}
		v.viewport.SetContent(lipgloss.NewStyle().Width(v.viewport.Width).Render(msg.fileTree.Tree.String()))
		return nil
	case volumeRemovedMsg:
		if msg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error deleting volume: %s", msg.Error.Error()),
					IsError: true,
				}
			}
		}
		v.list.RemoveItem(msg.Idx)
		return func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Volume %s deleted", msg.Name),
				IsError: false,
			}
		}
	case tea.KeyMsg:
		if v.isFilter {
			var filterCmds []tea.Cmd
			var listCmd tea.Cmd
			v.list, listCmd = v.list.Update(msg)
			filterCmds = append(filterCmds, listCmd)

			if key.Matches(msg, keys.Keys.Esc) {
				v.isFilter = !v.isFilter
				filterCmds = append(filterCmds, func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} })
			}
			return tea.Batch(filterCmds...)
		}

		switch {
		case key.Matches(msg, keys.Keys.FileTree):
			selected := v.list.SelectedItem()
			if selected == nil {
				return nil
			}
			vItem, ok := selected.(volumeItem)
			if !ok {
				return nil
			}
			vol := vItem.volume
			v.showFileTree = !v.showFileTree
			if v.showFileTree {
				v.loading = true
				v.SetSize(v.width, v.height)
				v.viewport.SetContent("")
				return tea.Batch(v.spinner.Tick, v.fetchFileTreeCmd(vol.Name))
			}
			v.SetSize(v.width, v.height)
			return nil
		case key.Matches(msg, keys.Keys.Refresh):
			v.loading = true
			return tea.Batch(v.spinner.Tick, v.updateVolumesCmd())
		case key.Matches(msg, keys.Keys.Delete):
			return v.deleteVolumeCmd()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			var listCmd tea.Cmd
			v.list, listCmd = v.list.Update(msg)
			return listCmd
		case key.Matches(msg, keys.Keys.ScrollUp, keys.Keys.ScrollDown):
			var vpCmd tea.Cmd
			v.viewport, vpCmd = v.viewport.Update(msg)
			return vpCmd
		case key.Matches(msg, keys.Keys.Filter):
			v.isFilter = !v.isFilter
			var listCmd tea.Cmd
			v.list, listCmd = v.list.Update(msg)
			return tea.Batch(listCmd, v.extendFilterHelpCommand())
		}
	}

	var listCmd tea.Cmd
	v.list, listCmd = v.list.Update(msg)
	cmds = append(cmds, listCmd)

	var vpCmd tea.Cmd
	v.viewport, vpCmd = v.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return tea.Batch(cmds...)
}

// View renders the list.
func (v *List) View() string {
	listContent := v.list.View()

	if v.loading {
		spinnerText := v.spinner.View() + " Loading..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, v.list.Width())
	}

	listView := theme.ListStyle.
		Width(v.list.Width()).
		Render(listContent)

	if !v.showFileTree {
		return listView
	}

	detailView := theme.ListStyle.
		Width(v.viewport.Width).
		Render(v.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

func (v *List) fetchFileTreeCmd(volumeName string) tea.Cmd {
	svc := v.volumeService
	return func() tea.Msg {
		ctx := context.Background()
		fileTree, err := svc.FileTree(ctx, volumeName)
		if err != nil {
			return volumeTreeLoadedMsg{error: fmt.Errorf("error getting volume file tree: %s", err.Error())}
		}
		return volumeTreeLoadedMsg{fileTree: fileTree}
	}
}

func (v *List) updateVolumesCmd() tea.Cmd {
	svc := v.volumeService
	return func() tea.Msg {
		ctx := context.Background()
		volumes, err := svc.List(ctx)
		if err != nil {
			return volumesLoadedMsg{error: err}
		}
		items := make([]list.Item, len(volumes))
		for idx, vol := range volumes {
			items[idx] = volumeItem{volume: vol}
		}
		return volumesLoadedMsg{items: items}
	}
}

func (v *List) deleteVolumeCmd() tea.Cmd {
	svc := v.volumeService
	items := v.list.Items()
	idx := v.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	item := items[idx]
	vi, ok := item.(volumeItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		err := svc.Remove(ctx, vi.volume.Name, true)
		return volumeRemovedMsg{Name: vi.volume.Name, Idx: idx, Error: err}
	}
}

func (v *List) extendFilterHelpCommand() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "exit"),
			),
		}}
	}
}
