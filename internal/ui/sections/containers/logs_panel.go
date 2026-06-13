package containers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

const (
	ellipsisWidth = 1
	hScrollStep   = 10
)

type logLine struct {
	content string
}

func (l logLine) Title() string       { return l.content }
func (l logLine) Description() string { return "" }
func (l logLine) FilterValue() string { return l.content }

type logDelegate struct {
	hOffset int
}

func newLogDelegate() *logDelegate {
	return &logDelegate{}
}

func (d *logDelegate) Height() int                             { return 1 }
func (d *logDelegate) Spacing() int                            { return 0 }
func (d *logDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d *logDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	line, ok := item.(logLine)
	if !ok {
		return
	}
	width := m.Width()
	if width < 2 { //nolint:mnd // minimum width for content + ellipsis
		return
	}
	content := line.content
	if index == m.Index() {
		runes := []rune(content)
		start := min(d.hOffset, max(0, len(runes)-1))
		visible := string(runes[start:])
		if len([]rune(visible)) > width {
			visible = string([]rune(visible)[:width])
		}
		fmt.Fprint(w, theme.SelectedLogLine.Render(visible))
	} else {
		runes := []rune(content)
		if len(runes) > width-ellipsisWidth {
			content = string(runes[:width-ellipsisWidth]) + "…"
		}
		fmt.Fprint(w, content)
	}
}

type logsPanel struct {
	ctx         context.Context
	logsSession *client.LogsSession
	list        list.Model
	lineBuffer  string
	prevIndex   int
	delegate    *logDelegate
	client      client.ContainerService
	logsCfg     config.LogsConfig
}

// logsOutputMsg is sent when logs output is received from the background reader.
type logsOutputMsg struct {
	output string
	err    error
}

type logsSessionStartedMsg struct {
	session *client.LogsSession
}

func NewLogsPanel(ctx context.Context, client client.ContainerService, logsCfg config.LogsConfig) sections.Panel {
	delegate := newLogDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	return &logsPanel{
		ctx:      ctx,
		client:   client,
		list:     l,
		delegate: delegate,
		logsCfg:  logsCfg,
	}
}

func (l *logsPanel) Init(item sections.ListItem) tea.Cmd {
	containerID := item.ID()
	log.Printf("[containers][logs-panel] Init: containerID=%q", containerID)
	return tea.Batch(l.init(containerID), l.extendHelpCmd())
}

func (l *logsPanel) Name() string {
	return "Logs"
}

func (l *logsPanel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case logsSessionStartedMsg:
		log.Printf("[containers][logs-panel] session started")
		l.logsSession = msg.session
		return l.readLogsOutput()
	case logsOutputMsg:
		if msg.err != nil {
			if l.logsSession == nil {
				return nil
			}
			if errors.Is(msg.err, io.EOF) {
				l.logsSession.Close()
				l.logsSession = nil
				return nil
			}
			l.Close()
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Logs session error. Err: %s", msg.err),
					IsError: true,
				}
			}
		}
		log.Printf("[containers][logs-panel] output chunk: bytes=%d", len(msg.output))
		l.appendLines(msg.output)
		return l.readLogsOutput()
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Keys.LogScrollLeft):
			l.delegate.hOffset = max(0, l.delegate.hOffset-hScrollStep)
			return nil
		case key.Matches(msg, keys.Keys.LogScrollRight):
			selected := l.list.SelectedItem()
			if selected != nil {
				line, ok := selected.(logLine)
				if ok {
					maxOffset := max(0, len([]rune(line.content))-l.list.Width())
					l.delegate.hOffset = min(l.delegate.hOffset+hScrollStep, maxOffset)
				}
			}
			return nil
		}
	}

	var cmd tea.Cmd
	l.list, cmd = l.list.Update(msg)
	newIndex := l.list.Index()
	if newIndex != l.prevIndex {
		l.delegate.hOffset = 0
		l.prevIndex = newIndex
	}
	return cmd
}

func (l *logsPanel) appendLines(chunk string) {
	combined := l.lineBuffer + chunk
	parts := strings.Split(combined, "\n")
	l.lineBuffer = parts[len(parts)-1]
	complete := parts[:len(parts)-1]
	wasEmpty := len(l.list.Items()) == 0
	for _, line := range complete {
		l.list.InsertItem(len(l.list.Items()), logLine{content: line})
	}
	if wasEmpty && len(l.list.Items()) > 0 {
		l.list.Select(0)
	}
}

func (l *logsPanel) View() string {
	return l.list.View()
}

func (l *logsPanel) Close() tea.Cmd {
	log.Printf("[containers][logs-panel] Close")
	if l.logsSession != nil {
		l.logsSession.Close()
		l.logsSession = nil
	}
	l.list.SetItems([]list.Item{})
	l.lineBuffer = ""
	l.delegate.hOffset = 0
	l.prevIndex = 0
	return func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} }
}

func (l *logsPanel) SetSize(width, height int) {
	l.list.SetSize(width, height)
}

func (l *logsPanel) readLogsOutput() tea.Cmd {
	session := l.logsSession
	if session == nil {
		return nil
	}
	return func() tea.Msg {
		buf := make([]byte, readBufSize)
		n, err := session.Reader.Read(buf)
		if err != nil {
			return logsOutputMsg{err: err}
		}

		return logsOutputMsg{output: string(buf[:n])}
	}
}

func (l *logsPanel) init(containerID string) tea.Cmd {
	return func() tea.Msg {
		session, err := l.client.Logs(l.ctx, containerID, client.LogOptions{
			Follow:     l.logsCfg.Follow,
			Tail:       l.logsCfg.Tail,
			Timestamps: l.logsCfg.Timestamps,
			Since:      l.logsCfg.Since,
		})
		if err != nil {
			return logsOutputMsg{err: err}
		}
		return logsSessionStartedMsg{session: session}
	}
}

func (l *logsPanel) extendHelpCmd() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			keys.Keys.ScrollUp,
			keys.Keys.ScrollDown,
			keys.Keys.LogScrollLeft,
			keys.Keys.LogScrollRight,
		}}
	}
}
