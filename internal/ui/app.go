package ui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/components"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

type bannerType int

const (
	bannerNone bannerType = iota
	bannerSuccess
	bannerError
)

const (
	bannerTimeoutSecs  = 3 // seconds before banner auto-clears
	bannerOverlayLines = 2 // lines from bottom for banner overlay position
)

// clearBannerMsg is sent to clear the banner after a timeout.
type clearBannerMsg struct{}

// autoRefreshMsg is sent periodically to refresh all views when a refresh interval is configured.
type autoRefreshMsg struct{}

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
	cfg             *config.Config
	client          service.DockerClient
	header          *components.Header
	containerList   *components.ContainerList
	imageList       *components.ImageList
	volumeList      *components.VolumeList
	statusBar       *components.StatusBar
	keys            *keys.KeyMap
	imagesKeys      *keys.ViewKeyMap
	containerKeys   *keys.ViewKeyMap
	volumeKeys      *keys.ViewKeyMap
	activeKeys      *keys.ViewKeyMap
	width           int
	height          int
	bannerMsg       string
	bannerKind      bannerType
	initErr         string
	refreshInterval time.Duration
}

func InitialModel(cfg *config.Config, client service.DockerClient) tea.Model {
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

	var refreshInterval time.Duration
	if cfg.Refresh.Interval != "" {
		d, err := time.ParseDuration(cfg.Refresh.Interval)
		if err != nil {
			initErr = "Invalid refresh interval: " + err.Error()
		} else {
			refreshInterval = d
		}
	}

	return &model{
		cfg:             cfg,
		client:          client,
		keys:            keys.Keys,
		imagesKeys:      keys.Keys.ImageKeyMap(),
		containerKeys:   keys.Keys.ContainerKeyMap(),
		volumeKeys:      keys.Keys.VolumeKeyMap(),
		header:          components.NewHeader(),
		containerList:   components.NewContainerList(containers, client.Containers()),
		imageList:       components.NewImageList(images, client),
		volumeList:      components.NewVolumeList(volumes, client.Volumes()),
		statusBar:       components.NewStatusBar(),
		initErr:         initErr,
		refreshInterval: refreshInterval,
	}
}

func (m *model) Init() tea.Cmd {
	if m.initErr != "" {
		return func() tea.Msg {
			return message.ShowBannerMsg{Message: m.initErr, IsError: true}
		}
	}

	if m.refreshInterval > 0 {
		return tea.Tick(m.refreshInterval, func(_ time.Time) tea.Msg {
			return autoRefreshMsg{}
		})
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

		// Reserve space for header
		m.header.SetWidth(msg.Width)
		headerHeight := lipgloss.Height(m.header.View())

		contentHeight := msg.Height - statusBarHeight - headerHeight

		m.containerList.SetSize(msg.Width, contentHeight)
		m.imageList.SetSize(msg.Width, contentHeight)
		m.volumeList.SetSize(msg.Width, contentHeight)
		m.statusBar.SetSize(msg.Width, statusBarHeight)

	case message.ShowBannerMsg:
		m.bannerMsg = msg.Message
		if msg.IsError {
			m.bannerKind = bannerError
		} else {
			m.bannerKind = bannerSuccess
		}
		return m, tea.Tick(bannerTimeoutSecs*time.Second, func(_ time.Time) tea.Msg {
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

	case autoRefreshMsg:
		_, cmd := m.forwardMessageToAll(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
		return m, tea.Batch(cmd, tea.Tick(m.refreshInterval, func(_ time.Time) tea.Msg {
			return autoRefreshMsg{}
		}))

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
		case key.Matches(msg, m.keys.Left):
			m.header.MoveLeft()
			return m, nil
		case key.Matches(msg, m.keys.Right):
			m.header.MoveRight()
			return m, nil
		case key.Matches(msg, m.keys.Refresh):
			return m.forwardMessageToActive(msg)
		case key.Matches(msg, m.keys.RefreshAll):
			return m.forwardMessageToAll(tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'r'},
			})
		case key.Matches(msg, m.keys.Help):
			m.statusBar.ToggleFullView()
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

	// Forward other messages to all components
	return m.forwardMessageToAll(msg)
}

func (m *model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var listView string
	var listKeyMap *keys.ViewKeyMap

	switch m.header.ActiveView() {
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

	m.activeKeys = listKeyMap
	m.statusBar.SetKeyMap(listKeyMap)

	content := lipgloss.JoinVertical(lipgloss.Left, m.header.View(), listView)

	if m.bannerMsg != "" {
		var style lipgloss.Style
		if m.bannerKind == bannerError {
			style = bannerErrorStyle
		} else {
			style = bannerSuccessStyle
		}
		bannerText := style.Render(m.bannerMsg)
		content = helper.OverlayBottomRight(bannerOverlayLines, content, bannerText, m.width)
	}

	return lipgloss.JoinVertical(lipgloss.Left, content, m.statusBar.View())
}

func (m *model) forwardMessageToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.header.ActiveView() {
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
