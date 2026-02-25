package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/tree"
)

// MockClient provides a mock implementation of DockerClient for development.
type MockClient struct {
	containers *mockContainerService
	images     *mockImageService
	volumes    *mockVolumeService
}

// NewMockClient creates a new mock Docker client with sample data.
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

// mockContainerService provides mock container data.
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
				return errors.New("container is running, use force to remove")
			}
			s.containers = append(s.containers[:i], s.containers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("container not found: %s", id)
}

func (s *mockContainerService) Logs(ctx context.Context, id string, opts LogOptions) (*LogsSession, error) {
	logs := `2024-01-15T10:30:00Z Starting application...
2024-01-15T10:30:01Z Loading configuration...
2024-01-15T10:30:02Z Connected to database
2024-01-15T10:30:03Z Server listening on port 3000
2024-01-15T10:30:10Z GET /health 200 5ms
2024-01-15T10:30:15Z GET /api/users 200 25ms
`

	pr, pw := io.Pipe()
	go func() {
		pw.Write([]byte(logs))
		pw.Close()
	}()

	return NewLogsSession(io.NopCloser(pr), func() {}), nil
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

// mockStatsJSON is a single Docker stats response frame used by the mock.
// CPU%: (2e8 - 1e8) / (2e10 - 1e10) * 4 * 100 = 4%
// Mem: 512 MiB usage / 8 GiB limit = 6.25%
const mockStatsJSON = `{"cpu_stats":{"cpu_usage":{"total_usage":200000000},"system_cpu_usage":20000000000,"online_cpus":4},"precpu_stats":{"cpu_usage":{"total_usage":100000000},"system_cpu_usage":10000000000},"memory_stats":{"usage":536870912,"limit":8589934592}}`

func (s *mockContainerService) Stats(ctx context.Context, id string) (*StatsSession, error) {
	for _, c := range s.containers {
		if c.ID == id || c.Name == id {
			if c.State != StateRunning {
				return nil, fmt.Errorf("container %s is not running", id)
			}

			pr, pw := io.Pipe()

			go func() {
				for {
					_, err := pw.Write([]byte(mockStatsJSON))
					if err != nil {
						pw.CloseWithError(err)
						return
					}
					time.Sleep(100 * time.Millisecond)
				}
			}()

			return NewStatsSession(
				pr,
				func() {
					pw.Close()
					pr.Close()
				},
			), nil
		}
	}
	return nil, fmt.Errorf("container not found: %s", id)
}

// mockExecWriter simulates shell output by echoing back commands with a fake prompt.
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

// mockImageService provides mock image data.
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

// mockVolumeService provides mock volume data.
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
				UsedCount: 1,
			},
			{
				Name:      "app_data",
				Driver:    "local",
				MountPath: "/var/lib/docker/volumes/app_data/_data",
				Size:      64 * 1024 * 1024, // 64 MB
				Created:   now.Add(-48 * time.Hour),
				UsedCount: 0,
			},
			{
				Name:      "nginx_config",
				Driver:    "local",
				MountPath: "/var/lib/docker/volumes/nginx_config/_data",
				Size:      1024 * 1024, // 1 MB
				Created:   now.Add(-72 * time.Hour),
				UsedCount: 1,
			},
		},
	}
}

func (s *mockVolumeService) List(ctx context.Context) ([]Volume, error) {
	return s.volumes, nil
}

func (s *mockVolumeService) Remove(ctx context.Context, name string, force bool) error {
	for i, v := range s.volumes {
		if v.Name == name {
			if v.UsedCount > 0 && !force {
				return fmt.Errorf("volume is in use by %d container(s)", v.UsedCount)
			}
			s.volumes = append(s.volumes[:i], s.volumes[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("volume not found: %s", name)
}

func (s *mockVolumeService) FileTree(ctx context.Context, name string) (VolumeFileTree, error) {
	for _, v := range s.volumes {
		if v.Name == name {
			t := tree.Root(name)
			files := []string{}

			switch name {
			case "postgres_data":
				pgdata := tree.Root("pgdata")
				pgdata.Child("PG_VERSION")
				pgdata.Child("postgresql.conf")
				pgdata.Child("pg_hba.conf")
				baseDir := tree.Root("base")
				baseDir.Child("1")
				baseDir.Child("13067")
				pgdata.Child(baseDir)
				t.Child(pgdata)
				files = []string{
					"pgdata/",
					"pgdata/PG_VERSION",
					"pgdata/postgresql.conf",
					"pgdata/pg_hba.conf",
					"pgdata/base/",
					"pgdata/base/1",
					"pgdata/base/13067",
				}
			case "nginx_config":
				t.Child("nginx.conf")
				confD := tree.Root("conf.d")
				confD.Child("default.conf")
				t.Child(confD)
				files = []string{"nginx.conf", "conf.d/", "conf.d/default.conf"}
			default:
				t.Child("data.bin")
				files = []string{"data.bin"}
			}

			return VolumeFileTree{Files: files, Tree: t}, nil
		}
	}
	return VolumeFileTree{}, fmt.Errorf("volume not found: %s", name)
}
