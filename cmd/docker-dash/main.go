package main

import (
	"context"
	"fmt"
	"os"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type focus int

const (
	focusSidebar focus = iota
	focusList
)

type model struct {
	client    service.DockerClient
	sidebar   *components.Sidebar
	imageList *components.ImageList
	focus     focus
	width     int
	height    int
}

func initialModel(client service.DockerClient) model {
	ctx := context.Background()
	images, _ := client.Images().List(ctx)

	return model{
		client:    client,
		sidebar:   components.NewSidebar(),
		imageList: components.NewImageList(images),
		focus:     focusSidebar,
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

		sidebarWidth := 24
		listWidth := msg.Width - sidebarWidth

		m.sidebar.SetSize(sidebarWidth, msg.Height)
		m.imageList.SetSize(listWidth, msg.Height)

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
		}
	}

	// Forward messages to focused component
	var cmd tea.Cmd
	if m.focus == focusList {
		cmd = m.imageList.Update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Mark which component is focused
	m.sidebar.SetFocused(m.focus == focusSidebar)

	sidebar := m.sidebar.View()
	list := m.imageList.View()

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, list)
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
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
