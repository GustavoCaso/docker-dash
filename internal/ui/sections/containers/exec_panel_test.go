package containers

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"

	"github.com/GustavoCaso/docker-dash/internal/client"
)

// driver runs tea.Cmds in goroutines and feeds results back into the panel,
// emulating the Bubble Tea runtime.
type driver struct {
	t    *testing.T
	p    *execPanel
	msgs chan tea.Msg
	seen []tea.Msg
}

func newDriver(t *testing.T, p *execPanel) *driver {
	return &driver{t: t, p: p, msgs: make(chan tea.Msg, 256)}
}

func (d *driver) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		msg := cmd()
		if msg != nil {
			d.msgs <- msg
		}
	}()
}

func (d *driver) send(msg tea.Msg) {
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			d.run(c)
		}
		return
	}
	d.seen = append(d.seen, msg)
	d.run(d.p.Update(msg))
}

func (d *driver) until(desc string, cond func() bool) {
	d.t.Helper()
	deadline := time.After(5 * time.Second)
	for !cond() {
		select {
		case msg := <-d.msgs:
			d.send(msg)
		case <-deadline:
			d.t.Fatalf("timeout waiting for %s; view:\n%s", desc, ansi.Strip(d.p.View()))
		}
	}
}

func (d *driver) typeText(s string) {
	for _, r := range s {
		switch r {
		case '\r':
			d.send(tea.KeyPressMsg{Code: tea.KeyEnter})
		default:
			d.send(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
	}
}

func plain(p *execPanel) string { return ansi.Strip(p.View()) }

// startShell attaches a real /bin/sh through a PTY, mirroring a Docker TTY exec stream.
func startShell(t *testing.T, d *driver) {
	t.Helper()
	cmd := exec.Command("/bin/sh")
	cmd.Env = []string{"PS1=PROMPT$ ", "TERM=xterm-256color", "PATH=/usr/bin:/bin"}
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("pty start: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	session := client.NewExecSession("test", ptmx, ptmx, func() { _ = ptmx.Close() })
	d.send(execSessionStartedMsg{session: session})
}

func newSizedPanel() *execPanel {
	p := NewExecPanel(context.Background(), client.NewMockClient().Containers()).(*execPanel)
	p.SetSize(80, 24)
	return p
}

func TestBubbletermShellEcho(t *testing.T) {
	p := newSizedPanel()
	d := newDriver(t, p)
	startShell(t, d)

	d.until("prompt", func() bool { return strings.Contains(plain(p), "PROMPT$") })
	d.typeText("echo hello-$((40+2))\r")
	d.until("echo output", func() bool { return strings.Contains(plain(p), "hello-42") })
}

func TestBubbletermCursorAddressingAndColor(t *testing.T) {
	p := newSizedPanel()
	d := newDriver(t, p)
	startShell(t, d)

	d.until("prompt", func() bool { return strings.Contains(plain(p), "PROMPT$") })
	// Clear screen, jump to row 10 col 20, print red text.
	d.typeText(`printf '\033[2J\033[10;20H\033[31mRED\033[0m'` + "\r")
	d.until("positioned text", func() bool {
		lines := strings.Split(plain(p), "\n")
		return len(lines) > 9 && strings.Index(lines[9], "RED") == 19
	})
	if !strings.Contains(p.View(), "\x1b[31m") {
		t.Fatalf("expected red SGR in rendered view, got %q", p.View())
	}
}

func TestBubbletermCtrlCInterrupts(t *testing.T) {
	p := newSizedPanel()
	d := newDriver(t, p)
	startShell(t, d)

	d.until("prompt", func() bool { return strings.Contains(plain(p), "PROMPT$") })
	d.typeText("sleep 30\r")
	time.Sleep(200 * time.Millisecond)
	d.send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	d.typeText("echo after-int\r")
	d.until("shell responsive after ctrl+c", func() bool { return strings.Contains(plain(p), "after-int") })
}

func TestBubbletermResize(t *testing.T) {
	p := newSizedPanel()
	d := newDriver(t, p)
	startShell(t, d)
	d.until("prompt", func() bool { return strings.Contains(plain(p), "PROMPT$") })

	p.SetSize(40, 10)
	d.send(tea.WindowSizeMsg{Width: 40, Height: 10})
	d.typeText("echo resized\r")
	d.until("resized frame", func() bool {
		lines := strings.Split(plain(p), "\n")
		return len(lines) == 10 && strings.Contains(plain(p), "resized")
	})
}

func TestBubbletermExitClosesPanel(t *testing.T) {
	p := newSizedPanel()
	d := newDriver(t, p)
	startShell(t, d)
	d.until("prompt", func() bool { return strings.Contains(plain(p), "PROMPT$") })

	d.typeText("exit\r")
	d.until("execCloseMsg", func() bool {
		for _, m := range d.seen {
			if _, ok := m.(execCloseMsg); ok {
				return true
			}
		}
		return false
	})
}

func TestBubbletermAltScreen(t *testing.T) {
	p := newSizedPanel()
	d := newDriver(t, p)
	startShell(t, d)
	d.until("prompt", func() bool { return strings.Contains(plain(p), "PROMPT$") })

	d.typeText("echo main-marker\r")
	d.until("main marker", func() bool { return strings.Contains(plain(p), "main-marker") })
	d.typeText(`printf '\033[?1049h\033[HALT''SCREEN'; read x; printf '\033[?1049l'` + "\r")
	d.until("alt screen", func() bool {
		v := plain(p)
		return strings.Contains(v, "ALTSCREEN") && !strings.Contains(v, "main-marker")
	})
	d.typeText("\r")
	d.until("main screen restored", func() bool {
		v := plain(p)
		return strings.Contains(v, "main-marker") && !strings.Contains(v, "ALTSCREEN")
	})
}

func TestBubbletermRendersCursor(t *testing.T) {
	p := newSizedPanel()
	d := newDriver(t, p)
	startShell(t, d)
	d.until("prompt", func() bool { return strings.Contains(plain(p), "PROMPT$") })

	d.typeText("ab")
	d.until("typed text", func() bool { return strings.Contains(plain(p), "PROMPT$ ab") })
	// Cursor sits right after "PROMPT$ ab" on the first row, drawn in reverse video.
	firstRow := strings.Split(p.View(), "\n")[0]
	if !strings.Contains(firstRow, "\x1b[7m") {
		t.Fatalf("expected reverse-video cursor on first row, got %q", firstRow)
	}
	if got := ansi.StringWidth(firstRow); got != 80 {
		t.Fatalf("cursor rendering changed row width: got %d, want 80", got)
	}
}
