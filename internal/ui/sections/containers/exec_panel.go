package containers

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

var noStyle = lipgloss.NewStyle()

// execOutputMsg is sent when exec output is received from the background reader.
type execOutputMsg struct {
	output string
	err    error
}

type execSessionStartedMsg struct {
	session *client.ExecSession
}

type execCloseMsg struct{}

type execPanel struct {
	service    client.ContainerService
	session    *client.ExecSession
	viewport   viewport.Model
	input      textinput.Model
	output     string
	history    []string
	historyIdx int
	width      int
}

func NewExecPanel(svc client.ContainerService) panel.Panel {
	ti := textinput.New()
	ti.Prompt = "$ "
	vp := viewport.New(0, 0)
	return &execPanel{
		service:  svc,
		input:    ti,
		viewport: vp,
		history:  []string{},
	}
}

func (e *execPanel) Init(containerID string) tea.Cmd {
	e.output = ""
	e.history = []string{}
	e.historyIdx = 0
	e.input.Focus()
	return tea.Batch(
		textinput.Blink,
		e.startSession(containerID),
		e.extendHelpCmd(),
	)
}

func (e *execPanel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case execSessionStartedMsg:
		e.session = msg.session
		return e.readOutput()
	case execOutputMsg:
		if msg.err != nil {
			if e.session == nil {
				return nil
			}
			e.Close()
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Exec session error. Err: %s", msg.err),
					IsError: true,
				}
			}
		}
		e.output += msg.output
		e.viewport.SetContent(noStyle.Width(e.width).Render(e.output))
		e.viewport.GotoBottom()
		return e.readOutput()
	case tea.KeyMsg:
		return e.handleKeyInput(msg)
	}

	// Forward blinking cursor ticks to the text input.
	var inputCmd tea.Cmd
	e.input, inputCmd = e.input.Update(msg)
	return inputCmd
}

func (e *execPanel) View() string {
	return lipgloss.JoinVertical(lipgloss.Left, e.viewport.View(), e.input.View())
}

func (e *execPanel) Close() {
	e.input.Blur()
	if e.session != nil {
		e.session.Close()
		e.session = nil
	}
	e.output = ""
	e.history = []string{}
	e.historyIdx = 0
	e.viewport.SetContent("")
}

func (e *execPanel) SetSize(width, height int) {
	e.width = width
	e.viewport.Width = width
	e.viewport.Height = height - 1 // reserve 1 line for input
}

func (e *execPanel) handleKeyInput(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Keys.Esc):
		return tea.Batch(
			e.closeSessionCmd(),
			func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} },
		)
	case key.Matches(msg, keys.Keys.Enter):
		if e.session == nil {
			return nil
		}
		cmd := e.input.Value()
		if cmd == "" {
			return nil
		}
		if strings.TrimSpace(cmd) == "clear" {
			e.input.Reset()
			e.output = ""
			e.viewport.SetContent("")
			return nil
		}
		e.history = append(e.history, cmd)
		e.historyIdx = len(e.history)
		e.input.Reset()
		_, err := e.session.Writer.Write([]byte(cmd + "\n"))
		if err != nil {
			e.Close()
			return func() tea.Msg {
				return message.ShowBannerMsg{Message: "Exec write failed", IsError: true}
			}
		}
		return nil
	case key.Matches(msg, keys.Keys.Up):
		if len(e.history) == 0 {
			return nil
		}
		switch {
		case e.historyIdx > 0:
			e.historyIdx--
		case e.historyIdx == len(e.history):
			e.historyIdx = len(e.history) - 1
		default:
			return nil
		}
		e.input.SetValue(e.history[e.historyIdx])
		return nil
	case key.Matches(msg, keys.Keys.Down):
		if len(e.history) == 0 || e.historyIdx == len(e.history) {
			return nil
		}
		e.historyIdx++
		if e.historyIdx == len(e.history) {
			e.input.Reset()
		} else {
			e.input.SetValue(e.history[e.historyIdx])
		}
		return nil
	default:
		var inputCmd tea.Cmd
		e.input, inputCmd = e.input.Update(msg)
		return inputCmd
	}
}

func (e *execPanel) startSession(containerID string) tea.Cmd {
	svc := e.service
	return func() tea.Msg {
		ctx := context.Background()
		session, err := svc.Exec(ctx, containerID)
		if err != nil {
			return execOutputMsg{err: err}
		}
		return execSessionStartedMsg{session: session}
	}
}

func (e *execPanel) closeSessionCmd() tea.Cmd {
	return func() tea.Msg {
		return execCloseMsg{}
	}
}

func (e *execPanel) readOutput() tea.Cmd {
	session := e.session
	if session == nil {
		return nil
	}
	return func() tea.Msg {
		buf := make([]byte, readBufSize)
		n, err := session.Reader.Read(buf)
		if err != nil {
			return execOutputMsg{err: err}
		}
		return execOutputMsg{output: string(buf[:n])}
	}
}

func (e *execPanel) extendHelpCmd() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "exit")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send command")),
			key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "history up")),
			key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "history down")),
		}}
	}
}
