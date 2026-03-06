package containers

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

type logsPanel struct {
	ctx         context.Context
	logsSession *client.LogsSession
	viewport    viewport.Model
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

func NewLogsPanel(ctx context.Context, client client.ContainerService) panel.Panel {
	return &logsPanel{
		ctx:      ctx,
		client:   client,
		viewport: viewport.New(0, 0),
	}
}

func (l *logsPanel) Init(containerID string) tea.Cmd {
	return tea.Batch(l.init(containerID), l.extendHelpCmd())
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
		l.viewport.SetContent(l.logsOutput)
		l.viewport.GotoBottom()
		return l.readLogsOutput()
	}

	var cmd tea.Cmd
	l.viewport, cmd = l.viewport.Update(msg)

	return cmd
}

func (l *logsPanel) View() string {
	return l.viewport.View()
}
func (l *logsPanel) Close() tea.Cmd {
	if l.logsSession != nil {
		l.logsSession.Close()
		l.logsSession = nil
	}
	l.logsOutput = ""
	l.viewport.SetContent("")

	return func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} }
}
func (l *logsPanel) SetSize(width, height int) {
	l.viewport.Width = width
	l.viewport.Height = height
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
		ctx := context.Background()
		session, err := l.client.Logs(ctx, containerID, client.LogOptions{Follow: true})
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
		}}
	}
}
