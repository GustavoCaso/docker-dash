package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui"
)

var Version = "dev"

func main() {
	configPath := flag.String("config", "", "path to config file (default: $HOME/.config/docker-dash.toml)")
	refreshConfig := flag.String(
		"refresh.interval",
		"",
		"Refresh interval. Override value from configuration file if exists",
	)
	host := flag.String("docker.host", "", "Docker host. Override value from configuration file if exists")
	debug := flag.Bool("debug", false, "Enable debug logging to ./docker-dash-debug.log")
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

	if validationErr := validateIntervals(cfg, os.Stderr); validationErr != nil {
		fmt.Fprintln(os.Stderr, validationErr)
		os.Exit(1)
	}

	if *debug {
		cfg.Debug.Enabled = true
	}

	var debugFile *os.File
	if cfg.Debug.Enabled {
		tempFile, tempErr := os.CreateTemp("", "docker-dash-debug")
		if tempErr != nil {
			fmt.Fprintf(os.Stderr, "Error creating temp folder for debug log: %v\n", tempErr)
			os.Exit(1)
		}
		_ = tempFile.Close()
		var logErr error
		debugFile, logErr = tea.LogToFile(tempFile.Name(), "[docker-dash]")
		if logErr != nil {
			fmt.Fprintf(os.Stderr, "Error opening debug log: %v\n", logErr)
			os.Exit(1)
		}
	} else {
		log.SetOutput(io.Discard)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Build Docker client from config
	dockerClient := setupDockerClient(cfg.Docker)
	if dockerClient == nil {
		dockerClient = client.NewMockClient()
	}

	p := tea.NewProgram(
		ui.New(ctx, Version, cfg, dockerClient),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, runErr := p.Run()
	cancel()

	if cfg.Debug.Enabled {
		fmt.Fprintf(os.Stderr, "debug file for this session is located at: %s\n", debugFile.Name())
		_ = debugFile.Close()
	}

	if closeErr := dockerClient.Close(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to close Docker client: %v\n", closeErr)
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", runErr)
		os.Exit(1)
	}

	os.Exit(0)
}

func validateIntervals(cfg *config.Config, stderr io.Writer) error {
	if cfg.Refresh.Interval != "" {
		_, err := time.ParseDuration(cfg.Refresh.Interval)
		if err != nil {
			return fmt.Errorf("invalid refresh interval %q: %w", cfg.Refresh.Interval, err)
		}
	}

	if !cfg.UpdateCheck.Enabled {
		return nil
	}

	d, err := time.ParseDuration(cfg.UpdateCheck.Interval)
	if err != nil {
		return fmt.Errorf("invalid update check interval %q: %w", cfg.UpdateCheck.Interval, err)
	}

	if d <= 0 {
		fmt.Fprintf(
			stderr,
			"Update check enabled, but non-positive interval configured %q. Skiping update check",
			cfg.UpdateCheck.Interval,
		)
		cfg.UpdateCheck.Enabled = false
	}

	return nil
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
