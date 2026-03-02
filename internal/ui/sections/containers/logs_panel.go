package containers

import (
	"context"
	"errors"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

type logsPanel struct {
	logsSession *client.LogsSession
	logsOutput  string
	client      client.ContainerService
}

// logsOutputMsg is sent when logs output is received from the background reader.
type logsOutputMsg struct {
	output string
	err    error
}

type logsSessionStartedMsg struct {
	session *client.LogsSession
}

func NewLogsPanel(client client.ContainerService) panel.Panel {
	return &logsPanel{
		client: client,
	}
}

func (l *logsPanel) Init(containerID string) tea.Cmd {
	return l.init(containerID)
}

func (l *logsPanel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case logsSessionStartedMsg:
		l.logsSession = msg.session
		return l.readLogsOutput()
	case logsOutputMsg:
		if msg.err != nil {
			if l.logsSession == nil {
				return nil // session was closed manually, ignore
			}
			if errors.Is(msg.err, io.EOF) {
				// Stream ended normally — keep logs visible, just stop reading.
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
		l.logsOutput += msg.output
		return l.readLogsOutput()
	}

	return nil
}

func (l *logsPanel) View() string {
	return l.logsOutput
}
func (l *logsPanel) Close() {
	if l.logsSession != nil {
		l.logsSession.Close()
		l.logsSession = nil
	}
	l.logsOutput = ""
}
func (l *logsPanel) SetSize(_, _ int) {}

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
		ctx := context.Background()
		session, err := l.client.Logs(ctx, containerID, client.LogOptions{Follow: true})
		if err != nil {
			return logsOutputMsg{err: err}
		}
		return logsSessionStartedMsg{session: session}
	}
}
