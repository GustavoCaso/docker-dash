package containers

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
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
	p.SetSize(100, 100)

	p.Update(logsOutputMsg{output: "first line\n"})
	p.Update(logsOutputMsg{output: "second line\n"})

	items := p.list.Items()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	line0, ok := items[0].(logLine)
	if !ok {
		t.Fatal("item[0] is not logLine")
	}
	if line0.content != "first line" {
		t.Errorf("item[0] = %q, want 'first line'", line0.content)
	}
	line1, ok := items[1].(logLine)
	if !ok {
		t.Fatal("item[1] is not logLine")
	}
	if line1.content != "second line" {
		t.Errorf("item[1] = %q, want 'second line'", line1.content)
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
	if p.delegate.hOffset != 0 {
		t.Errorf("Close() should reset hOffset, got %d", p.delegate.hOffset)
	}
	if p.prevIndex != 0 {
		t.Errorf("Close() should reset prevIndex, got %d", p.prevIndex)
	}
}

func TestLogsPanelCloseIsIdempotent(t *testing.T) {
	p := newTestLogsPanel()
	// Call Close with no session — should not panic.
	p.Close()
	p.Close()
}

func TestLogDelegateTruncatesNonSelectedLines(t *testing.T) {
	d := newLogDelegate()
	d.hOffset = 0

	items := []list.Item{
		logLine{content: strings.Repeat("x", 200)},
		logLine{content: "short"},
	}
	m := list.New(items, d, 50, 10)
	m.Select(1) // second item selected

	var buf strings.Builder
	d.Render(&buf, m, 0, items[0])
	rendered := buf.String()
	if len(rendered) > 55 { // 50 width + some style overhead
		t.Errorf("non-selected long line not truncated, len=%d", len(rendered))
	}
	if !strings.Contains(rendered, "…") {
		t.Errorf("expected ellipsis in truncated line, got %q", rendered)
	}
}

func TestLogDelegateAppliesHOffsetOnSelectedLine(t *testing.T) {
	d := newLogDelegate()
	d.hOffset = 5

	content := "0123456789abcdef"
	items := []list.Item{logLine{content: content}}
	m := list.New(items, d, 50, 10)
	m.Select(0)

	var buf strings.Builder
	d.Render(&buf, m, 0, items[0])
	rendered := buf.String()
	if strings.Contains(rendered, "01234") {
		t.Errorf("hOffset not applied: first 5 chars still visible in %q", rendered)
	}
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

func TestLogsPanelHScrollClamping(t *testing.T) {
	p := newTestLogsPanel()
	p.SetSize(10, 10)
	p.Update(logsOutputMsg{output: "0123456789abcdef\n"}) // 16 chars, width=10

	selected, ok := p.list.SelectedItem().(logLine)
	if !ok {
		t.Fatal("selected item is not logLine")
	}
	maxOffset := max(0, len([]rune(selected.content))-p.list.Width())

	// scroll right past max — hOffset must clamp at maxOffset
	scrollRight := tea.KeyMsg{Type: tea.KeyRight}
	for range 20 {
		p.Update(scrollRight)
	}
	if p.delegate.hOffset > maxOffset {
		t.Errorf("hOffset %d exceeds maxOffset %d", p.delegate.hOffset, maxOffset)
	}

	// scroll left past zero — hOffset must not go negative
	scrollLeft := tea.KeyMsg{Type: tea.KeyLeft}
	for range 20 {
		p.Update(scrollLeft)
	}
	if p.delegate.hOffset != 0 {
		t.Errorf("hOffset should not go below 0, got %d", p.delegate.hOffset)
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
