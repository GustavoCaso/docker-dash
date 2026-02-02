package components

import (
	"fmt"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var style = lipgloss.NewStyle().PaddingTop(1)

// ImageItem implements list.Item interface
type ImageItem struct {
	image service.Image
}

func (i ImageItem) Title() string       { return fmt.Sprintf("%s:%s", i.image.Repo, i.image.Tag) }
func (i ImageItem) Description() string { return formatSize(i.image.Size) }
func (i ImageItem) FilterValue() string { return i.image.Repo + ":" + i.image.Tag }

// ImageList wraps bubbles/list
type ImageList struct {
	list list.Model
}

// NewImageList creates a new image list
func NewImageList(images []service.Image) *ImageList {
	items := make([]list.Item, len(images))
	for i, img := range images {
		items[i] = ImageItem{image: img}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false) // We still support filtering with /
	l.SetShowStatusBar(false)

	return &ImageList{list: l}
}

// SetSize sets dimensions
func (i *ImageList) SetSize(width, height int) {
	x, y := style.GetFrameSize()
	i.list.SetSize(width-x, height-y)
}

// Update handles messages
func (i *ImageList) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	i.list, cmd = i.list.Update(msg)
	return cmd
}

// View renders the list
func (i *ImageList) View() string {

	return style.Render(i.list.View())
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
