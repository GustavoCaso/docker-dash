package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/tree"
)

// MockClient provides a mock implementation of DockerClient for development
type MockClient struct {
	containers *mockContainerService
	images     *mockImageService
	volumes    *mockVolumeService
}

// NewMockClient creates a new mock Docker client with sample data
func NewMockClient() *MockClient {
	return &MockClient{
		containers: newMockContainerService(),
		images:     newMockImageService(),
		volumes:    newMockVolumeService(),
	}
}

func (c *MockClient) Containers() ContainerService   { return c.containers }
func (c *MockClient) Images() ImageService           { return c.images }
func (c *MockClient) Volumes() VolumeService         { return c.volumes }
func (c *MockClient) Ping(ctx context.Context) error { return nil }
func (c *MockClient) Close() error                   { return nil }

// mockContainerService provides mock container data
type mockContainerService struct {
	containers []Container
}

func newMockContainerService() *mockContainerService {
	now := time.Now()
	return &mockContainerService{
		containers: []Container{
			{
				ID:      "abc123def456",
				Name:    "nginx-proxy",
				Image:   "nginx:latest",
				Status:  "Up 2 hours",
				State:   StateRunning,
				Created: now.Add(-2 * time.Hour),
				Ports: []PortMapping{
					{HostPort: 80, ContainerPort: 80, Protocol: "tcp"},
					{HostPort: 443, ContainerPort: 443, Protocol: "tcp"},
				},
				Mounts: []Mount{
					{Type: "volume", Source: "nginx_config", Destination: "/etc/nginx"},
				},
			},
			{
				ID:      "def456ghi789",
				Name:    "api-server",
				Image:   "node:18-alpine",
				Status:  "Up 5 hours",
				State:   StateRunning,
				Created: now.Add(-5 * time.Hour),
				Ports: []PortMapping{
					{HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"},
				},
				Mounts: []Mount{
					{Type: "bind", Source: "/app/src", Destination: "/app"},
				},
			},
			{
				ID:      "ghi789jkl012",
				Name:    "postgres-db",
				Image:   "postgres:15",
				Status:  "Up 1 day",
				State:   StateRunning,
				Created: now.Add(-24 * time.Hour),
				Ports: []PortMapping{
					{HostPort: 5432, ContainerPort: 5432, Protocol: "tcp"},
				},
				Mounts: []Mount{
					{Type: "volume", Source: "postgres_data", Destination: "/var/lib/postgresql/data"},
				},
			},
			{
				ID:      "jkl012mno345",
				Name:    "old-container",
				Image:   "alpine:3.14",
				Status:  "Exited (0) 3 days ago",
				State:   StateStopped,
				Created: now.Add(-72 * time.Hour),
				Ports:   []PortMapping{},
				Mounts:  []Mount{},
			},
		},
	}
}

func (s *mockContainerService) List(ctx context.Context) ([]Container, error) {
	return s.containers, nil
}

func (s *mockContainerService) Run(ctx context.Context, image Image) (string, error) {
	return "", nil
}

func (s *mockContainerService) Get(ctx context.Context, id string) (*Container, error) {
	for _, c := range s.containers {
		if c.ID == id || c.Name == id {
			return &c, nil
		}
	}
	return nil, fmt.Errorf("container not found: %s", id)
}

func (s *mockContainerService) Start(ctx context.Context, id string) error {
	for i, c := range s.containers {
		if c.ID == id || c.Name == id {
			s.containers[i].State = StateRunning
			s.containers[i].Status = "Up 1 second"
			return nil
		}
	}
	return fmt.Errorf("container not found: %s", id)
}

func (s *mockContainerService) Stop(ctx context.Context, id string) error {
	for i, c := range s.containers {
		if c.ID == id || c.Name == id {
			s.containers[i].State = StateStopped
			s.containers[i].Status = "Exited (0) 1 second ago"
			return nil
		}
	}
	return fmt.Errorf("container not found: %s", id)
}

func (s *mockContainerService) Restart(ctx context.Context, id string) error {
	for i, c := range s.containers {
		if c.ID == id || c.Name == id {
			s.containers[i].State = StateRunning
			s.containers[i].Status = "Up 1 second"
			return nil
		}
	}
	return fmt.Errorf("container not found: %s", id)
}

func (s *mockContainerService) Remove(ctx context.Context, id string, force bool) error {
	for i, c := range s.containers {
		if c.ID == id || c.Name == id {
			if c.State == StateRunning && !force {
				return fmt.Errorf("container is running, use force to remove")
			}
			s.containers = append(s.containers[:i], s.containers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("container not found: %s", id)
}

func (s *mockContainerService) Logs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	logs := `2024-01-15T10:30:00Z Starting application...
2024-01-15T10:30:01Z Loading configuration...
2024-01-15T10:30:02Z Connected to database
2024-01-15T10:30:03Z Server listening on port 3000
2024-01-15T10:30:10Z GET /health 200 5ms
2024-01-15T10:30:15Z GET /api/users 200 25ms
`
	return io.NopCloser(strings.NewReader(logs)), nil
}

func (s *mockContainerService) Exec(ctx context.Context, id string) (*ExecSession, error) {
	for _, c := range s.containers {
		if c.ID == id || c.Name == id {
			if c.State != StateRunning {
				return nil, fmt.Errorf("container %s is not running", id)
			}

			pr, pw := io.Pipe()

			return NewExecSession(
				pr,
				&mockExecWriter{pw: pw},
				func() {
					pw.Close()
					pr.Close()
				},
			), nil
		}
	}
	return nil, fmt.Errorf("container not found: %s", id)
}

// mockExecWriter simulates shell output by echoing back commands with a fake prompt
type mockExecWriter struct {
	pw *io.PipeWriter
}

func (w *mockExecWriter) Write(p []byte) (int, error) {
	cmd := strings.TrimSpace(string(p))
	output := fmt.Sprintf("$ %s\nmock output for: %s\n", cmd, cmd)
	_, err := w.pw.Write([]byte(output))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *mockExecWriter) Close() error {
	return w.pw.Close()
}

func (s *mockContainerService) FileTree(ctx context.Context, id string) (ContainerFileTree, error) {
	return ContainerFileTree{Files: []string{}, Tree: tree.New()}, nil
}

// mockImageService provides mock image data
type mockImageService struct {
	images []Image
}

func newMockImageService() *mockImageService {
	now := time.Now()
	return &mockImageService{
		images: []Image{
			{
				ID:       "sha256:nginx123",
				Repo:     "nginx",
				Tag:      "latest",
				Size:     142 * 1024 * 1024, // 142 MB
				Created:  now.Add(-24 * time.Hour),
				Dangling: false,
				UsedBy:   []string{"abc123def456"},
			},
			{
				ID:       "sha256:node456",
				Repo:     "node",
				Tag:      "18-alpine",
				Size:     178 * 1024 * 1024, // 178 MB
				Created:  now.Add(-48 * time.Hour),
				Dangling: false,
				UsedBy:   []string{"def456ghi789"},
			},
			{
				ID:       "sha256:postgres789",
				Repo:     "postgres",
				Tag:      "15",
				Size:     379 * 1024 * 1024, // 379 MB
				Created:  now.Add(-72 * time.Hour),
				Dangling: false,
				UsedBy:   []string{"ghi789jkl012"},
			},
			{
				ID:       "sha256:dangling001",
				Repo:     "<none>",
				Tag:      "<none>",
				Size:     85 * 1024 * 1024, // 85 MB
				Created:  now.Add(-168 * time.Hour),
				Dangling: true,
				UsedBy:   []string{},
			},
		},
	}
}

func (s *mockImageService) List(ctx context.Context) ([]Image, error) {
	return s.images, nil
}

func (s *mockImageService) Get(ctx context.Context, id string) (Image, error) {
	for _, img := range s.images {
		if img.ID == id {
			return img, nil
		}
	}
	return Image{}, fmt.Errorf("image not found: %s", id)
}

func (s *mockImageService) Remove(ctx context.Context, id string, force bool) error {
	for i, img := range s.images {
		if img.ID == id {
			if len(img.UsedBy) > 0 && !force {
				return fmt.Errorf("image is in use by %d container(s)", len(img.UsedBy))
			}
			s.images = append(s.images[:i], s.images[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("image not found: %s", id)
}

// mockVolumeService provides mock volume data
type mockVolumeService struct {
	volumes []Volume
}

func newMockVolumeService() *mockVolumeService {
	now := time.Now()
	return &mockVolumeService{
		volumes: []Volume{
			{
				Name:      "postgres_data",
				Driver:    "local",
				MountPath: "/var/lib/docker/volumes/postgres_data/_data",
				Size:      256 * 1024 * 1024, // 256 MB
				Created:   now.Add(-24 * time.Hour),
				UsedBy:    []string{"ghi789jkl012"},
			},
			{
				Name:      "app_data",
				Driver:    "local",
				MountPath: "/var/lib/docker/volumes/app_data/_data",
				Size:      64 * 1024 * 1024, // 64 MB
				Created:   now.Add(-48 * time.Hour),
				UsedBy:    []string{},
			},
			{
				Name:      "nginx_config",
				Driver:    "local",
				MountPath: "/var/lib/docker/volumes/nginx_config/_data",
				Size:      1024 * 1024, // 1 MB
				Created:   now.Add(-72 * time.Hour),
				UsedBy:    []string{"abc123def456"},
			},
		},
	}
}

func (s *mockVolumeService) List(ctx context.Context) ([]Volume, error) {
	return s.volumes, nil
}

func (s *mockVolumeService) Get(ctx context.Context, name string) (*Volume, error) {
	for _, v := range s.volumes {
		if v.Name == name {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("volume not found: %s", name)
}

func (s *mockVolumeService) Remove(ctx context.Context, name string, force bool) error {
	for i, v := range s.volumes {
		if v.Name == name {
			if len(v.UsedBy) > 0 && !force {
				return fmt.Errorf("volume is in use by %d container(s)", len(v.UsedBy))
			}
			s.volumes = append(s.volumes[:i], s.volumes[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("volume not found: %s", name)
}

func (s *mockVolumeService) Browse(ctx context.Context, name string, path string) ([]FileEntry, error) {
	// Return mock file entries
	return []FileEntry{
		{Name: "config", IsDir: true, Size: 4096, Mode: "drwxr-xr-x"},
		{Name: "data", IsDir: true, Size: 4096, Mode: "drwxr-xr-x"},
		{Name: "app.log", IsDir: false, Size: 1024, Mode: "-rw-r--r--"},
		{Name: "settings.json", IsDir: false, Size: 256, Mode: "-rw-r--r--"},
	}, nil
}
