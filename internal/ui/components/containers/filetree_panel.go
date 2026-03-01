package containers

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

// containersTreeLoadedMsg is sent when containers have been loaded asynchronously.
type containersTreeLoadedMsg struct {
	error    error
	fileTree client.ContainerFileTree
}

type filetreePanel struct {
	service  client.ContainerService
	viewport viewport.Model
}

func NewFileTreePanel(svc client.ContainerService) panel.Panel {
	return &filetreePanel{service: svc, viewport: viewport.New(0, 0)}
}

func (f *filetreePanel) Init(containerID string) tea.Cmd {
	return f.fetchCmd(containerID)
}

func (f *filetreePanel) Update(msg tea.Msg) tea.Cmd {
	treeMsg, ok := msg.(containersTreeLoadedMsg)
	if ok {
		if treeMsg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: treeMsg.error.Error(),
					IsError: true,
				}
			}
		}
		f.viewport.SetContent(treeMsg.fileTree.Tree.String())
		return nil
	}

	var cmd tea.Cmd
	f.viewport, cmd = f.viewport.Update(msg)

	return cmd
}

func (f *filetreePanel) View() string {
	return f.viewport.View()
}

func (f *filetreePanel) Close() {
	f.viewport.SetContent("")
}

func (f *filetreePanel) SetSize(width, height int) {
	f.viewport.Width = width
	f.viewport.Height = height
}

func (f *filetreePanel) fetchCmd(containerID string) tea.Cmd {
	svc := f.service
	return func() tea.Msg {
		ctx := context.Background()
		fileTree, err := svc.FileTree(ctx, containerID)
		if err != nil {
			return containersTreeLoadedMsg{error: fmt.Errorf("error getting the file tree: %w", err)}
		}
		return containersTreeLoadedMsg{fileTree: fileTree}
	}
}
