package volumes

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

type fileTreePanel struct {
	ctx      context.Context
	client   client.VolumeService
	viewport viewport.Model
}

// volumeTreeLoadedMsg is sent when the volume file tree is loaded.
type volumeTreeLoadedMsg struct {
	error    error
	fileTree client.VolumeFileTree
}

func newFileTreePanel(ctx context.Context, svc client.VolumeService) panel.Panel {
	return &fileTreePanel{
		ctx:      ctx,
		client:   svc,
		viewport: viewport.New(0, 0),
	}
}

func (f *fileTreePanel) Name() string {
	return "Files"
}

func (f *fileTreePanel) Init(volumeName string) tea.Cmd {
	return f.fetchCmd(volumeName)
}

func (f *fileTreePanel) Update(msg tea.Msg) tea.Cmd {
	dm, ok := msg.(volumeTreeLoadedMsg)
	if ok {
		if dm.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{Message: dm.error.Error(), IsError: true}
			}
		}
		f.viewport.SetContent(lipgloss.NewStyle().Width(f.viewport.Width).Render(dm.fileTree.Tree.String()))
		return nil
	}

	var cmd tea.Cmd
	f.viewport, cmd = f.viewport.Update(msg)
	return cmd
}

func (f *fileTreePanel) View() string {
	return f.viewport.View()
}

func (f *fileTreePanel) Close() tea.Cmd {
	f.viewport.SetContent("")
	return nil
}

func (f *fileTreePanel) SetSize(width, height int) {
	f.viewport.Width = width
	f.viewport.Height = height
}

func (f *fileTreePanel) fetchCmd(volumeName string) tea.Cmd {
	ctx := f.ctx
	svc := f.client
	return func() tea.Msg {
		fileTree, err := svc.FileTree(ctx, volumeName)
		if err != nil {
			return volumeTreeLoadedMsg{error: fmt.Errorf("error getting volume file tree: %s", err.Error())}
		}
		return volumeTreeLoadedMsg{fileTree: fileTree}
	}
}
