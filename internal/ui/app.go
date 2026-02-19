package ui

import (
	"context"
	"time"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/components"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type focus int

const (
	focusSidebar focus = iota
	focusList
)

type bannerType int

const (
	bannerNone bannerType = iota
	bannerSuccess
	bannerError
)

// clearBannerMsg is sent to clear the banner after a timeout
type clearBannerMsg struct{}

var (
	bannerSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("34")).
				Padding(0, 1)
	bannerErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("196")).
				Padding(0, 1)
)

type model struct {
	client        service.DockerClient
	sidebar       *components.Sidebar
	containerList *components.ContainerList
	imageList     *components.ImageList
	volumeList    *components.VolumeList
	statusBar     *components.StatusBar
	keys          *keys.KeyMap
	imagesKeys    *keys.ViewKeyMap
	containerKeys *keys.ViewKeyMap
	volumeKeys    *keys.ViewKeyMap
	activeKeys    *keys.ViewKeyMap
	sidebarKeys   *keys.ViewKeyMap
	focus         focus
	width         int
	height        int
	bannerMsg     string
	bannerKind    bannerType
	initErr       string
}

func InitialModel(client service.DockerClient) tea.Model {
	ctx := context.Background()
	containers, containersErr := client.Containers().List(ctx)
	images, imagesErr := client.Images().List(ctx)
	volumes, volumesErr := client.Volumes().List(ctx)

	var initErr string
	for _, err := range []error{containersErr, imagesErr, volumesErr} {
		if err != nil {
			initErr = "Failed to load data: " + err.Error()
			break
		}
	}

	return &model{
		client:        client,
		keys:          keys.Keys,
		sidebarKeys:   keys.Keys.SidebarKeyMap(),
		imagesKeys:    keys.Keys.ImageKeyMap(),
		containerKeys: keys.Keys.ContainerKeyMap(),
		volumeKeys:    keys.Keys.VolumeKeyMap(),
		sidebar:       components.NewSidebar(),
		containerList: components.NewContainerList(containers, client.Containers()),
		imageList:     components.NewImageList(images, client),
		volumeList:    components.NewVolumeList(volumes, client.Volumes()),
		statusBar:     components.NewStatusBar(),
		focus:         focusSidebar,
		initErr:       initErr,
	}
}

func (m *model) Init() tea.Cmd {
	if m.initErr != "" {
		return func() tea.Msg {
			return message.ShowBannerMsg{Message: m.initErr, IsError: true}
		}
	}
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Reserve space for status bar
		statusBarHeight := 1
		if m.statusBar.IsFullView() {
			statusBarHeight = lipgloss.Height(m.statusBar.View())
		}
		contentHeight := msg.Height - statusBarHeight

		sidebarWidth := 24
		listWidth := msg.Width - sidebarWidth

		m.sidebar.SetSize(sidebarWidth, contentHeight)
		m.containerList.SetSize(listWidth, contentHeight)
		m.imageList.SetSize(listWidth, contentHeight)
		m.volumeList.SetSize(listWidth, contentHeight)
		m.statusBar.SetSize(msg.Width, statusBarHeight)
	case message.ShowBannerMsg:
		m.bannerMsg = msg.Message
		if msg.IsError {
			m.bannerKind = bannerError
		} else {
			m.bannerKind = bannerSuccess
		}
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearBannerMsg{}
		})

	case message.BubbleUpMsg:
		if msg.OnlyActive {
			return m.forwardMessageToActive(msg.KeyMsg)
		}

		return m.forwardMessageToAll(msg.KeyMsg)
	case clearBannerMsg:
		m.bannerMsg = ""
		m.bannerKind = bannerNone
		return m, nil

	case message.AddContextualKeyBindingsMsg:
		m.activeKeys.ToggleContextual(msg.Bindings)
		return m, nil
	case message.ClearContextualKeyBindingsMsg:
		m.activeKeys.DisableContextual()
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.SwitchTab):
			// Toggle focus
			if m.focus == focusSidebar {
				m.focus = focusList
			} else {
				m.focus = focusSidebar
			}
			return m, nil
		case key.Matches(msg, m.keys.Up):
			// Navigate sidebar when focused
			if m.focus == focusSidebar {
				m.sidebar.MoveUp()
				return m, nil
			}
		case key.Matches(msg, m.keys.Down):
			// Navigate sidebar when focused
			if m.focus == focusSidebar {
				m.sidebar.MoveDown()
				return m, nil
			}
		case key.Matches(msg, m.keys.Refresh):
			return m.forwardMessageToActive(msg)
		case key.Matches(msg, m.keys.RefreshAll):
			return m.forwardMessageToAll(tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'r'},
			})
		case key.Matches(msg, m.keys.Help):
			m.statusBar.ToggleFullView()
			// We need to force updating the height of all the components
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{
					Width:  m.width,
					Height: m.height,
				}
			}
		}
	}

	// Forward key messages to focused component only
	if _, ok := msg.(tea.KeyMsg); ok {
		return m.forwardMessageToActive(msg)
	}

	// Forward other messages (async results, spinner ticks, etc.) to all components
	// so each component receives its own internal messages regardless of active view
	return m.forwardMessageToAll(msg)
}

func (m *model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Mark which component is focused
	m.sidebar.SetFocused(m.focus == focusSidebar)

	// Get the active list view and key bindings based on the active view
	var listView string
	var listKeyMap *keys.ViewKeyMap

	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		listView = m.containerList.View()
		listKeyMap = m.containerKeys
	case components.ViewImages:
		listView = m.imageList.View()
		listKeyMap = m.imagesKeys
	case components.ViewVolumes:
		listView = m.volumeList.View()
		listKeyMap = m.volumeKeys
	}

	// Set status bar bindings based on focused component
	if m.focus == focusList {
		m.activeKeys = listKeyMap
		m.statusBar.SetKeyMap(listKeyMap)
	} else {
		m.activeKeys = m.sidebarKeys
		m.statusBar.SetKeyMap(m.sidebarKeys)
	}

	sidebar := m.sidebar.View()

	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, listView)

	// Overlay banner on content area (before status bar)
	if m.bannerMsg != "" {
		var style lipgloss.Style
		if m.bannerKind == bannerError {
			style = bannerErrorStyle
		} else {
			style = bannerSuccessStyle
		}
		bannerText := style.Render(m.bannerMsg)
		content = helper.OverlayBottomRight(2, content, bannerText, m.width)
	}

	return lipgloss.JoinVertical(lipgloss.Left, content, m.statusBar.View())
}

func (m *model) forwardMessageToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		cmd = m.containerList.Update(msg)
	case components.ViewImages:
		cmd = m.imageList.Update(msg)
	case components.ViewVolumes:
		cmd = m.volumeList.Update(msg)
	}

	return m, cmd
}

func (m *model) forwardMessageToAll(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{
		m.containerList.Update(msg),
		m.imageList.Update(msg),
		m.volumeList.Update(msg),
	}
	return m, tea.Batch(cmds...)
}
