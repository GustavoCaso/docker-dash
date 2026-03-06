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
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
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
	parts = append(parts, helper.FormatSize(v.volume.Size))
	if v.volume.UsedCount > 0 {
		inUse := theme.StatusRunningStyle.Render("● in use")
		parts = append(parts, inUse)
	} else {
		parts = append(parts, "unused")
	}
	return strings.Join(parts, " ")
}
func (v volumeItem) FilterValue() string { return v.volume.Name }

// Section wraps bubbles/list for displaying volumes.
type Section struct {
	ctx           context.Context
	list          list.Model
	isFilter      bool
	viewport      viewport.Model
	volumeService client.VolumeService
	fileTreePanel panel.Panel
	activePanel   panel.Panel
	width, height int
	loading       bool
	spinner       spinner.Model
}

// New creates a new volume list.
func New(ctx context.Context, volumes []client.Volume, svc client.VolumeService) *Section {
	items := make([]list.Item, len(volumes))
	for i, v := range volumes {
		items[i] = volumeItem{volume: v}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &Section{
		ctx:           ctx,
		list:          l,
		viewport:      viewport.New(0, 0),
		volumeService: svc,
		fileTreePanel: newFileTreePanel(ctx, svc),
		spinner:       sp,
	}
}

// SetSize sets dimensions.
func (s *Section) SetSize(width, height int) {
	s.width = width
	s.height = height

	listX, listY := theme.ListStyle.GetFrameSize()

	if s.activePanel != nil {
		listWidth := int(float64(width) * listSplitRatio)
		detailWidth := width - listWidth

		s.list.SetSize(listWidth-listX, height-listY)
		s.viewport.Width = detailWidth - listX
		s.viewport.Height = height - listY
		s.activePanel.SetSize(s.viewport.Width, s.viewport.Height)
	} else {
		s.list.SetSize(width-listX, height-listY)
	}
}

// Update handles messages.
func (s *Section) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	if s.loading {
		var spinnerCmd tea.Cmd
		s.spinner, spinnerCmd = s.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case volumesLoadedMsg:
		s.loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error loading volumes: %s", msg.error.Error()),
					IsError: true,
				}
			}
		}
		cmd := s.list.SetItems(msg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	case volumeTreeLoadedMsg:
		s.loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: msg.error.Error(),
					IsError: true,
				}
			}
		}
		if s.activePanel != nil {
			return s.activePanel.Update(msg)
		}
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
		s.list.RemoveItem(msg.Idx)
		return func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Volume %s deleted", msg.Name),
				IsError: false,
			}
		}
	case tea.KeyMsg:
		if s.isFilter {
			var filterCmds []tea.Cmd
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			filterCmds = append(filterCmds, listCmd)

			if key.Matches(msg, keys.Keys.Esc) {
				s.isFilter = !s.isFilter
				filterCmds = append(filterCmds, func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} })
			}
			return tea.Batch(filterCmds...)
		}

		switch {
		case key.Matches(msg, keys.Keys.FileTree):
			if s.activePanel == s.fileTreePanel {
				cmd := s.activePanel.Close()
				s.activePanel = nil
				return cmd
			}

			selected := s.list.SelectedItem()
			if selected == nil {
				return nil
			}
			vItem, ok := selected.(volumeItem)
			if !ok {
				return nil
			}

			s.loading = true
			s.activePanel = s.fileTreePanel
			return tea.Batch(s.spinner.Tick, s.fileTreePanel.Init(vItem.volume.Name))
		case key.Matches(msg, keys.Keys.Refresh):
			s.loading = true
			return tea.Batch(s.spinner.Tick, s.updateVolumesCmd())
		case key.Matches(msg, keys.Keys.Delete):
			return s.confirmVolumeDelete()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			return tea.Batch(listCmd, s.clearDetails())
		case key.Matches(msg, keys.Keys.Filter):
			s.isFilter = !s.isFilter
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)

			if s.isFilter {
				return tea.Batch(listCmd, s.extendFilterHelpCommand())
			}
			return listCmd
		}
	}

	if s.activePanel != nil {
		cmds = append(cmds, s.activePanel.Update(msg))
	}

	return tea.Batch(cmds...)
}

// View renders the list.
func (s *Section) View() string {
	s.SetSize(s.width, s.height)
	listContent := s.list.View()

	if s.loading {
		spinnerText := s.spinner.View() + " Loading..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, s.list.Width())
	}

	listView := theme.ListStyle.
		Width(s.list.Width()).
		Render(listContent)

	if s.activePanel == nil {
		return listView
	}

	detailContent := s.activePanel.View()
	detailView := theme.ListStyle.
		Width(s.viewport.Width).
		Height(s.viewport.Height).
		Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

// Reset reset internal state to when a component is first initialized.
func (s *Section) Reset() tea.Cmd {
	s.isFilter = false
	s.viewport.SetContent("")
	s.SetSize(s.width, s.height)
	var cmd tea.Cmd
	if s.activePanel != nil {
		cmd = s.activePanel.Close()
		s.activePanel = nil
	}
	return cmd
}

func (s *Section) clearDetails() tea.Cmd {
	var cmd tea.Cmd
	if s.activePanel != nil {
		cmd = s.activePanel.Close()
		s.activePanel = nil
	}
	s.SetSize(s.width, s.height)
	s.viewport.SetContent("")
	return cmd
}

func (s *Section) updateVolumesCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.volumeService
	return func() tea.Msg {
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

func (s *Section) deleteVolumeCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.volumeService
	items := s.list.Items()
	idx := s.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	item := items[idx]
	vi, ok := item.(volumeItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := svc.Remove(ctx, vi.volume.Name, true)
		return volumeRemovedMsg{Name: vi.volume.Name, Idx: idx, Error: err}
	}
}

func (s *Section) extendFilterHelpCommand() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "exit"),
			),
		}}
	}
}

func (s *Section) confirmVolumeDelete() tea.Cmd {
	items := s.list.Items()
	idx := s.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}
	vi, ok := items[idx].(volumeItem)
	if !ok {
		return nil
	}
	deleteCmd := s.deleteVolumeCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Delete Volume",
			Body:      fmt.Sprintf("Delete volume %s?", vi.volume.Name),
			OnConfirm: deleteCmd,
		}
	}
}
