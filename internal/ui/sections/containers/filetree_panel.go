package containers

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

// containersTreeLoadedMsg is sent when containers have been loaded asynchronously.
type containersTreeLoadedMsg struct {
	error    error
	fileTree client.ContainerFileTree
}

type filetreePanel struct {
	ctx      context.Context
	service  client.ContainerService
	loading  bool
	spinner  spinner.Model
	viewport viewport.Model
}

func NewFileTreePanel(ctx context.Context, svc client.ContainerService) panel.Panel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &filetreePanel{ctx: ctx, service: svc, viewport: viewport.New(0, 0), spinner: sp}
}

func (f *filetreePanel) Name() string {
	return "Files"
}

func (f *filetreePanel) Init(containerID string) tea.Cmd {
	f.loading = true
	return tea.Batch(f.spinner.Tick, f.fetchCmd(containerID), f.extendHelpCmd())
}

func (f *filetreePanel) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	if f.loading {
		var spinnerCmd tea.Cmd
		f.spinner, spinnerCmd = f.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	treeMsg, ok := msg.(containersTreeLoadedMsg)
	if ok {
		f.loading = false
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

	return tea.Batch(append(cmds, cmd)...)
}

func (f *filetreePanel) View() string {
	content := f.viewport.View()

	// Overlay spinner in bottom right corner when loading
	if f.loading {
		spinnerText := f.spinner.View() + " Loading..."
		content = helper.OverlayBottomRight(1, content, spinnerText, f.viewport.Width)
	}

	return content
}

func (f *filetreePanel) Close() tea.Cmd {
	f.loading = false
	f.viewport.SetContent("")
	return func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} }
}

func (f *filetreePanel) SetSize(width, height int) {
	f.viewport.Width = width
	f.viewport.Height = height
}

func (f *filetreePanel) fetchCmd(containerID string) tea.Cmd {
	ctx := f.ctx
	svc := f.service
	return func() tea.Msg {
		fileTree, err := svc.FileTree(ctx, containerID)
		if err != nil {
			return containersTreeLoadedMsg{error: fmt.Errorf("error getting the file tree: %w", err)}
		}
		return containersTreeLoadedMsg{fileTree: fileTree}
	}
}

func (f *filetreePanel) extendHelpCmd() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			keys.Keys.ScrollUp,
			keys.Keys.ScrollDown,
		}}
	}
}
