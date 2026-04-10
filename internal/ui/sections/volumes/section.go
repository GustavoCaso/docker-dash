package volumes

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/base"
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
	base.Section
	ctx           context.Context
	volumeService client.VolumeService
}

// New creates a new volume list.
func New(ctx context.Context, volumes []client.Volume, svc client.VolumeService) *Section {
	items := make([]list.Item, len(volumes))
	for i, v := range volumes {
		items[i] = volumeItem{volume: v}
	}

	return &Section{
		ctx:           ctx,
		volumeService: svc,
		Section: base.Section{
			List:    base.NewList(items),
			Spinner: base.NewSpinner(),
		},
	}
}

func (s *Section) Init() tea.Cmd {
	return nil
}

// SetSize sets dimensions.
func (s *Section) SetSize(width, height int) {
	s.SetListSize(width, height)
}

// Update handles messages.
func (s *Section) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	if spinnerCmd := s.UpdateSpinner(msg); spinnerCmd != nil {
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case volumesLoadedMsg:
		log.Printf("[volumes] volumesLoadedMsg: count=%d", len(msg.items))
		s.Loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error loading volumes: %s", msg.error.Error()),
					IsError: true,
				}
			}
		}
		cmd := s.List.SetItems(msg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	case volumesPrunedMsg:
		log.Printf("[volumes] volumesPrunedMsg: deleted=%d spaceReclaimed=%d",
			msg.report.ItemsDeleted, msg.report.SpaceReclaimed)
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
		log.Printf("[volumes] volumeRemovedMsg: name=%q err=%v", msg.Name, msg.Error)
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
		log.Printf("[volumes] KeyMsg: key=%q", msg.String())
		if handled, filterCmds := s.HandleFilterKey(msg); handled {
			return tea.Batch(filterCmds...)
		}

		switch {
		case key.Matches(msg, keys.Keys.Refresh):
			s.Loading = true
			return tea.Batch(s.Spinner.Tick, s.updateVolumesCmd())
		case key.Matches(msg, keys.Keys.Prune):
			return s.confirmVolumePrune()
		case key.Matches(msg, keys.Keys.Delete):
			return s.confirmVolumeDelete()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			var listCmd tea.Cmd
			s.List, listCmd = s.List.Update(msg)
			return listCmd
		case key.Matches(msg, keys.Keys.Filter):
			return tea.Batch(s.ToggleFilter(msg)...)
		}
	}

	return tea.Batch(cmds...)
}

// View renders the list.
func (s *Section) View() string {
	s.SetSize(s.Width, s.Height)

	listView := s.RenderList("Loading...")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		listView,
	)
}

// Reset resets internal state to when a component is first initialized.
func (s *Section) Reset() tea.Cmd {
	s.IsFilter = false
	s.SetSize(s.Width, s.Height)
	return nil
}

func (s *Section) removeItem(idx int) {
	s.List.RemoveItem(idx)
	if len(s.List.Items()) > 0 {
		s.List.Select(min(idx, len(s.List.Items())-1))
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
	items := s.List.Items()
	idx := s.List.Index()
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
	items := s.List.Items()
	idx := s.List.Index()
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
