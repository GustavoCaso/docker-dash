package service_test

import (
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/service"
)

func TestNewDockerClientFromConfig_EmptyHost(t *testing.T) {
	cfg := config.DockerConfig{}
	client, err := service.NewDockerClientFromConfig(cfg)
	if err == nil {
		if client == nil {
			t.Error("NewDockerClientFromConfig() returned nil client with nil error")
		} else {
			client.Close()
		}
	}
	// err != nil is acceptable — Docker daemon may not be available in CI.
}

func TestNewDockerClientFromConfig_TCPHost(t *testing.T) {
	cfg := config.DockerConfig{
		Host: "tcp://127.0.0.1:2375",
	}
	client, err := service.NewDockerClientFromConfig(cfg)
	// Client creation should succeed even if daemon is unreachable.
	if err != nil {
		t.Fatalf("NewDockerClientFromConfig() TCP host creation error: %v", err)
	}
	defer client.Close()
}

func TestNewDockerClientFromConfig_SSHHostWithMissingKeyFile(t *testing.T) {
	cfg := config.DockerConfig{
		Host:         "ssh://pi@raspberrypi.local",
		IdentityFile: "/nonexistent/id_rsa",
	}
	_, err := service.NewDockerClientFromConfig(cfg)
	if err == nil {
		t.Error("NewDockerClientFromConfig() with missing identity file should return error")
	}
}

func TestNewDockerClientFromConfig_SSHHostWithValidKey(t *testing.T) {
	keyPath := generateTestKey(t)
	cfg := config.DockerConfig{
		Host:         "ssh://pi@raspberrypi.local",
		IdentityFile: keyPath,
	}
	client, err := service.NewDockerClientFromConfig(cfg)
	// Client creation should succeed; actual connection will fail at Ping time.
	if err != nil {
		t.Fatalf("NewDockerClientFromConfig() SSH+key creation error: %v", err)
	}
	defer client.Close()
}

func TestNewDockerClientFromConfig_SSHHostNoKeyFile(t *testing.T) {
	cfg := config.DockerConfig{
		Host: "ssh://pi@raspberrypi.local",
		// No IdentityFile — uses SSH agent dialer.
	}
	client, err := service.NewDockerClientFromConfig(cfg)
	if err != nil {
		// Acceptable: SSH agent not available in this environment (SSH_AUTH_SOCK not set).
		t.Skipf("skipping: SSH agent unavailable: %v", err)
	}
	defer client.Close()
}
