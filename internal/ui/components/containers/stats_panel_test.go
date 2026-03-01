package containers

import (
	"errors"
	"io"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func newTestStatsPanel() *statsPanel {
	return NewStatsPanel(client.NewMockClient().Containers()).(*statsPanel)
}

func TestStatsPanelInitStartsSession(t *testing.T) {
	p := newTestStatsPanel()
	cmd := p.Init("abc123def456") // running container in mock
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
	msg := cmd()
	sessionMsg, ok := msg.(statsSessionStartedMsg)
	if !ok {
		t.Fatalf("Init() cmd returned %T, want statsSessionStartedMsg", msg)
	}
	if sessionMsg.session == nil {
		t.Error("statsSessionStartedMsg.session is nil")
	}
	sessionMsg.session.Close()
}

func TestStatsPanelUpdateOnSessionStartedStoresSession(t *testing.T) {
	p := newTestStatsPanel()
	pr, pw := io.Pipe()
	session := client.NewStatsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })

	go func() {
		pw.Write([]byte("{}"))
		pw.Close()
	}()

	cmd := p.Update(statsSessionStartedMsg{session: session})
	if cmd == nil {
		t.Fatal("Update(statsSessionStartedMsg) returned nil cmd")
	}
	if p.session != session {
		t.Error("Update should store the session")
	}

	cmd() // drain
}

func TestStatsPanelErrorEmitsBanner(t *testing.T) {
	p := newTestStatsPanel()
	pr, pw := io.Pipe()
	p.session = client.NewStatsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })

	cmd := p.Update(statsOutputMsg{err: errors.New("stats broken")})

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

func TestStatsPanelCloseNilsSession(t *testing.T) {
	p := newTestStatsPanel()
	pr, pw := io.Pipe()
	p.session = client.NewStatsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })
	p.lastView = "some chart"

	p.Close()

	if p.session != nil {
		t.Error("Close() should nil out session")
	}
	if p.lastView != "" {
		t.Errorf("Close() should clear lastView, got %q", p.lastView)
	}
}

func TestStatsPanelCloseIsIdempotent(t *testing.T) {
	p := newTestStatsPanel()
	p.Close()
	p.Close() // must not panic
}

func TestStatsPanelSetSizeResizesCharts(t *testing.T) {
	p := newTestStatsPanel()
	p.SetSize(100, 40)

	if p.width != 100 {
		t.Errorf("width = %d, want 100", p.width)
	}
	if p.height != 40 {
		t.Errorf("height = %d, want 40", p.height)
	}
}

func TestStatsPanelViewReturnsLastView(t *testing.T) {
	p := newTestStatsPanel()
	p.lastView = "CPU chart"

	if p.View() != "CPU chart" {
		t.Errorf("View() = %q, want 'CPU chart'", p.View())
	}
}
