package containers

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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

const (
	// fileTreeEventBuffer bounds how many batches the producer can run ahead
	// of the UI before it blocks.
	fileTreeEventBuffer = 8
	fileTreeCacheTTL    = time.Minute
	fileTreeCacheSize   = 5
)

// fileTreeEvent is produced by the background file tree walk.
type fileTreeEvent struct {
	entries []client.FileEntry
	done    bool
	err     error
}

// fileTreeChunkMsg delivers one or more batches of file entries. Until done is
// set, the panel keeps waiting on events for more.
type fileTreeChunkMsg struct {
	requestID int
	events    <-chan fileTreeEvent
	entries   []client.FileEntry
	done      bool
	err       error
}

func (m *fileTreeChunkMsg) apply(ev fileTreeEvent) {
	m.entries = append(m.entries, ev.entries...)
	if ev.done {
		m.done = true
		m.err = ev.err
	}
}

type filesPanel struct {
	ctx           context.Context
	service       client.ContainerService
	cache         *fileTreeCache
	cancelFetch   context.CancelFunc
	builder       *client.FileTreeBuilder
	loading       bool
	width, height int
	containerID   string
	root          *client.FileNode
	visible       []*client.FileNode
	cursor        int
	requestID     int
}

func newFilesPanel(ctx context.Context, svc client.ContainerService) sections.Panel {
	return &filesPanel{
		ctx:     ctx,
		service: svc,
		cache:   newFileTreeCache(fileTreeCacheTTL, fileTreeCacheSize),
	}
}

func (f *filesPanel) Name() string {
	return "Files"
}

func (f *filesPanel) Init(listItem sections.ListItem) tea.Cmd {
	containerID := listItem.ID()
	log.Printf("[containers][files-panel] Init: containerID=%q", containerID)

	// The section re-initialises the active panel on every list refresh; keep
	// an in-flight load for the same container instead of restarting it.
	if f.loading && containerID == f.containerID {
		return f.extendHelpCmd()
	}

	// Switching container without a Close: drop the previous load.
	abortCmd := f.abortLoad()

	if root, ok := f.cache.get(containerID); ok {
		if root != f.root {
			f.root = root
			f.cursor = 0
		}
		f.containerID = containerID
		f.visible = computeVisible(f.root)
		f.cursor = min(f.cursor, max(len(f.visible)-1, 0))
		return tea.Batch(abortCmd, f.extendHelpCmd())
	}

	f.containerID = containerID
	f.loading = true
	f.builder = client.NewFileTreeBuilder()
	f.root = f.builder.Root()
	f.visible = nil
	f.cursor = 0
	f.requestID++
	requestID := f.requestID

	ctx, cancel := context.WithCancel(f.ctx)
	f.cancelFetch = cancel
	return tea.Batch(
		abortCmd,
		f.showSpinnerCmd(requestID),
		f.fetchCmd(ctx, containerID, requestID),
		f.extendHelpCmd(),
	)
}

// abortLoad cancels an in-flight load, invalidates its pending messages and
// returns the command hiding its spinner (nil when nothing was loading).
func (f *filesPanel) abortLoad() tea.Cmd {
	if !f.loading {
		return nil
	}
	requestID := f.requestID
	f.requestID++
	f.stopFetch()
	f.builder = nil
	f.loading = false
	return f.cancelSpinnerCmd(requestID)
}

func (f *filesPanel) stopFetch() {
	if f.cancelFetch != nil {
		f.cancelFetch()
		f.cancelFetch = nil
	}
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
	case fileTreeChunkMsg:
		return f.handleChunk(msg)

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
			if len(f.visible) == 0 {
				return nil
			}
			node := f.visible[f.cursor]
			return f.copyFromContainerCmd(node)
		}
	}

	return nil
}

func (f *filesPanel) handleChunk(msg fileTreeChunkMsg) tea.Cmd {
	if msg.requestID != f.requestID || f.builder == nil {
		return nil
	}

	for _, e := range msg.entries {
		f.builder.Add(e)
	}
	if len(msg.entries) > 0 {
		f.visible = computeVisible(f.root)
	}

	if !msg.done {
		return waitFileTreeCmd(msg.events, msg.requestID)
	}

	log.Printf("[containers][files-panel] file tree loaded: entries=%d err=%v", f.builder.Len(), msg.err)
	f.loading = false
	f.stopFetch()
	f.builder = nil

	if msg.err != nil {
		return tea.Batch(f.cancelSpinnerCmd(msg.requestID), func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("error getting the file tree: %v", msg.err),
				IsError: true,
			}
		})
	}

	f.cache.put(f.containerID, f.root)
	return f.cancelSpinnerCmd(msg.requestID)
}

func (f *filesPanel) View() string {
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

	if f.loading && f.builder != nil {
		fmt.Fprintf(&statusBar, "loading %d entries… ", f.builder.Len())
	}

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
	f.stopFetch()
	f.builder = nil
	f.containerID = ""
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

// fetchCmd starts walking the container filesystem in the background and
// returns the first chunk of entries. ctx cancellation stops the walk.
func (f *filesPanel) fetchCmd(ctx context.Context, containerID string, requestID int) tea.Cmd {
	svc := f.service
	return func() tea.Msg {
		events := make(chan fileTreeEvent, fileTreeEventBuffer)
		go func() {
			defer close(events)
			send := func(ev fileTreeEvent) {
				select {
				case events <- ev:
				case <-ctx.Done():
				}
			}
			err := svc.FileTree(ctx, containerID, func(entries []client.FileEntry) {
				send(fileTreeEvent{entries: entries})
			})
			send(fileTreeEvent{done: true, err: err})
		}()
		return waitFileTreeCmd(events, requestID)()
	}
}

// waitFileTreeCmd blocks for the next event, then drains whatever else is
// already buffered so the UI rebuilds its view once per burst of batches.
func waitFileTreeCmd(events <-chan fileTreeEvent, requestID int) tea.Cmd {
	return func() tea.Msg {
		msg := fileTreeChunkMsg{requestID: requestID, events: events}
		ev, ok := <-events
		if !ok {
			// Only happens when the walk was cancelled; the request is stale.
			msg.done = true
			return msg
		}
		msg.apply(ev)
		for !msg.done {
			select {
			case ev, ok = <-events:
				if !ok {
					msg.done = true
					return msg
				}
				msg.apply(ev)
			default:
				return msg
			}
		}
		return msg
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
			Message: fmt.Sprintf("%s got copied to host succesfully at %s", srcPath, dstDir),
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
			keys.Keys.CpFromContainerToHost,
		}}
	}
}
