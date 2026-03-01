package containers

import (
	"errors"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func newTestExecPanel() *execPanel {
	return NewExecPanel(client.NewMockClient().Containers()).(*execPanel)
}

func TestExecPanelInitStartsSession(t *testing.T) {
	p := newTestExecPanel()
	cmd := p.Init("abc123def456")
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
	msg := cmd()

	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatal("Init() not returned BatchMsg")
	}

	startSessionCmd := false
	extendCmd := false

	for _, cmd := range batch {
		msg := cmd()
		switch msg.(type) {
		case execSessionStartedMsg:
			startSessionCmd = true
		case message.AddContextualKeyBindingsMsg:
			extendCmd = true
		}
	}

	if !startSessionCmd {
		t.Fatal("Init() not returned execSessionStartedMsg msg")
	}

	if !extendCmd {
		t.Fatal("Init() not returned AddContextualKeyBindingsMsg msg")
	}
}

func TestExecPanelUpdateOnSessionStartedStoresSession(t *testing.T) {
	p := newTestExecPanel()
	pr, pw := io.Pipe()
	session := client.NewExecSession(io.NopCloser(pr), pw, func() { pr.Close(); pw.Close() })

	go func() {
		pw.Write([]byte("output\n"))
		pw.Close()
	}()

	cmd := p.Update(execSessionStartedMsg{session: session})
	if cmd == nil {
		t.Fatal("Update(execSessionStartedMsg) returned nil cmd")
	}
	if p.session != session {
		t.Error("Update should store the session")
	}

	cmd() // drain goroutine
}

func TestExecPanelAccumulatesOutput(t *testing.T) {
	p := newTestExecPanel()

	p.Update(execOutputMsg{output: "line one\n"})
	p.Update(execOutputMsg{output: "line two\n"})

	if !strings.Contains(p.output, "line one") {
		t.Errorf("output missing 'line one', got %q", p.output)
	}
	if !strings.Contains(p.output, "line two") {
		t.Errorf("output missing 'line two', got %q", p.output)
	}
}

func TestExecPanelErrorEmitsBanner(t *testing.T) {
	p := newTestExecPanel()
	pr, pw := io.Pipe()
	p.session = client.NewExecSession(io.NopCloser(pr), pw, func() { pr.Close(); pw.Close() })

	cmd := p.Update(execOutputMsg{err: errors.New("pipe broken")})

	if p.session != nil {
		t.Error("Update with error should close session")
	}
	if cmd == nil {
		t.Fatal("Update with error should return banner cmd")
	}
	result := cmd()
	bannerMsg, ok := result.(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", result)
	}
	if !bannerMsg.IsError {
		t.Error("ShowBannerMsg.IsError should be true")
	}
}

func TestExecPanelHandleEscEmitsCloseMsg(t *testing.T) {
	p := newTestExecPanel()
	pr, pw := io.Pipe()
	p.session = client.NewExecSession(io.NopCloser(pr), pw, func() { pr.Close(); pw.Close() })
	p.input.Focus()

	cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should return a cmd")
	}

	msg := cmd()

	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatal("Init() not returned BatchMsg")
	}

	closeSessionCmd := false
	clearBindings := false

	for _, cmd := range batch {
		msg := cmd()
		switch msg.(type) {
		case execCloseMsg:
			closeSessionCmd = true
		case message.ClearContextualKeyBindingsMsg:
			clearBindings = true
		}
	}

	if !closeSessionCmd {
		t.Fatal("Update() not returned execCloseMsg msg")
	}

	if !clearBindings {
		t.Fatal("Update() not returned ClearContextualKeyBindingsMsg msg")
	}
}

func TestExecPanelClearClearsOutput(t *testing.T) {
	p := newTestExecPanel()
	pr, pw := io.Pipe()
	p.session = client.NewExecSession(io.NopCloser(pr), pw, func() { pr.Close(); pw.Close() })
	p.output = "old output"
	p.input.Focus()

	runes := []rune("clear")
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: runes})
	p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if p.output != "" {
		t.Errorf("output should be cleared, got %q", p.output)
	}

	p.session.Close()
}

func TestExecPanelClearWithExtraSpaceClearsOutput(t *testing.T) {
	p := newTestExecPanel()
	pr, pw := io.Pipe()
	p.session = client.NewExecSession(io.NopCloser(pr), pw, func() { pr.Close(); pw.Close() })
	p.output = "old output"
	p.input.Focus()

	runes := []rune(" clear ")
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: runes})
	p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if p.output != "" {
		t.Errorf("output should be cleared, got %q", p.output)
	}

	p.session.Close()
}

func TestExecPanelHistoryNavigation(t *testing.T) {
	p := newTestExecPanel()
	pr, pw := io.Pipe()
	p.session = client.NewExecSession(io.NopCloser(pr), pw, func() { pr.Close(); pw.Close() })
	p.history = []string{"ls", "pwd"}
	p.historyIdx = len(p.history)
	p.input.Focus()

	// Press Up: should show "pwd" (most recent)
	p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.input.Value() != "pwd" {
		t.Errorf("after Up: input = %q, want 'pwd'", p.input.Value())
	}

	// Press Up again: should show "ls"
	p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.input.Value() != "ls" {
		t.Errorf("after second Up: input = %q, want 'ls'", p.input.Value())
	}

	// Press Down: should go back to "pwd"
	p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.input.Value() != "pwd" {
		t.Errorf("after Down: input = %q, want 'pwd'", p.input.Value())
	}

	p.session.Close()
}

func TestExecPanelCloseNilsSession(t *testing.T) {
	p := newTestExecPanel()
	pr, pw := io.Pipe()
	p.session = client.NewExecSession(io.NopCloser(pr), pw, func() { pr.Close(); pw.Close() })
	p.output = "some output"

	p.Close()

	if p.session != nil {
		t.Error("Close() should nil out session")
	}
	if p.output != "" {
		t.Errorf("Close() should clear output, got %q", p.output)
	}
}

func TestExecPanelCloseIsIdempotent(t *testing.T) {
	p := newTestExecPanel()
	p.Close()
	p.Close() // must not panic
}

func TestExecPanelSetSizeSizesViewport(t *testing.T) {
	p := newTestExecPanel()
	p.SetSize(100, 30)

	if p.width != 100 {
		t.Errorf("width = %d, want 100", p.width)
	}
	if p.viewport.Height != 29 { // height - 1 for input
		t.Errorf("viewport.Height = %d, want 29", p.viewport.Height)
	}
}
