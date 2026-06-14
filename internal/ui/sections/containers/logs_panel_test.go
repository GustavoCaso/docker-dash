package containers

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func newTestLogsPanel() *logsPanel {
	return NewLogsPanel(
		context.Background(),
		client.NewMockClient().Containers(),
		config.DefaultLogsConfig(),
	).(*logsPanel)
}

func TestLogsPanelInitStartsSession(t *testing.T) {
	p := newTestLogsPanel()
	cmd := p.Init(containerItem{container: client.Container{ID: "abc123def456"}})
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
	p.SetSize(100, 100)

	p.Update(logsOutputMsg{output: "first line\n"})
	p.Update(logsOutputMsg{output: "second line\n"})

	items := p.list.Items()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestLogsPanelClose(t *testing.T) {
	p := newTestLogsPanel()
	p.SetSize(100, 50)
	pr, pw := io.Pipe()
	p.logsSession = client.NewLogsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })
	p.appendLines("some output\n")

	p.Close()

	if p.logsSession != nil {
		t.Error("Close() should nil out logsSession")
	}
	if len(p.list.Items()) != 0 {
		t.Errorf("Close() should clear list items, got %d", len(p.list.Items()))
	}
	if p.lineBuffer != "" {
		t.Errorf("Close() should clear lineBuffer, got %q", p.lineBuffer)
	}
}

func TestLogsPanelCloseIsIdempotent(t *testing.T) {
	p := newTestLogsPanel()
	// Call Close with no session — should not panic.
	p.Close()
	p.Close()
}

func TestLogsPanelLineBuffering(t *testing.T) {
	p := newTestLogsPanel()
	p.SetSize(200, 50)

	// First chunk: incomplete line
	p.Update(logsOutputMsg{output: "partial"})
	if len(p.list.Items()) != 0 {
		t.Errorf("incomplete line should not create item, got %d items", len(p.list.Items()))
	}
	if p.lineBuffer != "partial" {
		t.Errorf("lineBuffer = %q, want %q", p.lineBuffer, "partial")
	}

	// Second chunk: completes the line and starts another
	p.Update(logsOutputMsg{output: " line\nsecond\n"})
	if len(p.list.Items()) != 2 {
		t.Errorf("expected 2 items, got %d", len(p.list.Items()))
	}
	if p.lineBuffer != "" {
		t.Errorf("lineBuffer should be empty, got %q", p.lineBuffer)
	}
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

func TestLogsPanelUsesLogsCfgOptions(t *testing.T) {
	cfg := config.LogsConfig{
		Follow:     false,
		Tail:       "50",
		Timestamps: true,
		Since:      "30m",
	}
	p := NewLogsPanel(
		context.Background(),
		client.NewMockClient().Containers(),
		cfg,
	).(*logsPanel)
	if p.logsCfg.Follow {
		t.Error("logsCfg.Follow not stored")
	}
	if p.logsCfg.Tail != "50" {
		t.Errorf("logsCfg.Tail = %q, want %q", p.logsCfg.Tail, "50")
	}
	if p.logsCfg.Since != "30m" {
		t.Errorf("logsCfg.Since = %q, want %q", p.logsCfg.Since, "30m")
	}
	if !p.logsCfg.Timestamps {
		t.Error("logsCfg.Timestamps not stored")
	}
}
