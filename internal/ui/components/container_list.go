package components

import (
	"context"
	"fmt"
	"strings"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/pkg/stdcopy"
)

// containersLoadedMsg is sent when containers have been loaded asynchronously
type containersLoadedMsg struct {
	error error
	items []list.Item
}

// containersTreeLoadedMsg is sent when containers have been loaded asynchronously
type containersTreeLoadedMsg struct {
	error    error
	fileTree service.ContainerFileTree
}

// ContainerActionMsg is sent when a container action completes
type ContainerActionMsg struct {
	ID     string
	Action string
	Idx    int
	Error  error
}

// Key bindings for container list actions
var containerDetailsKey = key.NewBinding(
	key.WithKeys("d"),
	key.WithHelp("d", "details"),
)

var containerRefreshKey = key.NewBinding(
	key.WithKeys("r"),
	key.WithHelp("r", "refresh"),
)

var containerDeleteKey = key.NewBinding(
	key.WithKeys("D"),
	key.WithHelp("D", "delete"),
)

var containerStartStopKey = key.NewBinding(
	key.WithKeys("s"),
	key.WithHelp("s", "start/stop"),
)

var containerRestartKey = key.NewBinding(
	key.WithKeys("R"),
	key.WithHelp("R", "restart"),
)

var logsKey = key.NewBinding(
	key.WithKeys("l"),
	key.WithHelp("l", "logs"),
)

var treeKey = key.NewBinding(
	key.WithKeys("t"),
	key.WithHelp("t", "files"),
)

// KeyBindings returns the key bindings for the current state
func (c *ContainerList) KeyBindings() []key.Binding {
	return []key.Binding{mainNavKey, secondaryNavKey, containerDetailsKey, logsKey, containerStartStopKey, containerRestartKey, containerRefreshKey, containerDeleteKey, treeKey}
}

// ContainerItem implements list.Item interface
type ContainerItem struct {
	container service.Container
}

func (c ContainerItem) ID() string    { return c.container.ID }
func (c ContainerItem) Title() string { return c.container.Name }
func (c ContainerItem) Description() string {
	stateIcon := theme.StatusIcon(string(c.container.State))
	stateStyle := theme.StatusStyle(string(c.container.State))
	state := stateStyle.Render(stateIcon + " " + string(c.container.State))
	return state + " " + c.container.Image + " " + c.ID()
}
func (c ContainerItem) FilterValue() string { return c.container.Name }

// ContainerList wraps bubbles/list for displaying containers
type ContainerList struct {
	list          list.Model
	viewport      viewport.Model
	service       service.ContainerService
	width, height int
	lastSelected  int
	showDetails   bool
	showLogs      bool
	showFileTree  bool
	loading       bool
	spinner       spinner.Model
}

// NewContainerList creates a new container list
func NewContainerList(containers []service.Container, svc service.ContainerService) *ContainerList {
	items := make([]list.Item, len(containers))
	for i, c := range containers {
		items[i] = ContainerItem{container: c}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)

	vp := viewport.New(0, 0)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	cl := &ContainerList{
		list:         l,
		viewport:     vp,
		lastSelected: -1,
		service:      svc,
		spinner:      sp,
	}

	return cl
}

// SetSize sets dimensions
func (c *ContainerList) SetSize(width, height int) {
	c.width = width
	c.height = height

	// Account for padding and borders
	listX, listY := listStyle.GetFrameSize()

	if c.showDetails || c.showLogs || c.showFileTree {
		// Split view: 40% list, 60% details
		listWidth := int(float64(width) * 0.4)
		detailWidth := width - listWidth

		c.list.SetSize(listWidth-listX, height-listY)
		c.viewport.Width = detailWidth - listX
		c.viewport.Height = height - listY
	} else {
		// Full width list when viewport is hidden
		c.list.SetSize(width-listX, height-listY)
	}
}

// Update handles messages
func (c *ContainerList) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Handle spinner ticks while loading
	if c.loading {
		var spinnerCmd tea.Cmd
		c.spinner, spinnerCmd = c.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	// Handle containers loaded message
	if loadedMsg, ok := msg.(containersLoadedMsg); ok {
		c.loading = false
		cmd := c.list.SetItems(loadedMsg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	}

	if loadedMsg, ok := msg.(containersTreeLoadedMsg); ok {
		c.loading = false
		c.viewport.SetContent(lipgloss.NewStyle().Width(c.viewport.Width).Render(loadedMsg.fileTree.Tree.String()))
		return nil
	}

	// Handle container action message
	if actionMsg, ok := msg.(ContainerActionMsg); ok {
		if actionMsg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error %s container: %s", actionMsg.Action, actionMsg.Error.Error()),
					IsError: true,
				}
			}
		}

		// For delete, remove from list
		if actionMsg.Action == "deleting" {
			c.list.RemoveItem(actionMsg.Idx)
		}

		// Refresh list after start/stop/restart
		if actionMsg.Action == "starting" || actionMsg.Action == "stopping" || actionMsg.Action == "restarting" {
			cmds = append(cmds, c.updateContainersCmd())
		}

		return tea.Batch(append(cmds, func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Container %s %s", shortID(actionMsg.ID), actionMsg.Action),
				IsError: false,
			}
		})...)
	}

	// Handle focus switching and actions
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "d":
			c.showDetails = !c.showDetails
			c.showLogs = false
			c.showFileTree = false
			c.SetSize(c.width, c.height) // Recalculate layout
			return nil
		case "r":
			c.loading = true
			return tea.Batch(c.spinner.Tick, c.updateContainersCmd())
		case "l":
			c.showLogs = !c.showLogs
			c.showDetails = false
			c.showFileTree = false
			c.SetSize(c.width, c.height) // Recalculate layout
			return nil
		case "t":
			c.showFileTree = !c.showFileTree
			if c.showFileTree {
				c.loading = true
				c.showDetails = false
				c.showLogs = false
			}
			c.SetSize(c.width, c.height) // Recalculate layout
			return tea.Batch(c.spinner.Tick, c.fetchFileTreeInformation())
		case "D":
			return c.deleteContainerCmd()
		case "s":
			return c.toggleContainerCmd()
		case "R":
			return c.restartContainerCmd()
		case "up", "down":
			var listCmd tea.Cmd
			c.list, listCmd = c.list.Update(msg)
			if c.list.Index() != c.lastSelected {
				c.lastSelected = c.list.Index()
			}
			return listCmd
		case "j", "k":
			var vpCmd tea.Cmd
			c.viewport, vpCmd = c.viewport.Update(msg)
			return vpCmd
		}
	}

	// Send the remaining of msg to both panels
	var listCmd tea.Cmd
	c.list, listCmd = c.list.Update(msg)
	cmds = append(cmds, listCmd)

	var vpCmd tea.Cmd
	c.viewport, vpCmd = c.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return tea.Batch(cmds...)
}

// View renders the list
func (c *ContainerList) View() string {
	listContent := c.list.View()

	// Overlay spinner in bottom right corner when loading
	if c.loading {
		spinnerText := c.spinner.View() + " Refreshing..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, c.list.Width())
	}

	listView := listStyle.
		Width(c.list.Width()).
		Render(listContent)

	// Only show viewport when details are toggled on
	if !c.showDetails && !c.showLogs && !c.showFileTree {
		return listView
	}

	c.updateDetails()

	detailView := listStyle.
		Width(c.viewport.Width).
		Render(c.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

func (c *ContainerList) deleteContainerCmd() tea.Cmd {
	svc := c.service
	items := c.list.Items()
	idx := c.lastSelected
	if idx < 0 || idx >= len(items) {
		return nil
	}

	item := items[idx]
	containerItem, ok := item.(ContainerItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		err := svc.Remove(ctx, containerItem.ID(), true)
		return ContainerActionMsg{ID: containerItem.ID(), Action: "deleting", Idx: idx, Error: err}
	}
}

func (c *ContainerList) toggleContainerCmd() tea.Cmd {
	svc := c.service
	items := c.list.Items()
	idx := c.lastSelected
	if idx < 0 || idx >= len(items) {
		return nil
	}

	item := items[idx]
	containerItem, ok := item.(ContainerItem)
	if !ok {
		return nil
	}

	container := containerItem.container
	return func() tea.Msg {
		ctx := context.Background()
		var err error
		var action string
		if container.State == service.StateRunning {
			action = "stopping"
			err = svc.Stop(ctx, container.ID)
		} else {
			action = "starting"
			err = svc.Start(ctx, container.ID)
		}
		return ContainerActionMsg{ID: container.ID, Action: action, Idx: idx, Error: err}
	}
}

func (c *ContainerList) restartContainerCmd() tea.Cmd {
	svc := c.service
	items := c.list.Items()
	idx := c.lastSelected
	if idx < 0 || idx >= len(items) {
		return nil
	}

	item := items[idx]
	containerItem, ok := item.(ContainerItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		err := svc.Restart(ctx, containerItem.ID())
		return ContainerActionMsg{ID: containerItem.ID(), Action: "restarting", Idx: idx, Error: err}
	}
}

func (c *ContainerList) updateContainersCmd() tea.Cmd {
	svc := c.service
	return func() tea.Msg {
		ctx := context.Background()
		containers, err := svc.List(ctx)
		if err != nil {
			return containersLoadedMsg{error: err}
		}
		items := make([]list.Item, len(containers))
		for idx, container := range containers {
			items[idx] = ContainerItem{container: container}
		}
		return containersLoadedMsg{items: items}
	}
}

func (c *ContainerList) fetchFileTreeInformation() tea.Cmd {
	return func() tea.Msg {
		selected := c.list.SelectedItem()
		if selected == nil {
			return containersTreeLoadedMsg{error: fmt.Errorf("no container selected")}
		}

		ctx := context.Background()
		container := selected.(ContainerItem).container
		fileTree, err := c.service.FileTree(ctx, container.ID)
		if err != nil {
			return containersTreeLoadedMsg{error: fmt.Errorf("Error getting the file tree: %s", err.Error())}
		}
		return containersTreeLoadedMsg{fileTree: fileTree}
	}
}

// updateDetails updates the viewport content based on selected container
func (c *ContainerList) updateDetails() {
	selected := c.list.SelectedItem()
	if selected == nil {
		c.viewport.SetContent("No container selected")
		return
	}

	container := selected.(ContainerItem).container

	if c.showFileTree {
		return
	}

	if c.showLogs {
		c.logsDetails()
		return
	}

	var content strings.Builder

	// Header
	fmt.Fprintf(&content, "Container: %s\n", container.Name)
	content.WriteString("═══════════════════════\n\n")

	// Basic info
	fmt.Fprintf(&content, "ID:      %s\n", shortID(container.ID))
	fmt.Fprintf(&content, "Image:   %s\n", container.Image)
	fmt.Fprintf(&content, "Status:  %s\n", container.Status)

	stateStyle := theme.StatusStyle(string(container.State))
	stateIcon := theme.StatusIcon(string(container.State))
	fmt.Fprintf(&content, "State:   %s\n", stateStyle.Render(stateIcon+" "+string(container.State)))

	fmt.Fprintf(&content, "Created: %s\n\n", container.Created.Format("2006-01-02 15:04:05"))

	// Ports
	if len(container.Ports) > 0 {
		content.WriteString("Ports:\n")
		for _, port := range container.Ports {
			fmt.Fprintf(&content, "  %d:%d/%s\n", port.HostPort, port.ContainerPort, port.Protocol)
		}
		content.WriteString("\n")
	}

	// Mounts
	if len(container.Mounts) > 0 {
		content.WriteString("Mounts:\n")
		for _, mount := range container.Mounts {
			fmt.Fprintf(&content, "  [%s] %s -> %s\n", mount.Type, mount.Source, mount.Destination)
		}
		content.WriteString("\n")
	}

	c.viewport.SetContent(content.String())
	c.viewport.GotoTop()
}

func (c *ContainerList) logsDetails() {
	selected := c.list.SelectedItem()
	if selected == nil {
		c.viewport.SetContent("No container selected")
		return
	}

	container := selected.(ContainerItem).container
	ctx := context.Background()
	reader, err := c.service.Logs(ctx, container.ID, service.LogOptions{})
	if err != nil {
		c.viewport.SetContent(fmt.Sprintf("Error reading logs: %s", err.Error()))
		return
	}
	buf := new(strings.Builder)
	_, err = stdcopy.StdCopy(buf, buf, reader)
	if err != nil {
		c.viewport.SetContent(fmt.Sprintf("Error reading logs: %s", err.Error()))
		return
	}

	c.viewport.SetContent(lipgloss.NewStyle().Width(c.viewport.Width).Render(buf.String()))
}
