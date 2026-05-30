package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all docker-dash configuration.
type Config struct {
	Docker      DockerConfig      `toml:"docker"`
	Refresh     RefreshConfig     `toml:"refresh"`
	Debug       DebugConfig       `toml:"debug"`
	UpdateCheck UpdateCheckConfig `toml:"update_check"`
	Logs        LogsConfig        `toml:"logs"`
}

// DockerConfig holds Docker client connection settings.
type DockerConfig struct {
	// Host is the Docker daemon URL. Accepts unix://, tcp://, ssh:// schemes.
	// If empty, the default local socket / DOCKER_HOST env var is used.
	Host string `toml:"host"`
}

// RefreshConfig holds refresh configuration.
type RefreshConfig struct {
	Interval string `toml:"interval"`
}

// DebugConfig holds debug/logging settings.
type DebugConfig struct {
	// Enabled writes debug logs to ./docker-dash-debug.log when true.
	Enabled bool `toml:"enabled"`
}

// UpdateCheckConfig holds settings for the automatic update checker.
type UpdateCheckConfig struct {
	// Enabled controls whether docker-dash periodically checks for newer versions of local Docker images in the registry.
	Enabled bool `toml:"enabled"`
	// Interval is the minimum time between update checks (e.g. "1h", "30m").
	Interval string `toml:"interval"`
}

// LogsConfig holds log streaming settings.
type LogsConfig struct {
	// Follow streams logs in real-time when true.
	Follow bool `toml:"follow"`
	// Tail is the number of lines to show from the end ("100", "all", etc).
	Tail string `toml:"tail"`
	// Timestamps prepends timestamps to each log line.
	Timestamps bool `toml:"timestamps"`
	// Since shows logs since a relative duration or timestamp (e.g. "2h", "10m").
	Since string `toml:"since"`
}

// DefaultLogsConfig returns sensible defaults for log streaming.
func DefaultLogsConfig() LogsConfig {
	return LogsConfig{
		Follow: true,
		Tail:   "100",
		Since:  "2h",
	}
}

// DefaultPath returns the default config file path: $HOME/.config/docker-dash.toml.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.toml"
	}
	return filepath.Join(home, ".config", "docker-dash", "config.toml")
}

// Load parses a TOML config file at path.
// If the file does not exist, an empty Config is returned with no error.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	cfg.Logs = DefaultLogsConfig()
	_, err := toml.DecodeFile(path, cfg)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprint(os.Stderr, "Config file not present. Using default values\n")
			return cfg, nil
		}
		return nil, err
	}
	return cfg, nil
}
