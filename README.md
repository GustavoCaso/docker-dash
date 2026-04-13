# docker-dash

[![Go version](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Tests](https://github.com/GustavoCaso/docker-dash/actions/workflows/go-tests.yml/badge.svg)](https://github.com/GustavoCaso/docker-dash/actions/workflows/go-tests.yml)
[![Lint](https://github.com/GustavoCaso/docker-dash/actions/workflows/go-lint.yml/badge.svg)](https://github.com/GustavoCaso/docker-dash/actions/workflows/go-lint.yml)
[![License](https://img.shields.io/github/license/GustavoCaso/docker-dash)](./LICENSE)

A keyboard-first Docker dashboard for the terminal.

Manage containers, images, volumes, networks, and compose projects without leaving your shell.

![docker-dash overview](./assets/overview.gif)

## Why docker-dash?

Switching between Docker commands and a GUI breaks focus. `docker-dash` keeps the feedback loop tight with a fast terminal interface for inspecting, filtering, and managing your Docker environment.

- Stay in the terminal instead of bouncing between tools
- Troubleshoot faster with logs, details, and exec access close at hand
- Work with local or remote Docker hosts, including SSH-based setups
- Manage compose projects alongside containers, images, volumes, and networks
- Develop without a Docker daemon thanks to the built-in mock client fallback

## What you can do

- Browse and manage containers, images, volumes, networks, and compose projects
- Inspect container logs, stats, details, and files interactively
- Start, stop, restart, and remove containers and compose projects
- Exec into running containers without leaving the TUI
- Filter resources quickly and inspect image layers
- Clean up unused resources with prune actions
- Check for image updates from the registry on a configurable interval and pull updates in one keystroke

## Quick start

### Install

Homebrew:

```bash
brew tap GustavoCaso/tap
brew install docker-dash
```

Go:

```bash
go install github.com/GustavoCaso/docker-dash/cmd/docker-dash@latest
```

Build from source:

```bash
make build
```

### Run

```bash
docker-dash
```

### A few things to try

1. Move between sections with `← / →`
2. Filter the current list with `/`
3. Refresh with `r` and open help with `?`
4. Manage resources with contextual actions such as `s`, `D`, `P`, `+`, and `c`

## Great fit for

- Developers who live in the terminal
- Docker users who want a faster feedback loop than memorizing many commands
- Remote and homelab workflows where SSH Docker hosts are common
- Anyone who wants quick visibility into running services, logs, and container state

## Common workflows

### Troubleshoot a container

Use the containers view to inspect logs and details, restart with `ctrl+R`, or start and stop with `s`.

### Clean up Docker resources

Prune unused containers, images, networks, and volumes directly from the relevant section with `P`.

### Connect to a remote Docker host

Pass a Docker host on the command line:

```bash
docker-dash --docker.host ssh://user@example-host
```

Or set it in your config file for everyday use.

## Configuration

By default, `docker-dash` looks for a config file at `~/.config/docker-dash/config.toml`.

```toml
[docker]
# Docker daemon URL. Accepts tcp, unix, and ssh schemes.
# Defaults to the DOCKER_HOST environment variable.
host = "ssh://pi5@pi5"

[refresh]
# Auto-refresh interval
# Examples: "500ms", "5s", "1m", "2m30s"
interval = "10s"

[debug]
enable = false

[update_check]
# Check Docker Hub for newer image versions on a configurable interval.
# When an update is available, a ⬆ icon appears next to the image in the list.
# Press "u" in the Images section to pull the update directly.
enabled = false
# Polling interval. Examples: "30m", "1h", "6h"
interval = "1h"
```

### CLI flags

| Flag | Description |
|---|---|
| `--config` | Path to config file (default: `~/.config/docker-dash/config.toml`) |
| `--docker.host` | Docker daemon URL (overrides config file) |
| `--refresh.interval` | Auto-refresh interval (overrides config file) |
| `--debug` | Enable debug logging (overrides config file) |

CLI flags take precedence over config values.

### Debug mode

When debug mode is enabled, `docker-dash` writes a debug log file to the system temporary directory.

## Keybindings

Press `?` in the app to see the full keymap. The most commonly used bindings are:

| Key | Action |
|---|---|
| `← / →` | Switch section |
| `↑ / ↓` | Move selection |
| `shift+← / shift+→` | Switch panels |
| `j / k` | Scroll down/up |
| `/` | Filter |
| `r / ctrl+r` | Refresh current view / refresh all |
| `?` | Toggle help |
| `q` | Quit |

Contextual actions depend on the active section. For example:

- `s` starts or stops containers and compose projects
- `D` deletes containers and networks, or brings compose projects down
- `P` prunes unused resources
- `+` pulls an image
- `c` creates and runs a container from an image
- `u` pulls an image update (Images section) or brings a compose project up (Compose section)

<details>
<summary>Full keybindings by section</summary>

### Images

| Key | Action |
|---|---|
| `d` | Delete image |
| `P` | Prune all unused images |
| `c` | Create and run container |
| `+` | Pull image |
| `u` | Pull image update (requires `update_check.enabled = true`; ⬆ icon indicates update available) |

### Containers

| Key | Action |
|---|---|
| `D` | Delete container |
| `P` | Prune unused container |
| `s` | Start/stop |
| `ctrl+R` | Restart |

### Volumes

| Key | Action |
|---|---|
| `d` | Delete volume |
| `P` | Prune all unused volumes |

### Networks

| Key | Action |
|---|---|
| `P` | Prune unused networks |
| `D` | Delete network |

### Compose

| Key | Action |
|---|---|
| `u` | Compose up |
| `D` | Compose down |
| `s` | Start/stop project |
| `ctrl+R` | Restart project |

### Global

| Key | Action |
|---|---|
| `r` | Refresh |
| `ctrl+r` | Refresh all |

</details>

## Development

```bash
make test       # Run tests
make lint       # Run linter
make build      # Build binary
```

Tests use a mock Docker client, so no Docker daemon is required.

## Requirements

- Go 1.26+
- Docker (optional — falls back to mock data without it)
