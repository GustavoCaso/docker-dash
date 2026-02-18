package main

import (
	"context"
	"fmt"
	"os"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

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
		ui.InitialModel(dockerClient),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
