package containers

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/helpers"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

type detailsMsg struct {
	output string
	err    error
}

type detailsPanel struct {
	service client.ContainerService
	content string
	width   int
}

// NewDetailsPanel creates a new panel.Panel that fetches and renders container details.
func NewDetailsPanel(svc client.ContainerService) panel.Panel {
	return &detailsPanel{service: svc}
}

func (d *detailsPanel) Init(containerID string) tea.Cmd {
	return d.fetchCmd(containerID)
}

func (d *detailsPanel) Update(msg tea.Msg) tea.Cmd {
	dm, ok := msg.(detailsMsg)
	if !ok {
		return nil
	}
	if dm.err != nil {
		return func() tea.Msg {
			return message.ShowBannerMsg{Message: dm.err.Error(), IsError: true}
		}
	}
	d.content = dm.output
	return nil
}

func (d *detailsPanel) View() string {
	return d.content
}

func (d *detailsPanel) Close() {
	d.content = ""
}

func (d *detailsPanel) SetSize(width, _ int) {
	d.width = width
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

func formatDetails(container *client.Container) string {
	var content strings.Builder

	fmt.Fprintf(&content, "Container: %s\n", container.Name)
	content.WriteString("═══════════════════════\n\n")

	fmt.Fprintf(&content, "ID:      %s\n", helpers.ShortID(container.ID))
	fmt.Fprintf(&content, "Image:   %s\n", container.Image)
	fmt.Fprintf(&content, "Status:  %s\n", container.Status)

	stateStyle := theme.GetContainerStatusStyle(string(container.State))
	stateIcon := theme.GetContainerStatusIcon(string(container.State))
	fmt.Fprintf(&content, "State:   %s\n", stateStyle.Render(stateIcon+" "+string(container.State)))

	fmt.Fprintf(&content, "Created: %s\n\n", container.Created.Format("2006-01-02 15:04:05"))

	if len(container.Ports) > 0 {
		content.WriteString("Ports:\n")
		for _, port := range container.Ports {
			fmt.Fprintf(&content, "  %d:%d/%s\n", port.HostPort, port.ContainerPort, port.Protocol)
		}
		content.WriteString("\n")
	}

	if len(container.Mounts) > 0 {
		content.WriteString("Mounts:\n")
		for _, mount := range container.Mounts {
			fmt.Fprintf(&content, "  [%s] %s -> %s\n", mount.Type, mount.Source, mount.Destination)
		}
		content.WriteString("\n")
	}

	return content.String()
}
