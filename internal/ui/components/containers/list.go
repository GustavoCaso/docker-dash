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
	"github.com/GustavoCaso/docker-dash/internal/ui/components/helpers"
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
	return state + " " + c.container.Image + " " + helpers.ShortID(c.ID())
}
func (c containerItem) FilterValue() string { return c.container.Name }

const (
	listSplitRatio = 0.4  // fraction of width used by list in split view
	readBufSize    = 4096 // buffer size for reading container output
)

// List wraps bubbles/list for displaying containers.
type List struct {
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
func New(containers []client.Container, svc client.ContainerService) *List {
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

	cl := &List{
		list:          l,
		viewport:      vp,
		service:       svc,
		spinner:       sp,
		logsPanel:     NewLogsPanel(svc),
		filetreePanel: NewFileTreePanel(svc),
		execPanel:     NewExecPanel(svc),
		statsPanel:    NewStatsPanel(svc),
		detailsPanel:  NewDetailsPanel(svc),
	}

	return cl
}

// SetSize sets dimensions.
func (c *List) SetSize(width, height int) {
	c.width = width
	c.height = height

	// Account for padding and borders
	listX, listY := theme.ListStyle.GetFrameSize()

	if c.activePanel != nil {
		// Split view: 40% list, 60% details
		listWidth := int(float64(width) * listSplitRatio)
		detailWidth := width - listWidth

		c.list.SetSize(listWidth-listX, height-listY)
		c.viewport.Width = detailWidth - listX
		c.viewport.Height = height - listY
		c.activePanel.SetSize(c.viewport.Width, c.viewport.Height)
	} else {
		// Full width list when viewport is hidden
		c.list.SetSize(width-listX, height-listY)
	}
}

// Update handles messages.
//
//nolint:gocyclo // Update routes all container list events; splitting would scatter event handling logic
func (c *List) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Handle spinner ticks while loading
	if c.loading {
		var spinnerCmd tea.Cmd
		c.spinner, spinnerCmd = c.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case containersLoadedMsg:
		c.loading = false
		cmd := c.list.SetItems(msg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	case containersTreeLoadedMsg:
		c.loading = false
		if c.activePanel != nil {
			return c.activePanel.Update(msg)
		}
		return nil
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
			c.list.RemoveItem(msg.Idx)
		}

		// Refresh list after start/stop/restart
		if msg.Action == "starting" || msg.Action == "stopping" || msg.Action == "restarting" {
			cmds = append(cmds, c.updateContainersCmd())
		}

		return tea.Batch(append(cmds, func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Container %s %s", helpers.ShortID(msg.ID), msg.Action),
				IsError: false,
			}
		})...)
	case statsSessionStartedMsg:
		c.loading = false
		if c.activePanel != nil {
			return c.activePanel.Update(msg)
		}
		return nil
	case execCloseMsg:
		if c.activePanel != nil {
			c.activePanel.Close()
			c.activePanel = nil
		}
		return func() tea.Msg {
			return message.ShowBannerMsg{Message: "Exec session closed", IsError: false}
		}
	case tea.KeyMsg:
		// When exec is active, route ALL keys directly to it.
		if ep, ok := c.activePanel.(*execPanel); ok {
			return ep.Update(msg)
		}

		if c.isFilter {
			var filterCmds []tea.Cmd
			var listCmd tea.Cmd
			c.list, listCmd = c.list.Update(msg)
			filterCmds = append(filterCmds, listCmd)

			if key.Matches(msg, keys.Keys.Esc) {
				c.isFilter = !c.isFilter
				filterCmds = append(filterCmds, func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} })
			}
			return tea.Batch(filterCmds...)
		}

		switch {
		case key.Matches(msg, keys.Keys.ContainerInfo):
			if c.activePanel == c.detailsPanel {
				c.detailsPanel.Close()
				c.activePanel = nil
				return nil
			}
			selected := c.list.SelectedItem()
			if selected == nil {
				return nil
			}
			cItem, ok := selected.(containerItem)
			if !ok {
				return nil
			}
			c.activePanel = c.detailsPanel
			return c.detailsPanel.Init(cItem.container.ID)
		case key.Matches(msg, keys.Keys.Refresh):
			c.loading = true
			return tea.Batch(c.spinner.Tick, c.updateContainersCmd())
		case key.Matches(msg, keys.Keys.ContainerLogs):
			if c.activePanel != nil && c.activePanel == c.logsPanel {
				c.logsPanel.Close()
				c.activePanel = nil
				return nil
			}

			selected := c.list.SelectedItem()
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

			c.activePanel = c.logsPanel
			return c.logsPanel.Init(cItem.container.ID)
		case key.Matches(msg, keys.Keys.FileTree):
			if c.activePanel == c.filetreePanel {
				c.filetreePanel.Close()
				c.activePanel = nil
				return nil
			}
			selected := c.list.SelectedItem()
			if selected == nil {
				return nil
			}
			cItem, ok := selected.(containerItem)
			if !ok {
				return nil
			}
			c.activePanel = c.filetreePanel
			c.loading = true
			return tea.Batch(c.spinner.Tick, c.filetreePanel.Init(cItem.container.ID))
		case key.Matches(msg, keys.Keys.ContainerStats):
			if c.activePanel == c.statsPanel {
				c.statsPanel.Close()
				c.activePanel = nil
				return nil
			}
			selected := c.list.SelectedItem()
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
			c.activePanel = c.statsPanel
			c.loading = true
			return tea.Batch(c.spinner.Tick, c.statsPanel.Init(cItem.container.ID))
		case key.Matches(msg, keys.Keys.ContainerExec):
			if c.activePanel == c.execPanel {
				c.activePanel.Close()
				c.activePanel = nil
				return nil
			}
			selected := c.list.SelectedItem()
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
			c.activePanel = c.execPanel
			return c.execPanel.Init(cItem.container.ID)
		case key.Matches(msg, keys.Keys.ContainerDelete):
			return c.deleteContainerCmd()
		case key.Matches(msg, keys.Keys.ContainerStartStop):
			return c.toggleContainerCmd()
		case key.Matches(msg, keys.Keys.ContainerRestart):
			return c.restartContainerCmd()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			var listCmd tea.Cmd
			c.list, listCmd = c.list.Update(msg)
			c.clearDetails()
			return listCmd
		case key.Matches(msg, keys.Keys.ScrollUp, keys.Keys.ScrollDown):
			var vpCmd tea.Cmd
			c.viewport, vpCmd = c.viewport.Update(msg)
			return vpCmd
		case key.Matches(msg, keys.Keys.Filter):
			c.isFilter = !c.isFilter
			var listCmd tea.Cmd
			c.list, listCmd = c.list.Update(msg)
			return tea.Batch(listCmd, c.extendFilterHelpCommand())
		}
	}

	// Send the remaining of msg to both panels
	var listCmd tea.Cmd
	c.list, listCmd = c.list.Update(msg)
	cmds = append(cmds, listCmd)

	var vpCmd tea.Cmd
	c.viewport, vpCmd = c.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	if c.activePanel != nil {
		cmds = append(cmds, c.activePanel.Update(msg))
	}

	return tea.Batch(cmds...)
}

// View renders the list.
func (c *List) View() string {
	c.SetSize(c.width, c.height)
	listContent := c.list.View()

	// Overlay spinner in bottom right corner when loading
	if c.loading {
		spinnerText := c.spinner.View() + " Refreshing..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, c.list.Width())
	}

	listView := theme.ListStyle.
		Width(c.list.Width()).
		Render(listContent)

	// Only show viewport when an active panel is open
	if c.activePanel == nil {
		return listView
	}

	detailContent := c.activePanel.View()
	c.viewport.SetContent(detailContent)

	detailView := theme.ListStyle.
		Width(c.viewport.Width).
		Height(c.viewport.Height).
		Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

func (c *List) clearViewPort() {
	c.viewport.SetContent("")
}

func (c *List) clearDetails() {
	if c.activePanel != nil {
		c.activePanel.Close()
		c.activePanel = nil
	}
	c.clearViewPort()
}

func (c *List) deleteContainerCmd() tea.Cmd {
	svc := c.service
	items := c.list.Items()
	idx := c.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	ci, ok := items[idx].(containerItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		err := svc.Remove(ctx, ci.ID(), true)
		return containerActionMsg{ID: ci.ID(), Action: "deleting", Idx: idx, Error: err}
	}
}

func (c *List) toggleContainerCmd() tea.Cmd {
	svc := c.service
	items := c.list.Items()
	idx := c.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	ci, ok := items[idx].(containerItem)
	if !ok {
		return nil
	}

	container := ci.container
	return func() tea.Msg {
		ctx := context.Background()
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

func (c *List) restartContainerCmd() tea.Cmd {
	svc := c.service
	items := c.list.Items()
	idx := c.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	ci, ok := items[idx].(containerItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		err := svc.Restart(ctx, ci.ID())
		return containerActionMsg{ID: ci.ID(), Action: "restarting", Idx: idx, Error: err}
	}
}

func (c *List) updateContainersCmd() tea.Cmd {
	svc := c.service
	return func() tea.Msg {
		ctx := context.Background()
		containers, err := svc.List(ctx)
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

func (c *List) extendFilterHelpCommand() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "exit"),
			),
		}}
	}
}
