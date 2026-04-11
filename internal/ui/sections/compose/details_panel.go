package compose

import (
	"fmt"
	"log"
	"strings"

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
	log.Print("[compose][details-panel] Init")
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

func formatProjectDetails(p client.ComposeProject) string {
	var content strings.Builder

	label := theme.DetailLabelStyle
	value := theme.DetailValueStyle

	fmt.Fprintf(&content, "%s%s\n", label.Render("Name"), value.Render(p.Name))

	if p.WorkingDir != "" {
		fmt.Fprintf(&content, "%s%s\n", label.Render("Working Dir"), value.Render(p.WorkingDir))
	}

	if p.ConfigFiles != "" {
		fmt.Fprintf(&content, "%s%s\n", label.Render("Config Files"), value.Render(p.ConfigFiles))
	}

	fmt.Fprintf(&content, "%s%d\n", label.Render("Services"), len(p.Services))

	if len(p.Services) > 0 {
		content.WriteString("\n")
		content.WriteString(label.Render("Service List"))
		content.WriteString("\n")
		for _, svc := range p.Services {
			stateStyle := theme.GetContainerStatusStyle(svc.State)
			icon := theme.GetContainerStatusIcon(svc.State)
			fmt.Fprintf(&content, "  %s %s  %s\n", icon, value.Render(svc.Name), stateStyle.Render(svc.State))
			if svc.Image != "" {
				fmt.Fprintf(&content, " %s", svc.Image)
				content.WriteString("\n")
			}
			content.WriteString("\n")
		}
	}

	return content.String()
}
