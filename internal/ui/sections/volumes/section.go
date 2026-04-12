package volumes

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
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
	*base.Section
	ctx           context.Context
	volumeService client.VolumeService
}

// New creates a new volume list.
func New(ctx context.Context, volumes []client.Volume, svc client.VolumeService) *Section {
	items := make([]list.Item, len(volumes))
	for i, v := range volumes {
		items[i] = volumeItem{volume: v}
	}

	s := &Section{
		ctx:           ctx,
		volumeService: svc,
		Section:       base.New(sections.VolumesSection, items, []panel.Panel{}),
	}

	s.LoadingText = "Loading..."
	s.RefreshCmd = s.updateVolumesCmd
	s.PruneCmd = s.confirmVolumePrune
	s.HandleMsg = s.handleMsg
	s.HandleKey = s.handleKey

	return s
}

func (s *Section) handleMsg(msg tea.Msg) base.UpdateResult {
	switch msg := msg.(type) {
	case volumesLoadedMsg:
		log.Printf("[volumes] volumesLoadedMsg: count=%d", len(msg.items))
		if msg.error != nil {
			return base.UpdateResult{
				Cmd: func() tea.Msg {
					return message.ShowBannerMsg{
						Message: fmt.Sprintf("Error loading volumes: %s", msg.error.Error()),
						IsError: true,
					}
				},
				Handled:     true,
				StopSpinner: true,
			}
		}
		return base.UpdateResult{Cmd: s.List.SetItems(msg.items), Handled: true, StopSpinner: true}
	case volumesPrunedMsg:
		log.Printf("[volumes] volumesPrunedMsg: deleted=%d spaceReclaimed=%d",
			msg.report.ItemsDeleted, msg.report.SpaceReclaimed)
		if msg.err != nil {
			return base.UpdateResult{
				Cmd: func() tea.Msg {
					return message.ShowBannerMsg{
						Message: "Error pruning volumes: " + msg.err.Error(),
						IsError: true,
					}
				},
				Handled:     true,
				StopSpinner: true,
			}
		}
		summary := fmt.Sprintf(
			"Pruned %d volumes, reclaimed %s",
			msg.report.ItemsDeleted,
			helper.FormatSize(msg.report.SpaceReclaimed),
		)
		return base.UpdateResult{
			Cmd: tea.Batch(s.updateVolumesCmd(), func() tea.Msg {
				return message.ShowBannerMsg{Message: summary, IsError: false}
			}),
			Handled: true,
		}
	case volumeRemovedMsg:
		log.Printf("[volumes] volumeRemovedMsg: name=%q err=%v", msg.Name, msg.Error)
		if msg.Error != nil {
			return base.UpdateResult{
				Cmd: func() tea.Msg {
					return message.ShowBannerMsg{
						Message: fmt.Sprintf("Error deleting volume: %s", msg.Error.Error()),
						IsError: true,
					}
				},
				Handled:     true,
				StopSpinner: true,
			}
		}
		s.RemoveItem(msg.Idx)
		return base.UpdateResult{
			Cmd: func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Volume %s deleted", msg.Name),
					IsError: false,
				}
			},
			Handled:     true,
			StopSpinner: true,
		}
	}
	return base.UpdateResult{}
}

func (s *Section) handleKey(msg tea.KeyMsg) base.UpdateResult {
	if key.Matches(msg, keys.Keys.Delete) {
		return base.UpdateResult{Cmd: s.confirmVolumeDelete(), Handled: true}
	}
	return base.UpdateResult{}
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
			OnConfirm: s.WithSpinner(pruneCmd),
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
			OnConfirm: s.WithSpinner(deleteCmd),
		}
	}
}
