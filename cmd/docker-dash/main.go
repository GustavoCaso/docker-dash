package main

import (
	"context"
	"fmt"
	"os"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/components"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	client    service.DockerClient
	imageList *components.ImageList
	width     int
	height    int
}

func initialModel(client service.DockerClient) model {
	ctx := context.Background()
	images, _ := client.Images().List(ctx)
	imageList := components.NewImageList(images)

	return model{
		client:    client,
		imageList: imageList,
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
		m.imageList.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	// Forward all messages to the image list
	var cmd tea.Cmd
	cmd = m.imageList.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	return m.imageList.View()
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
