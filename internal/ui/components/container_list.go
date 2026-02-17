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
	"github.com/charmbracelet/bubbles/textinput"
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

// execOutputMsg is sent when exec output is received from the background reader
type execOutputMsg struct {
	output string
	err    error
}

type execSessionStartedMsg struct {
	session *service.ExecSession
}

type execCloseMsg struct{}

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

var execKey = key.NewBinding(
	key.WithKeys("e"),
	key.WithHelp("e", "exec"),
)

// KeyBindings returns the key bindings for the current state
func (c *ContainerList) KeyBindings() []key.Binding {
	return []key.Binding{mainNavKey, secondaryNavKey, containerDetailsKey, logsKey, containerStartStopKey, containerRestartKey, containerRefreshKey, containerDeleteKey, treeKey, execKey}
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
	return state + " " + c.container.Image + " " + shortID(c.ID())
}
func (c ContainerItem) FilterValue() string { return c.container.Name }

// ContainerList wraps bubbles/list for displaying containers
type ContainerList struct {
	list                    list.Model
	viewport                viewport.Model
	service                 service.ContainerService
	width, height           int
	showDetails             bool
	showLogs                bool
	showFileTree            bool
	loading                 bool
	spinner                 spinner.Model
	showExec                bool
	execSession             *service.ExecSession
	execInput               textinput.Model
	execHistory             []string
	execHistoryCurrentIndex int
	execOutput              string
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

	ti := textinput.New()
	ti.Prompt = "$ "

	cl := &ContainerList{
		list:         l,
		viewport:     vp,
		service:      svc,
		spinner:      sp,
		execInput:    ti,
		execHistory:  []string{},
	}

	return cl
}

// SetSize sets dimensions
func (c *ContainerList) SetSize(width, height int) {
	c.width = width
	c.height = height

	// Account for padding and borders
	listX, listY := listStyle.GetFrameSize()

	if c.showDetails || c.showLogs || c.showFileTree || c.showExec {
		// Split view: 40% list, 60% details
		listWidth := int(float64(width) * 0.4)
		detailWidth := width - listWidth

		c.list.SetSize(listWidth-listX, height-listY)
		c.viewport.Width = detailWidth - listX
		if c.showExec {
			c.viewport.Height = height - listY - 1
		} else {
			c.viewport.Height = height - listY
		}
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
		if loadedMsg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: loadedMsg.error.Error(),
					IsError: true,
				}
			}
		}
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

	if startMsg, ok := msg.(execSessionStartedMsg); ok {
		c.execSession = startMsg.session
		return c.readExecOutput()
	}

	if outputMsg, ok := msg.(execOutputMsg); ok {
		if outputMsg.err != nil {
			if c.execSession == nil {
				return nil // session was closed manually, ignore
			}
			c.closeExec()
			return func() tea.Msg {
				return message.ShowBannerMsg{Message: fmt.Sprintf("Exec session error. Err: %s", outputMsg.err), IsError: true}
			}
		}
		c.execOutput += outputMsg.output
		c.viewport.SetContent(lipgloss.NewStyle().Width(c.viewport.Width).Render(c.execOutput))
		c.viewport.GotoBottom()
		return c.readExecOutput()
	}

	if _, ok := msg.(execCloseMsg); ok {
		c.closeExec()
		c.SetSize(c.width, c.height)
		return func() tea.Msg {
			return message.ShowBannerMsg{Message: "Exec session closed", IsError: false}
		}
	}

	// Handle focus switching and actions
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// When exec is active, intercept all keys for the text input
		if c.showExec {
			switch keyMsg.String() {
			case "esc":
				return c.closeExecSessionMsg()
			case "enter":
				if c.execSession == nil {
					return nil
				}
				cmd := c.execInput.Value()
				if cmd == "" {
					return nil
				}
				c.execHistory = append(c.execHistory, cmd)
				c.execHistoryCurrentIndex = len(c.execHistory) // sentinel: not browsing
				c.execInput.Reset()
				_, err := c.execSession.Writer.Write([]byte(cmd + "\n"))
				if err != nil {
					c.closeExec()
					return func() tea.Msg {
						return message.ShowBannerMsg{Message: "Exec write failed", IsError: true}
					}
				}
				return nil
			case "up":
				if len(c.execHistory) == 0 {
					return nil
				}
				if c.execHistoryCurrentIndex > 0 {
					c.execHistoryCurrentIndex--
				} else if c.execHistoryCurrentIndex == len(c.execHistory) {
					// Start browsing from the most recent entry
					c.execHistoryCurrentIndex = len(c.execHistory) - 1
				} else {
					// Already at oldest entry
					return nil
				}
				c.execInput.SetValue(c.execHistory[c.execHistoryCurrentIndex])
				return nil
			case "down":
				if len(c.execHistory) == 0 || c.execHistoryCurrentIndex == len(c.execHistory) {
					return nil
				}
				c.execHistoryCurrentIndex++
				if c.execHistoryCurrentIndex == len(c.execHistory) {
					// Past newest entry — clear input
					c.execInput.Reset()
				} else {
					c.execInput.SetValue(c.execHistory[c.execHistoryCurrentIndex])
				}
				return nil
			default:
				var inputCmd tea.Cmd
				c.execInput, inputCmd = c.execInput.Update(msg)
				return inputCmd
			}
		}

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
				selected := c.list.SelectedItem()
				if selected == nil {
					return nil
				}
				container := selected.(ContainerItem).container
				c.loading = true
				c.showDetails = false
				c.showLogs = false
				c.SetSize(c.width, c.height) // Recalculate layout
				c.viewport.SetContent("")
				return tea.Batch(c.spinner.Tick, c.fetchFileTreeInformation(container.ID))
			}
			c.SetSize(c.width, c.height) // Recalculate layout
			return nil
		case "e":
			selected := c.list.SelectedItem()
			if selected == nil {
				return nil
			}
			container := selected.(ContainerItem).container
			if container.State != service.StateRunning {
				return func() tea.Msg {
					return message.ShowBannerMsg{Message: "Container is not running", IsError: true}
				}
			}
			c.showExec = true
			c.showDetails = false
			c.showLogs = false
			c.showFileTree = false
			c.execOutput = ""
			c.execInput.Focus()
			c.SetSize(c.width, c.height)
			return tea.Batch(textinput.Blink, c.startExecSession(container.ID))
		case "D":
			return c.deleteContainerCmd()
		case "s":
			return c.toggleContainerCmd()
		case "R":
			return c.restartContainerCmd()
		case "up", "down":
			var listCmd tea.Cmd
			c.list, listCmd = c.list.Update(msg)
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

	var tiCmd tea.Cmd
	c.execInput, tiCmd = c.execInput.Update(msg)
	cmds = append(cmds, tiCmd)

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
	if !c.showDetails && !c.showLogs && !c.showFileTree && !c.showExec {
		return listView
	}

	c.updateDetails()

	var detailContent string
	if c.showExec {
		vpView := c.viewport.View()
		inputView := c.execInput.View()
		detailContent = lipgloss.JoinVertical(lipgloss.Left, vpView, inputView)
	} else {
		detailContent = c.viewport.View()
	}

	detailView := listStyle.
		Width(c.viewport.Width).
		Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

func (c *ContainerList) deleteContainerCmd() tea.Cmd {
	svc := c.service
	items := c.list.Items()
	idx := c.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	containerItem, ok := items[idx].(ContainerItem)
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
	idx := c.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	containerItem, ok := items[idx].(ContainerItem)
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
	idx := c.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	containerItem, ok := items[idx].(ContainerItem)
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

func (c *ContainerList) fetchFileTreeInformation(containerID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		fileTree, err := c.service.FileTree(ctx, containerID)
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

	if c.showExec {
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

func (c *ContainerList) startExecSession(containerID string) tea.Cmd {
	svc := c.service
	return func() tea.Msg {
		ctx := context.Background()
		session, err := svc.Exec(ctx, containerID)
		if err != nil {
			return execOutputMsg{err: err}
		}
		return execSessionStartedMsg{session: session}
	}
}

func (c *ContainerList) closeExec() {
	c.showExec = false
	c.execInput.Blur()
	if c.execSession != nil {
		c.execSession.Close()
		c.execSession = nil
	}
	c.viewport.SetContent("")
	c.execHistoryCurrentIndex = 0
	c.execHistory = []string{}
	c.execOutput = ""
}

func (c *ContainerList) readExecOutput() tea.Cmd {
	session := c.execSession
	if session == nil {
		return nil
	}
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, err := session.Reader.Read(buf)
		if err != nil {
			return execOutputMsg{err: err}
		}
		return execOutputMsg{output: string(buf[:n])}
	}
}

func (c *ContainerList) closeExecSessionMsg() tea.Cmd {
	return func() tea.Msg {
		return execCloseMsg{}
	}
}
