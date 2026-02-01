# Docker Dash Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Docker Desktop-inspired TUI for managing containers, images, and volumes.

**Architecture:** Layered architecture with service interfaces for Docker operations, state management for UI state, and Bubble Tea components for rendering. The service layer uses interfaces to allow future remote Docker support.

**Tech Stack:** Go, Bubble Tea (TUI), Lip Gloss (styling), Docker SDK for Go

---

## Task 1: Project Initialization

**Files:**
- Create: `go.mod`
- Create: `cmd/docker-dash/main.go`

**Step 1: Initialize Go module**

```bash
cd /Users/gustavocaso/src/github.com/GustavoCaso/docker-dash/.worktrees/feature-initial-implementation
go mod init github.com/GustavoCaso/docker-dash
```

**Step 2: Create main.go with minimal Bubble Tea app**

Create `cmd/docker-dash/main.go`:

```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct{}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	return "Docker Dash - Press q to quit\n"
}

func main() {
	p := tea.NewProgram(model{}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 3: Download dependencies**

```bash
go mod tidy
```

**Step 4: Verify it runs**

```bash
go run ./cmd/docker-dash
```

Expected: Shows "Docker Dash - Press q to quit", exits on 'q'.

**Step 5: Commit**

```bash
git add go.mod go.sum cmd/
git commit -m "feat: initialize project with minimal Bubble Tea app"
```

---

## Task 2: Define Service Layer Interfaces

**Files:**
- Create: `internal/service/types.go`
- Create: `internal/service/docker.go`

**Step 1: Create domain types**

Create `internal/service/types.go`:

```go
package service

import "time"

// Container represents a Docker container
type Container struct {
	ID      string
	Name    string
	Image   string
	Status  string
	State   ContainerState
	Created time.Time
	Ports   []PortMapping
	Mounts  []Mount
}

type ContainerState string

const (
	StateRunning    ContainerState = "running"
	StateStopped    ContainerState = "stopped"
	StatePaused     ContainerState = "paused"
	StateRestarting ContainerState = "restarting"
)

type PortMapping struct {
	HostPort      uint16
	ContainerPort uint16
	Protocol      string
}

type Mount struct {
	Type        string // "volume", "bind", "tmpfs"
	Source      string
	Destination string
}

// Image represents a Docker image
type Image struct {
	ID        string
	Repo      string
	Tag       string
	Size      int64
	Created   time.Time
	Dangling  bool
	UsedBy    []string // Container IDs using this image
}

// Volume represents a Docker volume
type Volume struct {
	Name      string
	Driver    string
	MountPath string
	Size      int64
	Created   time.Time
	UsedBy    []string // Container IDs using this volume
}

// FileEntry represents a file in a volume
type FileEntry struct {
	Name  string
	IsDir bool
	Size  int64
	Mode  string
}

// LogOptions configures log streaming
type LogOptions struct {
	Follow     bool
	Tail       string
	Timestamps bool
}
```

**Step 2: Create service interfaces**

Create `internal/service/docker.go`:

```go
package service

import (
	"context"
	"io"
)

// DockerClient provides access to Docker services
type DockerClient interface {
	Containers() ContainerService
	Images() ImageService
	Volumes() VolumeService
	Ping(ctx context.Context) error
	Close() error
}

// ContainerService manages Docker containers
type ContainerService interface {
	List(ctx context.Context) ([]Container, error)
	Get(ctx context.Context, id string) (*Container, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
	Remove(ctx context.Context, id string, force bool) error
	Logs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error)
}

// ImageService manages Docker images
type ImageService interface {
	List(ctx context.Context) ([]Image, error)
	Get(ctx context.Context, id string) (*Image, error)
	Remove(ctx context.Context, id string, force bool) error
}

// VolumeService manages Docker volumes
type VolumeService interface {
	List(ctx context.Context) ([]Volume, error)
	Get(ctx context.Context, name string) (*Volume, error)
	Remove(ctx context.Context, name string, force bool) error
	Browse(ctx context.Context, name string, path string) ([]FileEntry, error)
}
```

**Step 3: Verify compilation**

```bash
go build ./internal/service/...
```

Expected: No errors.

**Step 4: Commit**

```bash
git add internal/service/
git commit -m "feat: define Docker service layer interfaces"
```

---

## Task 3: Create Mock Docker Client for Development

**Files:**
- Create: `internal/service/mock.go`
- Create: `internal/service/mock_test.go`

**Step 1: Write test for mock client**

Create `internal/service/mock_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/service"
)

func TestMockClient_Ping(t *testing.T) {
	client := service.NewMockClient()
	defer client.Close()

	err := client.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping() error = %v, want nil", err)
	}
}

func TestMockClient_ContainerList(t *testing.T) {
	client := service.NewMockClient()
	defer client.Close()

	containers, err := client.Containers().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(containers) == 0 {
		t.Error("List() returned empty, want sample containers")
	}
}

func TestMockClient_ImageList(t *testing.T) {
	client := service.NewMockClient()
	defer client.Close()

	images, err := client.Images().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(images) == 0 {
		t.Error("List() returned empty, want sample images")
	}
}

func TestMockClient_VolumeList(t *testing.T) {
	client := service.NewMockClient()
	defer client.Close()

	volumes, err := client.Volumes().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(volumes) == 0 {
		t.Error("List() returned empty, want sample volumes")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/service/... -v
```

Expected: FAIL - `NewMockClient` undefined.

**Step 3: Implement mock client**

Create `internal/service/mock.go`:

```go
package service

import (
	"context"
	"io"
	"strings"
	"time"
)

// MockClient provides a mock Docker client for development
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

func (c *MockClient) Containers() ContainerService { return c.containers }
func (c *MockClient) Images() ImageService         { return c.images }
func (c *MockClient) Volumes() VolumeService       { return c.volumes }
func (c *MockClient) Ping(ctx context.Context) error { return nil }
func (c *MockClient) Close() error                 { return nil }

// Mock Container Service
type mockContainerService struct {
	containers []Container
}

func newMockContainerService() *mockContainerService {
	return &mockContainerService{
		containers: []Container{
			{
				ID:      "a1b2c3d4e5f6",
				Name:    "nginx-proxy",
				Image:   "nginx:alpine",
				Status:  "Up 2 hours",
				State:   StateRunning,
				Created: time.Now().Add(-2 * time.Hour),
				Ports: []PortMapping{
					{HostPort: 80, ContainerPort: 80, Protocol: "tcp"},
					{HostPort: 443, ContainerPort: 443, Protocol: "tcp"},
				},
			},
			{
				ID:      "b2c3d4e5f6a7",
				Name:    "api-server",
				Image:   "node:18-alpine",
				Status:  "Up 5 minutes",
				State:   StateRunning,
				Created: time.Now().Add(-5 * time.Minute),
				Ports: []PortMapping{
					{HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"},
				},
				Mounts: []Mount{
					{Type: "volume", Source: "app_data", Destination: "/app/data"},
				},
			},
			{
				ID:      "c3d4e5f6a7b8",
				Name:    "postgres-db",
				Image:   "postgres:15",
				Status:  "Up 2 hours",
				State:   StateRunning,
				Created: time.Now().Add(-2 * time.Hour),
				Ports: []PortMapping{
					{HostPort: 5432, ContainerPort: 5432, Protocol: "tcp"},
				},
				Mounts: []Mount{
					{Type: "volume", Source: "postgres_data", Destination: "/var/lib/postgresql/data"},
				},
			},
			{
				ID:      "d4e5f6a7b8c9",
				Name:    "old-container",
				Image:   "alpine:3.18",
				Status:  "Exited (0) 3 days ago",
				State:   StateStopped,
				Created: time.Now().Add(-72 * time.Hour),
			},
		},
	}
}

func (s *mockContainerService) List(ctx context.Context) ([]Container, error) {
	return s.containers, nil
}

func (s *mockContainerService) Get(ctx context.Context, id string) (*Container, error) {
	for _, c := range s.containers {
		if c.ID == id || c.Name == id {
			return &c, nil
		}
	}
	return nil, nil
}

func (s *mockContainerService) Start(ctx context.Context, id string) error   { return nil }
func (s *mockContainerService) Stop(ctx context.Context, id string) error    { return nil }
func (s *mockContainerService) Restart(ctx context.Context, id string) error { return nil }
func (s *mockContainerService) Remove(ctx context.Context, id string, force bool) error {
	return nil
}

func (s *mockContainerService) Logs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	logs := "2024-01-15T10:00:00Z Starting server...\n2024-01-15T10:00:01Z Server listening on :3000\n"
	return io.NopCloser(strings.NewReader(logs)), nil
}

// Mock Image Service
type mockImageService struct {
	images []Image
}

func newMockImageService() *mockImageService {
	return &mockImageService{
		images: []Image{
			{
				ID:      "sha256:abc123",
				Repo:    "nginx",
				Tag:     "alpine",
				Size:    23 * 1024 * 1024,
				Created: time.Now().Add(-24 * time.Hour),
				UsedBy:  []string{"a1b2c3d4e5f6"},
			},
			{
				ID:      "sha256:def456",
				Repo:    "node",
				Tag:     "18-alpine",
				Size:    152 * 1024 * 1024,
				Created: time.Now().Add(-72 * time.Hour),
				UsedBy:  []string{"b2c3d4e5f6a7"},
			},
			{
				ID:       "sha256:ghi789",
				Repo:     "<none>",
				Tag:      "<none>",
				Size:     89 * 1024 * 1024,
				Created:  time.Now().Add(-14 * 24 * time.Hour),
				Dangling: true,
			},
		},
	}
}

func (s *mockImageService) List(ctx context.Context) ([]Image, error) {
	return s.images, nil
}

func (s *mockImageService) Get(ctx context.Context, id string) (*Image, error) {
	for _, img := range s.images {
		if img.ID == id {
			return &img, nil
		}
	}
	return nil, nil
}

func (s *mockImageService) Remove(ctx context.Context, id string, force bool) error {
	return nil
}

// Mock Volume Service
type mockVolumeService struct {
	volumes []Volume
}

func newMockVolumeService() *mockVolumeService {
	return &mockVolumeService{
		volumes: []Volume{
			{
				Name:      "postgres_data",
				Driver:    "local",
				MountPath: "/var/lib/docker/volumes/postgres_data/_data",
				Size:      2300 * 1024 * 1024,
				Created:   time.Now().Add(-14 * 24 * time.Hour),
				UsedBy:    []string{"c3d4e5f6a7b8"},
			},
			{
				Name:      "app_data",
				Driver:    "local",
				MountPath: "/var/lib/docker/volumes/app_data/_data",
				Size:      156 * 1024 * 1024,
				Created:   time.Now().Add(-3 * 24 * time.Hour),
				UsedBy:    []string{"b2c3d4e5f6a7"},
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
	return nil, nil
}

func (s *mockVolumeService) Remove(ctx context.Context, name string, force bool) error {
	return nil
}

func (s *mockVolumeService) Browse(ctx context.Context, name string, path string) ([]FileEntry, error) {
	return []FileEntry{
		{Name: "base", IsDir: true, Size: 0, Mode: "drwx------"},
		{Name: "global", IsDir: true, Size: 0, Mode: "drwx------"},
		{Name: "pg_wal", IsDir: true, Size: 0, Mode: "drwx------"},
		{Name: "postgresql.conf", IsDir: false, Size: 4200, Mode: "-rw-------"},
	}, nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/service/... -v
```

Expected: All tests PASS.

**Step 5: Commit**

```bash
git add internal/service/mock.go internal/service/mock_test.go
git commit -m "feat: add mock Docker client for development"
```

---

## Task 4: Create Theme and Styling

**Files:**
- Create: `internal/ui/theme/theme.go`

**Step 1: Create theme with Docker Desktop colors**

Create `internal/ui/theme/theme.go`:

```go
package theme

import "github.com/charmbracelet/lipgloss"

// Docker Desktop inspired colors
var (
	// Primary colors
	DockerBlue   = lipgloss.Color("#1D63ED")
	DockerDark   = lipgloss.Color("#0B1929")
	DockerLight  = lipgloss.Color("#E5F0FF")

	// Status colors
	StatusRunning = lipgloss.Color("#2ECC71")
	StatusStopped = lipgloss.Color("#6C7A89")
	StatusError   = lipgloss.Color("#E74C3C")
	StatusPaused  = lipgloss.Color("#F39C12")

	// UI colors
	BorderColor     = lipgloss.Color("#394867")
	TextPrimary     = lipgloss.Color("#FFFFFF")
	TextSecondary   = lipgloss.Color("#8B9DC3")
	TextMuted       = lipgloss.Color("#5D6D7E")
	HighlightBg     = lipgloss.Color("#1D3557")
	SelectedBg      = lipgloss.Color("#2C5282")
)

// Nerd Font icons
const (
	IconDocker    = "󰡨"
	IconContainer = "󰆍"
	IconImage     = "󰋊"
	IconVolume    = "󱁤"
	IconRunning   = "▶"
	IconStopped   = "■"
	IconExpanded  = "▼"
	IconCollapsed = "▶"
	IconFolder    = ""
	IconFile      = ""
	IconWarning   = "⚠"
)

// Styles
var (
	// Sidebar styles
	SidebarStyle = lipgloss.NewStyle().
		Width(14).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderRight(true).
		BorderForeground(BorderColor).
		Padding(1, 1)

	SidebarItemStyle = lipgloss.NewStyle().
		Foreground(TextSecondary).
		Padding(0, 1)

	SidebarActiveStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Background(SelectedBg).
		Bold(true).
		Padding(0, 1)

	// Main content styles
	MainPanelStyle = lipgloss.NewStyle().
		Padding(1, 2)

	HeaderStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		MarginBottom(1)

	// List item styles
	ListItemStyle = lipgloss.NewStyle().
		Foreground(TextPrimary)

	ListItemSelectedStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Background(HighlightBg).
		Bold(true)

	// Detail styles
	DetailStyle = lipgloss.NewStyle().
		Foreground(TextSecondary).
		MarginLeft(3)

	DetailLabelStyle = lipgloss.NewStyle().
		Foreground(TextMuted)

	DetailValueStyle = lipgloss.NewStyle().
		Foreground(TextPrimary)

	// Status styles
	StatusRunningStyle = lipgloss.NewStyle().
		Foreground(StatusRunning)

	StatusStoppedStyle = lipgloss.NewStyle().
		Foreground(StatusStopped)

	StatusErrorStyle = lipgloss.NewStyle().
		Foreground(StatusError)

	// Action button styles
	ActionButtonStyle = lipgloss.NewStyle().
		Foreground(DockerBlue).
		Background(lipgloss.Color("#1A1A2E")).
		Padding(0, 1).
		MarginRight(1)

	ActionButtonActiveStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Background(DockerBlue).
		Bold(true).
		Padding(0, 1).
		MarginRight(1)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
		Foreground(TextMuted).
		Background(DockerDark).
		Padding(0, 1)

	// Help text
	HelpStyle = lipgloss.NewStyle().
		Foreground(TextMuted)
)

// StatusStyle returns the appropriate style for a container state
func StatusStyle(state string) lipgloss.Style {
	switch state {
	case "running":
		return StatusRunningStyle
	case "stopped", "exited":
		return StatusStoppedStyle
	default:
		return StatusStoppedStyle
	}
}
```

**Step 2: Verify compilation**

```bash
go build ./internal/ui/theme/...
```

Expected: No errors.

**Step 3: Commit**

```bash
git add internal/ui/theme/
git commit -m "feat: add Docker Desktop inspired theme and styling"
```

---

## Task 5: Create Sidebar Component

**Files:**
- Create: `internal/ui/components/sidebar.go`
- Create: `internal/ui/components/sidebar_test.go`

**Step 1: Write test for sidebar**

Create `internal/ui/components/sidebar_test.go`:

```go
package components_test

import (
	"strings"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/ui/components"
)

func TestSidebar_View(t *testing.T) {
	sidebar := components.NewSidebar()

	view := sidebar.View()

	// Should contain all navigation items
	if !strings.Contains(view, "Cont") {
		t.Error("View() should contain Containers item")
	}
	if !strings.Contains(view, "Image") {
		t.Error("View() should contain Images item")
	}
	if !strings.Contains(view, "Volume") {
		t.Error("View() should contain Volumes item")
	}
}

func TestSidebar_Navigation(t *testing.T) {
	sidebar := components.NewSidebar()

	// Default should be containers (index 0)
	if sidebar.ActiveIndex() != 0 {
		t.Errorf("ActiveIndex() = %d, want 0", sidebar.ActiveIndex())
	}

	// Move down
	sidebar.MoveDown()
	if sidebar.ActiveIndex() != 1 {
		t.Errorf("After MoveDown, ActiveIndex() = %d, want 1", sidebar.ActiveIndex())
	}

	// Move up
	sidebar.MoveUp()
	if sidebar.ActiveIndex() != 0 {
		t.Errorf("After MoveUp, ActiveIndex() = %d, want 0", sidebar.ActiveIndex())
	}

	// Move up at top should wrap to bottom
	sidebar.MoveUp()
	if sidebar.ActiveIndex() != 2 {
		t.Errorf("MoveUp at top should wrap, ActiveIndex() = %d, want 2", sidebar.ActiveIndex())
	}
}

func TestSidebar_ActiveView(t *testing.T) {
	sidebar := components.NewSidebar()

	if sidebar.ActiveView() != components.ViewContainers {
		t.Errorf("ActiveView() = %v, want ViewContainers", sidebar.ActiveView())
	}

	sidebar.MoveDown()
	if sidebar.ActiveView() != components.ViewImages {
		t.Errorf("ActiveView() = %v, want ViewImages", sidebar.ActiveView())
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/ui/components/... -v
```

Expected: FAIL - package doesn't exist.

**Step 3: Implement sidebar component**

Create `internal/ui/components/sidebar.go`:

```go
package components

import (
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// View represents the active main view
type View int

const (
	ViewContainers View = iota
	ViewImages
	ViewVolumes
)

type sidebarItem struct {
	icon  string
	label string
	view  View
}

// Sidebar represents the navigation sidebar
type Sidebar struct {
	items       []sidebarItem
	activeIndex int
	focused     bool
	height      int
}

// NewSidebar creates a new sidebar component
func NewSidebar() *Sidebar {
	return &Sidebar{
		items: []sidebarItem{
			{icon: theme.IconContainer, label: "Cont.", view: ViewContainers},
			{icon: theme.IconImage, label: "Images", view: ViewImages},
			{icon: theme.IconVolume, label: "Volumes", view: ViewVolumes},
		},
		activeIndex: 0,
		focused:     true,
	}
}

// SetHeight sets the sidebar height
func (s *Sidebar) SetHeight(h int) {
	s.height = h
}

// SetFocused sets whether the sidebar is focused
func (s *Sidebar) SetFocused(focused bool) {
	s.focused = focused
}

// IsFocused returns whether the sidebar is focused
func (s *Sidebar) IsFocused() bool {
	return s.focused
}

// ActiveIndex returns the currently selected index
func (s *Sidebar) ActiveIndex() int {
	return s.activeIndex
}

// ActiveView returns the currently selected view
func (s *Sidebar) ActiveView() View {
	return s.items[s.activeIndex].view
}

// MoveUp moves selection up, wrapping to bottom
func (s *Sidebar) MoveUp() {
	s.activeIndex--
	if s.activeIndex < 0 {
		s.activeIndex = len(s.items) - 1
	}
}

// MoveDown moves selection down, wrapping to top
func (s *Sidebar) MoveDown() {
	s.activeIndex++
	if s.activeIndex >= len(s.items) {
		s.activeIndex = 0
	}
}

// View renders the sidebar
func (s *Sidebar) View() string {
	var items []string

	// Docker logo at top
	logoStyle := lipgloss.NewStyle().
		Foreground(theme.DockerBlue).
		Bold(true).
		MarginBottom(1)
	items = append(items, logoStyle.Render(theme.IconDocker))

	// Navigation items
	for i, item := range s.items {
		var style lipgloss.Style
		if i == s.activeIndex {
			if s.focused {
				style = theme.SidebarActiveStyle
			} else {
				style = theme.SidebarItemStyle.
					Foreground(theme.TextPrimary).
					Bold(true)
			}
		} else {
			style = theme.SidebarItemStyle
		}

		itemText := item.icon + " " + item.label
		items = append(items, style.Render(itemText))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	sidebarStyle := theme.SidebarStyle.Height(s.height)
	if s.focused {
		sidebarStyle = sidebarStyle.BorderForeground(theme.DockerBlue)
	}

	return sidebarStyle.Render(content)
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/ui/components/... -v
```

Expected: All tests PASS.

**Step 5: Commit**

```bash
git add internal/ui/components/
git commit -m "feat: add sidebar navigation component"
```

---

## Task 6: Create Container List Component

**Files:**
- Create: `internal/ui/components/container_list.go`
- Create: `internal/ui/components/container_list_test.go`

**Step 1: Write test for container list**

Create `internal/ui/components/container_list_test.go`:

```go
package components_test

import (
	"strings"
	"testing"
	"time"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/components"
)

func TestContainerList_View(t *testing.T) {
	containers := []service.Container{
		{
			ID:     "abc123",
			Name:   "nginx-proxy",
			Image:  "nginx:alpine",
			State:  service.StateRunning,
			Status: "Up 2 hours",
		},
		{
			ID:     "def456",
			Name:   "postgres-db",
			Image:  "postgres:15",
			State:  service.StateStopped,
			Status: "Exited (0) 1 day ago",
		},
	}

	list := components.NewContainerList(containers)
	view := list.View()

	if !strings.Contains(view, "nginx-proxy") {
		t.Error("View() should contain container name")
	}
	if !strings.Contains(view, "postgres-db") {
		t.Error("View() should contain second container name")
	}
}

func TestContainerList_Navigation(t *testing.T) {
	containers := []service.Container{
		{ID: "1", Name: "container-1", State: service.StateRunning},
		{ID: "2", Name: "container-2", State: service.StateRunning},
		{ID: "3", Name: "container-3", State: service.StateStopped},
	}

	list := components.NewContainerList(containers)

	if list.SelectedIndex() != 0 {
		t.Errorf("SelectedIndex() = %d, want 0", list.SelectedIndex())
	}

	list.MoveDown()
	if list.SelectedIndex() != 1 {
		t.Errorf("After MoveDown, SelectedIndex() = %d, want 1", list.SelectedIndex())
	}

	list.MoveUp()
	if list.SelectedIndex() != 0 {
		t.Errorf("After MoveUp, SelectedIndex() = %d, want 0", list.SelectedIndex())
	}
}

func TestContainerList_Expand(t *testing.T) {
	containers := []service.Container{
		{
			ID:      "abc123",
			Name:    "nginx-proxy",
			Image:   "nginx:alpine",
			State:   service.StateRunning,
			Status:  "Up 2 hours",
			Created: time.Now(),
			Ports: []service.PortMapping{
				{HostPort: 80, ContainerPort: 80, Protocol: "tcp"},
			},
		},
	}

	list := components.NewContainerList(containers)

	if list.IsExpanded() {
		t.Error("Container should not be expanded initially")
	}

	list.ToggleExpand()
	if !list.IsExpanded() {
		t.Error("Container should be expanded after toggle")
	}

	view := list.View()
	if !strings.Contains(view, "abc123") {
		t.Error("Expanded view should show container ID")
	}
}

func TestContainerList_SelectedContainer(t *testing.T) {
	containers := []service.Container{
		{ID: "abc123", Name: "nginx-proxy"},
		{ID: "def456", Name: "postgres-db"},
	}

	list := components.NewContainerList(containers)
	selected := list.SelectedContainer()

	if selected == nil {
		t.Fatal("SelectedContainer() returned nil")
	}

	if selected.ID != "abc123" {
		t.Errorf("SelectedContainer().ID = %s, want abc123", selected.ID)
	}

	list.MoveDown()
	selected = list.SelectedContainer()
	if selected.ID != "def456" {
		t.Errorf("After MoveDown, SelectedContainer().ID = %s, want def456", selected.ID)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/ui/components/... -v
```

Expected: FAIL - `NewContainerList` undefined.

**Step 3: Implement container list component**

Create `internal/ui/components/container_list.go`:

```go
package components

import (
	"fmt"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// ContainerList displays a list of containers with inline expansion
type ContainerList struct {
	containers    []service.Container
	selectedIndex int
	expandedIndex int // -1 if none expanded
	focused       bool
	width         int
	height        int
	actionIndex   int // which action button is selected
	actionsFocused bool
}

// NewContainerList creates a new container list
func NewContainerList(containers []service.Container) *ContainerList {
	return &ContainerList{
		containers:    containers,
		selectedIndex: 0,
		expandedIndex: -1,
		focused:       false,
	}
}

// SetContainers updates the container list
func (l *ContainerList) SetContainers(containers []service.Container) {
	l.containers = containers
	if l.selectedIndex >= len(containers) {
		l.selectedIndex = max(0, len(containers)-1)
	}
}

// SetSize sets the component dimensions
func (l *ContainerList) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// SetFocused sets whether the list is focused
func (l *ContainerList) SetFocused(focused bool) {
	l.focused = focused
}

// IsFocused returns whether the list is focused
func (l *ContainerList) IsFocused() bool {
	return l.focused
}

// SelectedIndex returns the currently selected index
func (l *ContainerList) SelectedIndex() int {
	return l.selectedIndex
}

// SelectedContainer returns the currently selected container
func (l *ContainerList) SelectedContainer() *service.Container {
	if len(l.containers) == 0 {
		return nil
	}
	return &l.containers[l.selectedIndex]
}

// IsExpanded returns whether the selected container is expanded
func (l *ContainerList) IsExpanded() bool {
	return l.expandedIndex == l.selectedIndex
}

// MoveUp moves selection up
func (l *ContainerList) MoveUp() {
	if l.selectedIndex > 0 {
		l.selectedIndex--
	}
}

// MoveDown moves selection down
func (l *ContainerList) MoveDown() {
	if l.selectedIndex < len(l.containers)-1 {
		l.selectedIndex++
	}
}

// ToggleExpand toggles expansion of selected container
func (l *ContainerList) ToggleExpand() {
	if l.expandedIndex == l.selectedIndex {
		l.expandedIndex = -1
		l.actionsFocused = false
	} else {
		l.expandedIndex = l.selectedIndex
		l.actionIndex = 0
	}
}

// SetActionsFocused sets whether actions are focused
func (l *ContainerList) SetActionsFocused(focused bool) {
	l.actionsFocused = focused
}

// ActionsFocused returns whether actions are focused
func (l *ContainerList) ActionsFocused() bool {
	return l.actionsFocused && l.expandedIndex >= 0
}

// MoveActionLeft moves action selection left
func (l *ContainerList) MoveActionLeft() {
	if l.actionIndex > 0 {
		l.actionIndex--
	}
}

// MoveActionRight moves action selection right
func (l *ContainerList) MoveActionRight() {
	actions := l.getActions()
	if l.actionIndex < len(actions)-1 {
		l.actionIndex++
	}
}

// SelectedAction returns the currently selected action
func (l *ContainerList) SelectedAction() string {
	actions := l.getActions()
	if l.actionIndex >= 0 && l.actionIndex < len(actions) {
		return actions[l.actionIndex]
	}
	return ""
}

func (l *ContainerList) getActions() []string {
	if l.expandedIndex < 0 {
		return nil
	}
	c := l.containers[l.expandedIndex]
	if c.State == service.StateRunning {
		return []string{"Logs", "Shell", "Stop", "Restart", "Remove"}
	}
	return []string{"Start", "Remove"}
}

// RunningCount returns count of running containers
func (l *ContainerList) RunningCount() int {
	count := 0
	for _, c := range l.containers {
		if c.State == service.StateRunning {
			count++
		}
	}
	return count
}

// StoppedCount returns count of stopped containers
func (l *ContainerList) StoppedCount() int {
	count := 0
	for _, c := range l.containers {
		if c.State == service.StateStopped {
			count++
		}
	}
	return count
}

// View renders the container list
func (l *ContainerList) View() string {
	if len(l.containers) == 0 {
		return theme.HelpStyle.Render("No containers found")
	}

	var lines []string

	// Header
	header := fmt.Sprintf("Containers (%d running, %d stopped)",
		l.RunningCount(), l.StoppedCount())
	lines = append(lines, theme.HeaderStyle.Render(header))
	lines = append(lines, "")

	for i, c := range l.containers {
		isSelected := i == l.selectedIndex
		isExpanded := i == l.expandedIndex

		// Container row
		line := l.renderContainerRow(c, isSelected, isExpanded)
		lines = append(lines, line)

		// Expanded details
		if isExpanded {
			details := l.renderContainerDetails(c)
			lines = append(lines, details)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (l *ContainerList) renderContainerRow(c service.Container, selected, expanded bool) string {
	// Status icon
	var icon string
	if expanded {
		icon = theme.IconExpanded
	} else {
		icon = theme.IconCollapsed
	}

	// State indicator
	var stateIcon string
	var stateStyle lipgloss.Style
	switch c.State {
	case service.StateRunning:
		stateIcon = theme.IconRunning
		stateStyle = theme.StatusRunningStyle
	case service.StateStopped:
		stateIcon = theme.IconStopped
		stateStyle = theme.StatusStoppedStyle
	default:
		stateIcon = theme.IconStopped
		stateStyle = theme.StatusStoppedStyle
	}

	// Format ports
	portsStr := ""
	if len(c.Ports) > 0 {
		for i, p := range c.Ports {
			if i > 0 {
				portsStr += ", "
			}
			portsStr += fmt.Sprintf("%d:%d", p.HostPort, p.ContainerPort)
			if i >= 1 { // Only show first 2 ports
				if len(c.Ports) > 2 {
					portsStr += "..."
				}
				break
			}
		}
	}

	// Build row
	name := lipgloss.NewStyle().Width(20).Render(c.Name)
	state := stateStyle.Render(stateIcon + " " + string(c.State))
	stateCol := lipgloss.NewStyle().Width(14).Render(state)

	row := fmt.Sprintf("%s  %s %s   %s", icon, name, stateCol, portsStr)

	if selected && l.focused {
		return theme.ListItemSelectedStyle.Render(row)
	}
	return theme.ListItemStyle.Render(row)
}

func (l *ContainerList) renderContainerDetails(c service.Container) string {
	var lines []string

	// Tree-style details
	addDetail := func(label, value string, isLast bool) {
		prefix := "   ├─ "
		if isLast && !l.actionsFocused {
			prefix = "   └─ "
		}
		line := theme.DetailLabelStyle.Render(prefix+label+": ") +
			theme.DetailValueStyle.Render(value)
		lines = append(lines, line)
	}

	addDetail("ID", c.ID[:12], false)
	addDetail("Image", c.Image, false)
	addDetail("Status", c.Status, false)

	if len(c.Ports) > 0 {
		ports := ""
		for i, p := range c.Ports {
			if i > 0 {
				ports += ", "
			}
			ports += fmt.Sprintf("%d→%d", p.HostPort, p.ContainerPort)
		}
		addDetail("Ports", ports, false)
	}

	if len(c.Mounts) > 0 {
		mounts := fmt.Sprintf("%d volume(s)", len(c.Mounts))
		addDetail("Mounts", mounts, true)
	} else {
		// Re-render last item with └
		if len(lines) > 0 {
			last := lines[len(lines)-1]
			lines[len(lines)-1] = "   └─" + last[6:]
		}
	}

	// Action buttons
	actions := l.getActions()
	var buttons []string
	for i, action := range actions {
		var style lipgloss.Style
		if l.actionsFocused && i == l.actionIndex {
			style = theme.ActionButtonActiveStyle
		} else {
			style = theme.ActionButtonStyle
		}
		buttons = append(buttons, style.Render("["+action+"]"))
	}

	actionsLine := "   └─ " + lipgloss.JoinHorizontal(lipgloss.Left, buttons...)
	lines = append(lines, actionsLine)

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/ui/components/... -v
```

Expected: All tests PASS.

**Step 5: Commit**

```bash
git add internal/ui/components/container_list.go internal/ui/components/container_list_test.go
git commit -m "feat: add container list component with inline expansion"
```

---

## Task 7: Create Image List Component

**Files:**
- Create: `internal/ui/components/image_list.go`

**Step 1: Implement image list component**

Create `internal/ui/components/image_list.go`:

```go
package components

import (
	"fmt"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// ImageList displays a list of images with inline expansion
type ImageList struct {
	images        []service.Image
	selectedIndex int
	expandedIndex int
	focused       bool
	width         int
	height        int
	actionIndex   int
	actionsFocused bool
}

// NewImageList creates a new image list
func NewImageList(images []service.Image) *ImageList {
	return &ImageList{
		images:        images,
		selectedIndex: 0,
		expandedIndex: -1,
		focused:       false,
	}
}

// SetImages updates the image list
func (l *ImageList) SetImages(images []service.Image) {
	l.images = images
	if l.selectedIndex >= len(images) {
		l.selectedIndex = max(0, len(images)-1)
	}
}

// SetSize sets the component dimensions
func (l *ImageList) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// SetFocused sets whether the list is focused
func (l *ImageList) SetFocused(focused bool) {
	l.focused = focused
}

// IsFocused returns whether the list is focused
func (l *ImageList) IsFocused() bool {
	return l.focused
}

// SelectedIndex returns the currently selected index
func (l *ImageList) SelectedIndex() int {
	return l.selectedIndex
}

// SelectedImage returns the currently selected image
func (l *ImageList) SelectedImage() *service.Image {
	if len(l.images) == 0 {
		return nil
	}
	return &l.images[l.selectedIndex]
}

// IsExpanded returns whether the selected image is expanded
func (l *ImageList) IsExpanded() bool {
	return l.expandedIndex == l.selectedIndex
}

// MoveUp moves selection up
func (l *ImageList) MoveUp() {
	if l.selectedIndex > 0 {
		l.selectedIndex--
	}
}

// MoveDown moves selection down
func (l *ImageList) MoveDown() {
	if l.selectedIndex < len(l.images)-1 {
		l.selectedIndex++
	}
}

// ToggleExpand toggles expansion of selected image
func (l *ImageList) ToggleExpand() {
	if l.expandedIndex == l.selectedIndex {
		l.expandedIndex = -1
		l.actionsFocused = false
	} else {
		l.expandedIndex = l.selectedIndex
		l.actionIndex = 0
	}
}

// SetActionsFocused sets whether actions are focused
func (l *ImageList) SetActionsFocused(focused bool) {
	l.actionsFocused = focused
}

// ActionsFocused returns whether actions are focused
func (l *ImageList) ActionsFocused() bool {
	return l.actionsFocused && l.expandedIndex >= 0
}

// MoveActionLeft moves action selection left
func (l *ImageList) MoveActionLeft() {
	if l.actionIndex > 0 {
		l.actionIndex--
	}
}

// MoveActionRight moves action selection right
func (l *ImageList) MoveActionRight() {
	if l.actionIndex < 1 { // Only 2 actions: Inspect, Remove
		l.actionIndex++
	}
}

// SelectedAction returns the currently selected action
func (l *ImageList) SelectedAction() string {
	actions := []string{"Inspect", "Remove"}
	if l.actionIndex >= 0 && l.actionIndex < len(actions) {
		return actions[l.actionIndex]
	}
	return ""
}

// formatSize formats bytes to human readable size
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// View renders the image list
func (l *ImageList) View() string {
	if len(l.images) == 0 {
		return theme.HelpStyle.Render("No images found")
	}

	var lines []string

	// Header
	header := fmt.Sprintf("Images (%d total)", len(l.images))
	lines = append(lines, theme.HeaderStyle.Render(header))
	lines = append(lines, "")

	for i, img := range l.images {
		isSelected := i == l.selectedIndex
		isExpanded := i == l.expandedIndex

		line := l.renderImageRow(img, isSelected, isExpanded)
		lines = append(lines, line)

		if isExpanded {
			details := l.renderImageDetails(img)
			lines = append(lines, details)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (l *ImageList) renderImageRow(img service.Image, selected, expanded bool) string {
	var icon string
	if expanded {
		icon = theme.IconExpanded
	} else {
		icon = theme.IconCollapsed
	}

	repo := lipgloss.NewStyle().Width(18).Render(img.Repo)
	tag := lipgloss.NewStyle().Width(14).Foreground(theme.TextSecondary).Render(img.Tag)
	size := lipgloss.NewStyle().Width(10).Render(formatSize(img.Size))

	row := fmt.Sprintf("%s  %s %s %s", icon, repo, tag, size)

	if img.Dangling {
		row += " " + theme.StatusErrorStyle.Render(theme.IconWarning+" dangling")
	}

	if selected && l.focused {
		return theme.ListItemSelectedStyle.Render(row)
	}
	return theme.ListItemStyle.Render(row)
}

func (l *ImageList) renderImageDetails(img service.Image) string {
	var lines []string

	addDetail := func(label, value string) {
		prefix := "   ├─ "
		line := theme.DetailLabelStyle.Render(prefix+label+": ") +
			theme.DetailValueStyle.Render(value)
		lines = append(lines, line)
	}

	// Truncate ID for display
	displayID := img.ID
	if len(displayID) > 19 {
		displayID = displayID[:19] + "..."
	}
	addDetail("ID", displayID)
	addDetail("Created", img.Created.Format("2006-01-02"))
	addDetail("Size", formatSize(img.Size))

	if len(img.UsedBy) > 0 {
		usedBy := fmt.Sprintf("%d container(s)", len(img.UsedBy))
		addDetail("Used by", usedBy)
	}

	// Action buttons
	actions := []string{"Inspect", "Remove"}
	var buttons []string
	for i, action := range actions {
		var style lipgloss.Style
		if l.actionsFocused && i == l.actionIndex {
			style = theme.ActionButtonActiveStyle
		} else {
			style = theme.ActionButtonStyle
		}
		buttons = append(buttons, style.Render("["+action+"]"))
	}

	actionsLine := "   └─ " + lipgloss.JoinHorizontal(lipgloss.Left, buttons...)
	lines = append(lines, actionsLine)

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
```

**Step 2: Verify compilation**

```bash
go build ./internal/ui/components/...
```

Expected: No errors.

**Step 3: Commit**

```bash
git add internal/ui/components/image_list.go
git commit -m "feat: add image list component with inline expansion"
```

---

## Task 8: Create Volume List Component

**Files:**
- Create: `internal/ui/components/volume_list.go`

**Step 1: Implement volume list component**

Create `internal/ui/components/volume_list.go`:

```go
package components

import (
	"fmt"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// VolumeList displays a list of volumes with inline expansion
type VolumeList struct {
	volumes       []service.Volume
	selectedIndex int
	expandedIndex int
	focused       bool
	width         int
	height        int
	actionIndex   int
	actionsFocused bool
}

// NewVolumeList creates a new volume list
func NewVolumeList(volumes []service.Volume) *VolumeList {
	return &VolumeList{
		volumes:       volumes,
		selectedIndex: 0,
		expandedIndex: -1,
		focused:       false,
	}
}

// SetVolumes updates the volume list
func (l *VolumeList) SetVolumes(volumes []service.Volume) {
	l.volumes = volumes
	if l.selectedIndex >= len(volumes) {
		l.selectedIndex = max(0, len(volumes)-1)
	}
}

// SetSize sets the component dimensions
func (l *VolumeList) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// SetFocused sets whether the list is focused
func (l *VolumeList) SetFocused(focused bool) {
	l.focused = focused
}

// IsFocused returns whether the list is focused
func (l *VolumeList) IsFocused() bool {
	return l.focused
}

// SelectedIndex returns the currently selected index
func (l *VolumeList) SelectedIndex() int {
	return l.selectedIndex
}

// SelectedVolume returns the currently selected volume
func (l *VolumeList) SelectedVolume() *service.Volume {
	if len(l.volumes) == 0 {
		return nil
	}
	return &l.volumes[l.selectedIndex]
}

// IsExpanded returns whether the selected volume is expanded
func (l *VolumeList) IsExpanded() bool {
	return l.expandedIndex == l.selectedIndex
}

// MoveUp moves selection up
func (l *VolumeList) MoveUp() {
	if l.selectedIndex > 0 {
		l.selectedIndex--
	}
}

// MoveDown moves selection down
func (l *VolumeList) MoveDown() {
	if l.selectedIndex < len(l.volumes)-1 {
		l.selectedIndex++
	}
}

// ToggleExpand toggles expansion of selected volume
func (l *VolumeList) ToggleExpand() {
	if l.expandedIndex == l.selectedIndex {
		l.expandedIndex = -1
		l.actionsFocused = false
	} else {
		l.expandedIndex = l.selectedIndex
		l.actionIndex = 0
	}
}

// SetActionsFocused sets whether actions are focused
func (l *VolumeList) SetActionsFocused(focused bool) {
	l.actionsFocused = focused
}

// ActionsFocused returns whether actions are focused
func (l *VolumeList) ActionsFocused() bool {
	return l.actionsFocused && l.expandedIndex >= 0
}

// MoveActionLeft moves action selection left
func (l *VolumeList) MoveActionLeft() {
	if l.actionIndex > 0 {
		l.actionIndex--
	}
}

// MoveActionRight moves action selection right
func (l *VolumeList) MoveActionRight() {
	if l.actionIndex < 2 { // 3 actions: Browse, Inspect, Remove
		l.actionIndex++
	}
}

// SelectedAction returns the currently selected action
func (l *VolumeList) SelectedAction() string {
	actions := []string{"Browse", "Inspect", "Remove"}
	if l.actionIndex >= 0 && l.actionIndex < len(actions) {
		return actions[l.actionIndex]
	}
	return ""
}

// View renders the volume list
func (l *VolumeList) View() string {
	if len(l.volumes) == 0 {
		return theme.HelpStyle.Render("No volumes found")
	}

	var lines []string

	// Header
	header := fmt.Sprintf("Volumes (%d total)", len(l.volumes))
	lines = append(lines, theme.HeaderStyle.Render(header))
	lines = append(lines, "")

	for i, vol := range l.volumes {
		isSelected := i == l.selectedIndex
		isExpanded := i == l.expandedIndex

		line := l.renderVolumeRow(vol, isSelected, isExpanded)
		lines = append(lines, line)

		if isExpanded {
			details := l.renderVolumeDetails(vol)
			lines = append(lines, details)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (l *VolumeList) renderVolumeRow(vol service.Volume, selected, expanded bool) string {
	var icon string
	if expanded {
		icon = theme.IconExpanded
	} else {
		icon = theme.IconCollapsed
	}

	name := lipgloss.NewStyle().Width(22).Render(vol.Name)
	driver := lipgloss.NewStyle().Width(10).Foreground(theme.TextSecondary).Render(vol.Driver)
	size := lipgloss.NewStyle().Width(10).Render(formatSize(vol.Size))

	row := fmt.Sprintf("%s  %s %s %s", icon, name, driver, size)

	if selected && l.focused {
		return theme.ListItemSelectedStyle.Render(row)
	}
	return theme.ListItemStyle.Render(row)
}

func (l *VolumeList) renderVolumeDetails(vol service.Volume) string {
	var lines []string

	addDetail := func(label, value string) {
		prefix := "   ├─ "
		line := theme.DetailLabelStyle.Render(prefix+label+": ") +
			theme.DetailValueStyle.Render(value)
		lines = append(lines, line)
	}

	addDetail("Mount", vol.MountPath)
	addDetail("Created", vol.Created.Format("2006-01-02"))

	if len(vol.UsedBy) > 0 {
		usedBy := fmt.Sprintf("%d container(s)", len(vol.UsedBy))
		addDetail("Used by", usedBy)
	}

	// Action buttons
	actions := []string{"Browse", "Inspect", "Remove"}
	var buttons []string
	for i, action := range actions {
		var style lipgloss.Style
		if l.actionsFocused && i == l.actionIndex {
			style = theme.ActionButtonActiveStyle
		} else {
			style = theme.ActionButtonStyle
		}
		buttons = append(buttons, style.Render("["+action+"]"))
	}

	actionsLine := "   └─ " + lipgloss.JoinHorizontal(lipgloss.Left, buttons...)
	lines = append(lines, actionsLine)

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
```

**Step 2: Verify compilation**

```bash
go build ./internal/ui/components/...
```

Expected: No errors.

**Step 3: Commit**

```bash
git add internal/ui/components/volume_list.go
git commit -m "feat: add volume list component with inline expansion"
```

---

## Task 9: Create Main App Model

**Files:**
- Modify: `cmd/docker-dash/main.go`

**Step 1: Update main.go with full app model**

Replace `cmd/docker-dash/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/components"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FocusZone represents which UI zone has focus
type FocusZone int

const (
	FocusSidebar FocusZone = iota
	FocusList
	FocusActions
)

type model struct {
	client         service.DockerClient
	sidebar        *components.Sidebar
	containerList  *components.ContainerList
	imageList      *components.ImageList
	volumeList     *components.VolumeList
	focusZone      FocusZone
	width          int
	height         int
	err            error
}

func initialModel(client service.DockerClient) model {
	ctx := context.Background()

	// Load initial data
	containers, _ := client.Containers().List(ctx)
	images, _ := client.Images().List(ctx)
	volumes, _ := client.Volumes().List(ctx)

	sidebar := components.NewSidebar()
	sidebar.SetFocused(true)

	containerList := components.NewContainerList(containers)
	imageList := components.NewImageList(images)
	volumeList := components.NewVolumeList(volumes)

	return model{
		client:        client,
		sidebar:       sidebar,
		containerList: containerList,
		imageList:     imageList,
		volumeList:    volumeList,
		focusZone:     FocusSidebar,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		return m, nil

	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.cycleFocusForward()
			return m, nil
		case "shift+tab":
			m.cycleFocusBackward()
			return m, nil
		case "?":
			// TODO: Show help overlay
			return m, nil
		case "r":
			return m, m.refresh()
		}

		// Zone-specific keys
		switch m.focusZone {
		case FocusSidebar:
			return m.updateSidebar(msg)
		case FocusList:
			return m.updateList(msg)
		case FocusActions:
			return m.updateActions(msg)
		}
	}

	return m, nil
}

func (m *model) updateSizes() {
	sidebarWidth := 16
	mainWidth := m.width - sidebarWidth - 2
	contentHeight := m.height - 3 // Status bar

	m.sidebar.SetHeight(contentHeight)
	m.containerList.SetSize(mainWidth, contentHeight)
	m.imageList.SetSize(mainWidth, contentHeight)
	m.volumeList.SetSize(mainWidth, contentHeight)
}

func (m *model) cycleFocusForward() {
	switch m.focusZone {
	case FocusSidebar:
		m.focusZone = FocusList
		m.sidebar.SetFocused(false)
		m.setListFocused(true)
	case FocusList:
		if m.currentListExpanded() {
			m.focusZone = FocusActions
			m.setListFocused(false)
			m.setActionsFocused(true)
		} else {
			m.focusZone = FocusSidebar
			m.setListFocused(false)
			m.sidebar.SetFocused(true)
		}
	case FocusActions:
		m.focusZone = FocusSidebar
		m.setActionsFocused(false)
		m.sidebar.SetFocused(true)
	}
}

func (m *model) cycleFocusBackward() {
	switch m.focusZone {
	case FocusSidebar:
		if m.currentListExpanded() {
			m.focusZone = FocusActions
			m.sidebar.SetFocused(false)
			m.setActionsFocused(true)
		} else {
			m.focusZone = FocusList
			m.sidebar.SetFocused(false)
			m.setListFocused(true)
		}
	case FocusList:
		m.focusZone = FocusSidebar
		m.setListFocused(false)
		m.sidebar.SetFocused(true)
	case FocusActions:
		m.focusZone = FocusList
		m.setActionsFocused(false)
		m.setListFocused(true)
	}
}

func (m *model) setListFocused(focused bool) {
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		m.containerList.SetFocused(focused)
	case components.ViewImages:
		m.imageList.SetFocused(focused)
	case components.ViewVolumes:
		m.volumeList.SetFocused(focused)
	}
}

func (m *model) setActionsFocused(focused bool) {
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		m.containerList.SetActionsFocused(focused)
	case components.ViewImages:
		m.imageList.SetActionsFocused(focused)
	case components.ViewVolumes:
		m.volumeList.SetActionsFocused(focused)
	}
}

func (m *model) currentListExpanded() bool {
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		return m.containerList.IsExpanded()
	case components.ViewImages:
		return m.imageList.IsExpanded()
	case components.ViewVolumes:
		return m.volumeList.IsExpanded()
	}
	return false
}

func (m model) updateSidebar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.sidebar.MoveUp()
	case "down", "j":
		m.sidebar.MoveDown()
	case "enter":
		m.focusZone = FocusList
		m.sidebar.SetFocused(false)
		m.setListFocused(true)
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		return m.updateContainerList(msg)
	case components.ViewImages:
		return m.updateImageList(msg)
	case components.ViewVolumes:
		return m.updateVolumeList(msg)
	}
	return m, nil
}

func (m model) updateContainerList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.containerList.MoveUp()
	case "down", "j":
		m.containerList.MoveDown()
	case "enter":
		m.containerList.ToggleExpand()
	case "l": // Quick key: logs
		// TODO: Open log viewer
	case "s": // Quick key: start/stop
		return m, m.toggleContainer()
	case "x": // Quick key: exec
		// TODO: Open shell
	}
	return m, nil
}

func (m model) updateImageList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.imageList.MoveUp()
	case "down", "j":
		m.imageList.MoveDown()
	case "enter":
		m.imageList.ToggleExpand()
	}
	return m, nil
}

func (m model) updateVolumeList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.volumeList.MoveUp()
	case "down", "j":
		m.volumeList.MoveDown()
	case "enter":
		m.volumeList.ToggleExpand()
	}
	return m, nil
}

func (m model) updateActions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		switch msg.String() {
		case "left", "h":
			m.containerList.MoveActionLeft()
		case "right", "l":
			m.containerList.MoveActionRight()
		case "enter":
			return m, m.executeContainerAction()
		}
	case components.ViewImages:
		switch msg.String() {
		case "left", "h":
			m.imageList.MoveActionLeft()
		case "right", "l":
			m.imageList.MoveActionRight()
		case "enter":
			return m, m.executeImageAction()
		}
	case components.ViewVolumes:
		switch msg.String() {
		case "left", "h":
			m.volumeList.MoveActionLeft()
		case "right", "l":
			m.volumeList.MoveActionRight()
		case "enter":
			return m, m.executeVolumeAction()
		}
	}
	return m, nil
}

func (m model) toggleContainer() tea.Cmd {
	c := m.containerList.SelectedContainer()
	if c == nil {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		if c.State == service.StateRunning {
			m.client.Containers().Stop(ctx, c.ID)
		} else {
			m.client.Containers().Start(ctx, c.ID)
		}
		return refreshMsg{}
	}
}

func (m model) executeContainerAction() tea.Cmd {
	action := m.containerList.SelectedAction()
	c := m.containerList.SelectedContainer()
	if c == nil {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		switch action {
		case "Start":
			m.client.Containers().Start(ctx, c.ID)
		case "Stop":
			m.client.Containers().Stop(ctx, c.ID)
		case "Restart":
			m.client.Containers().Restart(ctx, c.ID)
		case "Remove":
			m.client.Containers().Remove(ctx, c.ID, false)
		case "Logs":
			// TODO: Open log viewer
		case "Shell":
			// TODO: Open shell
		}
		return refreshMsg{}
	}
}

func (m model) executeImageAction() tea.Cmd {
	action := m.imageList.SelectedAction()
	img := m.imageList.SelectedImage()
	if img == nil {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		switch action {
		case "Remove":
			m.client.Images().Remove(ctx, img.ID, false)
		case "Inspect":
			// TODO: Show inspect details
		}
		return refreshMsg{}
	}
}

func (m model) executeVolumeAction() tea.Cmd {
	action := m.volumeList.SelectedAction()
	vol := m.volumeList.SelectedVolume()
	if vol == nil {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		switch action {
		case "Remove":
			m.client.Volumes().Remove(ctx, vol.Name, false)
		case "Browse":
			// TODO: Open file browser
		case "Inspect":
			// TODO: Show inspect details
		}
		return refreshMsg{}
	}
}

type refreshMsg struct{}

func (m model) refresh() tea.Cmd {
	return func() tea.Msg {
		return refreshMsg{}
	}
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Sidebar
	sidebar := m.sidebar.View()

	// Main content
	var mainContent string
	switch m.sidebar.ActiveView() {
	case components.ViewContainers:
		mainContent = m.containerList.View()
	case components.ViewImages:
		mainContent = m.imageList.View()
	case components.ViewVolumes:
		mainContent = m.volumeList.View()
	}

	mainPanel := theme.MainPanelStyle.Render(mainContent)

	// Layout
	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainPanel)

	// Status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, content, statusBar)
}

func (m model) renderStatusBar() string {
	var hints string
	switch m.focusZone {
	case FocusSidebar:
		hints = "↑↓ navigate • Enter select • Tab switch focus • q quit"
	case FocusList:
		hints = "↑↓ navigate • Enter expand • Tab switch focus • ? help"
	case FocusActions:
		hints = "←→ select action • Enter execute • Tab switch focus"
	}

	style := theme.StatusBarStyle.Width(m.width)
	return style.Render(hints)
}

func main() {
	// Use mock client for now
	client := service.NewMockClient()
	defer client.Close()

	p := tea.NewProgram(
		initialModel(client),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 2: Verify it compiles and runs**

```bash
go build ./cmd/docker-dash && ./docker-dash
```

Expected: TUI launches with sidebar and container list. Navigation works. Press 'q' to quit.

**Step 3: Commit**

```bash
git add cmd/docker-dash/main.go
git commit -m "feat: implement main app with sidebar navigation and list views"
```

---

## Task 10: Add Real Docker Client

**Files:**
- Create: `internal/service/local.go`

**Step 1: Implement local Docker client**

Create `internal/service/local.go`:

```go
package service

import (
	"context"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// LocalDockerClient connects to the local Docker daemon
type LocalDockerClient struct {
	cli        *client.Client
	containers *localContainerService
	images     *localImageService
	volumes    *localVolumeService
}

// NewLocalDockerClient creates a client connected to the local Docker socket
func NewLocalDockerClient() (*LocalDockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	c := &LocalDockerClient{cli: cli}
	c.containers = &localContainerService{cli: cli}
	c.images = &localImageService{cli: cli}
	c.volumes = &localVolumeService{cli: cli}

	return c, nil
}

func (c *LocalDockerClient) Containers() ContainerService { return c.containers }
func (c *LocalDockerClient) Images() ImageService         { return c.images }
func (c *LocalDockerClient) Volumes() VolumeService       { return c.volumes }

func (c *LocalDockerClient) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

func (c *LocalDockerClient) Close() error {
	return c.cli.Close()
}

// Local Container Service
type localContainerService struct {
	cli *client.Client
}

func (s *localContainerService) List(ctx context.Context) ([]Container, error) {
	containers, err := s.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	result := make([]Container, len(containers))
	for i, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		state := StateStopped
		switch c.State {
		case "running":
			state = StateRunning
		case "paused":
			state = StatePaused
		case "restarting":
			state = StateRestarting
		}

		ports := make([]PortMapping, 0)
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, PortMapping{
					HostPort:      p.PublicPort,
					ContainerPort: p.PrivatePort,
					Protocol:      p.Type,
				})
			}
		}

		mounts := make([]Mount, len(c.Mounts))
		for j, m := range c.Mounts {
			mounts[j] = Mount{
				Type:        string(m.Type),
				Source:      m.Source,
				Destination: m.Destination,
			}
		}

		result[i] = Container{
			ID:      c.ID,
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			State:   state,
			Created: timeFromUnix(c.Created),
			Ports:   ports,
			Mounts:  mounts,
		}
	}

	return result, nil
}

func (s *localContainerService) Get(ctx context.Context, id string) (*Container, error) {
	c, err := s.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}

	state := StateStopped
	if c.State.Running {
		state = StateRunning
	} else if c.State.Paused {
		state = StatePaused
	} else if c.State.Restarting {
		state = StateRestarting
	}

	return &Container{
		ID:     c.ID,
		Name:   strings.TrimPrefix(c.Name, "/"),
		Image:  c.Config.Image,
		Status: c.State.Status,
		State:  state,
	}, nil
}

func (s *localContainerService) Start(ctx context.Context, id string) error {
	return s.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (s *localContainerService) Stop(ctx context.Context, id string) error {
	return s.cli.ContainerStop(ctx, id, container.StopOptions{})
}

func (s *localContainerService) Restart(ctx context.Context, id string) error {
	return s.cli.ContainerRestart(ctx, id, container.StopOptions{})
}

func (s *localContainerService) Remove(ctx context.Context, id string, force bool) error {
	return s.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
}

func (s *localContainerService) Logs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	return s.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Tail:       opts.Tail,
		Timestamps: opts.Timestamps,
	})
}

// Local Image Service
type localImageService struct {
	cli *client.Client
}

func (s *localImageService) List(ctx context.Context) ([]Image, error) {
	images, err := s.cli.ImageList(ctx, image.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	result := make([]Image, len(images))
	for i, img := range images {
		repo := "<none>"
		tag := "<none>"
		if len(img.RepoTags) > 0 {
			parts := strings.SplitN(img.RepoTags[0], ":", 2)
			repo = parts[0]
			if len(parts) > 1 {
				tag = parts[1]
			}
		}

		result[i] = Image{
			ID:       img.ID,
			Repo:     repo,
			Tag:      tag,
			Size:     img.Size,
			Created:  timeFromUnix(img.Created),
			Dangling: len(img.RepoTags) == 0 || img.RepoTags[0] == "<none>:<none>",
		}
	}

	return result, nil
}

func (s *localImageService) Get(ctx context.Context, id string) (*Image, error) {
	img, _, err := s.cli.ImageInspectWithRaw(ctx, id)
	if err != nil {
		return nil, err
	}

	repo := "<none>"
	tag := "<none>"
	if len(img.RepoTags) > 0 {
		parts := strings.SplitN(img.RepoTags[0], ":", 2)
		repo = parts[0]
		if len(parts) > 1 {
			tag = parts[1]
		}
	}

	return &Image{
		ID:   img.ID,
		Repo: repo,
		Tag:  tag,
		Size: img.Size,
	}, nil
}

func (s *localImageService) Remove(ctx context.Context, id string, force bool) error {
	_, err := s.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: force})
	return err
}

// Local Volume Service
type localVolumeService struct {
	cli *client.Client
}

func (s *localVolumeService) List(ctx context.Context) ([]Volume, error) {
	resp, err := s.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]Volume, len(resp.Volumes))
	for i, v := range resp.Volumes {
		result[i] = Volume{
			Name:      v.Name,
			Driver:    v.Driver,
			MountPath: v.Mountpoint,
			// Note: Size requires disk usage API call, skipping for now
		}
	}

	return result, nil
}

func (s *localVolumeService) Get(ctx context.Context, name string) (*Volume, error) {
	v, err := s.cli.VolumeInspect(ctx, name)
	if err != nil {
		return nil, err
	}

	return &Volume{
		Name:      v.Name,
		Driver:    v.Driver,
		MountPath: v.Mountpoint,
	}, nil
}

func (s *localVolumeService) Remove(ctx context.Context, name string, force bool) error {
	return s.cli.VolumeRemove(ctx, name, force)
}

func (s *localVolumeService) Browse(ctx context.Context, name string, path string) ([]FileEntry, error) {
	// Volume browsing requires running a container to access the volume
	// For now, return empty - this is a complex feature
	return nil, nil
}

// Helper to get containers using a volume
func (s *localVolumeService) getVolumeUsage(ctx context.Context, volumeName string) ([]string, error) {
	containers, err := s.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("volume", volumeName)),
	})
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(containers))
	for i, c := range containers {
		ids[i] = c.ID
	}
	return ids, nil
}

// timeFromUnix converts Unix timestamp to time.Time
func timeFromUnix(unix int64) (t time.Time) {
	return time.Unix(unix, 0)
}
```

**Step 2: Add missing import**

Add time import at top of `internal/service/local.go`:

```go
import (
	"context"
	"io"
	"strings"
	"time"
	// ... rest of imports
)
```

**Step 3: Download Docker SDK**

```bash
go mod tidy
```

**Step 4: Verify compilation**

```bash
go build ./internal/service/...
```

Expected: No errors.

**Step 5: Commit**

```bash
git add internal/service/local.go go.mod go.sum
git commit -m "feat: add local Docker client implementation"
```

---

## Task 11: Update Main to Use Real Docker Client

**Files:**
- Modify: `cmd/docker-dash/main.go`

**Step 1: Update main() to try real client first**

In `cmd/docker-dash/main.go`, update the `main()` function:

```go
func main() {
	// Try to connect to real Docker, fall back to mock
	var client service.DockerClient
	var err error

	realClient, err := service.NewLocalDockerClient()
	if err == nil {
		// Test connection
		if pingErr := realClient.Ping(context.Background()); pingErr == nil {
			client = realClient
		} else {
			realClient.Close()
		}
	}

	// Fall back to mock client
	if client == nil {
		client = service.NewMockClient()
	}
	defer client.Close()

	p := tea.NewProgram(
		initialModel(client),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 2: Add context import**

Ensure `context` is imported at top of file.

**Step 3: Build and test**

```bash
go build ./cmd/docker-dash && ./docker-dash
```

Expected: If Docker is running, shows real containers. Otherwise uses mock data.

**Step 4: Commit**

```bash
git add cmd/docker-dash/main.go
git commit -m "feat: connect to real Docker daemon with mock fallback"
```

---

## Task 12: Add Refresh on Data Changes

**Files:**
- Modify: `cmd/docker-dash/main.go`

**Step 1: Handle refresh message to reload data**

Add to the `Update` method's switch statement:

```go
case refreshMsg:
	ctx := context.Background()
	containers, _ := m.client.Containers().List(ctx)
	images, _ := m.client.Images().List(ctx)
	volumes, _ := m.client.Volumes().List(ctx)

	m.containerList.SetContainers(containers)
	m.imageList.SetImages(images)
	m.volumeList.SetVolumes(volumes)
	return m, nil
```

**Step 2: Verify it works**

```bash
go build ./cmd/docker-dash && ./docker-dash
```

Expected: Pressing 'r' refreshes the data. After container start/stop, list updates.

**Step 3: Commit**

```bash
git add cmd/docker-dash/main.go
git commit -m "feat: add data refresh on actions and manual trigger"
```

---

## Summary

This plan implements the core docker-dash TUI with:

1. **Project setup** - Go module with Bubble Tea
2. **Service layer** - Interfaces for Docker operations with mock and real implementations
3. **Theme** - Docker Desktop inspired colors and Nerd Font icons
4. **Components** - Sidebar, Container List, Image List, Volume List with inline expansion
5. **Main app** - Focus zone navigation, keyboard handling, actions

**Not implemented in this phase:**
- Log viewer (full-screen overlay)
- Shell exec (requires PTY handling)
- Volume browser (requires container to access volume)
- Configuration file loading
- Help overlay

These can be added in follow-up tasks.
