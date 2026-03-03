package containers

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

type detailsMsg struct {
	output string
	err    error
}

type detailsPanel struct {
	service  client.ContainerService
	viewport viewport.Model
}

// NewDetailsPanel creates a new panel.Panel that fetches and renders container details.
func NewDetailsPanel(svc client.ContainerService) panel.Panel {
	return &detailsPanel{service: svc, viewport: viewport.New(0, 0)}
}

func (d *detailsPanel) Init(containerID string) tea.Cmd {
	return tea.Batch(d.fetchCmd(containerID), d.extendHelpCmd())
}

func (d *detailsPanel) Update(msg tea.Msg) tea.Cmd {
	dm, ok := msg.(detailsMsg)
	if ok {
		if dm.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{Message: dm.err.Error(), IsError: true}
			}
		}
		d.viewport.SetContent(dm.output)
		return nil
	}

	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)

	return cmd
}

func (d *detailsPanel) View() string {
	return d.viewport.View()
}

func (d *detailsPanel) Close() tea.Cmd {
	d.viewport.SetContent("")
	return func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} }
}

func (d *detailsPanel) SetSize(width, height int) {
	d.viewport.Width = width
	d.viewport.Height = height
}

func (d *detailsPanel) fetchCmd(containerID string) tea.Cmd {
	svc := d.service
	return func() tea.Msg {
		ctx := context.Background()
		container, err := svc.Get(ctx, containerID)
		if err != nil {
			return detailsMsg{err: fmt.Errorf("error getting container details: %w", err)}
		}
		return detailsMsg{output: formatDetails(container)}
	}
}

func formatDetails(c *client.Container) string {
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "Container: %s\n", c.Name)
	b.WriteString("═══════════════════════\n\n")

	// General
	b.WriteString("=== General ===\n")
	fmt.Fprintf(&b, "ID:      %s\n", helper.ShortID(c.ID))
	fmt.Fprintf(&b, "Image:   %s\n", c.Image)
	fmt.Fprintf(&b, "Status:  %s\n", c.Status)
	stateStyle := theme.GetContainerStatusStyle(string(c.State))
	stateIcon := theme.GetContainerStatusIcon(string(c.State))
	fmt.Fprintf(&b, "State:   %s\n", stateStyle.Render(stateIcon+" "+string(c.State)))
	fmt.Fprintf(&b, "Created: %s\n\n", c.Created.Format("2006-01-02 15:04:05"))

	// Networking
	b.WriteString("=== Networking ===\n")
	fmt.Fprintf(&b, "Hostname:     %s\n", c.Hostname)
	fmt.Fprintf(&b, "Network Mode: %s\n", c.NetworkMode)
	if len(c.Networks) > 0 {
		b.WriteString("Networks:\n")
		for _, n := range c.Networks {
			line := fmt.Sprintf("  %-15s %s", n.Name, n.IPAddress)
			if n.Gateway != "" {
				line += fmt.Sprintf("  gw:%s", n.Gateway)
			}
			if len(n.Aliases) > 0 {
				line += fmt.Sprintf("  aliases:%s", strings.Join(n.Aliases, ","))
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n")

	// Ports
	if len(c.Ports) > 0 {
		b.WriteString("=== Ports ===\n")
		for _, port := range c.Ports {
			fmt.Fprintf(&b, "  %d→%d/%s\n", port.HostPort, port.ContainerPort, port.Protocol)
		}
		b.WriteString("\n")
	}

	// Runtime
	b.WriteString("=== Runtime ===\n")
	if len(c.Entrypoint) > 0 {
		fmt.Fprintf(&b, "Entrypoint: %s\n", strings.Join(c.Entrypoint, " "))
	}
	if len(c.Cmd) > 0 {
		fmt.Fprintf(&b, "Command:    %s\n", strings.Join(c.Cmd, " "))
	}
	if c.WorkingDir != "" {
		fmt.Fprintf(&b, "WorkDir:    %s\n", c.WorkingDir)
	}
	if len(c.Env) > 0 {
		b.WriteString("Env:\n")
		for _, e := range c.Env {
			fmt.Fprintf(&b, "  %s\n", e)
		}
	}
	if len(c.Labels) > 0 {
		b.WriteString("Labels:\n")
		for k, v := range c.Labels {
			fmt.Fprintf(&b, "  %s=%s\n", k, v)
		}
	}
	b.WriteString("\n")

	// Resources
	b.WriteString("=== Resources ===\n")
	memStr := "unlimited"
	if c.MemoryLimit > 0 {
		memStr = helper.FormatSize(c.MemoryLimit)
	}
	fmt.Fprintf(&b, "Memory:     %s\n", memStr)
	if c.CPUShares > 0 {
		fmt.Fprintf(&b, "CPU Shares: %d\n", c.CPUShares)
	}
	fmt.Fprintf(&b, "Restart:    %s\n", c.RestartPolicy)
	fmt.Fprintf(&b, "Privileged: %v\n\n", c.Privileged)

	// Mounts
	if len(c.Mounts) > 0 {
		b.WriteString("=== Mounts ===\n")
		for _, mount := range c.Mounts {
			fmt.Fprintf(&b, "  [%s] %s → %s\n", mount.Type, mount.Source, mount.Destination)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (d *detailsPanel) extendHelpCmd() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			keys.Keys.ScrollUp,
			keys.Keys.ScrollDown,
		}}
	}
}
