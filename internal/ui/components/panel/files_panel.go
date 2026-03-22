package panel

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

var highlightStyle = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("63")).Foreground(lipgloss.Color("230"))
var normalStyle = lipgloss.NewStyle()
var statusBarSpace = 2

// fileNodeLoadedMsg is sent when containers have been loaded asynchronously.
type fileNodeLoadedMsg struct {
	err      error
	fileNode *client.FileNode
}

type filesPanel struct {
	ctx           context.Context
	service       fileService
	loading       bool
	spinner       spinner.Model
	width, height int
	root          *client.FileNode
	visible       []*client.FileNode
	cursor        int
}

type fileService interface {
	FileTree(ctx context.Context, ID string) (*client.FileNode, error)
}

func NewFilesPanel(ctx context.Context, svc fileService) Panel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &filesPanel{ctx: ctx, service: svc, width: 0, height: 0, spinner: sp}
}

func (f *filesPanel) Name() string {
	return "Files"
}

func (f *filesPanel) Init(containerID string) tea.Cmd {
	log.Printf("[files-panel] Init: containerID=%q", containerID)
	f.loading = true
	return tea.Batch(f.spinner.Tick, f.fetchCmd(containerID), f.extendHelpCmd())
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
	var cmds []tea.Cmd

	if f.loading {
		var spinnerCmd tea.Cmd
		f.spinner, spinnerCmd = f.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case fileNodeLoadedMsg:
		log.Printf("[files-panel] fileNodeLoadedMsg: err=%v", msg.err)
		f.loading = false
		if msg.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: msg.err.Error(),
					IsError: true,
				}
			}
		}
		f.root = msg.fileNode
		f.visible = computeVisible(f.root)
		return nil

	case tea.KeyMsg:
		log.Printf("[files-panel] KeyMsg: key=%q", msg.String())
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
		}
	}

	return tea.Batch(cmds...)
}

func (f *filesPanel) View() string {
	if f.loading {
		spinnerText := f.spinner.View() + " Loading..."
		content := helper.OverlayBottomRight(1, "", spinnerText, f.width)
		return content
	}

	if len(f.visible) == 0 {
		return ""
	}

	// reserve 1 for status bar + 1 for padding.
	treeLines := max(f.height-statusBarSpace, 1)

	var fileTree strings.Builder

	// Determine scroll window
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

	// Status bar
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
	log.Printf("[files-panel] Close")
	f.loading = false
	f.root = nil
	f.visible = []*client.FileNode{}
	return func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} }
}

func (f *filesPanel) SetSize(width, height int) {
	f.width = width
	f.height = height
}

func (f *filesPanel) fetchCmd(containerID string) tea.Cmd {
	ctx := f.ctx
	svc := f.service
	return func() tea.Msg {
		fileNode, err := svc.FileTree(ctx, containerID)
		if err != nil {
			return fileNodeLoadedMsg{err: fmt.Errorf("error getting the file tree: %w", err)}
		}
		return fileNodeLoadedMsg{fileNode: fileNode}
	}
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
