package panel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func newTestFileTreePanel() *filesPanel {
	return NewFilesPanel(context.Background(), client.NewMockClient().Containers()).(*filesPanel)
}

func TestFileTreePanelInitFetchesTree(t *testing.T) {
	p := newTestFileTreePanel()
	cmd := p.Init("abc123def456")
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
	if !p.loading {
		t.Error("Init() Must set loading state")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init() cmd returned %T, want tea.BatchMsg", msg)
	}
	containersTreeLoaded := false
	tickMsg := false
	extendCmd := false

	for _, cmd := range batch {
		msg := cmd()
		switch msg.(type) {
		case fileNodeLoadedMsg:
			containersTreeLoaded = true
		case message.AddContextualKeyBindingsMsg:
			extendCmd = true
		case spinner.TickMsg:
			tickMsg = true
		}
	}

	if !containersTreeLoaded {
		t.Fatal("Init() not returned fileNodeLoadedMsg msg")
	}

	if !extendCmd {
		t.Fatal("Init() not returned AddContextualKeyBindingsMsg msg")
	}

	if !tickMsg {
		t.Fatal("Init() not returned pinner.TickMsg msg")
	}
}

func TestFileTreePanelUpdateSetsContent(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)

	cmd := p.Init("abc123def456")
	msg := cmd()

	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init() cmd returned %T, want tea.BatchMsg", msg)
	}

	for _, cmd := range batch {
		// Update returns nil on success (no follow-up cmd needed)
		p.Update(cmd())
	}

	if p.loading {
		t.Error("Update() Must set loading state to false. Got true")
	}

	// MockClient.FileTree returns an empty tree — just ensure no panic and content is set.
	// An empty tree still renders as some string (possibly empty); what matters is no panic.
	_ = p.View()
}

func TestFileTreePanelUpdateWithError(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)

	errMsg := fileNodeLoadedMsg{err: errors.New("fetch failed")}
	cmd := p.Update(errMsg)

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

func TestFileTreePanelCloseResets(t *testing.T) {
	p := newTestFileTreePanel()

	p.Close()

	if p.loading {
		t.Error("Close() Must set loading state to false. Got true")
	}

	if p.View() != "" {
		t.Errorf("Close() should clear View, got %q", p.View())
	}
}

func TestFileTreePanelCloseIsIdempotent(t *testing.T) {
	p := newTestFileTreePanel()
	p.Close()
	p.Close() // must not panic
}

func TestFileTreePanelViewReturnsViewPort(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)
	p.visible = []*client.FileNode{
		{
			Name:  "test",
			IsDir: true,
			Depth: 2,
		},
	}

	if !strings.Contains(p.View(), "▼ test/") {
		t.Errorf("View() = %q, want to contain '▼ test/'", p.View())
	}
}

func TestFileTreePanelViewReturnsLoadingState(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)
	p.loading = true

	if !strings.Contains(p.View(), "Loading") {
		t.Errorf("View() = %q, want to contain 'Loading'", p.View())
	}
}

func TestFileTreePanelSetSizeStoresWidth(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(100, 50)

	if p.width != 100 {
		t.Errorf("SetSize should store width=100, got %d", p.width)
	}

	if p.height != 50 {
		t.Errorf("SetSize should store height=50, got %d", p.height)
	}
}
