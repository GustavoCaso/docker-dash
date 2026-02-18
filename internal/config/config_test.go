package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/config"
)

func TestDefaultPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "docker-dash.toml")
	got := config.DefaultPath()
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/docker-dash.toml")
	if err != nil {
		t.Errorf("Load() with missing file should not error, got: %v", err)
	}
	if cfg.Docker.Host != "" {
		t.Errorf("Load() missing file: Docker.Host = %q, want empty", cfg.Docker.Host)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(`
[docker]
host = "ssh://pi@raspberrypi.local"
identity_file = "~/.ssh/id_rsa"
`)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(f.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Docker.Host != "ssh://pi@raspberrypi.local" {
		t.Errorf("Docker.Host = %q, want %q", cfg.Docker.Host, "ssh://pi@raspberrypi.local")
	}
	if cfg.Docker.IdentityFile != "~/.ssh/id_rsa" {
		t.Errorf("Docker.IdentityFile = %q, want %q", cfg.Docker.IdentityFile, "~/.ssh/id_rsa")
	}
}

func TestLoad_EmptyDockerSection(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("# no docker section\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(f.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Docker.Host != "" {
		t.Errorf("Docker.Host = %q, want empty", cfg.Docker.Host)
	}
}

func TestLoad_HostOnly(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("[docker]\nhost = \"tcp://192.168.1.10:2375\"\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(f.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Docker.Host != "tcp://192.168.1.10:2375" {
		t.Errorf("Docker.Host = %q, want tcp://192.168.1.10:2375", cfg.Docker.Host)
	}
	if cfg.Docker.IdentityFile != "" {
		t.Errorf("Docker.IdentityFile = %q, want empty", cfg.Docker.IdentityFile)
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("this is [ not valid toml\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, err = config.Load(f.Name())
	if err == nil {
		t.Error("Load() with invalid TOML should return an error, got nil")
	}
}
