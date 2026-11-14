package containers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/taigrr/bubbleterm"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
)

var noStyle = lipgloss.NewStyle()

var cursorStyle = lipgloss.NewStyle().Reverse(true)

const (
	defaultVtWidth  = 120
	defaultVtHeight = 30
	minimumVpWidth  = 30
	minimumVpHeight = 10
)

// execOutputMsg reports an exec session error.
type execOutputMsg struct {
	err error
}

type execSessionStartedMsg struct {
	session *client.ExecSession
}

// execCloseMsg is sent when the remote shell's output stream ends. It carries
// the session so a stale message from a replaced session is ignored.
type execCloseMsg struct {
	session *client.ExecSession
}

// eofReader wraps the exec reader so the panel learns when the shell exits;
// bubbleterm's pipe read loop stops silently on EOF.
type eofReader struct {
	r    io.Reader
	once sync.Once
	done chan struct{}
}

func (e *eofReader) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	if err != nil {
		e.once.Do(func() { close(e.done) })
	}
	return n, err
}

// execPanel runs an interactive shell inside the container, rendered by the
// bubbleterm terminal emulator. While focused with a live session it captures
// all keyboard input (see sections.InputCapturer).
type execPanel struct {
	ctx     context.Context
	service client.ContainerService
	session *client.ExecSession
	term    *bubbleterm.Model
	width   int
	height  int
}

func NewExecPanel(ctx context.Context, svc client.ContainerService) sections.Panel {
	return &execPanel{ctx: ctx, service: svc}
}

func (e *execPanel) Init(item sections.ListItem) tea.Cmd {
	container, ok := item.InnerItem().(client.Container)
	if !ok {
		return func() tea.Msg {
			return execOutputMsg{err: errors.New("error getting exec container. Item not a container")}
		}
	}

	if container.State != client.StateRunning {
		return func() tea.Msg {
			return execOutputMsg{err: errors.New("container is not running")}
		}
	}

	log.Printf("[containers][exec-panel] Init: containerID=%q", item.ID())
	return tea.Batch(e.startSession(container), e.extendHelpCmd())
}

func (e *execPanel) Name() string {
	return "Exec"
}

// ownsSession reports whether session is the panel's current session.
func (e *execPanel) ownsSession(session *client.ExecSession) bool {
	return session != nil && e.session == session
}

// CapturesInput implements sections.InputCapturer.
func (e *execPanel) CapturesInput() bool {
	return e.term != nil
}

func (e *execPanel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case execSessionStartedMsg:
		return e.attach(msg.session)
	case execOutputMsg:
		if msg.err != nil {
			return e.fail(msg.err)
		}
		return nil
	case tea.WindowSizeMsg:
		return e.execResize(e.width, e.height)
	case tea.MouseMsg:
		// bubbleterm forwards raw screen coordinates; the panel sits at an
		// offset inside the split layout, so mouse events would be misplaced.
		return nil
	}

	if e.term == nil {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return e.sendKey(msg)
	case tea.PasteMsg:
		// bubbleterm ignores paste events.
		if err := e.term.GetEmulator().SendKey(msg.Content); err != nil {
			return e.fail(err)
		}
		return nil
	}

	_, cmd := e.term.Update(msg)
	return cmd
}

// sendKey writes a key press to the shell synchronously. bubbleterm returns the
// write as a tea.Cmd, and Bubble Tea runs each Cmd in its own goroutine, so
// keystrokes would otherwise reach the shell out of order.
func (e *execPanel) sendKey(msg tea.KeyPressMsg) tea.Cmd {
	_, cmd := e.term.Update(msg)
	if cmd == nil {
		return nil
	}
	if result := cmd(); result != nil {
		return func() tea.Msg { return result }
	}
	return nil
}

func (e *execPanel) View() string {
	if e.term == nil {
		return ""
	}
	return e.renderCursor(e.term.View().Content)
}

// renderCursor draws the emulator cursor as a reverse-video cell; bubbleterm
// renders screen contents only.
func (e *execPanel) renderCursor(content string) string {
	pos, visible := e.term.GetEmulator().Cursor()
	if !visible {
		return content
	}
	rows := strings.Split(content, "\n")
	if pos.Y < 0 || pos.Y >= len(rows) || pos.X < 0 {
		return content
	}
	row := rows[pos.Y]
	if pos.X >= ansi.StringWidth(row) {
		return content
	}
	cell := ansi.Strip(ansi.Cut(row, pos.X, pos.X+1))
	if cell == "" {
		cell = " "
	}
	rows[pos.Y] = ansi.Truncate(row, pos.X, "") + cursorStyle.Render(cell) + ansi.TruncateLeft(row, pos.X+1, "")
	return strings.Join(rows, "\n")
}

func (e *execPanel) Close() tea.Cmd {
	log.Printf("[containers][exec-panel] closing session")
	if e.term != nil {
		_ = e.term.Close()
		e.term = nil
	}
	if e.session != nil {
		e.session.Close()
		e.session = nil
	}
	return func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} }
}

func (e *execPanel) SetSize(width, height int) {
	e.width = width
	e.height = height
	if e.term != nil {
		// Resize synchronously; SetSize cannot return a Cmd.
		_ = e.term.GetEmulator().Resize(max(width, 1), max(height, 1))
	}
}

func (e *execPanel) attach(session *client.ExecSession) tea.Cmd {
	log.Printf("[containers][exec-panel] session started")
	if e.term != nil {
		_ = e.term.Close()
	}
	e.session = session
	done := make(chan struct{})
	reader := &eofReader{r: session.Reader, done: done}

	writer := newQueuedWriter(session.Writer)
	term, err := bubbleterm.NewWithPipes(max(e.width, 1), max(e.height, 1), reader, writer)
	if err != nil {
		return e.fail(err)
	}
	e.term = term

	return tea.Batch(
		term.Init(),
		e.execResize(e.width, e.height),
		func() tea.Msg {
			<-done
			return execCloseMsg{session: session}
		},
	)
}

func (e *execPanel) fail(err error) tea.Cmd {
	return tea.Batch(e.Close(), func() tea.Msg {
		return message.ShowBannerMsg{
			Message: fmt.Sprintf("Exec session error. Err: %s", err),
			IsError: true,
		}
	})
}

func (e *execPanel) execResize(width, height int) tea.Cmd {
	if e.session == nil {
		return nil
	}
	svc := e.service
	execID := e.session.ID
	ctx := e.ctx

	vpWidth := uint(max(width, minimumVpWidth))
	vpHeight := uint(max(height, minimumVpHeight))

	return func() tea.Msg {
		if err := svc.ExecResize(ctx, execID, vpWidth, vpHeight); err != nil {
			return execOutputMsg{err: err}
		}
		return nil
	}
}

func (e *execPanel) startSession(container client.Container) tea.Cmd {
	ctx := e.ctx
	svc := e.service

	return func() tea.Msg {
		session, err := svc.Exec(ctx, container.ID, defaultVtWidth, defaultVtHeight)
		if err != nil {
			return execOutputMsg{err: err}
		}
		return execSessionStartedMsg{session: session}
	}
}

func (e *execPanel) extendHelpCmd() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			keys.Keys.ReleaseInput,
		}}
	}
}
