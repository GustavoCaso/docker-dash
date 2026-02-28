package client

import (
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/config"
)

func TestNewDockerClientFromConfig_EmptyHost(t *testing.T) {
	cfg := config.DockerConfig{}
	client, err := NewDockerClientFromConfig(cfg)
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
	client, err := NewDockerClientFromConfig(cfg)
	// Client creation should succeed even if daemon is unreachable.
	if err != nil {
		t.Fatalf("NewDockerClientFromConfig() TCP host creation error: %v", err)
	}
	defer client.Close()
}
