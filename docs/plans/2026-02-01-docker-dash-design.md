# Docker Dash - Design Document

A Docker Desktop-inspired TUI for developers, built with Go and Bubble Tea.

## Overview

`docker-dash` is a terminal-based Docker management tool that provides a familiar Docker Desktop experience for developers who prefer working in the terminal. It offers full visibility and control over containers, images, and volumes through an intuitive keyboard-driven interface.

**Target Audience:** Developers already comfortable with Docker CLI who want a faster, more visual workflow.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Bubble Tea App                   │
├─────────────────────────────────────────────────────┤
│  UI Layer (Views)                                   │
│  ├── Sidebar Component                              │
│  ├── Container List/Detail View                     │
│  ├── Image List/Detail View                         │
│  └── Volume List/Detail View (with file browser)   │
├─────────────────────────────────────────────────────┤
│  State Layer (Models)                               │
│  ├── App State (active view, selected items)        │
│  ├── Container State                                │
│  ├── Image State                                    │
│  └── Volume State                                   │
├─────────────────────────────────────────────────────┤
│  Service Layer (Docker Client Abstraction)          │
│  ├── ContainerService interface                     │
│  ├── ImageService interface                         │
│  └── VolumeService interface                        │
├─────────────────────────────────────────────────────┤
│  Docker Client (github.com/docker/docker/client)    │
│  └── Currently: Local socket                        │
│  └── Future: Remote hosts, contexts                 │
└─────────────────────────────────────────────────────┘
```

### Directory Structure

```
docker-dash/
├── cmd/docker-dash/main.go
├── internal/
│   ├── ui/           # Bubble Tea components
│   ├── state/        # Application state management
│   ├── service/      # Docker service interfaces + implementations
│   └── config/       # Configuration loading (XDG)
└── go.mod
```

## UI Layout

```
┌──────────┬─────────────────────────────────────────────┐
│ 󰡨        │  Containers (3 running, 2 stopped)          │
│          ├─────────────────────────────────────────────┤
│ 󰆍 Cont.  │  ▶  nginx-proxy         running    2h ago  │
│          │  ▼  api-server           running    5m ago  │
│ 󰋊 Images │  │  ├─ ID: a1b2c3d4                         │
│          │  │  ├─ Image: node:18-alpine                │
│ 󱁤 Volumes│  │  ├─ Ports: 3000→3000                     │
│          │  │  └─ [Logs] [Shell] [Stop] [Remove]       │
│          │  ▶  postgres-db          running    2h ago  │
│          │  ■  old-container        stopped    3d ago  │
│          │  ■  test-runner          stopped    1d ago  │
├──────────┼─────────────────────────────────────────────┤
│ ↑↓ nav   │  Press ? for help                          │
└──────────┴─────────────────────────────────────────────┘
```

### Components

- **Sidebar (fixed width ~12 chars):** Nerd Font icons + labels, highlighted selection, shows active view
- **Main List:** Resource list with inline expansion. `▶` collapsed, `▼` expanded, status icons indicate state
- **Detail Panel:** Appears inline when item expanded. Shows metadata + action buttons
- **Status Bar:** Keyboard hints, context-sensitive help

### Visual Styling

- Primary accent: Docker blue (`#1D63ED`)
- Running status: Green
- Stopped status: Gray
- Error status: Red
- Icons: Nerd Fonts
- Borders: Rounded corners using Lip Gloss

## Keyboard Navigation

### Focus Zones

Tab cycles between: Sidebar → Main List → Detail Actions → Sidebar...

### Global Keys

| Key | Action |
|-----|--------|
| `Tab` | Move focus to next zone |
| `Shift+Tab` | Move focus to previous zone |
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit application |
| `/` | Search/filter current list |
| `Esc` | Close overlay / collapse expanded / clear filter |
| `r` | Refresh current view |

### List Navigation (when Main List focused)

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection |
| `Enter` | Expand/collapse selected item |
| `Home` / `End` | Jump to first/last item |
| `Page Up` / `Page Down` | Scroll by page |

### Detail Actions (when actions focused)

| Key | Action |
|-----|--------|
| `←` / `→` | Move between action buttons |
| `Enter` | Execute selected action |

### Container-Specific Quick Keys

| Key | Action |
|-----|--------|
| `l` | View logs (opens log viewer) |
| `s` | Start/Stop toggle |
| `x` | Exec into shell |

## Features

### Containers

**List Display:**
```
▶  nginx-proxy         running    2h ago   80:80, 443:443
```

**Expanded Detail:**
```
▼  api-server           running    5m ago
   ├─ ID:      a1b2c3d4e5f6
   ├─ Image:   node:18-alpine
   ├─ Status:  Up 5 minutes (healthy)
   ├─ Ports:   3000→3000, 9229→9229
   ├─ Mounts:  2 volumes, 1 bind mount
   └─ [Logs] [Shell] [Stop] [Restart] [Remove]
```

**Actions:**

| Action | Behavior |
|--------|----------|
| Logs | Full-screen log viewer with follow mode, search, timestamps toggle |
| Shell | Prompts for shell (`/bin/sh` or `/bin/bash`), opens interactive terminal |
| Start/Stop | Toggle based on current state, with confirmation for stop |
| Restart | Restarts container, shows brief "Restarting..." indicator |
| Remove | Confirmation required, option to force-remove if running |

**Log Viewer Features:**
- Real-time follow (tail -f style)
- Pause/resume streaming
- Search within logs (`/` key)
- Toggle timestamps
- Copy selection to clipboard
- `Esc` to return to container list

### Images

**List Display:**
```
▶  node                 18-alpine     152 MB    3 days ago
▶  postgres             15            412 MB    1 week ago
▶  <none>               <none>        89 MB     2 weeks ago   ⚠ dangling
```

**Expanded Detail:**
```
▼  node                 18-alpine     152 MB
   ├─ ID:        sha256:a1b2c3d4...
   ├─ Created:   2024-01-15
   ├─ Size:      152 MB (45 MB compressed)
   ├─ Used by:   api-server, test-runner (2 containers)
   └─ [Inspect] [Remove]
```

**Actions:**
- **Inspect:** Show full image metadata (labels, env, entrypoint, layers)
- **Remove:** Confirmation required, warns if containers are using it

### Volumes

**List Display:**
```
▶  postgres_data        local         2.3 GB    2 weeks ago
▶  redis_cache          local         156 MB    3 days ago
```

**Expanded Detail:**
```
▼  postgres_data        local         2.3 GB
   ├─ Mount:     /var/lib/docker/volumes/postgres_data/_data
   ├─ Used by:   postgres-db (1 container)
   └─ [Browse] [Inspect] [Remove]
```

**Volume Browser:**
```
 postgres_data
├──  base/
├──  global/
├──  pg_wal/
└──  postgresql.conf     4.2 KB
```

Read-only file-tree navigation showing file sizes and permissions.

## Service Layer

Interfaces designed for future remote Docker support:

```go
type DockerClient interface {
    Containers() ContainerService
    Images() ImageService
    Volumes() VolumeService
    Ping(ctx context.Context) error
}

type ContainerService interface {
    List(ctx context.Context) ([]Container, error)
    Get(ctx context.Context, id string) (*Container, error)
    Start(ctx context.Context, id string) error
    Stop(ctx context.Context, id string) error
    Restart(ctx context.Context, id string) error
    Remove(ctx context.Context, id string, force bool) error
    Logs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error)
    Exec(ctx context.Context, id string, cmd []string) (ExecSession, error)
    Stats(ctx context.Context, id string) (*Stats, error)
}

type ImageService interface {
    List(ctx context.Context) ([]Image, error)
    Get(ctx context.Context, id string) (*Image, error)
    Remove(ctx context.Context, id string, force bool) error
}

type VolumeService interface {
    List(ctx context.Context) ([]Volume, error)
    Get(ctx context.Context, name string) (*Volume, error)
    Remove(ctx context.Context, name string, force bool) error
    Browse(ctx context.Context, name string, path string) ([]FileEntry, error)
}
```

**Implementation Strategy:**
- `LocalDockerClient` implements `DockerClient` using `/var/run/docker.sock`
- Future: `RemoteDockerClient` implements same interfaces over TCP
- Factory function reads config to decide which to instantiate

**Error Handling:**
- All errors wrapped with context
- UI shows user-friendly messages, logs full errors for debugging

## Configuration

**Location:** `~/.config/docker-dash/config.yaml`

```yaml
# Docker connection
docker:
  host: "unix:///var/run/docker.sock"  # Future: tcp://remote:2375

# UI preferences
ui:
  refresh_interval: 2s          # How often to poll Docker
  show_timestamps: true         # Show timestamps in logs by default
  confirm_destructive: true     # Confirm before remove/stop actions
  default_shell: "/bin/sh"      # Default shell for exec

# Keybindings (future enhancement)
# keys:
#   nav_up: ["k", "up"]
#   nav_down: ["j", "down"]
```

**Config Loading Priority:**
1. CLI flags (highest)
2. Environment variables (`DOCKER_DASH_*`)
3. Config file
4. Built-in defaults (lowest)

**CLI Flags:**
```
docker-dash                    # Start TUI
docker-dash --version          # Print version
docker-dash --config PATH      # Use custom config file
docker-dash --docker-host URL  # Override Docker host
```

## Dependencies

- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/charmbracelet/bubbles` - Common components
- `github.com/docker/docker/client` - Docker SDK

## Out of Scope (v1)

- Image pull/build
- Network management
- Docker Compose projects
- Remote Docker hosts
- Custom keybindings

These features are intentionally deferred to keep the initial scope manageable.
