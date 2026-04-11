package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// MockClient provides a mock implementation of DockerClient for development.
type MockClient struct {
	containers *mockContainerService
	images     *mockImageService
	volumes    *mockVolumeService
	networks   *mockNetworkService
	compose    *mockComposeProjectService
}

// NewMockClient creates a new mock Docker client with sample data.
func NewMockClient() *MockClient {
	return &MockClient{
		containers: newMockContainerService(),
		images:     newMockImageService(),
		volumes:    newMockVolumeService(),
		networks:   newMockNetworkService(),
		compose:    newMockComposeProjectService(),
	}
}

func (c *MockClient) Containers() ContainerService   { return c.containers }
func (c *MockClient) Images() ImageService           { return c.images }
func (c *MockClient) Volumes() VolumeService         { return c.volumes }
func (c *MockClient) Networks() NetworkService       { return c.networks }
func (c *MockClient) Compose() ComposeProjectService { return c.compose }
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
				Hostname:    "nginx-proxy",
				NetworkMode: "bridge",
				Networks: []NetworkInfo{
					{Name: "bridge", IPAddress: "172.17.0.2", Gateway: "172.17.0.1"},
				},
				Cmd:        []string{"nginx", "-g", "daemon off;"},
				Entrypoint: []string{"/docker-entrypoint.sh"},
				WorkingDir: "/usr/share/nginx/html",
				Env: []string{
					"NGINX_VERSION=1.25.3",
					"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				},
				Labels:        map[string]string{"maintainer": "NGINX Docker Maintainers"},
				MemoryLimit:   512 * 1024 * 1024, // 512 MB
				CPUShares:     1024,
				RestartPolicy: "unless-stopped",
				Privileged:    false,
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
				Hostname:    "api-server",
				NetworkMode: "bridge",
				Networks: []NetworkInfo{
					{Name: "bridge", IPAddress: "172.17.0.3", Gateway: "172.17.0.1"},
					{Name: "app-network", IPAddress: "172.20.0.2", Gateway: "172.20.0.1", Aliases: []string{"api"}},
				},
				Cmd:           []string{"node", "server.js"},
				Entrypoint:    []string{},
				WorkingDir:    "/app",
				Env:           []string{"NODE_ENV=production", "PORT=3000"},
				Labels:        map[string]string{"com.docker.compose.service": "api"},
				MemoryLimit:   0, // unlimited
				CPUShares:     512,
				RestartPolicy: "on-failure:3",
				Privileged:    false,
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
				Hostname:    "postgres-db",
				NetworkMode: "bridge",
				Networks: []NetworkInfo{
					{Name: "app-network", IPAddress: "172.20.0.3", Gateway: "172.20.0.1"},
				},
				Cmd:           []string{"postgres"},
				Entrypoint:    []string{"docker-entrypoint.sh"},
				WorkingDir:    "",
				Env:           []string{"POSTGRES_DB=app", "POSTGRES_USER=admin", "PGDATA=/var/lib/postgresql/data"},
				Labels:        map[string]string{},
				MemoryLimit:   1024 * 1024 * 1024, // 1 GB
				CPUShares:     0,
				RestartPolicy: "always",
				Privileged:    false,
			},
			{
				ID:            "jkl012mno345",
				Name:          "old-container",
				Image:         "alpine:3.14",
				Status:        "Exited (0) 3 days ago",
				State:         StateStopped,
				Created:       now.Add(-72 * time.Hour),
				Ports:         []PortMapping{},
				Mounts:        []Mount{},
				Hostname:      "old-container",
				NetworkMode:   "bridge",
				Networks:      []NetworkInfo{},
				Cmd:           []string{"sh"},
				Entrypoint:    []string{},
				WorkingDir:    "/",
				Env:           []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
				Labels:        map[string]string{},
				MemoryLimit:   0,
				CPUShares:     0,
				RestartPolicy: "no",
				Privileged:    false,
			},
			{
				ID:      "mno345pqr678",
				Name:    "redis-cache",
				Image:   "redis:7-alpine",
				Status:  "Up 3 hours",
				State:   StateRunning,
				Created: now.Add(-3 * time.Hour),
				Ports: []PortMapping{
					{HostPort: 6379, ContainerPort: 6379, Protocol: "tcp"},
				},
				Mounts: []Mount{
					{Type: "volume", Source: "redis_data", Destination: "/data"},
				},
				Hostname:    "redis-cache",
				NetworkMode: "bridge",
				Networks: []NetworkInfo{
					{Name: "app-network", IPAddress: "172.20.0.4", Gateway: "172.20.0.1"},
				},
				Cmd:           []string{"redis-server", "--appendonly", "yes"},
				Entrypoint:    []string{"docker-entrypoint.sh"},
				WorkingDir:    "/data",
				Env:           []string{},
				Labels:        map[string]string{"com.docker.compose.service": "cache"},
				MemoryLimit:   256 * 1024 * 1024, // 256 MB
				CPUShares:     256,
				RestartPolicy: "unless-stopped",
				Privileged:    false,
			},
			{
				ID:            "pqr678stu901",
				Name:          "worker-crashed",
				Image:         "python:3.11-slim",
				Status:        "Exited (1) 1 hour ago",
				State:         StateStopped,
				Created:       now.Add(-6 * time.Hour),
				Ports:         []PortMapping{},
				Mounts:        []Mount{},
				Hostname:      "worker-crashed",
				NetworkMode:   "bridge",
				Networks:      []NetworkInfo{},
				Cmd:           []string{"python", "worker.py"},
				Entrypoint:    []string{},
				WorkingDir:    "/app",
				Env:           []string{"PYTHONUNBUFFERED=1", "WORKER_CONCURRENCY=4"},
				Labels:        map[string]string{"com.docker.compose.service": "worker"},
				MemoryLimit:   512 * 1024 * 1024, // 512 MB
				CPUShares:     512,
				RestartPolicy: "on-failure:5",
				Privileged:    false,
			},
		},
	}
}

func (s *mockContainerService) List(ctx context.Context) ([]Container, error) {
	return s.containers, nil
}

func (s *mockContainerService) Run(_ context.Context, _ Image, _ RunOptions) (string, error) {
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
// Mem: 512 MiB usage / 8 GiB limit = 6.25%.
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

func (s *mockContainerService) FileTree(ctx context.Context, id string) (*FileNode, error) {
	return &FileNode{}, nil
}

func (s *mockContainerService) Prune(_ context.Context, _ PruneOptions) (PruneReport, error) {
	var count int
	var remaining []Container
	for _, c := range s.containers {
		if c.State == StateStopped {
			count++
		} else {
			remaining = append(remaining, c)
		}
	}
	s.containers = remaining
	return PruneReport{ItemsDeleted: count, SpaceReclaimed: 0}, nil
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
				ID:       "sha256:redis012",
				Repo:     "redis",
				Tag:      "7-alpine",
				Size:     40 * 1024 * 1024, // 40 MB
				Created:  now.Add(-48 * time.Hour),
				Dangling: false,
				UsedBy:   []string{"mno345pqr678"},
			},
			{
				ID:       "sha256:python345",
				Repo:     "python",
				Tag:      "3.11-slim",
				Size:     130 * 1024 * 1024, // 130 MB
				Created:  now.Add(-96 * time.Hour),
				Dangling: false,
				UsedBy:   []string{"pqr678stu901"},
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
			{
				ID:       "sha256:dangling002",
				Repo:     "<none>",
				Tag:      "<none>",
				Size:     52 * 1024 * 1024, // 52 MB
				Created:  now.Add(-240 * time.Hour),
				Dangling: true,
				UsedBy:   []string{},
			},
		},
	}
}

func (s *mockImageService) List(ctx context.Context) ([]Image, error) {
	return s.images, nil
}

func (s *mockImageService) FetchLayers(ctx context.Context, id string) []Layer {
	return []Layer{}
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

func (s *mockImageService) Prune(_ context.Context, opts PruneOptions) (PruneReport, error) {
	var count int
	var spaceReclaimed uint64
	var remaining []Image
	for _, img := range s.images {
		unused := img.Dangling || (opts.All && len(img.UsedBy) == 0)
		if unused {
			count++
			spaceReclaimed += uint64(img.Size)
		} else {
			remaining = append(remaining, img)
		}
	}
	s.images = remaining
	return PruneReport{ItemsDeleted: count, SpaceReclaimed: spaceReclaimed}, nil
}

func (s *mockImageService) Pull(_ context.Context, imageRef, platform string) error {
	return nil
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
				Name:      "redis_data",
				Driver:    "local",
				MountPath: "/var/lib/docker/volumes/redis_data/_data",
				Size:      8 * 1024 * 1024, // 8 MB
				Created:   now.Add(-3 * time.Hour),
				UsedCount: 1,
			},
			{
				Name:      "nginx_config",
				Driver:    "local",
				MountPath: "/var/lib/docker/volumes/nginx_config/_data",
				Size:      1024 * 1024, // 1 MB
				Created:   now.Add(-72 * time.Hour),
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
				Name:      "build_cache",
				Driver:    "local",
				MountPath: "/var/lib/docker/volumes/build_cache/_data",
				Size:      512 * 1024 * 1024, // 512 MB
				Created:   now.Add(-120 * time.Hour),
				UsedCount: 0,
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

func (s *mockVolumeService) Prune(_ context.Context, opts PruneOptions) (PruneReport, error) {
	var count int
	var spaceReclaimed uint64
	var remaining []Volume
	for _, v := range s.volumes {
		// Without All, only prune anonymous volumes (UsedCount==0 and no name).
		// In the mock all volumes are named, so without All nothing is pruned.
		// With All, prune all unused (UsedCount==0) volumes.
		if opts.All && v.UsedCount == 0 {
			count++
			spaceReclaimed += uint64(v.Size)
		} else {
			remaining = append(remaining, v)
		}
	}
	s.volumes = remaining
	return PruneReport{ItemsDeleted: count, SpaceReclaimed: spaceReclaimed}, nil
}

// mockNetworkService provides mock network data.
type mockNetworkService struct {
	networks []Network
}

func newMockNetworkService() *mockNetworkService {
	now := time.Now()
	return &mockNetworkService{
		networks: []Network{
			{
				ID:       "abc123def456abc1",
				Name:     "bridge",
				Driver:   "bridge",
				Scope:    "local",
				Internal: false,
				Created:  now.Add(-72 * time.Hour),
				ConnectedContainers: []NetworkContainer{
					{Name: "nginx-proxy", IPv4Address: "172.17.0.2/16", MacAddress: "02:42:ac:11:00:02"},
					{Name: "api-server", IPv4Address: "172.17.0.3/16", MacAddress: "02:42:ac:11:00:03"},
					{Name: "postgres-db", IPv4Address: "172.17.0.4/16", MacAddress: "02:42:ac:11:00:04"},
				},
				IPAM: NetworkIPAM{Subnet: "172.17.0.0/16", Gateway: "172.17.0.1"},
			},
			{
				ID:                  "def456ghi789def4",
				Name:                "host",
				Driver:              "host",
				Scope:               "local",
				Internal:            false,
				Created:             now.Add(-72 * time.Hour),
				ConnectedContainers: []NetworkContainer{},
				IPAM:                NetworkIPAM{},
			},
			{
				ID:                  "ghi789jkl012ghi7",
				Name:                "none",
				Driver:              "null",
				Scope:               "local",
				Internal:            false,
				Created:             now.Add(-72 * time.Hour),
				ConnectedContainers: []NetworkContainer{},
				IPAM:                NetworkIPAM{},
			},
			{
				ID:       "jkl012mno345jkl0",
				Name:     "app-network",
				Driver:   "bridge",
				Scope:    "local",
				Internal: false,
				Created:  now.Add(-24 * time.Hour),
				ConnectedContainers: []NetworkContainer{
					{Name: "api-server", IPv4Address: "172.20.0.2/16", MacAddress: "02:42:ac:14:00:02"},
					{Name: "postgres-db", IPv4Address: "172.20.0.3/16", MacAddress: "02:42:ac:14:00:03"},
					{Name: "redis-cache", IPv4Address: "172.20.0.4/16", MacAddress: "02:42:ac:14:00:04"},
				},
				IPAM: NetworkIPAM{Subnet: "172.20.0.0/16", Gateway: "172.20.0.1"},
			},
			{
				ID:                  "mno345pqr678mno3",
				Name:                "monitoring",
				Driver:              "bridge",
				Scope:               "local",
				Internal:            true,
				Created:             now.Add(-48 * time.Hour),
				ConnectedContainers: []NetworkContainer{},
				IPAM:                NetworkIPAM{Subnet: "172.25.0.0/16", Gateway: "172.25.0.1"},
			},
		},
	}
}

func (s *mockNetworkService) List(ctx context.Context) ([]Network, error) {
	return s.networks, nil
}

func (s *mockNetworkService) Remove(ctx context.Context, id string) error {
	for i, n := range s.networks {
		if n.ID == id || n.Name == id {
			s.networks = append(s.networks[:i], s.networks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("network not found: %s", id)
}

func (s *mockNetworkService) Prune(_ context.Context, _ PruneOptions) (PruneReport, error) {
	var count int
	var remaining []Network
	for _, n := range s.networks {
		if len(n.ConnectedContainers) == 0 {
			count++
		} else {
			remaining = append(remaining, n)
		}
	}
	s.networks = remaining
	return PruneReport{ItemsDeleted: count, SpaceReclaimed: 0}, nil
}

// mockComposeProjectService provides mock Compose project data.
type mockComposeProjectService struct {
	projects []ComposeProject
}

func newMockComposeProjectService() *mockComposeProjectService {
	return &mockComposeProjectService{
		projects: []ComposeProject{
			{
				Name:        "web-app",
				WorkingDir:  "/home/user/projects/web-app",
				ConfigFiles: "/home/user/projects/web-app/docker-compose.yml",
				Services: []ComposeServiceInfo{
					{Name: "api", State: "running", Image: "node:18-alpine"},
					{Name: "db", State: "running", Image: "postgres:15"},
					{Name: "cache", State: "running", Image: "redis:7-alpine"},
				},
			},
			{
				Name:        "monitoring",
				WorkingDir:  "/home/user/projects/monitoring",
				ConfigFiles: "/home/user/projects/monitoring/compose.yaml",
				Services: []ComposeServiceInfo{
					{Name: "prometheus", State: "running", Image: "prom/prometheus:latest"},
					{Name: "grafana", State: "exited", Image: "grafana/grafana:latest"},
				},
			},
		},
	}
}

func (s *mockComposeProjectService) List(_ context.Context) ([]ComposeProject, error) {
	return s.projects, nil
}

func (s *mockComposeProjectService) Up(_ context.Context, project ComposeProject, _ ComposeUpOptions) error {
	return s.setProjectState(project, "running")
}

func (s *mockComposeProjectService) Down(_ context.Context, project ComposeProject, _ ComposeDownOptions) error {
	for idx, existing := range s.projects {
		if existing.Identity() == project.Identity() {
			s.projects = append(s.projects[:idx], s.projects[idx+1:]...)
			return nil
		}
	}

	return fmt.Errorf("compose project not found: %s", project.Name)
}

func (s *mockComposeProjectService) Start(_ context.Context, project ComposeProject, _ ComposeStartOptions) error {
	return s.setProjectState(project, "running")
}

func (s *mockComposeProjectService) Stop(_ context.Context, project ComposeProject, _ ComposeStopOptions) error {
	return s.setProjectState(project, "exited")
}

func (s *mockComposeProjectService) Restart(_ context.Context, project ComposeProject, _ ComposeRestartOptions) error {
	return s.setProjectState(project, "running")
}

func (s *mockComposeProjectService) setProjectState(project ComposeProject, state string) error {
	for projectIdx, existing := range s.projects {
		if existing.Identity() != project.Identity() {
			continue
		}

		for serviceIdx := range s.projects[projectIdx].Services {
			s.projects[projectIdx].Services[serviceIdx].State = state
		}
		return nil
	}

	return fmt.Errorf("compose project not found: %s", project.Name)
}
