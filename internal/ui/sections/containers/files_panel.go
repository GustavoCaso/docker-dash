package containers

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
)

var highlightStyle = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("63")).Foreground(lipgloss.Color("230"))
var normalStyle = lipgloss.NewStyle()
var statusBarSpace = 2

// fileNodeLoadedMsg is sent when the file tree has been loaded asynchronously.
type fileNodeLoadedMsg struct {
	requestID int
	err       error
	fileNode  *client.FileNode
}

type filesPanel struct {
	ctx           context.Context
	service       client.ContainerService
	loading       bool
	width, height int
	containerID   string
	root          *client.FileNode
	visible       []*client.FileNode
	cursor        int
	requestID     int
}

func newFilesPanel(ctx context.Context, svc client.ContainerService) sections.Panel {
	return &filesPanel{ctx: ctx, service: svc}
}

func (f *filesPanel) Name() string {
	return "Files"
}

func (f *filesPanel) Init(listItem sections.ListItem) tea.Cmd {
	f.containerID = listItem.ID()
	log.Printf("[containers][files-panel] Init: containerID=%q", f.containerID)
	f.loading = true
	f.requestID++
	requestID := f.requestID
	return tea.Batch(f.showSpinnerCmd(requestID), f.fetchCmd(f.containerID, requestID), f.extendHelpCmd())
}

func computeVisible(root *client.FileNode) []*client.FileNode {
	var result []*client.FileNode
	var walk func(n *client.FileNode)
	walk = func(n *client.FileNode) {
		if n != root {
			result = append(result, n)
		}
		if n.IsDir && !n.Collapsed {
			for _, c := range n.Children {
				walk(c)
			}
		}
	}
	walk(root)
	return result
}

func (f *filesPanel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case fileNodeLoadedMsg:
		log.Printf("[containers][files-panel] fileNodeLoadedMsg: requestID=%d err=%v", msg.requestID, msg.err)
		if msg.requestID != f.requestID {
			return nil
		}
		f.loading = false
		if msg.err != nil {
			return tea.Batch(f.cancelSpinnerCmd(msg.requestID), func() tea.Msg {
				return message.ShowBannerMsg{
					Message: msg.err.Error(),
					IsError: true,
				}
			})
		}
		f.root = msg.fileNode
		f.visible = computeVisible(f.root)
		return f.cancelSpinnerCmd(msg.requestID)

	case tea.KeyPressMsg:
		log.Printf("[containers][files-panel] KeyMsg: key=%q", msg.String())
		switch {
		case key.Matches(msg, keys.Keys.ScrollUp):
			if f.cursor > 0 {
				f.cursor--
			}
		case key.Matches(msg, keys.Keys.ScrollDown):
			if f.cursor < len(f.visible)-1 {
				f.cursor++
			}
		case key.Matches(msg, keys.Keys.Space):
			if len(f.visible) > 0 {
				node := f.visible[f.cursor]
				if node.IsDir {
					node.Collapsed = !node.Collapsed
					f.visible = computeVisible(f.root)
					if f.cursor >= len(f.visible) {
						f.cursor = len(f.visible) - 1
					}
				}
			}
		case key.Matches(msg, keys.Keys.CpFromContainerToHost):
			node := f.visible[f.cursor]
			return f.copyFromContainerCmd(node)
		}
	}

	return nil
}

func (f *filesPanel) View() string {
	if f.loading {
		return ""
	}

	if len(f.visible) == 0 {
		return ""
	}

	// reserve 1 for status bar + 1 for padding.
	treeLines := max(f.height-statusBarSpace, 1)

	var fileTree strings.Builder

	start := 0
	if f.cursor >= treeLines {
		start = f.cursor - treeLines + 1
	}
	end := min(start+treeLines, len(f.visible))
	height := 0
	for i := start; i < end; i++ {
		height++
		node := f.visible[i]
		indent := strings.Repeat("  ", node.Depth-1)

		var prefix string
		if node.IsDir {
			if node.Collapsed {
				prefix = "▶ "
			} else {
				prefix = "▼ "
			}
		} else {
			prefix = "  "
		}

		label := node.Name
		if node.IsDir {
			label += "/"
		}
		if node.Linkname != "" {
			label += " -> " + node.Linkname
		}

		line := indent + prefix + label
		if i == f.cursor {
			line = highlightStyle.Render(line)
		}
		fileTree.WriteString(line + "\n")
	}

	if height < treeLines {
		extraLines := treeLines - height
		for range extraLines {
			fileTree.WriteString("\n")
		}
	}

	var statusBar strings.Builder

	if len(f.visible) > 0 {
		node := f.visible[f.cursor]
		if node.IsDir {
			statusBar.WriteString(
				normalStyle.Width(f.width).Render(fmt.Sprintf("items: %d path: %s/", len(node.Children), node.Path)),
			)
		} else {
			statusBar.WriteString(
				normalStyle.Width(f.width).
					Render(fmt.Sprintf("size: %s mode: %s path: %s", helper.FormatSize(node.Size), node.Mode.String(), node.Path)),
			)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Top, fileTree.String(), statusBar.String())
}

func (f *filesPanel) Close() tea.Cmd {
	log.Printf("[containers][files-panel] Close")
	requestID := f.requestID
	f.requestID++
	f.loading = false
	f.root = nil
	f.visible = []*client.FileNode{}
	f.cursor = 0
	return tea.Batch(
		f.cancelSpinnerCmd(requestID),
		func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} },
	)
}

func (f *filesPanel) SetSize(width, height int) {
	f.width = width
	f.height = height
}

func (f *filesPanel) fetchCmd(containerID string, requestID int) tea.Cmd {
	ctx := f.ctx
	svc := f.service
	return func() tea.Msg {
		fileNode, err := svc.FileTree(ctx, containerID)
		if err != nil {
			return fileNodeLoadedMsg{requestID: requestID, err: fmt.Errorf("error getting the file tree: %w", err)}
		}
		return fileNodeLoadedMsg{requestID: requestID, fileNode: fileNode}
	}
}

func (f *filesPanel) copyFromContainerCmd(node *client.FileNode) tea.Cmd {
	ctx := f.ctx
	svc := f.service
	containerID := f.containerID
	srcPath := node.Path

	return func() tea.Msg {
		rc, err := svc.CopyFromContainer(ctx, containerID, srcPath)
		if err != nil {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("error copying %q from container %q: %v", srcPath, containerID, err),
				IsError: true,
			}
		}
		defer rc.Close()

		dstDir, err := os.Getwd()
		if err != nil {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("error getting current dir: %v", err),
				IsError: true,
			}
		}

		if err = helper.ExtractTarToWorkingDir(dstDir, rc); err != nil {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("error extracting tar to %q: %v", srcPath, err),
				IsError: true,
			}
		}

		return message.ShowBannerMsg{
			Message: fmt.Sprintf("%s got copied to host succesfully", srcPath),
			IsError: false,
		}
	}
}

func (f *filesPanel) showSpinnerCmd(requestID int) tea.Cmd {
	return func() tea.Msg {
		return message.ShowSpinnerMsg{
			ID:   f.spinnerID(requestID),
			Text: "Loading files...",
			Scope: message.SpinnerScope{
				Section: string(sections.ContainersSection),
				Panel:   f.Name(),
			},
		}
	}
}

func (f *filesPanel) cancelSpinnerCmd(requestID int) tea.Cmd {
	return func() tea.Msg {
		return message.CancelSpinnerMsg{ID: f.spinnerID(requestID)}
	}
}

func (f *filesPanel) spinnerID(requestID int) string {
	return fmt.Sprintf("%s.files.%d", string(sections.ContainersSection), requestID)
}

func (f *filesPanel) extendHelpCmd() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			keys.Keys.ScrollUp,
			keys.Keys.ScrollDown,
			keys.Keys.Space,
		}}
	}
}
