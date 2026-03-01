package containers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types/container"

	"github.com/NimbleMarkets/ntcharts/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/linechart/streamlinechart"

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

// containersTreeLoadedMsg is sent when containers have been loaded asynchronously.
type containersTreeLoadedMsg struct {
	error    error
	fileTree client.ContainerFileTree
}

// containerActionMsg is sent when a container action completes.
type containerActionMsg struct {
	ID     string
	Action string
	Idx    int
	Error  error
}

// execOutputMsg is sent when exec output is received from the background reader.
type execOutputMsg struct {
	output string
	err    error
}

type execSessionStartedMsg struct {
	session *client.ExecSession
}

// statsOutputMsg is sent when stats output is received.
type statsOutputMsg struct {
	cpuPercetange    float64
	memoryPercentage float64
	memoryUsage      float64
	memoryLimit      float64
	networkRead      float64
	netwrokWrite     float64
	ioRead           float64
	ioWrite          float64
	err              error
}

type statsSessionStartedMsg struct {
	session *client.StatsSession
}

type execCloseMsg struct{}

type detailsMsg struct {
	output string
	err    error
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
	listSplitRatio   = 0.4   // fraction of width used by list in split view
	readBufSize      = 4096  // buffer size for reading container output
	cpuPercentFactor = 100.0 // multiplier to convert CPU ratio to percentage
	chartHalves      = 2     // divisor to split chart space in half
	netIOChartLines  = 2     // lines reserved for net/io chart label and legend
)

// List wraps bubbles/list for displaying containers.
type List struct {
	list                    list.Model
	isFilter                bool
	viewport                viewport.Model
	service                 client.ContainerService
	width, height           int
	activePanel             panel.Panel
	logsPanel               panel.Panel
	showDetails             bool
	showFileTree            bool
	loading                 bool
	spinner                 spinner.Model
	showExec                bool
	execSession             *client.ExecSession
	execInput               textinput.Model
	execHistory             []string
	execHistoryCurrentIndex int
	execOutput              string
	showStats               bool
	statsSession            *client.StatsSession
	cpuStreamlinechart      streamlinechart.Model
	memStreamlinechart      streamlinechart.Model
	networkStreamlinechart  streamlinechart.Model
	ioStreamlinechart       streamlinechart.Model
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

	ti := textinput.New()
	ti.Prompt = "$ "

	cl := &List{
		list:        l,
		viewport:    vp,
		service:     svc,
		spinner:     sp,
		logsPanel:   NewLogsPanel(svc),
		execInput:   ti,
		execHistory: []string{},
		cpuStreamlinechart: streamlinechart.New(
			1,
			1,
			streamlinechart.WithStyles(runes.ArcLineStyle, lipgloss.NewStyle().Foreground(theme.DockerBlue)),
		),
		memStreamlinechart: streamlinechart.New(
			1,
			1,
			streamlinechart.WithStyles(runes.ArcLineStyle, lipgloss.NewStyle().Foreground(theme.StatusRunning)),
		),
		networkStreamlinechart: streamlinechart.New(
			1,
			1,
			streamlinechart.WithStyles(runes.ArcLineStyle, lipgloss.NewStyle().Foreground(theme.StatusRunning)),
			streamlinechart.WithDataSetStyles(
				"write",
				runes.ArcLineStyle,
				lipgloss.NewStyle().Foreground(theme.StatusPaused),
			),
		),
		ioStreamlinechart: streamlinechart.New(
			1,
			1,
			streamlinechart.WithStyles(runes.ArcLineStyle, lipgloss.NewStyle().Foreground(theme.DockerBlue)),
			streamlinechart.WithDataSetStyles(
				"write",
				runes.ArcLineStyle,
				lipgloss.NewStyle().Foreground(theme.StatusError),
			),
		),
	}

	return cl
}

// SetSize sets dimensions.
func (c *List) SetSize(width, height int) {
	c.width = width
	c.height = height

	// Account for padding and borders
	listX, listY := theme.ListStyle.GetFrameSize()

	if c.showDetails || c.showFileTree || c.showExec || c.showStats || c.activePanel != nil {
		// Split view: 40% list, 60% details
		listWidth := int(float64(width) * listSplitRatio)
		detailWidth := width - listWidth

		c.list.SetSize(listWidth-listX, height-listY)
		c.viewport.Width = detailWidth - listX
		if c.showExec {
			c.viewport.Height = height - listY - 1
		} else {
			c.viewport.Height = height - listY
		}
		chartWidth := (detailWidth - listX) / chartHalves
		cpuMemChartHeight := (height-listY)/chartHalves - 1              // subtract 1 for the label line
		netIOChartHeight := (height-listY)/chartHalves - netIOChartLines // subtract 2 for label + legend line
		c.cpuStreamlinechart.Resize(chartWidth, cpuMemChartHeight)
		c.memStreamlinechart.Resize(chartWidth, cpuMemChartHeight)
		c.networkStreamlinechart.Resize(chartWidth, netIOChartHeight)
		c.ioStreamlinechart.Resize(chartWidth, netIOChartHeight)
		if c.activePanel != nil {
			c.activePanel.SetSize(width, height)
		}
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
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: msg.error.Error(),
					IsError: true,
				}
			}
		}
		c.viewport.SetContent(lipgloss.NewStyle().Width(c.viewport.Width).Render(msg.fileTree.Tree.String()))
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
	case execSessionStartedMsg:
		c.execSession = msg.session
		return c.readExecOutput()
	case execOutputMsg:
		if msg.err != nil {
			if c.execSession == nil {
				return nil // session was closed manually, ignore
			}
			c.closeExec()
			return func() tea.Msg {
				return message.ShowBannerMsg{Message: fmt.Sprintf("Exec session error. Err: %s", msg.err), IsError: true}
			}
		}
		c.execOutput += msg.output
		c.viewport.SetContent(lipgloss.NewStyle().Width(c.viewport.Width).Render(c.execOutput))
		c.viewport.GotoBottom()
		return c.readExecOutput()
	case statsSessionStartedMsg:
		c.loading = false
		c.statsSession = msg.session
		return c.readStatsOutput()
	case statsOutputMsg:
		if msg.err != nil {
			if c.statsSession == nil {
				return nil // session was closed manually, ignore
			}
			c.closeStatsSession()
			return func() tea.Msg {
				return message.ShowBannerMsg{Message: fmt.Sprintf("Stats session error. Err: %s", msg.err), IsError: true}
			}
		}
		c.cpuStreamlinechart.Push(msg.cpuPercetange)
		c.cpuStreamlinechart.Draw()
		c.memStreamlinechart.Push(msg.memoryPercentage)
		c.memStreamlinechart.Draw()
		c.networkStreamlinechart.Push(msg.networkRead)
		c.networkStreamlinechart.PushDataSet("write", msg.netwrokWrite)
		c.networkStreamlinechart.DrawAll()
		c.ioStreamlinechart.Push(msg.ioRead)
		c.ioStreamlinechart.PushDataSet("write", msg.ioWrite)
		c.ioStreamlinechart.DrawAll()

		cpuLabel := fmt.Sprintf("CPU %.2f%%", msg.cpuPercetange)
		memLabel := fmt.Sprintf("MEM %.2f%% (%s / %s)", msg.memoryPercentage, formatBytes(uint64(msg.memoryUsage)), formatBytes(uint64(msg.memoryLimit)))
		netLabel := fmt.Sprintf("NET  rx:%s tx:%s", formatBytes(uint64(msg.networkRead)), formatBytes(uint64(msg.netwrokWrite)))
		ioLabel := fmt.Sprintf("I/O  r:%s w:%s", formatBytes(uint64(msg.ioRead)), formatBytes(uint64(msg.ioWrite)))

		netReadLegend := lipgloss.NewStyle().Foreground(theme.StatusRunning).Render("● read")
		netWriteLegend := lipgloss.NewStyle().Foreground(theme.StatusPaused).Render("● write")
		netLegend := netReadLegend + "  " + netWriteLegend

		ioReadLegend := lipgloss.NewStyle().Foreground(theme.DockerBlue).Render("● read")
		ioWriteLegend := lipgloss.NewStyle().Foreground(theme.StatusError).Render("● write")
		ioLegend := ioReadLegend + "  " + ioWriteLegend

		row1 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left, cpuLabel, c.cpuStreamlinechart.View()),
			lipgloss.JoinVertical(lipgloss.Left, memLabel, c.memStreamlinechart.View()),
		)
		row2 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left, netLabel, netLegend, c.networkStreamlinechart.View()),
			lipgloss.JoinVertical(lipgloss.Left, ioLabel, ioLegend, c.ioStreamlinechart.View()),
		)
		combined := lipgloss.JoinVertical(lipgloss.Left, row1, row2)
		c.viewport.SetContent(lipgloss.NewStyle().Width(c.viewport.Width).Render(combined))
		return c.readStatsOutput()
	case detailsMsg:
		if msg.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{Message: msg.err.Error(), IsError: true}
			}
		}
		c.viewport.SetContent(lipgloss.NewStyle().Width(c.viewport.Width).Render(msg.output))
		return nil
	case execCloseMsg:
		c.closeExec()
		return func() tea.Msg {
			return message.ShowBannerMsg{Message: "Exec session closed", IsError: false}
		}
	case tea.KeyMsg:
		// When exec is active, intercept all keys for the text input
		if c.showExec {
			return c.handleExecInut(msg)
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
			c.showDetails = !c.showDetails
			if c.showDetails {
				return c.detailsCmd()
			}
			c.clearViewPort()
			return nil
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
			c.showFileTree = !c.showFileTree
			if c.showFileTree {
				selected := c.list.SelectedItem()
				if selected == nil {
					return nil
				}
				cItem, ok := selected.(containerItem)
				if !ok {
					return nil
				}
				c.loading = true
				return tea.Batch(c.spinner.Tick, c.fetchFileTreeInformation(cItem.container.ID))
			}
			c.clearViewPort()
			return nil
		case key.Matches(msg, keys.Keys.ContainerStats):
			c.showStats = !c.showStats
			if c.showStats {
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
				c.loading = true
				return c.startStatsSession(cItem.container.ID)
			}
			c.closeStatsSession()
			return nil
		case key.Matches(msg, keys.Keys.ContainerExec):
			c.showExec = !c.showExec
			if c.showExec {
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

				c.execOutput = ""
				c.execInput.Focus()
				return tea.Batch(textinput.Blink, c.startExecSession(cItem.container.ID), c.extendExecHelpCommand())
			}
			c.clearViewPort()
			return nil
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

	// Handle Blink Cmd
	if c.showExec {
		var inputCmd tea.Cmd
		c.execInput, inputCmd = c.execInput.Update(msg)
		cmds = append(cmds, inputCmd)
	}

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

	// Only show viewport when details are toggled on
	if c.activePanel == nil && !c.showDetails && !c.showFileTree && !c.showExec && !c.showStats {
		return listView
	}

	detailContent := c.viewport.View()

	if c.activePanel != nil {
		detailContent = c.activePanel.View()
		c.viewport.SetContent(detailContent)
	}
	if c.showExec {
		inputView := c.execInput.View()
		detailContent = lipgloss.JoinVertical(lipgloss.Left, detailContent, inputView)
	}

	detailView := theme.ListStyle.
		Width(c.viewport.Width).
		Height(c.viewport.Height).
		Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

func (c *List) handleExecInut(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Keys.Esc):
		return tea.Batch(c.closeExecSessionMsg(), func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} })
	case key.Matches(msg, keys.Keys.Enter):
		if c.execSession == nil {
			return nil
		}
		cmd := c.execInput.Value()
		if cmd == "" {
			return nil
		}

		if strings.TrimSpace(cmd) == "clear" {
			c.execInput.Reset()
			c.execOutput = ""
			c.clearViewPort()
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
	case key.Matches(msg, keys.Keys.Up):
		if len(c.execHistory) == 0 {
			return nil
		}
		switch {
		case c.execHistoryCurrentIndex > 0:
			c.execHistoryCurrentIndex--
		case c.execHistoryCurrentIndex == len(c.execHistory):
			// Start browsing from the most recent entry
			c.execHistoryCurrentIndex = len(c.execHistory) - 1
		default:
			// Already at oldest entry
			return nil
		}
		c.execInput.SetValue(c.execHistory[c.execHistoryCurrentIndex])
		return nil
	case key.Matches(msg, keys.Keys.Down):
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

func (c *List) clearViewPort() {
	c.viewport.SetContent("")
}

func (c *List) clearDetails() {
	c.showDetails = false
	c.showFileTree = false
	c.showExec = false
	c.showStats = false
	if c.activePanel != nil {
		c.activePanel.Close()
		c.activePanel = nil
	}
	c.execOutput = ""
	c.clearViewPort()
}

func (c *List) detailsCmd() tea.Cmd {
	return func() tea.Msg {
		selected := c.list.SelectedItem()
		if selected == nil {
			return detailsMsg{
				err: errors.New("no container selected"),
			}
		}
		cItem, ok := selected.(containerItem)
		if !ok {
			return detailsMsg{err: errors.New("no container selected")}
		}
		container := cItem.container
		var content strings.Builder

		// Header
		fmt.Fprintf(&content, "Container: %s\n", container.Name)
		content.WriteString("═══════════════════════\n\n")

		// Basic info
		fmt.Fprintf(&content, "ID:      %s\n", helpers.ShortID(container.ID))
		fmt.Fprintf(&content, "Image:   %s\n", container.Image)
		fmt.Fprintf(&content, "Status:  %s\n", container.Status)

		stateStyle := theme.GetContainerStatusStyle(string(container.State))
		stateIcon := theme.GetContainerStatusIcon(string(container.State))
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
		return detailsMsg{
			output: content.String(),
		}
	}
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

func (c *List) fetchFileTreeInformation(containerID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		fileTree, err := c.service.FileTree(ctx, containerID)
		if err != nil {
			return containersTreeLoadedMsg{error: fmt.Errorf("error getting the file tree: %s", err.Error())}
		}
		return containersTreeLoadedMsg{fileTree: fileTree}
	}
}

func (c *List) startExecSession(containerID string) tea.Cmd {
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

func (c *List) startStatsSession(containerID string) tea.Cmd {
	svc := c.service
	return func() tea.Msg {
		ctx := context.Background()
		session, err := svc.Stats(ctx, containerID)
		if err != nil {
			return statsOutputMsg{err: err}
		}
		return statsSessionStartedMsg{session: session}
	}
}

func (c *List) closeStatsSession() {
	c.showStats = false
	if c.statsSession != nil {
		c.statsSession.Close()
		c.statsSession = nil
	}
	c.clearViewPort()
}

func (c *List) extendExecHelpCommand() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "exit"),
			),
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "send command"),
			),
			key.NewBinding(
				key.WithKeys("up"),
				key.WithHelp("↑", "history up"),
			),
			key.NewBinding(
				key.WithKeys("down"),
				key.WithHelp("↓", "history down"),
			),
		}}
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

func (c *List) closeExec() {
	c.showExec = false
	c.execInput.Blur()
	if c.execSession != nil {
		c.execSession.Close()
		c.execSession = nil
	}
	c.clearViewPort()
	c.execHistoryCurrentIndex = 0
	c.execHistory = []string{}
	c.execOutput = ""
}

func (c *List) readExecOutput() tea.Cmd {
	session := c.execSession
	if session == nil {
		return nil
	}
	return func() tea.Msg {
		buf := make([]byte, readBufSize)
		n, err := session.Reader.Read(buf)
		if err != nil {
			return execOutputMsg{err: err}
		}
		return execOutputMsg{output: string(buf[:n])}
	}
}

func (c *List) readStatsOutput() tea.Cmd {
	session := c.statsSession
	if session == nil {
		return nil
	}
	return func() tea.Msg {
		buf := make([]byte, readBufSize)
		n, err := session.Reader.Read(buf)
		if err != nil {
			return statsOutputMsg{err: err}
		}
		var stats container.StatsResponse
		err = json.Unmarshal(buf[:n], &stats)
		if err != nil {
			return statsOutputMsg{err: err}
		}
		cpuDelta := stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage
		systemCPUDelta := stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage
		numberCpus := stats.CPUStats.OnlineCPUs

		percentageCPUUsage := float64(cpuDelta) / float64(systemCPUDelta) * float64(numberCpus) * cpuPercentFactor

		memUsage := float64(stats.MemoryStats.Usage) - float64(stats.MemoryStats.Stats["cache"])
		memLimit := float64(stats.MemoryStats.Limit)
		var memPercentage float64
		if memLimit > 0 {
			memPercentage = memUsage / memLimit * cpuPercentFactor
		}

		networkRead := float64(0.0)
		networkWrite := float64(0.0)

		for _, stat := range stats.Networks {
			networkRead += float64(stat.RxBytes)
			networkWrite += float64(stat.TxBytes)
		}

		ioRead := float64(0.0)
		ioWrite := float64(0.0)

		for _, stat := range stats.BlkioStats.IoServiceBytesRecursive {
			if stat.Op == "read" {
				ioRead += float64(stat.Value)
			}

			if stat.Op == "write" {
				ioWrite += float64(stat.Value)
			}
		}

		return statsOutputMsg{
			cpuPercetange:    percentageCPUUsage,
			memoryPercentage: memPercentage,
			memoryUsage:      memUsage,
			memoryLimit:      memLimit,
			networkRead:      networkRead,
			netwrokWrite:     networkWrite,
			ioRead:           ioRead,
			ioWrite:          ioWrite,
		}
	}
}

func (c *List) closeExecSessionMsg() tea.Cmd {
	return func() tea.Msg {
		return execCloseMsg{}
	}
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
