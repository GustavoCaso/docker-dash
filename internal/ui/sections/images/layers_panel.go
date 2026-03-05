package images

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
)

type layersPanel struct {
	ctx      context.Context
	client   client.ImageService
	viewport viewport.Model
}

type layersOutputMsg struct {
	output string
	err    error
}

func NewLayersPanel(ctx context.Context, client client.ImageService) panel.Panel {
	return &layersPanel{
		ctx:      ctx,
		client:   client,
		viewport: viewport.New(0, 0),
	}
}

func (l *layersPanel) Init(imageID string) tea.Cmd {
	return tea.Batch(l.fetchCmd(imageID), l.extendHelpCmd())
}

func (l *layersPanel) Update(msg tea.Msg) tea.Cmd {
	dm, ok := msg.(layersOutputMsg)
	if ok {
		if dm.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{Message: dm.err.Error(), IsError: true}
			}
		}
		l.viewport.SetContent(dm.output)
		return nil
	}

	var cmd tea.Cmd
	l.viewport, cmd = l.viewport.Update(msg)

	return cmd
}

func (l *layersPanel) View() string {
	return l.viewport.View()
}

func (l *layersPanel) Close() tea.Cmd {
	l.viewport.SetContent("")
	return func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} }
}

func (l *layersPanel) SetSize(width, height int) {
	l.viewport.Width = width
	l.viewport.Height = height
}

func (l *layersPanel) fetchCmd(imageID string) tea.Cmd {
	ctx := l.ctx
	svc := l.client
	return func() tea.Msg {
		image, err := svc.Get(ctx, imageID)
		if err != nil {
			return layersOutputMsg{err: fmt.Errorf("error getting image details: %w", err)}
		}
		return layersOutputMsg{output: formatDetails(image)}
	}
}

func formatDetails(img client.Image) string {
	var content strings.Builder

	// Header
	fmt.Fprintf(&content, "Layers for %s:%s\n", img.Repo, img.Tag)
	content.WriteString("═══════════════════════\n\n")

	const maxLayerCmdLen = 50 // max chars to show for a layer command
	if len(img.Layers) == 0 {
		content.WriteString("No layer information available\n")
	} else {
		for idx, layer := range img.Layers {
			cmd := helper.TruncateCommand(layer.Command, maxLayerCmdLen)
			fmt.Fprintf(&content, "%2d. %s\n", idx+1, cmd)
			fmt.Fprintf(&content, "    Size: %-10s  ID: %s\n\n",
				helper.FormatSize(layer.Size),
				helper.ShortID(layer.ID))
		}
	}

	return content.String()
}

func (l *layersPanel) extendHelpCmd() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			keys.Keys.ScrollUp,
			keys.Keys.ScrollDown,
		}}
	}
}
