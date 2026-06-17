package images

import (
	"context"
	"fmt"
	"log"

	tea "charm.land/bubbletea/v2"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/scrolllist"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
)

type layersPanel struct {
	ctx    context.Context
	client client.ImageService
	list   scrolllist.Model
}

type layersOutputMsg struct {
	lines []string
	err   error
}

func NewLayersPanel(ctx context.Context, client client.ImageService) sections.Panel {
	return &layersPanel{
		ctx:    ctx,
		client: client,
		list:   scrolllist.New(),
	}
}

func (l *layersPanel) Name() string {
	return "Layers"
}

func (l *layersPanel) Init(item sections.ListItem) tea.Cmd {
	imageID := item.ID()
	log.Printf("[images][layers-panel] Init: imageID=%q", imageID)
	return l.fetchCmd(imageID)
}

func (l *layersPanel) Update(msg tea.Msg) tea.Cmd {
	if dm, ok := msg.(layersOutputMsg); ok {
		log.Printf("[images][layers-panel] layersOutputMsg: count=%d err=%v", len(dm.lines), dm.err)
		if dm.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{Message: dm.err.Error(), IsError: true}
			}
		}
		l.list.SetLines(dm.lines)
		return nil
	}

	return l.list.Update(msg)
}

func (l *layersPanel) View() string {
	return l.list.View()
}

func (l *layersPanel) Close() tea.Cmd {
	l.list.Reset()
	return nil
}

func (l *layersPanel) SetSize(width, height int) {
	l.list.SetSize(width, height)
}

func (l *layersPanel) fetchCmd(imageID string) tea.Cmd {
	ctx := l.ctx
	svc := l.client
	return func() tea.Msg {
		layers := svc.FetchLayers(ctx, imageID)
		return layersOutputMsg{lines: formatLayerLines(layers)}
	}
}

func formatLayerLines(layers []client.Layer) []string {
	if len(layers) == 0 {
		return []string{"No layer information available"}
	}

	lines := make([]string, 0, len(layers)*3) //nolint:mnd // 3 lines per layer: header, detail, blank line
	for idx, layer := range layers {
		lines = append(lines, fmt.Sprintf("%2d. %s", idx+1, helper.StripCommand(layer.Command)))
		lines = append(lines, fmt.Sprintf("    Size: %-10s  ID: %s",
			helper.FormatSize(layer.Size),
			helper.ShortID(layer.ID)))
		lines = append(lines, "")
	}

	// Remove trailing blank line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}
