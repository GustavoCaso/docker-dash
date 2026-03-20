package volumes

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// volumesLoadedMsg is sent when volumes have been loaded asynchronously.
type volumesLoadedMsg struct {
	error error
	items []list.Item
}

// volumesPrunedMsg is sent when a volume prune completes.
type volumesPrunedMsg struct {
	report client.PruneReport
	err    error
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
	volumeService client.VolumeService
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
		volumeService: svc,
		spinner:       sp,
	}
}

func (s *Section) Init() tea.Cmd {
	return nil
}

// SetSize sets dimensions.
func (s *Section) SetSize(width, height int) {
	s.width = width
	s.height = height

	// Account for padding and borders
	listX, listY := theme.ListStyle.GetFrameSize()

	s.list.SetSize(width-listX, height-listY)
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
	case volumesPrunedMsg:
		if msg.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: "Error pruning volumes: " + msg.err.Error(),
					IsError: true,
				}
			}
		}
		summary := fmt.Sprintf(
			"Pruned %d volumes, reclaimed %s",
			msg.report.ItemsDeleted,
			helper.FormatSize(msg.report.SpaceReclaimed),
		)
		return tea.Batch(s.updateVolumesCmd(), func() tea.Msg {
			return message.ShowBannerMsg{Message: summary, IsError: false}
		})
	case volumeRemovedMsg:
		if msg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error deleting volume: %s", msg.Error.Error()),
					IsError: true,
				}
			}
		}
		s.removeItem(msg.Idx)
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
		case key.Matches(msg, keys.Keys.Refresh):
			s.loading = true
			return tea.Batch(s.spinner.Tick, s.updateVolumesCmd())
		case key.Matches(msg, keys.Keys.Prune):
			return s.confirmVolumePrune()
		case key.Matches(msg, keys.Keys.Delete):
			return s.confirmVolumeDelete()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			return listCmd
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

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		listView,
	)
}

// Reset resets internal state to when a component is first initialized.
func (s *Section) Reset() tea.Cmd {
	s.isFilter = false
	s.SetSize(s.width, s.height)
	return nil
}

func (s *Section) removeItem(idx int) {
	s.list.RemoveItem(idx)
	if len(s.list.Items()) > 0 {
		s.list.Select(min(idx, len(s.list.Items())-1))
	}
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

func (s *Section) pruneVolumesCmd() tea.Cmd {
	ctx, svc := s.ctx, s.volumeService
	return func() tea.Msg {
		report, err := svc.Prune(ctx, client.PruneOptions{All: true})
		return volumesPrunedMsg{report: report, err: err}
	}
}

func (s *Section) confirmVolumePrune() tea.Cmd {
	pruneCmd := s.pruneVolumesCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Prune Volumes",
			Body:      "Remove all unused volumes (including named)?",
			OnConfirm: pruneCmd,
		}
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
