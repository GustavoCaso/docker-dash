package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui"
)

func main() {
	configPath := flag.String("config", "", "path to config file (default: $HOME/.config/docker-dash.toml)")
	refreshConfig := flag.String(
		"refresh.interval",
		"",
		"Refresh interval. Override value from configuration file if exists",
	)
	host := flag.String("docker.host", "", "Docker host. Override value from configuration file if exists")
	flag.Parse()

	// Resolve config file path
	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	// Load config; missing file is silently ignored (Load handles it gracefully)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config %s: %v.\n", cfgPath, err)
		os.Exit(1)
	}

	if *host != "" {
		cfg.Docker.Host = *host
		fmt.Fprintf(os.Stderr, "Override Docker host configuration with %s\n", *host)
	}

	if *refreshConfig != "" {
		cfg.Refresh.Interval = *refreshConfig
		fmt.Fprintf(os.Stderr, "Override refresh intervalconfiguration with %s\n", *refreshConfig)
	}

	// Build Docker client from config
	dockerClient := setupDockerClient(cfg.Docker)
	if dockerClient == nil {
		dockerClient = client.NewMockClient()
	}

	p := tea.NewProgram(
		ui.InitialModel(cfg, dockerClient),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, runErr := p.Run()
	if closeErr := dockerClient.Close(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to close Docker client: %v\n", closeErr)
	}
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", runErr)
		os.Exit(1)
	}
}

// setupDockerClient tries to connect to the Docker daemon and returns a client,
// or nil if the connection fails (falling back to mock data).
func setupDockerClient(cfg config.DockerConfig) client.Client {
	realClient, err := client.NewDockerClientFromConfig(cfg)
	if err != nil {
		return nil
	}

	pingErr := realClient.Ping(context.Background())
	if pingErr != nil {
		_ = realClient.Close()
		fmt.Fprintf(os.Stderr, "Warning: Docker daemon unreachable (%v) — falling back to mock data\n", pingErr)
		return nil
	}

	return realClient
}
