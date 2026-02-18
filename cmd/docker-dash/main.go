package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	configPath := flag.String("config", "", "path to config file (default: $HOME/.config/docker-dash.toml)")
	flag.Parse()

	// Resolve config file path
	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	// Load config; missing file is silently ignored (Load handles it gracefully)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config %s: %v\n", cfgPath, err)
		os.Exit(1)
	}

	// Build Docker client from config
	var dockerClient service.DockerClient

	realClient, err := service.NewDockerClientFromConfig(cfg.Docker)
	if err != nil {
		if cfg.Docker.Host != "" {
			fmt.Fprintf(os.Stderr, "Warning: could not create Docker client: %v — falling back to mock data\n", err)
		}
	} else if pingErr := realClient.Ping(context.Background()); pingErr != nil {
		realClient.Close()
		if cfg.Docker.Host != "" {
			fmt.Fprintf(os.Stderr, "Warning: Docker daemon unreachable (%v) — falling back to mock data\n", pingErr)
		}
	} else {
		dockerClient = realClient
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
