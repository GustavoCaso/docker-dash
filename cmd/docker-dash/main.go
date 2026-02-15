package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/components"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
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
	statusBar     *components.StatusBar
	focus         focus
	width         int
	height        int
	bannerMsg     string
	bannerKind    bannerType
}

// Key bindings for sidebar
var tabKey = key.NewBinding(
	key.WithKeys("tab", "shift+tab"),
	key.WithHelp("tab", "change focus"),
)

var sidebarNavKey = key.NewBinding(
	key.WithKeys("up", "down"),
	key.WithHelp("up/down", "navigate"),
)

// KeyBindings returns the key bindings for sidebar focus
var sidebarBindings = []key.Binding{sidebarNavKey, tabKey}

func initialModel(client service.DockerClient) model {
	ctx := context.Background()
	containers, _ := client.Containers().List(ctx)
	images, _ := client.Images().List(ctx)

	return model{
		client:        client,
		sidebar:       components.NewSidebar(),
		containerList: components.NewContainerList(containers, client.Containers()),
		imageList:     components.NewImageList(images, client),
		statusBar:     components.NewStatusBar(),
		focus:         focusSidebar,
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

		// Reserve space for status bar
		statusBarHeight := lipgloss.Height(m.statusBar.View())
		contentHeight := msg.Height - statusBarHeight

		sidebarWidth := 24
		listWidth := msg.Width - sidebarWidth

		m.sidebar.SetSize(sidebarWidth, contentHeight)
		m.containerList.SetSize(listWidth, contentHeight)
		m.imageList.SetSize(listWidth, contentHeight)
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

	case clearBannerMsg:
		m.bannerMsg = ""
		m.bannerKind = bannerNone
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "shift+tab":
			// Toggle focus
			if m.focus == focusSidebar {
				m.focus = focusList
			} else {
				m.focus = focusSidebar
			}
			return m, nil
		case "up":
			// Navigate sidebar when focused
			if m.focus == focusSidebar {
				m.sidebar.MoveUp()
				return m, nil
			}
		case "down":
			// Navigate sidebar when focused
			if m.focus == focusSidebar {
				m.sidebar.MoveDown()
				return m, nil
			}
		}
	}

	// Forward messages to focused component
	var cmd tea.Cmd
	if m.focus == focusList {
		switch m.sidebar.ActiveView() {
		case components.ViewContainers:
			cmd = m.containerList.Update(msg)
		case components.ViewImages:
			cmd = m.imageList.Update(msg)
		}
	}
	return m, cmd
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Mark which component is focused
	m.sidebar.SetFocused(m.focus == focusSidebar)

	// Get the active list view and key bindings based on the active view
	var listView string
	var listBindings []key.Binding

	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		listView = m.containerList.View()
		listBindings = m.containerList.KeyBindings()
	case components.ViewImages:
		listView = m.imageList.View()
		listBindings = m.imageList.KeyBindings()
	}

	// Set status bar bindings based on focused component
	if m.focus == focusList {
		m.statusBar.SetBindings(listBindings)
	} else {
		m.statusBar.SetBindings(sidebarBindings)
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

func main() {
	var dockerClient service.DockerClient
	var err error

	realClient, err := service.NewLocalDockerClient()
	if err == nil {
		if pingErr := realClient.Ping(context.Background()); pingErr == nil {
			dockerClient = realClient
		} else {
			realClient.Close()
		}
	}

	if dockerClient == nil {
		dockerClient = service.NewMockClient()
	}
	defer dockerClient.Close()

	p := tea.NewProgram(
		initialModel(dockerClient),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
