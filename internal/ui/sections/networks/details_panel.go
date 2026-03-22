package networks

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

type detailsPanel struct {
	viewport viewport.Model
}

func newDetailsPanel() panel.Panel {
	return &detailsPanel{
		viewport: viewport.New(0, 0),
	}
}

func (d *detailsPanel) Init(content string) tea.Cmd {
	log.Printf("[network][details-panel] Init: networkID=%q", content)
	d.viewport.SetContent(content)
	return nil
}

func (d *detailsPanel) Name() string {
	return "Details"
}

func (d *detailsPanel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return cmd
}

func (d *detailsPanel) View() string {
	return d.viewport.View()
}

func (d *detailsPanel) Close() tea.Cmd {
	d.viewport.SetContent("")
	return nil
}

func (d *detailsPanel) SetSize(width, height int) {
	d.viewport.Width = width
	d.viewport.Height = height
}

const shortIDLen = 12

func formatNetworkDetails(n client.Network) string {
	id := n.ID
	if len(id) > shortIDLen {
		id = id[:shortIDLen]
	}

	var content strings.Builder

	label := theme.DetailLabelStyle
	value := theme.DetailValueStyle

	fmt.Fprintf(&content, "%s%s\n", label.Render("ID"), value.Render(id))
	fmt.Fprintf(&content, "%s%s\n", label.Render("Driver"), value.Render(n.Driver))
	fmt.Fprintf(&content, "%s%s\n", label.Render("Scope"), value.Render(n.Scope))

	internalStr := "false"
	if n.Internal {
		internalStr = "true"
	}
	fmt.Fprintf(&content, "%s%s\n", label.Render("Internal"), value.Render(internalStr))

	if n.IPAM.Subnet != "" {
		fmt.Fprintf(&content, "%s%s\n", label.Render("Subnet"), value.Render(n.IPAM.Subnet))
	}
	if n.IPAM.Gateway != "" {
		fmt.Fprintf(&content, "%s%s\n", label.Render("Gateway"), value.Render(n.IPAM.Gateway))
	}

	fmt.Fprintf(&content, "%s%s\n", label.Render("Containers"), value.Render(strconv.Itoa(len(n.ConnectedContainers))))

	if len(n.ConnectedContainers) > 0 {
		content.WriteString("\n")
		content.WriteString(label.Render("Connected Containers"))
		content.WriteString("\n")
		for _, c := range n.ConnectedContainers {
			ip := c.IPv4Address
			if ip == "" {
				ip = c.IPv6Address
			}
			if ip == "" {
				fmt.Fprintf(&content, "  %s\n", value.Render(c.Name))
			} else {
				fmt.Fprintf(&content, "  %s  %s\n", value.Render(c.Name), label.Render(ip))
			}
		}
	}

	fmt.Fprintf(&content, "\n%s%s\n", label.Render("Created"), value.Render(n.Created.Format(time.RFC3339)))

	return content.String()
}
