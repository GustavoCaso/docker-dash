package ui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/confirmation"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/header"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/statusbar"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/containers"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/images"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/networks"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/volumes"
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
	cfg              *config.Config
	client           client.Client
	header           *header.Header
	containerSection sections.Section
	imageSection     sections.Section
	volumeSection    sections.Section
	networkSection   sections.Section
	statusBar        *statusbar.StatusBar
	keys             *keys.KeyMap
	imagesKeys       *keys.ViewKeyMap
	containerKeys    *keys.ViewKeyMap
	volumeKeys       *keys.ViewKeyMap
	networkKeys      *keys.ViewKeyMap
	activeKeys       *keys.ViewKeyMap
	width            int
	height           int
	bannerMsg        string
	bannerKind       bannerType
	initErr          string
	confirmation     confirmation.Model
	pendingCmd       tea.Cmd
	showConfirmation bool
	refreshInterval  time.Duration
}

func InitialModel(ctx context.Context, version string, cfg *config.Config, client client.Client) tea.Model {
	containersList, containersErr := client.Containers().List(ctx)
	imagesList, imagesErr := client.Images().List(ctx)
	volumesList, volumesErr := client.Volumes().List(ctx)
	networksList, networksErr := client.Networks().List(ctx)

	var initErr string
	for _, err := range []error{containersErr, imagesErr, volumesErr, networksErr} {
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
		cfg:              cfg,
		client:           client,
		keys:             keys.Keys,
		imagesKeys:       keys.Keys.ImageKeyMap(),
		containerKeys:    keys.Keys.ContainerKeyMap(),
		volumeKeys:       keys.Keys.VolumeKeyMap(),
		networkKeys:      keys.Keys.NetworkKeyMap(),
		header:           header.New(version),
		containerSection: containers.New(ctx, containersList, client.Containers()),
		imageSection:     images.New(ctx, imagesList, client),
		volumeSection:    volumes.New(ctx, volumesList, client.Volumes()),
		networkSection:   networks.New(ctx, networksList, client.Networks()),
		statusBar:        statusbar.New(),
		initErr:          initErr,
		confirmation:     confirmation.New(),
		refreshInterval:  refreshInterval,
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
	if m.showConfirmation {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "y":
				cmd := m.pendingCmd
				m.showConfirmation = false
				m.pendingCmd = nil
				return m, cmd
			case "n", "esc":
				m.showConfirmation = false
				m.pendingCmd = nil
				return m, nil
			}
			// Swallow all other keys while modal is visible
			return m, nil
		}
	}

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

		m.containerSection.SetSize(msg.Width, contentHeight)
		m.imageSection.SetSize(msg.Width, contentHeight)
		m.volumeSection.SetSize(msg.Width, contentHeight)
		m.networkSection.SetSize(msg.Width, contentHeight)
		m.statusBar.SetSize(msg.Width, statusBarHeight)

	case message.ShowConfirmationMsg:
		m.confirmation.Init(msg.Title, msg.Body)
		m.pendingCmd = msg.OnConfirm
		m.showConfirmation = true
		return m, nil

	case message.HideConfirmationMsg:
		m.showConfirmation = false
		m.pendingCmd = nil
		return m, nil

	case message.ShowBannerMsg:
		m.bannerMsg = msg.Message
		if msg.IsError {
			m.bannerKind = bannerError
		} else {
			m.bannerKind = bannerSuccess
		}

		clearTimeout := msg.ClearTimeout
		if clearTimeout <= 0 {
			clearTimeout = bannerTimeoutSecs * time.Second
		}

		return m, tea.Tick(clearTimeout, func(_ time.Time) tea.Msg {
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
			section := m.activeSection()
			m.header.MoveLeft()
			return m, section.Reset()
		case key.Matches(msg, m.keys.Right):
			section := m.activeSection()
			m.header.MoveRight()
			return m, section.Reset()
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

	if m.showConfirmation {
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.confirmation.View(),
		)
	}

	var listView string
	var listKeyMap *keys.ViewKeyMap

	switch m.header.ActiveView() {
	case header.ViewContainers:
		listView = m.containerSection.View()
		listKeyMap = m.containerKeys
	case header.ViewImages:
		listView = m.imageSection.View()
		listKeyMap = m.imagesKeys
	case header.ViewVolumes:
		listView = m.volumeSection.View()
		listKeyMap = m.volumeKeys
	case header.ViewNetworks:
		listView = m.networkSection.View()
		listKeyMap = m.networkKeys
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
	section := m.activeSection()
	cmd := section.Update(msg)

	return m, cmd
}

func (m *model) forwardMessageToAll(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{
		m.containerSection.Update(msg),
		m.imageSection.Update(msg),
		m.volumeSection.Update(msg),
		m.networkSection.Update(msg),
	}
	return m, tea.Batch(cmds...)
}

func (m *model) activeSection() sections.Section {
	var section sections.Section
	switch m.header.ActiveView() {
	case header.ViewContainers:
		section = m.containerSection
	case header.ViewImages:
		section = m.imageSection
	case header.ViewVolumes:
		section = m.volumeSection
	case header.ViewNetworks:
		section = m.networkSection
	}
	return section
}
