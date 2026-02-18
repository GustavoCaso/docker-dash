package components

import (
	"context"
	"fmt"
	"strings"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// volumesLoadedMsg is sent when volumes have been loaded asynchronously
type volumesLoadedMsg struct {
	error error
	items []list.Item
}

// volumeTreeLoadedMsg is sent when the volume file tree is loaded
type volumeTreeLoadedMsg struct {
	error    error
	fileTree service.VolumeFileTree
}

// volumeRemovedMsg is sent when a volume deletion completes
type volumeRemovedMsg struct {
	Name  string
	Idx   int
	Error error
}

// volumeItem implements list.Item interface
type volumeItem struct {
	volume service.Volume
}

func (v volumeItem) Title() string { return v.volume.Name }
func (v volumeItem) Description() string {
	var parts []string
	parts = append(parts, v.volume.Driver)
	parts = append(parts, formatSize(v.volume.Size))
	if v.volume.UsedCount > 0 {
		inUse := theme.StatusRunningStyle.Render("● in use")
		parts = append(parts, inUse)
	} else {
		parts = append(parts, "unused")
	}
	return strings.Join(parts, " ")
}
func (v volumeItem) FilterValue() string { return v.volume.Name }

// VolumeList wraps bubbles/list for displaying volumes
type VolumeList struct {
	list          list.Model
	viewport      viewport.Model
	volumeService service.VolumeService
	width, height int
	showFileTree  bool
	loading       bool
	spinner       spinner.Model
}

// NewVolumeList creates a new volume list
func NewVolumeList(volumes []service.Volume, svc service.VolumeService) *VolumeList {
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

	return &VolumeList{
		list:          l,
		viewport:      vp,
		volumeService: svc,
		spinner:       sp,
	}
}

// SetSize sets dimensions
func (v *VolumeList) SetSize(width, height int) {
	v.width = width
	v.height = height

	listX, listY := listStyle.GetFrameSize()

	if v.showFileTree {
		listWidth := int(float64(width) * 0.4)
		detailWidth := width - listWidth

		v.list.SetSize(listWidth-listX, height-listY)
		v.viewport.Width = detailWidth - listX
		v.viewport.Height = height - listY
	} else {
		v.list.SetSize(width-listX, height-listY)
	}
}

// Update handles messages
func (v *VolumeList) Update(msg tea.Msg) tea.Cmd {
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
		switch {
		case key.Matches(msg, keys.Keys.FileTree):
			selected := v.list.SelectedItem()
			if selected == nil {
				return nil
			}
			vol := selected.(volumeItem).volume
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

// View renders the list
func (v *VolumeList) View() string {
	listContent := v.list.View()

	if v.loading {
		spinnerText := v.spinner.View() + " Loading..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, v.list.Width())
	}

	listView := listStyle.
		Width(v.list.Width()).
		Render(listContent)

	if !v.showFileTree {
		return listView
	}

	detailView := listStyle.
		Width(v.viewport.Width).
		Render(v.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

func (v *VolumeList) fetchFileTreeCmd(volumeName string) tea.Cmd {
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

func (v *VolumeList) updateVolumesCmd() tea.Cmd {
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

func (v *VolumeList) deleteVolumeCmd() tea.Cmd {
	svc := v.volumeService
	items := v.list.Items()
	idx := v.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	item := items[idx]
	volumeItem, ok := item.(volumeItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		err := svc.Remove(ctx, volumeItem.volume.Name, true)
		return volumeRemovedMsg{Name: volumeItem.volume.Name, Idx: idx, Error: err}
	}
}
