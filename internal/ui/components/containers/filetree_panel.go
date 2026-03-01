package containers

import (
	"context"
	"fmt"

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
	service client.ContainerService
	content string
	width   int
}

func NewFileTreePanel(svc client.ContainerService) panel.Panel {
	return &filetreePanel{service: svc}
}

func (f *filetreePanel) Init(containerID string) tea.Cmd {
	return f.fetchCmd(containerID)
}

func (f *filetreePanel) Update(msg tea.Msg) tea.Cmd {
	treeMsg, ok := msg.(containersTreeLoadedMsg)
	if !ok {
		return nil
	}
	if treeMsg.error != nil {
		return func() tea.Msg {
			return message.ShowBannerMsg{
				Message: treeMsg.error.Error(),
				IsError: true,
			}
		}
	}
	f.content = treeMsg.fileTree.Tree.String()
	return nil
}

func (f *filetreePanel) View() string {
	return f.content
}

func (f *filetreePanel) Close() {
	f.content = ""
}

func (f *filetreePanel) SetSize(width, _ int) {
	f.width = width
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
