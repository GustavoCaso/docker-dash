package main

import (
	"context"
	"fmt"
	"os"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/components"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FocusZone represents which UI zone has focus
type FocusZone int

const (
	FocusSidebar FocusZone = iota
	FocusList
	FocusActions
)

type model struct {
	client         service.DockerClient
	sidebar        *components.Sidebar
	containerList  *components.ContainerList
	imageList      *components.ImageList
	volumeList     *components.VolumeList
	focusZone      FocusZone
	width          int
	height         int
	err            error
}

func initialModel(client service.DockerClient) model {
	ctx := context.Background()

	// Load initial data
	containers, _ := client.Containers().List(ctx)
	images, _ := client.Images().List(ctx)
	volumes, _ := client.Volumes().List(ctx)

	sidebar := components.NewSidebar()
	sidebar.SetFocused(true)

	containerList := components.NewContainerList(containers)
	imageList := components.NewImageList(images)
	volumeList := components.NewVolumeList(volumes)

	return model{
		client:        client,
		sidebar:       sidebar,
		containerList: containerList,
		imageList:     imageList,
		volumeList:    volumeList,
		focusZone:     FocusSidebar,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		return m, nil

	case refreshMsg:
		ctx := context.Background()
		containers, _ := m.client.Containers().List(ctx)
		images, _ := m.client.Images().List(ctx)
		volumes, _ := m.client.Volumes().List(ctx)

		m.containerList.SetContainers(containers)
		m.imageList.SetImages(images)
		m.volumeList.SetVolumes(volumes)
		return m, nil

	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.cycleFocusForward()
			return m, nil
		case "shift+tab":
			m.cycleFocusBackward()
			return m, nil
		case "?":
			// TODO: Show help overlay
			return m, nil
		case "r":
			return m, m.refresh()
		}

		// Zone-specific keys
		switch m.focusZone {
		case FocusSidebar:
			return m.updateSidebar(msg)
		case FocusList:
			return m.updateList(msg)
		case FocusActions:
			return m.updateActions(msg)
		}
	}

	return m, nil
}

func (m *model) updateSizes() {
	sidebarWidth := 16
	mainWidth := m.width - sidebarWidth - 2
	contentHeight := m.height - 3 // Status bar

	m.sidebar.SetHeight(contentHeight)
	m.containerList.SetSize(mainWidth, contentHeight)
	m.imageList.SetSize(mainWidth, contentHeight)
	m.volumeList.SetSize(mainWidth, contentHeight)
}

func (m *model) cycleFocusForward() {
	switch m.focusZone {
	case FocusSidebar:
		m.focusZone = FocusList
		m.sidebar.SetFocused(false)
		m.setListFocused(true)
	case FocusList:
		if m.currentListExpanded() {
			m.focusZone = FocusActions
			m.setListFocused(false)
			m.setActionsFocused(true)
		} else {
			m.focusZone = FocusSidebar
			m.setListFocused(false)
			m.sidebar.SetFocused(true)
		}
	case FocusActions:
		m.focusZone = FocusSidebar
		m.setActionsFocused(false)
		m.sidebar.SetFocused(true)
	}
}

func (m *model) cycleFocusBackward() {
	switch m.focusZone {
	case FocusSidebar:
		if m.currentListExpanded() {
			m.focusZone = FocusActions
			m.sidebar.SetFocused(false)
			m.setActionsFocused(true)
		} else {
			m.focusZone = FocusList
			m.sidebar.SetFocused(false)
			m.setListFocused(true)
		}
	case FocusList:
		m.focusZone = FocusSidebar
		m.setListFocused(false)
		m.sidebar.SetFocused(true)
	case FocusActions:
		m.focusZone = FocusList
		m.setActionsFocused(false)
		m.setListFocused(true)
	}
}

func (m *model) setListFocused(focused bool) {
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		m.containerList.SetFocused(focused)
	case components.ViewImages:
		m.imageList.SetFocused(focused)
	case components.ViewVolumes:
		m.volumeList.SetFocused(focused)
	}
}

func (m *model) setActionsFocused(focused bool) {
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		m.containerList.SetActionsFocused(focused)
	case components.ViewImages:
		m.imageList.SetActionsFocused(focused)
	case components.ViewVolumes:
		m.volumeList.SetActionsFocused(focused)
	}
}

func (m *model) currentListExpanded() bool {
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		return m.containerList.IsExpanded(m.containerList.SelectedIndex())
	case components.ViewImages:
		return m.imageList.IsExpanded(m.imageList.SelectedIndex())
	case components.ViewVolumes:
		return m.volumeList.IsExpanded(m.volumeList.SelectedIndex())
	}
	return false
}

func (m model) updateSidebar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.sidebar.MoveUp()
	case "down", "j":
		m.sidebar.MoveDown()
	case "enter":
		m.focusZone = FocusList
		m.sidebar.SetFocused(false)
		m.setListFocused(true)
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		return m.updateContainerList(msg)
	case components.ViewImages:
		return m.updateImageList(msg)
	case components.ViewVolumes:
		return m.updateVolumeList(msg)
	}
	return m, nil
}

func (m model) updateContainerList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.containerList.MoveUp()
	case "down", "j":
		m.containerList.MoveDown()
	case "enter":
		m.containerList.ToggleExpand()
	case "l": // Quick key: logs
		// TODO: Open log viewer
	case "s": // Quick key: start/stop
		return m, m.toggleContainer()
	case "x": // Quick key: exec
		// TODO: Open shell
	}
	return m, nil
}

func (m model) updateImageList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.imageList.MoveUp()
	case "down", "j":
		m.imageList.MoveDown()
	case "enter":
		m.imageList.ToggleExpand()
	}
	return m, nil
}

func (m model) updateVolumeList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.volumeList.MoveUp()
	case "down", "j":
		m.volumeList.MoveDown()
	case "enter":
		m.volumeList.ToggleExpand()
	}
	return m, nil
}

func (m model) updateActions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		switch msg.String() {
		case "left", "h":
			m.containerList.MoveActionLeft()
		case "right", "l":
			m.containerList.MoveActionRight()
		case "enter":
			return m, m.executeContainerAction()
		}
	case components.ViewImages:
		switch msg.String() {
		case "left", "h":
			m.imageList.MoveActionLeft()
		case "right", "l":
			m.imageList.MoveActionRight()
		case "enter":
			return m, m.executeImageAction()
		}
	case components.ViewVolumes:
		switch msg.String() {
		case "left", "h":
			m.volumeList.MoveActionLeft()
		case "right", "l":
			m.volumeList.MoveActionRight()
		case "enter":
			return m, m.executeVolumeAction()
		}
	}
	return m, nil
}

func (m model) toggleContainer() tea.Cmd {
	c := m.containerList.SelectedContainer()
	if c == nil {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		if c.State == service.StateRunning {
			m.client.Containers().Stop(ctx, c.ID)
		} else {
			m.client.Containers().Start(ctx, c.ID)
		}
		return refreshMsg{}
	}
}

// Container actions: Running - ["Logs", "Shell", "Stop", "Restart", "Remove"]
// Container actions: Stopped - ["Start", "Remove"]
var containerActionsRunning = []string{"Logs", "Shell", "Stop", "Restart", "Remove"}
var containerActionsStopped = []string{"Start", "Remove"}

func (m model) executeContainerAction() tea.Cmd {
	actionIndex := m.containerList.SelectedAction()
	c := m.containerList.SelectedContainer()
	if c == nil {
		return nil
	}

	// Determine the action name based on container state
	var actionName string
	if c.State == service.StateRunning {
		if actionIndex < len(containerActionsRunning) {
			actionName = containerActionsRunning[actionIndex]
		}
	} else {
		if actionIndex < len(containerActionsStopped) {
			actionName = containerActionsStopped[actionIndex]
		}
	}

	return func() tea.Msg {
		ctx := context.Background()
		switch actionName {
		case "Start":
			m.client.Containers().Start(ctx, c.ID)
		case "Stop":
			m.client.Containers().Stop(ctx, c.ID)
		case "Restart":
			m.client.Containers().Restart(ctx, c.ID)
		case "Remove":
			m.client.Containers().Remove(ctx, c.ID, false)
		case "Logs":
			// TODO: Open log viewer
		case "Shell":
			// TODO: Open shell
		}
		return refreshMsg{}
	}
}

// Image actions: ["Inspect", "Remove"]
var imageActions = []string{"Inspect", "Remove"}

func (m model) executeImageAction() tea.Cmd {
	actionIndex := m.imageList.SelectedAction()
	img := m.imageList.SelectedImage()
	if img == nil {
		return nil
	}

	var actionName string
	if actionIndex < len(imageActions) {
		actionName = imageActions[actionIndex]
	}

	return func() tea.Msg {
		ctx := context.Background()
		switch actionName {
		case "Remove":
			m.client.Images().Remove(ctx, img.ID, false)
		case "Inspect":
			// TODO: Show inspect details
		}
		return refreshMsg{}
	}
}

// Volume actions: ["Browse", "Inspect", "Remove"]
var volumeActions = []string{"Browse", "Inspect", "Remove"}

func (m model) executeVolumeAction() tea.Cmd {
	actionIndex := m.volumeList.SelectedAction()
	vol := m.volumeList.SelectedVolume()
	if vol == nil {
		return nil
	}

	var actionName string
	if actionIndex < len(volumeActions) {
		actionName = volumeActions[actionIndex]
	}

	return func() tea.Msg {
		ctx := context.Background()
		switch actionName {
		case "Remove":
			m.client.Volumes().Remove(ctx, vol.Name, false)
		case "Browse":
			// TODO: Open file browser
		case "Inspect":
			// TODO: Show inspect details
		}
		return refreshMsg{}
	}
}

type refreshMsg struct{}

func (m model) refresh() tea.Cmd {
	return func() tea.Msg {
		return refreshMsg{}
	}
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Sidebar
	sidebar := m.sidebar.View()

	// Main content
	var mainContent string
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		mainContent = m.containerList.View()
	case components.ViewImages:
		mainContent = m.imageList.View()
	case components.ViewVolumes:
		mainContent = m.volumeList.View()
	}

	mainPanel := theme.MainPanelStyle.Render(mainContent)

	// Layout
	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainPanel)

	// Status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, content, statusBar)
}

func (m model) renderStatusBar() string {
	var hints string
	switch m.focusZone {
	case FocusSidebar:
		hints = "↑↓ navigate • Enter select • Tab switch focus • q quit"
	case FocusList:
		hints = "↑↓ navigate • Enter expand • Tab switch focus • ? help"
	case FocusActions:
		hints = "←→ select action • Enter execute • Tab switch focus"
	}

	style := theme.StatusBarStyle.Width(m.width)
	return style.Render(hints)
}

func main() {
	// Use mock client for now
	client := service.NewMockClient()
	defer client.Close()

	p := tea.NewProgram(
		initialModel(client),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
