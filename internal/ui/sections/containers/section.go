package containers

import (
	"context"
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/base"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// containersLoadedMsg is sent when containers have been loaded asynchronously.
type containersLoadedMsg struct {
	error error
	items []list.Item
}

// containersPrunedMsg is sent when a container prune completes.
type containersPrunedMsg struct {
	report client.PruneReport
	err    error
}

// containerActionMsg is sent when a container action completes.
type containerActionMsg struct {
	ID     string
	Action string
	Idx    int
	Error  error
}

// containerItem implements list.Item interface.
type containerItem struct {
	container client.Container
}

func (c containerItem) ID() string    { return c.container.ID }
func (c containerItem) Title() string { return c.container.Name }
func (c containerItem) Description() string {
	stateIcon := theme.GetContainerStatusIcon(string(c.container.State))
	stateStyle := theme.GetContainerStatusStyle(string(c.container.State))
	state := stateStyle.Render(stateIcon + " " + string(c.container.State))
	return state + " " + c.container.Image + " " + helper.ShortID(c.ID())
}
func (c containerItem) FilterValue() string { return c.container.Name }

const (
	readBufSize = 4096 // buffer size for reading container output
)

// Section wraps bubbles/list for displaying containers.
type Section struct {
	base.Section
	ctx     context.Context
	service client.ContainerService
}

// New creates a new container list.
func New(ctx context.Context, containers []client.Container, svc client.ContainerService) *Section {
	items := make([]list.Item, len(containers))
	for i, c := range containers {
		items[i] = containerItem{container: c}
	}

	cl := &Section{
		ctx:     ctx,
		service: svc,
		Section: base.Section{
			List:    base.NewList(items),
			Spinner: base.NewSpinner(),
			Panels: []panel.Panel{
				NewDetailsPanel(ctx, svc),
				NewLogsPanel(ctx, svc),
				NewStatsPanel(ctx, svc),
				panel.NewFilesPanel(ctx, svc),
				NewExecPanel(ctx, svc),
			},
			ActivePanelIdx: 0,
			ActivePanelInitFn: func(item list.Item) string {
				ci, ok := item.(containerItem)
				if !ok {
					return ""
				}
				return ci.container.ID
			},
		},
	}

	cl.Name = "containers"
	cl.LoadingText = "Refreshing..."
	cl.RefreshCmd = cl.updateContainersCmd
	cl.PruneCmd = cl.confirmContainerPrune
	cl.HandleMsg = cl.handleMsg
	cl.HandleKey = cl.handleKey

	return cl
}

func (s *Section) handleMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case containersLoadedMsg:
		log.Printf("[containers] containersLoadedMsg: count=%d", len(msg.items))
		s.Loading = false
		return s.List.SetItems(msg.items), true
	case containersPrunedMsg:
		log.Printf(
			"[containers] containersPrunedMsg: deleted=%d spaceReclaimed=%d",
			msg.report.ItemsDeleted,
			msg.report.SpaceReclaimed,
		)
		if msg.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: "Error pruning containers: " + msg.err.Error(),
					IsError: true,
				}
			}, true
		}
		summary := fmt.Sprintf(
			"Pruned %d containers, reclaimed %s",
			msg.report.ItemsDeleted,
			helper.FormatSize(msg.report.SpaceReclaimed),
		)
		return tea.Batch(s.updateContainersCmd(), func() tea.Msg {
			return message.ShowBannerMsg{Message: summary, IsError: false}
		}), true
	case containerActionMsg:
		log.Printf(
			"[containers] containerActionMsg: action=%q containerID=%q err=%v",
			msg.Action,
			msg.ID,
			msg.Error,
		)
		if msg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error %s container: %s", msg.Action, msg.Error.Error()),
					IsError: true,
				}
			}, true
		}
		var cmds []tea.Cmd
		if msg.Action == "deleting" {
			cmds = append(cmds, s.RemoveItemAndUpdatePanel(msg.Idx))
		}
		if msg.Action == "starting" || msg.Action == "stopping" || msg.Action == "restarting" {
			cmds = append(cmds, s.updateContainersCmd())
		}
		return tea.Batch(append(cmds, func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Container %s %s", helper.ShortID(msg.ID), msg.Action),
				IsError: false,
			}
		})...), true
	case execCloseMsg:
		log.Printf("[containers] execCloseMsg")
		s.ActivePanel().Close()
		return func() tea.Msg {
			return message.ShowBannerMsg{Message: "Exec session closed", IsError: false}
		}, true
	}
	return nil, false
}

func (s *Section) handleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	// When exec panel is active, route ALL keys directly to it.
	if ep, ok := s.ActivePanel().(*execPanel); ok {
		log.Print("[containers] forward message to exec panel")
		return ep.Update(msg), true
	}
	switch {
	case key.Matches(msg, keys.Keys.ContainerDelete):
		return s.confirmContainerDelete(), true
	case key.Matches(msg, keys.Keys.ContainerStartStop):
		return s.confirmContainerToggle(), true
	case key.Matches(msg, keys.Keys.ContainerRestart):
		return s.confirmContainerRestart(), true
	}
	return nil, false
}

func (s *Section) deleteContainerCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.service
	items := s.List.Items()
	idx := s.List.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	ci, ok := items[idx].(containerItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := svc.Remove(ctx, ci.ID(), true)
		return containerActionMsg{ID: ci.ID(), Action: "deleting", Idx: idx, Error: err}
	}
}

func (s *Section) toggleContainerCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.service
	items := s.List.Items()
	idx := s.List.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	ci, ok := items[idx].(containerItem)
	if !ok {
		return nil
	}

	container := ci.container
	return func() tea.Msg {
		var err error
		var action string
		if container.State == client.StateRunning {
			action = "stopping"
			err = svc.Stop(ctx, container.ID)
		} else {
			action = "starting"
			err = svc.Start(ctx, container.ID)
		}
		return containerActionMsg{ID: container.ID, Action: action, Idx: idx, Error: err}
	}
}

func (s *Section) restartContainerCmd() tea.Cmd {
	svc := s.service
	items := s.List.Items()
	idx := s.List.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	ci, ok := items[idx].(containerItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := svc.Restart(s.ctx, ci.ID())
		return containerActionMsg{ID: ci.ID(), Action: "restarting", Idx: idx, Error: err}
	}
}

func (s *Section) updateContainersCmd() tea.Cmd {
	svc := s.service
	return func() tea.Msg {
		containers, err := svc.List(s.ctx)
		if err != nil {
			return containersLoadedMsg{error: err}
		}
		items := make([]list.Item, len(containers))
		for idx, container := range containers {
			items[idx] = containerItem{container: container}
		}
		return containersLoadedMsg{items: items}
	}
}

func (s *Section) pruneContainersCmd() tea.Cmd {
	ctx, svc := s.ctx, s.service
	return func() tea.Msg {
		report, err := svc.Prune(ctx, client.PruneOptions{})
		return containersPrunedMsg{report: report, err: err}
	}
}

func (s *Section) confirmContainerPrune() tea.Cmd {
	pruneCmd := s.pruneContainersCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Prune Containers",
			Body:      "Remove all stopped containers?",
			OnConfirm: pruneCmd,
		}
	}
}

func (s *Section) confirmContainerDelete() tea.Cmd {
	items := s.List.Items()
	idx := s.List.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}
	ci, ok := items[idx].(containerItem)
	if !ok {
		return nil
	}
	deleteCmd := s.deleteContainerCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Delete Container",
			Body:      fmt.Sprintf("Delete container %s?", helper.ShortID(ci.ID())),
			OnConfirm: deleteCmd,
		}
	}
}

func (s *Section) confirmContainerToggle() tea.Cmd {
	items := s.List.Items()
	idx := s.List.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}
	ci, ok := items[idx].(containerItem)
	if !ok {
		return nil
	}
	action := "Stop"
	if ci.container.State != client.StateRunning {
		action = "Start"
	}
	toggleCmd := s.toggleContainerCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     fmt.Sprintf("%s Container", action),
			Body:      fmt.Sprintf("%s container %s?", action, helper.ShortID(ci.ID())),
			OnConfirm: toggleCmd,
		}
	}
}

func (s *Section) confirmContainerRestart() tea.Cmd {
	items := s.List.Items()
	idx := s.List.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}
	ci, ok := items[idx].(containerItem)
	if !ok {
		return nil
	}
	restartCmd := s.restartContainerCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Restart Container",
			Body:      fmt.Sprintf("Restart container %s?", helper.ShortID(ci.ID())),
			OnConfirm: restartCmd,
		}
	}
}


