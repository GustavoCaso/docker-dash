package containers

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func newTestLogsPanel() *logsPanel {
	return NewLogsPanel(context.Background(), client.NewMockClient().Containers()).(*logsPanel)
}

func TestLogsPanelInitStartsSession(t *testing.T) {
	p := newTestLogsPanel()
	cmd := p.Init("abc123def456")
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init() cmd returned %T, want tea.BatchMsg", msg)
	}
	logsSessionStarted := false
	extendCmd := false

	for _, cmd := range batch {
		msg := cmd()
		switch msg.(type) {
		case logsSessionStartedMsg:
			logsSessionStarted = true
		case message.AddContextualKeyBindingsMsg:
			extendCmd = true
		}
	}

	if !logsSessionStarted {
		t.Fatal("Init() not returned logsSessionStartedMsg msg")
	}

	if !extendCmd {
		t.Fatal("Init() not returned AddContextualKeyBindingsMsg msg")
	}
}

func TestLogsPanelUpdateOnSessionStartedStoresSession(t *testing.T) {
	p := newTestLogsPanel()
	pr, pw := io.Pipe()
	session := client.NewLogsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })

	go func() {
		pw.Write([]byte("log line\n"))
		pw.Close()
	}()

	cmd := p.Update(logsSessionStartedMsg{session: session})
	if cmd == nil {
		t.Fatal("Update(logsSessionStartedMsg) returned nil cmd")
	}
	if p.logsSession != session {
		t.Error("Update(logsSessionStartedMsg) should store the session on the panel")
	}

	// Drain the cmd so the goroutine can exit cleanly.
	cmd()
}

func TestLogsPanelUpdateOnSessionStartedReadsOutput(t *testing.T) {
	p := newTestLogsPanel()
	pr, pw := io.Pipe()
	session := client.NewLogsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })

	go func() {
		pw.Write([]byte("hello logs\n"))
		pw.Close()
	}()

	cmd := p.Update(logsSessionStartedMsg{session: session})
	msg := cmd()
	outputMsg, ok := msg.(logsOutputMsg)
	if !ok {
		t.Fatalf("readLogsOutput returned %T, want logsOutputMsg", msg)
	}
	if !strings.Contains(outputMsg.output, "hello logs") {
		t.Errorf("output = %q, want to contain 'hello logs'", outputMsg.output)
	}
}

func TestLogsPanelAccumulatesOutput(t *testing.T) {
	p := newTestLogsPanel()

	p.Update(logsOutputMsg{output: "first line\n"})
	p.Update(logsOutputMsg{output: "second line\n"})

	view := p.View()
	if !strings.Contains(view, "first line") {
		t.Errorf("View() missing 'first line', got %q", view)
	}
	if !strings.Contains(view, "second line") {
		t.Errorf("View() missing 'second line', got %q", view)
	}
}

func TestLogsPanelClose(t *testing.T) {
	p := newTestLogsPanel()
	pr, pw := io.Pipe()
	p.logsSession = client.NewLogsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })
	p.logsOutput = "some output"

	p.Close()

	if p.logsSession != nil {
		t.Error("Close() should nil out logsSession")
	}
	if p.logsOutput != "" {
		t.Errorf("Close() should clear logsOutput, got %q", p.logsOutput)
	}
}

func TestLogsPanelCloseIsIdempotent(t *testing.T) {
	p := newTestLogsPanel()
	// Call Close with no session — should not panic.
	p.Close()
	p.Close()
}

func TestLogsPanelErrorEmitsShowsBanner(t *testing.T) {
	p := newTestLogsPanel()
	pr, pw := io.Pipe()
	p.logsSession = client.NewLogsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })

	cmd := p.Update(logsOutputMsg{err: errors.New("connection reset")})

	if p.logsSession != nil {
		t.Error("Update with error should close the session")
	}
	if cmd == nil {
		t.Fatal("Update with error should return a banner cmd")
	}
	msg := cmd()
	bannerMsg, ok := msg.(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("returned %T, want message.ShowBannerMsg", msg)
	}
	if !bannerMsg.IsError {
		t.Error("ShowBannerMsg.IsError should be true for session errors")
	}
}
