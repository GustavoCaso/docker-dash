package containers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/scrolllist"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
)

type logsPanel struct {
	ctx         context.Context
	logsSession *client.LogsSession
	list        scrolllist.Model
	lineBuffer  string
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
	return &logsPanel{
		ctx:     ctx,
		client:  client,
		list:    scrolllist.New(),
		logsCfg: logsCfg,
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
	}

	return l.list.Update(msg)
}

func (l *logsPanel) appendLines(chunk string) {
	combined := l.lineBuffer + chunk
	parts := strings.Split(combined, "\n")
	l.lineBuffer = parts[len(parts)-1]
	complete := parts[:len(parts)-1]
	for _, line := range complete {
		l.list.AppendLine(line)
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
	l.list.Reset()
	l.lineBuffer = ""
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
