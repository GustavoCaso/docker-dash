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
	Docker DockerConfig `toml:"docker"`
}

// DockerConfig holds Docker client connection settings.
type DockerConfig struct {
	// Host is the Docker daemon URL. Accepts unix://, tcp://, ssh:// schemes.
	// If empty, the default local socket / DOCKER_HOST env var is used.
	Host string `toml:"host"`
}

// DefaultPath returns the default config file path: $HOME/.config/docker-dash.toml.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "docker-dash.toml"
	}
	return filepath.Join(home, ".config", "docker-dash.toml")
}

// Load parses a TOML config file at path.
// If the file does not exist, an empty Config is returned with no error.
func Load(path string) (*Config, error) {
	cfg := &Config{}
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
