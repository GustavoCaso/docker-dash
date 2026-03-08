package containers

import (
	"context"
	"fmt"

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
	listSplitRatio = 0.4  // fraction of width used by list in split view
	readBufSize    = 4096 // buffer size for reading container output
)

// Section wraps bubbles/list for displaying containers.
type Section struct {
	ctx           context.Context
	list          list.Model
	isFilter      bool
	viewport      viewport.Model
	service       client.ContainerService
	width, height int
	activePanel   panel.Panel
	logsPanel     panel.Panel
	filetreePanel panel.Panel
	execPanel     panel.Panel
	statsPanel    panel.Panel
	detailsPanel  panel.Panel
	loading       bool
	spinner       spinner.Model
}

// New creates a new container list.
func New(ctx context.Context, containers []client.Container, svc client.ContainerService) *Section {
	items := make([]list.Item, len(containers))
	for i, c := range containers {
		items[i] = containerItem{container: c}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)

	vp := viewport.New(0, 0)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	cl := &Section{
		ctx:           ctx,
		list:          l,
		viewport:      vp,
		service:       svc,
		spinner:       sp,
		logsPanel:     NewLogsPanel(ctx, svc),
		filetreePanel: NewFileTreePanel(ctx, svc),
		execPanel:     NewExecPanel(ctx, svc),
		statsPanel:    NewStatsPanel(ctx, svc),
		detailsPanel:  NewDetailsPanel(ctx, svc),
	}

	return cl
}

// SetSize sets dimensions.
func (s *Section) SetSize(width, height int) {
	s.width = width
	s.height = height

	// Account for padding and borders
	listX, listY := theme.ListStyle.GetFrameSize()

	if s.activePanel != nil {
		// Split view: 40% list, 60% details
		listWidth := int(float64(width) * listSplitRatio)
		detailWidth := width - listWidth

		s.list.SetSize(listWidth-listX, height-listY)
		s.viewport.Width = detailWidth - listX
		s.viewport.Height = height - listY
		s.activePanel.SetSize(s.viewport.Width, s.viewport.Height)
	} else {
		// Full width list when viewport is hidden
		s.list.SetSize(width-listX, height-listY)
	}
}

// Update handles messages.
//
//nolint:gocyclo // Update routes all container list events; splitting would scatter event handling logic
func (s *Section) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Handle spinner ticks while loading
	if s.loading {
		var spinnerCmd tea.Cmd
		s.spinner, spinnerCmd = s.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case containersLoadedMsg:
		s.loading = false
		cmd := s.list.SetItems(msg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	case containersTreeLoadedMsg:
		s.loading = false
		if s.activePanel != nil {
			return s.activePanel.Update(msg)
		}
		return nil
	case containersPrunedMsg:
		if msg.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: "Error pruning containers: " + msg.err.Error(),
					IsError: true,
				}
			}
		}
		summary := fmt.Sprintf(
			"Pruned %d containers, reclaimed %s",
			msg.report.ItemsDeleted,
			helper.FormatSize(int64(msg.report.SpaceReclaimed)),
		)
		return tea.Batch(s.updateContainersCmd(), func() tea.Msg {
			return message.ShowBannerMsg{Message: summary, IsError: false}
		})
	case containerActionMsg:
		if msg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error %s container: %s", msg.Action, msg.Error.Error()),
					IsError: true,
				}
			}
		}

		// For delete, remove from list
		if msg.Action == "deleting" {
			s.list.RemoveItem(msg.Idx)
		}

		// Refresh list after start/stop/restart
		if msg.Action == "starting" || msg.Action == "stopping" || msg.Action == "restarting" {
			cmds = append(cmds, s.updateContainersCmd())
		}

		return tea.Batch(append(cmds, func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Container %s %s", helper.ShortID(msg.ID), msg.Action),
				IsError: false,
			}
		})...)
	case statsSessionStartedMsg:
		s.loading = false
		if s.activePanel != nil {
			return s.activePanel.Update(msg)
		}
		return nil
	case execCloseMsg:
		if s.activePanel != nil {
			s.activePanel.Close()
			s.activePanel = nil
		}
		return func() tea.Msg {
			return message.ShowBannerMsg{Message: "Exec session closed", IsError: false}
		}
	case tea.KeyMsg:
		// When exec is active, route ALL keys directly to it.
		if ep, ok := s.activePanel.(*execPanel); ok {
			return ep.Update(msg)
		}

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
		case key.Matches(msg, keys.Keys.ContainerInfo):
			if s.activePanel == s.detailsPanel {
				cmd := s.detailsPanel.Close()
				s.activePanel = nil
				return cmd
			}
			selected := s.list.SelectedItem()
			if selected == nil {
				return nil
			}
			cItem, ok := selected.(containerItem)
			if !ok {
				return nil
			}
			s.activePanel = s.detailsPanel
			return s.detailsPanel.Init(cItem.container.ID)
		case key.Matches(msg, keys.Keys.Refresh):
			s.loading = true
			return tea.Batch(s.spinner.Tick, s.updateContainersCmd())
		case key.Matches(msg, keys.Keys.ContainerLogs):
			if s.activePanel != nil && s.activePanel == s.logsPanel {
				cmd := s.logsPanel.Close()
				s.activePanel = nil
				return cmd
			}

			selected := s.list.SelectedItem()
			if selected == nil {
				return nil
			}
			cItem, ok := selected.(containerItem)
			if !ok {
				return nil
			}
			if cItem.container.State != client.StateRunning {
				return func() tea.Msg {
					return message.ShowBannerMsg{Message: "Container is not running", IsError: true}
				}
			}

			s.activePanel = s.logsPanel
			return s.logsPanel.Init(cItem.container.ID)
		case key.Matches(msg, keys.Keys.FileTree):
			if s.activePanel == s.filetreePanel {
				cmd := s.filetreePanel.Close()
				s.activePanel = nil
				return cmd
			}
			selected := s.list.SelectedItem()
			if selected == nil {
				return nil
			}
			cItem, ok := selected.(containerItem)
			if !ok {
				return nil
			}
			s.activePanel = s.filetreePanel
			s.loading = true
			return tea.Batch(s.spinner.Tick, s.filetreePanel.Init(cItem.container.ID))
		case key.Matches(msg, keys.Keys.ContainerStats):
			if s.activePanel == s.statsPanel {
				cmd := s.statsPanel.Close()
				s.activePanel = nil
				return cmd
			}
			selected := s.list.SelectedItem()
			if selected == nil {
				return nil
			}
			cItem, ok := selected.(containerItem)
			if !ok {
				return nil
			}
			if cItem.container.State != client.StateRunning {
				return func() tea.Msg {
					return message.ShowBannerMsg{Message: "Container is not running", IsError: true}
				}
			}
			s.activePanel = s.statsPanel
			s.loading = true
			return tea.Batch(s.spinner.Tick, s.statsPanel.Init(cItem.container.ID))
		case key.Matches(msg, keys.Keys.ContainerExec):
			if s.activePanel == s.execPanel {
				cmd := s.activePanel.Close()
				s.activePanel = nil
				return cmd
			}
			selected := s.list.SelectedItem()
			if selected == nil {
				return nil
			}
			cItem, ok := selected.(containerItem)
			if !ok {
				return nil
			}
			if cItem.container.State != client.StateRunning {
				return func() tea.Msg {
					return message.ShowBannerMsg{Message: "Container is not running", IsError: true}
				}
			}
			s.activePanel = s.execPanel
			return s.execPanel.Init(cItem.container.ID)
		case key.Matches(msg, keys.Keys.Prune):
			return s.confirmContainerPrune()
		case key.Matches(msg, keys.Keys.ContainerDelete):
			return s.confirmContainerDelete()
		case key.Matches(msg, keys.Keys.ContainerStartStop):
			return s.confirmContainerToggle()
		case key.Matches(msg, keys.Keys.ContainerRestart):
			return s.confirmContainerRestart()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			return tea.Batch(listCmd, s.clearDetails())
		case key.Matches(msg, keys.Keys.Filter):
			s.isFilter = !s.isFilter
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			return tea.Batch(listCmd, s.extendFilterHelpCommand())
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

	// Overlay spinner in bottom right corner when loading
	if s.loading {
		spinnerText := s.spinner.View() + " Refreshing..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, s.list.Width())
	}

	listView := theme.ListStyle.
		Width(s.list.Width()).
		Render(listContent)

	// Only show viewport when an active panel is open
	if s.activePanel == nil {
		return listView
	}

	detailContent := s.activePanel.View()
	s.viewport.SetContent(detailContent)

	detailView := theme.ListStyle.
		Width(s.viewport.Width).
		Height(s.viewport.Height).
		Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

// Reset reset internal state to when a component isfirst initialized.
func (s *Section) Reset() tea.Cmd {
	var cmd tea.Cmd
	s.isFilter = false
	if s.activePanel != nil {
		cmd = s.activePanel.Close()
		s.activePanel = nil
	}
	s.viewport.SetContent("")
	s.SetSize(s.width, s.height)
	return cmd
}

func (s *Section) clearViewPort() {
	s.viewport.SetContent("")
}

func (s *Section) clearDetails() tea.Cmd {
	var cmd tea.Cmd
	if s.activePanel != nil {
		cmd = s.activePanel.Close()
		s.activePanel = nil
	}
	s.clearViewPort()

	return cmd
}

func (s *Section) deleteContainerCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.service
	items := s.list.Items()
	idx := s.list.Index()
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
	items := s.list.Items()
	idx := s.list.Index()
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
	items := s.list.Items()
	idx := s.list.Index()
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
	items := s.list.Items()
	idx := s.list.Index()
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
	items := s.list.Items()
	idx := s.list.Index()
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
	items := s.list.Items()
	idx := s.list.Index()
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
